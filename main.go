package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"hostberry/internal/auth"
	"hostberry/internal/config"
	"hostberry/internal/constants"
	"hostberry/internal/database"
	"hostberry/internal/i18n"
	"hostberry/internal/models"
)

var templatesFS embed.FS

var staticFS embed.FS

func main() {
	if err := config.Load(); err != nil {
		i18n.LogTfatal("logs.config_load_error", err)
	}
	config.Normalize(i18n.LogTf)

	if err := i18n.Init("locales"); err != nil {
		i18n.LogTf("logs.i18n_init_warning", err)
	}

	if err := database.Init(); err != nil {
		i18n.LogTfatal("logs.db_init_error", err)
	}

	// Establecer idioma de logs desde configuración del sistema (después de inicializar BD)
	if configs, err := database.GetAllConfigs(); err == nil {
		if lang, ok := configs["log_language"]; ok && lang != "" {
			i18n.SetLogLanguage(lang)
		}
	}

	i18n.LogTln("logs.checking_admin")
	createDefaultAdmin()

	// Iniciar autoconexión WiFi en segundo plano
	go func() {
		// Esperar menos tiempo (5 segundos es suficiente)
		i18n.LogTf("logs.wifi_auto_wait")
		time.Sleep(5 * time.Second)

		// Intentar detectar interfaz (menos intentos, más rápido)
		var interfaceName string
		for attempt := 0; attempt < 3; attempt++ {
			interfaceName = detectWiFiInterface()
			if interfaceName != "" {
				// Verificar que la interfaz realmente existe
				cmd := exec.Command("sh", "-c", fmt.Sprintf("ip link show %s 2>/dev/null", interfaceName))
				if err := cmd.Run(); err == nil {
					i18n.LogTf("logs.wifi_interface_detected", interfaceName)
					break
				}
			}
			if attempt < 2 {
				i18n.LogTf("logs.wifi_interface_wait", attempt+1)
				time.Sleep(2 * time.Second)
			}
		}

		if interfaceName != "" {
			i18n.LogTf("logs.wifi_auto_start", interfaceName)
			autoConnectToLastNetwork(interfaceName)
		} else {
			i18n.LogT("logs.wifi_interface_not_found")
		}
	}()

	app := createApp()

	setupRoutes(app)

	addr := fmt.Sprintf("%s:%d", config.AppConfig.Server.Host, config.AppConfig.Server.Port)
	i18n.LogTf("logs.server_starting", addr)
	i18n.LogTf("logs.server_config",
		config.AppConfig.Server.Debug,
		config.AppConfig.Server.ReadTimeout,
		config.AppConfig.Server.WriteTimeout)

	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
		<-sigint
		i18n.LogTln("logs.server_stopping")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := app.ShutdownWithContext(ctx); err != nil {
			i18n.LogTf("logs.server_shutdown_error", err)
		}
		os.Exit(0)
	}()

	i18n.LogTf("logs.server_ready", addr)

	// Si hay TLS configurado y los ficheros existen, levantar en HTTPS directamente.
	if config.AppConfig.Server.TLSCertFile != "" && config.AppConfig.Server.TLSKeyFile != "" {
		if _, err := os.Stat(config.AppConfig.Server.TLSCertFile); err == nil {
			if _, err := os.Stat(config.AppConfig.Server.TLSKeyFile); err == nil {
				if err := app.ListenTLS(addr, config.AppConfig.Server.TLSCertFile, config.AppConfig.Server.TLSKeyFile); err != nil {
					i18n.LogTfatal("logs.server_start_error", err)
				}
				return
			}
		}
	}

	// Fallback: HTTP normal (útil detrás de proxy inverso tipo nginx/traefik)
	if err := app.Listen(addr); err != nil {
		i18n.LogTfatal("logs.server_start_error", err)
	}
}

