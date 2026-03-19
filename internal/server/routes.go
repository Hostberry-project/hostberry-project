package server

import "github.com/gofiber/fiber/v2"

// SetupRoutes registra todas las rutas HTTP de la aplicación.
func SetupRoutes(app *fiber.App) {
	setupHealthRoutes(app)
	setupWebRoutes(app)
	setupApiRoutes(app)
}

