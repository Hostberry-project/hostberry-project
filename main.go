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
	"hostberry/internal/health"
	"hostberry/internal/i18n"
	middleware "hostberry/internal/middleware"
	"hostberry/internal/models"
	webtemplates "hostberry/internal/templates"
	wifiHandlers "hostberry/internal/wifi"
	sys "hostberry/internal/system"
	hostapdHandlers "hostberry/internal/hostapd"
	netHandlers "hostberry/internal/network"
	vpnHandlers "hostberry/internal/vpn"
	torHandlers "hostberry/internal/tor"
	adblockHandlers "hostberry/internal/adblock"
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
			wifiHandlers.AutoConnectToLastNetwork(interfaceName)
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
	engine := webtemplates.CreateTemplateEngine(templatesFS)
	if engine == nil {
		i18n.LogTfatal("logs.template_engine_error")
	}

	i18n.LogT("logs.template_engine_created")

	app := fiber.New(fiber.Config{
		Views:        engine,
		ReadTimeout:  time.Duration(config.AppConfig.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(config.AppConfig.Server.WriteTimeout) * time.Second,
		ErrorHandler: middleware.ErrorHandler,
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
	app.Use(middleware.SecurityHeadersMiddleware)
	app.Use(middleware.EnforceHTTPSMiddleware)

	app.Use(middleware.LoggingMiddleware)

	app.Use(i18n.LanguageMiddleware)

	app.Use(middleware.RequestIDMiddleware)

	app.Use("/api/", middleware.RateLimitMiddleware)

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
	app.Get("/health", health.HealthCheckHandler)
	app.Get("/health/ready", health.ReadinessCheckHandler)
	app.Get("/health/live", health.LivenessCheckHandler)
	// Métricas: endpoint público pero sin información sensible (para Prometheus/monitorización).
	app.Get("/metrics", health.MetricsHandler)

	web := app.Group("/")
	{
		web.Get("/login", loginHandler)
		web.Get("/first-login", firstLoginPageHandler)
		web.Get("/", indexHandler)

		protected := web.Group("/", middleware.RequireAuth)
		protected.Get("/dashboard", dashboardHandler)
		protected.Get("/settings", settingsHandler)
		protected.Get("/network", netHandlers.NetworkPageHandler)
		protected.Get("/wifi", wifiHandlers.WifiPageHandler)
		protected.Get("/wifi-scan", wifiHandlers.WifiScanPageHandler)
		protected.Get("/vpn", vpnHandlers.VpnPageHandler)
		protected.Get("/wireguard", vpnHandlers.WireguardPageHandler)
		protected.Get("/adblock", adblockHandlers.AdblockPageHandler)
		protected.Get("/tor", torHandlers.TorPageHandler)
		protected.Get("/hostapd", hostapdHandlers.HostapdPageHandler)
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
			auth.Post("/logout", middleware.RequireAuth, logoutAPIHandler)
			auth.Get("/me", middleware.RequireAuth, meHandler)
			auth.Post("/change-password", middleware.RequireAuth, changePasswordAPIHandler)
			auth.Post("/first-login/change", firstLoginChangeAPIHandler)
			auth.Post("/profile", middleware.RequireAuth, updateProfileAPIHandler)
			auth.Post("/preferences", middleware.RequireAuth, updatePreferencesAPIHandler)
		}

		system := api.Group("/system", middleware.RequireAuth)
		{
			system.Get("/stats", systemStatsHandler)
			system.Get("/info", systemInfoHandler)
			system.Get("/https-info", sys.SystemHttpsInfoHandler)
			system.Get("/logs", systemLogsHandler)
			system.Get("/activity", sys.SystemActivityHandler)
			system.Get("/network", sys.SystemNetworkHandler)
			system.Get("/updates", sys.SystemUpdatesHandler)
			system.Get("/services", systemServicesHandler)
			system.Get("/metrics", health.MetricsSummaryHandler)
			system.Post("/backup", middleware.RequireAdmin, sys.SystemBackupHandler)
			system.Post("/config", middleware.RequireAdmin, sys.SystemConfigHandler)
			system.Post("/updates/execute", middleware.RequireAdmin, sys.SystemUpdatesExecuteHandler)
			system.Post("/updates/project", middleware.RequireAdmin, sys.SystemUpdatesProjectHandler)
			system.Post("/notifications/test-email", middleware.RequireAdmin, sys.SystemNotificationsTestEmailHandler)
			system.Post("/restart", middleware.RequireAdmin, systemRestartHandler)
			system.Post("/shutdown", middleware.RequireAdmin, systemShutdownHandler)
		}

		network := api.Group("/network", middleware.RequireAuth)
		{
			network.Get("/status", netHandlers.NetworkStatusHandler)
			network.Get("/interfaces", networkInterfacesHandler)
			network.Get("/routing", netHandlers.NetworkRoutingHandler)
			network.Post("/firewall/toggle", middleware.RequireAdmin, netHandlers.NetworkFirewallToggleHandler)
			network.Post("/speedtest", middleware.RequireAdmin, netHandlers.NetworkSpeedtestHandler)
			network.Get("/config", netHandlers.NetworkConfigHandler)
			network.Post("/config", netHandlers.NetworkConfigHandler)
		}

		wifi := api.Group("/wifi", middleware.RequireAuth)
		{
			wifi.Get("/status", wifiHandlers.WifiStatusHandler)
			wifi.Get("/scan", wifiScanHandler)
			wifi.Post("/scan", wifiScanHandler)
			wifi.Get("/interfaces", wifiInterfacesHandler)
			wifi.Post("/connect", wifiHandlers.WifiConnectHandler)
			wifi.Post("/disconnect", wifiHandlers.WifiLegacyDisconnectHandler)
			wifi.Get("/networks", wifiHandlers.WifiNetworksHandler)
			wifi.Get("/clients", wifiHandlers.WifiClientsHandler)
			wifi.Post("/toggle", middleware.RequireAdmin, wifiHandlers.WifiToggleHandler)
			wifi.Post("/unblock", middleware.RequireAdmin, wifiHandlers.WifiUnblockHandler)
			wifi.Post("/software-switch", middleware.RequireAdmin, wifiHandlers.WifiSoftwareSwitchHandler)
			wifi.Post("/config", middleware.RequireAdmin, wifiHandlers.WifiConfigHandler)
		}

		vpn := api.Group("/vpn", middleware.RequireAuth)
		{
			vpn.Get("/status", vpnHandlers.VpnStatusHandler)
			vpn.Get("/config", vpnHandlers.VpnGetConfigHandler)
			vpn.Post("/connect", vpnHandlers.VpnConnectHandler)
			vpn.Get("/connections", vpnHandlers.VpnConnectionsHandler)
			vpn.Get("/servers", vpnHandlers.VpnServersHandler)
			vpn.Get("/clients", vpnHandlers.VpnClientsHandler)
			vpn.Post("/toggle", middleware.RequireAdmin, vpnHandlers.VpnToggleHandler)
			vpn.Post("/config", middleware.RequireAdmin, vpnHandlers.VpnConfigHandler)
			vpn.Post("/connections/:name/toggle", middleware.RequireAdmin, vpnHandlers.VpnConnectionToggleHandler)
			vpn.Post("/certificates/generate", middleware.RequireAdmin, vpnHandlers.VpnCertificatesGenerateHandler)
		}

		hostapd := api.Group("/hostapd", middleware.RequireAuth)
		{
			hostapd.Get("/access-points", hostapdHandlers.HostapdAccessPointsHandler)
			hostapd.Get("/clients", hostapdHandlers.HostapdClientsHandler)
			hostapd.Get("/config", hostapdHandlers.HostapdGetConfigHandler)
			hostapd.Get("/diagnostics", hostapdHandlers.HostapdDiagnosticsHandler)
			hostapd.Post("/create-ap0", middleware.RequireAdmin, hostapdHandlers.HostapdCreateAp0Handler)
			hostapd.Post("/toggle", middleware.RequireAdmin, hostapdHandlers.HostapdToggleHandler)
			hostapd.Post("/restart", middleware.RequireAdmin, hostapdHandlers.HostapdRestartHandler)
			hostapd.Post("/config", middleware.RequireAdmin, hostapdHandlers.HostapdConfigHandler)
		}

		help := api.Group("/help", middleware.RequireAuth)
		{
			help.Post("/contact", sys.HelpContactHandler)
		}

		api.Get("/translations/:lang", i18n.TranslationsHandler)

		wireguard := api.Group("/wireguard", middleware.RequireAuth)
		{
			wireguard.Get("/status", vpnHandlers.WireguardStatusHandler)
			wireguard.Get("/interfaces", vpnHandlers.WireguardInterfacesHandler)
			wireguard.Get("/peers", vpnHandlers.WireguardPeersHandler)
			wireguard.Get("/config", vpnHandlers.WireguardGetConfigHandler)
			wireguard.Post("/config", middleware.RequireAdmin, vpnHandlers.WireguardConfigHandler)
			wireguard.Post("/toggle", middleware.RequireAdmin, vpnHandlers.WireguardToggleHandler)
			wireguard.Post("/restart", middleware.RequireAdmin, vpnHandlers.WireguardRestartHandler)
		}

		adblock := api.Group("/adblock", middleware.RequireAuth)
		{
			adblock.Get("/status", adblockHandlers.AdblockStatusHandler)
			adblock.Get("/lists", sys.AdblockListsHandler)
			adblock.Get("/domains", sys.AdblockDomainsHandler)
			adblock.Post("/enable", middleware.RequireAdmin, adblockHandlers.AdblockEnableHandler)
			adblock.Post("/disable", middleware.RequireAdmin, adblockHandlers.AdblockDisableHandler)
			adblock.Post("/update", middleware.RequireAdmin, sys.AdblockUpdateHandler)
			adblock.Post("/lists/:name/toggle", middleware.RequireAdmin, sys.AdblockToggleListHandler)
			adblock.Post("/domains/:name/toggle", middleware.RequireAdmin, sys.AdblockToggleDomainHandler)
			adblock.Post("/config", middleware.RequireAdmin, sys.AdblockConfigHandler)

			// DNSCrypt (sub-sección de AdBlock)
			dnscrypt := adblock.Group("/dnscrypt")
			{
				dnscrypt.Get("/status", adblockHandlers.DnscryptStatusHandler)
				dnscrypt.Post("/install", middleware.RequireAdmin, adblockHandlers.DnscryptInstallHandler)
				dnscrypt.Post("/configure", middleware.RequireAdmin, adblockHandlers.DnscryptConfigureHandler)
				dnscrypt.Post("/enable", middleware.RequireAdmin, adblockHandlers.DnscryptEnableHandler)
				dnscrypt.Post("/disable", middleware.RequireAdmin, adblockHandlers.DnscryptDisableHandler)
			}

			// Blocky (proxy DNS y ad-blocker, configuración desde la web)
			adblock.Get("/blocky/status", adblockHandlers.BlockyStatusHandler)
			adblock.Get("/blocky/config", adblockHandlers.BlockyConfigHandler)
			adblock.Post("/blocky/install", middleware.RequireAdmin, adblockHandlers.BlockyInstallHandler)
			adblock.Post("/blocky/configure", middleware.RequireAdmin, adblockHandlers.BlockyConfigureHandler)
			adblock.Post("/blocky/enable", middleware.RequireAdmin, adblockHandlers.BlockyEnableHandler)
			adblock.Post("/blocky/disable", middleware.RequireAdmin, adblockHandlers.BlockyDisableHandler)
			adblock.Get("/blocky/api/*", adblockHandlers.BlockyAPIProxyHandler)
			adblock.Post("/blocky/api/*", adblockHandlers.BlockyAPIProxyHandler)
		}

		tor := api.Group("/tor", middleware.RequireAuth)
		{
			tor.Get("/status", torHandlers.TorStatusHandler)
			tor.Post("/install", middleware.RequireAdmin, torHandlers.TorInstallHandler)
			tor.Post("/configure", middleware.RequireAdmin, torHandlers.TorConfigureHandler)
			tor.Post("/enable", middleware.RequireAdmin, torHandlers.TorEnableHandler)
			tor.Post("/disable", middleware.RequireAdmin, torHandlers.TorDisableHandler)
			tor.Get("/circuit", torHandlers.TorCircuitHandler)
			tor.Post("/iptables-enable", middleware.RequireAdmin, torHandlers.TorIptablesEnableHandler)
			tor.Post("/iptables-disable", middleware.RequireAdmin, torHandlers.TorIptablesDisableHandler)
		}
	}

	wifiLegacy := app.Group("/api/wifi", middleware.RequireAuth)
	wifiLegacy.Get("/status", wifiHandlers.WifiLegacyStatusHandler)
	wifiLegacy.Get("/stored_networks", wifiHandlers.WifiLegacyStoredNetworksHandler)
	wifiLegacy.Get("/autoconnect", wifiHandlers.WifiLegacyAutoconnectHandler)
	wifiLegacy.Get("/scan", wifiHandlers.WifiLegacyScanHandler)
	wifiLegacy.Post("/disconnect", wifiHandlers.WifiLegacyDisconnectHandler)
}

func indexHandler(c *fiber.Ctx) error {
	token := c.Cookies("access_token")

	if token != "" {
		claims, err := auth.ValidateToken(token)
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
	return webtemplates.RenderTemplate(c, "dashboard", fiber.Map{
		"Title": i18n.T(c, "dashboard.title", "Dashboard"),
	})
}

func loginHandler(c *fiber.Ctx) error {
	return webtemplates.RenderTemplate(c, "login", fiber.Map{
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

	return webtemplates.RenderTemplate(c, "settings", fiber.Map{
		"Title":         i18n.T(c, "navigation.settings", "Settings"),
		"settings":      configs,
		"settings_json": string(configsJSON),
	})
}

func systemStatsHandler(c *fiber.Ctx) error {
	stats := sys.GetSystemStats()
	return c.JSON(stats)
}

func systemRestartHandler(c *fiber.Ctx) error {
	user, ok := middleware.GetUser(c)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "No autorizado"})
	}
	userID := user.ID

	result := sys.SystemRestart(user.Username)
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

	result := wifiHandlers.ScanWiFiNetworks(interfaceName)
	if networks, ok := result["networks"]; ok {
		return c.JSON(networks)
	}
	return c.JSON([]fiber.Map{})
}