func createApp() *fiber.App {
	engine := createTemplateEngine()
	if engine == nil {
		i18n.LogTfatal("logs.template_engine_error")
	}

	i18n.LogT("logs.template_engine_created")

	app := fiber.New(fiber.Config{
		Views:        engine,
		ReadTimeout:  time.Duration(config.AppConfig.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(config.AppConfig.Server.WriteTimeout) * time.Second,
		ErrorHandler: errorHandler,
	})

	if app.Config().Views == nil {
		i18n.LogTfatal("logs.template_views_error")
	}
	i18n.LogTln("logs.template_views_ok")

	setupStaticFiles(app)

	app.Use(logger.New(logger.Config{
		Format:     "${time} | ${status} | ${latency} | ${ip} | ${method} | ${path}\n",
		TimeFormat: "15:04:05",
		TimeZone:   "Local",
		Output:     os.Stdout,
		Next: func(c *fiber.Ctx) bool {
			path := c.Path()
			return strings.HasPrefix(path, "/static/") &&
				(strings.HasSuffix(path, ".css") ||
					strings.HasSuffix(path, ".js") ||
					strings.HasSuffix(path, ".png") ||
					strings.HasSuffix(path, ".jpg") ||
					strings.HasSuffix(path, ".jpeg") ||
					strings.HasSuffix(path, ".gif") ||
					strings.HasSuffix(path, ".ico") ||
					strings.HasSuffix(path, ".svg") ||
					strings.HasSuffix(path, ".woff") ||
					strings.HasSuffix(path, ".woff2") ||
					strings.HasSuffix(path, ".ttf") ||
					strings.HasSuffix(path, ".eot"))
		},
	}))
	app.Use(compress.New())
	corsOrigins := "*"
	if !config.AppConfig.Server.Debug {
		corsOrigins = "http://localhost:" + fmt.Sprintf("%d", config.AppConfig.Server.Port) + ",http://127.0.0.1:" + fmt.Sprintf("%d", config.AppConfig.Server.Port)
	}
	app.Use(cors.New(cors.Config{
		AllowOrigins:     corsOrigins,
		AllowCredentials: true,
		AllowMethods:     "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders:     "Content-Type,Authorization",
		MaxAge:           3600,
	}))

	// Middleware de seguridad: cabeceras y, opcionalmente, redirección HTTP→HTTPS.
	app.Use(securityHeadersMiddleware)
	app.Use(enforceHTTPSMiddleware)

	app.Use(loggingMiddleware)

	app.Use(i18n.LanguageMiddleware)

	app.Use(requestIDMiddleware)

	app.Use("/api/", rateLimitMiddleware)

	return app
}

func setupStaticFiles(app *fiber.App) {
	if _, err := os.Stat("./website/static"); err == nil {
		app.Static("/static", "./website/static", fiber.Static{
			Compress:  true,
			ByteRange: true,
		})
		i18n.LogTln("logs.static_files_loaded")
	} else {
		staticSubFS, err := fs.Sub(staticFS, "website/static")
		if err != nil {
			i18n.LogTf("logs.static_files_embed_error", err)
			i18n.LogT("logs.static_files_not_found")
		} else {
			app.Get("/static/*", func(c *fiber.Ctx) error {
				path := c.Params("*")
				file, err := staticSubFS.Open(path)
				if err != nil {
					return c.Status(404).SendString("Not found")
				}
				defer file.Close()

				stat, err := file.Stat()
				if err != nil {
					return c.Status(500).SendString("Error reading file")
				}

				c.Type(filepath.Ext(path))
				return c.SendStream(file, int(stat.Size()))
			})
			i18n.LogTln("logs.static_files_embedded")
		}
	}
}

func setupRoutes(app *fiber.App) {
	app.Get("/health", healthCheckHandler)
	app.Get("/health/ready", readinessCheckHandler)
	app.Get("/health/live", livenessCheckHandler)
	// Métricas: endpoint público pero sin información sensible (para Prometheus/monitorización).
	app.Get("/metrics", metricsHandler)

	web := app.Group("/")
	{
		web.Get("/login", loginHandler)
		web.Get("/first-login", firstLoginPageHandler)
		web.Get("/", indexHandler)

		protected := web.Group("/", requireAuth)
		protected.Get("/dashboard", dashboardHandler)
		protected.Get("/settings", settingsHandler)
		protected.Get("/network", networkPageHandler)
		protected.Get("/wifi", wifiPageHandler)
		protected.Get("/wifi-scan", wifiScanPageHandler)
		protected.Get("/vpn", vpnPageHandler)
		protected.Get("/wireguard", wireguardPageHandler)
		protected.Get("/adblock", adblockPageHandler)
		protected.Get("/tor", torPageHandler)
		protected.Get("/hostapd", hostapdPageHandler)
		protected.Get("/setup-wizard", setupWizardPageHandler)
		protected.Get("/setup-wizard/vpn", setupWizardVpnPageHandler)
		protected.Get("/setup-wizard/wireguard", setupWizardWireguardPageHandler)
		protected.Get("/setup-wizard/tor", setupWizardTorPageHandler)
		protected.Get("/profile", profilePageHandler)
		protected.Get("/system", systemPageHandler)
		protected.Get("/monitoring", monitoringPageHandler)
		protected.Get("/update", updatePageHandler)
	}

	api := app.Group("/api/v1")
	{
		auth := api.Group("/auth")
		{
			auth.Post("/login", loginAPIHandler)
			auth.Post("/logout", requireAuth, logoutAPIHandler)
			auth.Get("/me", requireAuth, meHandler)
			auth.Post("/change-password", requireAuth, changePasswordAPIHandler)
			auth.Post("/first-login/change", firstLoginChangeAPIHandler)
			auth.Post("/profile", requireAuth, updateProfileAPIHandler)
			auth.Post("/preferences", requireAuth, updatePreferencesAPIHandler)
		}

		system := api.Group("/system", requireAuth)
		{
			system.Get("/stats", systemStatsHandler)
			system.Get("/info", systemInfoHandler)
			system.Get("/https-info", systemHttpsInfoHandler)
			system.Get("/logs", systemLogsHandler)
			system.Get("/activity", systemActivityHandler)
			system.Get("/network", systemNetworkHandler)
			system.Get("/updates", systemUpdatesHandler)
			system.Get("/services", systemServicesHandler)
			system.Get("/metrics", metricsSummaryHandler)
			system.Post("/backup", requireAdmin, systemBackupHandler)
			system.Post("/config", requireAdmin, systemConfigHandler)
			system.Post("/updates/execute", requireAdmin, systemUpdatesExecuteHandler)
			system.Post("/updates/project", requireAdmin, systemUpdatesProjectHandler)
			system.Post("/notifications/test-email", requireAdmin, systemNotificationsTestEmailHandler)
			system.Post("/restart", requireAdmin, systemRestartHandler)
			system.Post("/shutdown", requireAdmin, systemShutdownHandler)
		}

		network := api.Group("/network", requireAuth)
		{
			network.Get("/status", networkStatusHandler)
			network.Get("/interfaces", networkInterfacesHandler)
			network.Get("/routing", networkRoutingHandler)
			network.Post("/firewall/toggle", requireAdmin, networkFirewallToggleHandler)
			network.Post("/speedtest", requireAdmin, networkSpeedtestHandler)
			network.Get("/config", networkConfigHandler)
			network.Post("/config", networkConfigHandler)
		}

		wifi := api.Group("/wifi", requireAuth)
		{
			wifi.Get("/status", wifiStatusHandler)
			wifi.Get("/scan", wifiScanHandler)
			wifi.Post("/scan", wifiScanHandler)
			wifi.Get("/interfaces", wifiInterfacesHandler)
			wifi.Post("/connect", wifiConnectHandler)
			wifi.Post("/disconnect", wifiLegacyDisconnectHandler)
			wifi.Get("/networks", wifiNetworksHandler)
			wifi.Get("/clients", wifiClientsHandler)
			wifi.Post("/toggle", requireAdmin, wifiToggleHandler)
			wifi.Post("/unblock", requireAdmin, wifiUnblockHandler)
			wifi.Post("/software-switch", requireAdmin, wifiSoftwareSwitchHandler)
			wifi.Post("/config", requireAdmin, wifiConfigHandler)
		}

		vpn := api.Group("/vpn", requireAuth)
		{
			vpn.Get("/status", vpnStatusHandler)
			vpn.Get("/config", vpnGetConfigHandler)
			vpn.Post("/connect", vpnConnectHandler)
			vpn.Get("/connections", vpnConnectionsHandler)
			vpn.Get("/servers", vpnServersHandler)
			vpn.Get("/clients", vpnClientsHandler)
			vpn.Post("/toggle", requireAdmin, vpnToggleHandler)
			vpn.Post("/config", requireAdmin, vpnConfigHandler)
			vpn.Post("/connections/:name/toggle", requireAdmin, vpnConnectionToggleHandler)
			vpn.Post("/certificates/generate", requireAdmin, vpnCertificatesGenerateHandler)
		}

		hostapd := api.Group("/hostapd", requireAuth)
		{
			hostapd.Get("/access-points", hostapdAccessPointsHandler)
			hostapd.Get("/clients", hostapdClientsHandler)
			hostapd.Get("/config", hostapdGetConfigHandler)
			hostapd.Get("/diagnostics", hostapdDiagnosticsHandler)
			hostapd.Post("/create-ap0", requireAdmin, hostapdCreateAp0Handler)
			hostapd.Post("/toggle", requireAdmin, hostapdToggleHandler)
			hostapd.Post("/restart", requireAdmin, hostapdRestartHandler)
			hostapd.Post("/config", requireAdmin, hostapdConfigHandler)
		}

		help := api.Group("/help", requireAuth)
		{
			help.Post("/contact", helpContactHandler)
		}

		api.Get("/translations/:lang", translationsHandler)

		wireguard := api.Group("/wireguard", requireAuth)
		{
			wireguard.Get("/status", wireguardStatusHandler)
			wireguard.Get("/interfaces", wireguardInterfacesHandler)
			wireguard.Get("/peers", wireguardPeersHandler)
			wireguard.Get("/config", wireguardGetConfigHandler)
			wireguard.Post("/config", requireAdmin, wireguardConfigHandler)
			wireguard.Post("/toggle", requireAdmin, wireguardToggleHandler)
			wireguard.Post("/restart", requireAdmin, wireguardRestartHandler)
		}

		adblock := api.Group("/adblock", requireAuth)
		{
			adblock.Get("/status", adblockStatusHandler)
			adblock.Get("/lists", adblockListsHandler)
			adblock.Get("/domains", adblockDomainsHandler)
			adblock.Post("/enable", requireAdmin, adblockEnableHandler)
			adblock.Post("/disable", requireAdmin, adblockDisableHandler)
			adblock.Post("/update", requireAdmin, adblockUpdateHandler)
			adblock.Post("/lists/:name/toggle", requireAdmin, adblockToggleListHandler)
			adblock.Post("/domains/:name/toggle", requireAdmin, adblockToggleDomainHandler)
			adblock.Post("/config", requireAdmin, adblockConfigHandler)

			// DNSCrypt (sub-sección de AdBlock)
			dnscrypt := adblock.Group("/dnscrypt")
			{
				dnscrypt.Get("/status", dnscryptStatusHandler)
				dnscrypt.Post("/install", requireAdmin, dnscryptInstallHandler)
				dnscrypt.Post("/configure", requireAdmin, dnscryptConfigureHandler)
				dnscrypt.Post("/enable", requireAdmin, dnscryptEnableHandler)
				dnscrypt.Post("/disable", requireAdmin, dnscryptDisableHandler)
			}

			// Blocky (proxy DNS y ad-blocker, configuración desde la web)
			adblock.Get("/blocky/status", blockyStatusHandler)
			adblock.Get("/blocky/config", blockyConfigHandler)
			adblock.Post("/blocky/install", requireAdmin, blockyInstallHandler)
			adblock.Post("/blocky/configure", requireAdmin, blockyConfigureHandler)
			adblock.Post("/blocky/enable", requireAdmin, blockyEnableHandler)
			adblock.Post("/blocky/disable", requireAdmin, blockyDisableHandler)
			adblock.Get("/blocky/api/*", blockyAPIProxyHandler)
			adblock.Post("/blocky/api/*", blockyAPIProxyHandler)
		}

		tor := api.Group("/tor", requireAuth)
		{
			tor.Get("/status", torStatusHandler)
			tor.Post("/install", requireAdmin, torInstallHandler)
			tor.Post("/configure", requireAdmin, torConfigureHandler)
			tor.Post("/enable", requireAdmin, torEnableHandler)
			tor.Post("/disable", requireAdmin, torDisableHandler)
			tor.Get("/circuit", torCircuitHandler)
			tor.Post("/iptables-enable", requireAdmin, torIptablesEnableHandler)
			tor.Post("/iptables-disable", requireAdmin, torIptablesDisableHandler)
		}
	}

	wifiLegacy := app.Group("/api/wifi", requireAuth)
	wifiLegacy.Get("/status", wifiLegacyStatusHandler)
	wifiLegacy.Get("/stored_networks", wifiLegacyStoredNetworksHandler)
	wifiLegacy.Get("/autoconnect", wifiLegacyAutoconnectHandler)
	wifiLegacy.Get("/scan", wifiLegacyScanHandler)
	wifiLegacy.Post("/disconnect", wifiLegacyDisconnectHandler)
}

func indexHandler(c *fiber.Ctx) error {
	token := c.Cookies("access_token")

	if token != "" {
		claims, err := ValidateToken(token)
		if err == nil {
			var user models.User
			if err := database.DB.First(&user, claims.UserID).Error; err == nil && user.IsActive {
				return c.Redirect("/dashboard")
			}
		}
	}

	return c.Redirect("/login")
}

func dashboardHandler(c *fiber.Ctx) error {
	return renderTemplate(c, "dashboard", fiber.Map{
		"Title": i18n.T(c, "dashboard.title", "Dashboard"),
	})
}

func loginHandler(c *fiber.Ctx) error {
	return renderTemplate(c, "login", fiber.Map{
		"Title":                        i18n.T(c, "auth.login", "Login"),
		"ShowDefaultCredentialsNotice": auth.IsDefaultAdminCredentialsInUse(),
	})
}

func settingsHandler(c *fiber.Ctx) error {
	configs, _ := database.GetAllConfigs()

	if _, exists := configs["max_login_attempts"]; !exists || configs["max_login_attempts"] == "" {
		configs["max_login_attempts"] = "3"
	}
	if _, exists := configs["session_timeout"]; !exists || configs["session_timeout"] == "" {
		configs["session_timeout"] = "60"
	}
	if _, exists := configs["cache_enabled"]; !exists || configs["cache_enabled"] == "" {
		configs["cache_enabled"] = "true"
	}
	if _, exists := configs["cache_size"]; !exists || configs["cache_size"] == "" {
		configs["cache_size"] = "75"
	}

	configsJSON, _ := json.Marshal(configs)

	return renderTemplate(c, "settings", fiber.Map{
		"Title":         i18n.T(c, "navigation.settings", "Settings"),
		"settings":      configs,
		"settings_json": string(configsJSON),
	})
}

func systemStatsHandler(c *fiber.Ctx) error {
	stats := getSystemStats()
	return c.JSON(stats)
}

func systemRestartHandler(c *fiber.Ctx) error {
	user, ok := GetUser(c)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "No autorizado"})
	}
	userID := user.ID

	result := systemRestart(user.Username)
	if success, ok := result["success"].(bool); ok && success {
		database.InsertLog("INFO", database.LogMsg("Sistema reiniciado correctamente", user.Username), "system", &userID)
		return c.JSON(result)
	}

	if errMsg, ok := result["error"].(string); ok {
		database.InsertLog("ERROR", database.LogMsgErr("reiniciar sistema", errMsg, user.Username), "system", &userID)
		return c.Status(500).JSON(fiber.Map{"error": errMsg})
	}

	return c.JSON(result)
}

