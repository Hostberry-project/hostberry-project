package main

import (
	"embed"
	"hostberry/internal/config"
	"hostberry/internal/database"
	"hostberry/internal/i18n"
	server "hostberry/internal/server"
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
			interfaceName = wifiHandlers.DetectWiFiInterface()
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

	server.SetupRoutes(app)

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

	server.SetupStaticFiles(app, staticFS)

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