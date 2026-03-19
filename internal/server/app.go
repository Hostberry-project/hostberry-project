package server

import (
	"embed"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"hostberry/internal/config"
	"hostberry/internal/i18n"
	middleware "hostberry/internal/middleware"
	webtemplates "hostberry/internal/templates"
)

// CreateApp construye el `fiber.App` con engine de templates y middleware base.
func CreateApp(templatesFS embed.FS, staticFS embed.FS) *fiber.App {
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

	SetupStaticFiles(app, staticFS)

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
		AllowHeaders:     "Content-Type,Authorization,X-HostBerry-WiFi-Setup-Token",
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