func detectWiFiInterface() string {
	cmd := exec.Command("sh", "-c", "ip -o link show | awk -F': ' '{print $2}' | grep -E '^wlan|^wl' | head -1")
	out, err := cmd.Output()
	if err == nil {
		iface := strings.TrimSpace(string(out))
		if iface != "" {
			return iface
		}
	}

	return constants.DefaultWiFiInterface
}

func wifiInterfacesHandler(c *fiber.Ctx) error {
	var interfaces []fiber.Map

	cmd := exec.Command("sh", "-c", "ip -o link show | awk -F': ' '{print $2}' | grep -E '^wlan|^wl'")
	out, err := cmd.Output()
	if err == nil {
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		for _, ifaceName := range lines {
			ifaceName = strings.TrimSpace(ifaceName)
			if ifaceName != "" {
				stateCmd := exec.Command("sh", "-c", fmt.Sprintf("cat /sys/class/net/%s/operstate 2>/dev/null", ifaceName))
				stateOut, _ := stateCmd.Output()
				state := strings.TrimSpace(string(stateOut))
				if state == "" {
					state = "unknown"
				}

				interfaces = append(interfaces, fiber.Map{
					"name":  ifaceName,
					"type":  "wifi",
					"state": state,
				})
			}
		}
	}

	if len(interfaces) == 0 {
		interfaces = append(interfaces, fiber.Map{
			"name":  constants.DefaultWiFiInterface,
			"type":  "wifi",
			"state": "unknown",
		})
	}

	return c.JSON(fiber.Map{
		"success":    true,
		"interfaces": interfaces,
	})
}

func wifiScanHandler(c *fiber.Ctx) error {
	// Para el setup wizard puede que no exista sesión/token.
	// Escanear redes no requiere usuario; solo la interfaz a usar.

	interfaceName := c.Query("interface", "")
	if interfaceName == "" {
		var req struct {
			Interface string `json:"interface"`
		}
		if err := c.BodyParser(&req); err == nil {
			interfaceName = req.Interface
		}
	}

	if interfaceName == "" {
		interfaceName = detectWiFiInterface()
	}
	if interfaceName == "" {
		interfaceName = constants.DefaultWiFiInterface
	}

	result := scanWiFiNetworks(interfaceName)
	if networks, ok := result["networks"]; ok {
		return c.JSON(networks)
	}
	return c.JSON([]fiber.Map{})
}
