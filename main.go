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
	"hostberry/internal/config"
	"hostberry/internal/constants"
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

	if err := InitI18n("locales"); err != nil {
		LogTf("logs.i18n_init_warning", err)
	}

	if err := initDatabase(); err != nil {
		i18n.LogTfatal("logs.db_init_error", err)
	}

	// Establecer idioma de logs desde configuración del sistema (después de inicializar BD)
	if configs, err := GetAllConfigs(); err == nil {
		if lang, ok := configs["log_language"]; ok && lang != "" {
			i18n.SetLogLanguage(lang)
		}
	}

	LogTln("logs.checking_admin")
	createDefaultAdmin()

	// Iniciar autoconexión WiFi en segundo plano
	go func() {
		// Esperar menos tiempo (5 segundos es suficiente)
		LogTf("logs.wifi_auto_wait")
		time.Sleep(5 * time.Second)

		// Intentar detectar interfaz (menos intentos, más rápido)
		var interfaceName string
		for attempt := 0; attempt < 3; attempt++ {
			interfaceName = detectWiFiInterface()
			if interfaceName != "" {
				// Verificar que la interfaz realmente existe
				cmd := exec.Command("sh", "-c", fmt.Sprintf("ip link show %s 2>/dev/null", interfaceName))
				if err := cmd.Run(); err == nil {
					LogTf("logs.wifi_interface_detected", interfaceName)
					break
				}
			}
			if attempt < 2 {
				LogTf("logs.wifi_interface_wait", attempt+1)
				time.Sleep(2 * time.Second)
			}
		}

		if interfaceName != "" {
			LogTf("logs.wifi_auto_start", interfaceName)
			autoConnectToLastNetwork(interfaceName)
		} else {
			LogT("logs.wifi_interface_not_found")
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
		LogTln("logs.server_stopping")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := app.ShutdownWithContext(ctx); err != nil {
			LogTf("logs.server_shutdown_error", err)
		}
		os.Exit(0)
	}()

	LogTf("logs.server_ready", addr)

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
		LogTfatal("logs.template_engine_error")
	}

	LogT("logs.template_engine_created")

	app := fiber.New(fiber.Config{
		Views:        engine,
		ReadTimeout:  time.Duration(config.AppConfig.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(config.AppConfig.Server.WriteTimeout) * time.Second,
		ErrorHandler: errorHandler,
	})

	if app.Config().Views == nil {
		LogTfatal("logs.template_views_error")
	}
	LogTln("logs.template_views_ok")

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

	app.Use(LanguageMiddleware)

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
		LogTln("logs.static_files_loaded")
	} else {
		staticSubFS, err := fs.Sub(staticFS, "website/static")
		if err != nil {
			LogTf("logs.static_files_embed_error", err)
			LogT("logs.static_files_not_found")
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
			LogTln("logs.static_files_embedded")
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
			auth.Post("/first-login/change", firstLoginChange