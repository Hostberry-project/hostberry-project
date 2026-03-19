package server

import (
	"github.com/gofiber/fiber/v2"
	health "hostberry/internal/health"
)

func setupHealthRoutes(app *fiber.App) {
	app.Get("/health", health.HealthCheckHandler)
	app.Get("/health/ready", health.ReadinessCheckHandler)
	app.Get("/health/live", health.LivenessCheckHandler)

	// Métricas: endpoint público pero sin información sensible (para Prometheus/monitorización).
	app.Get("/metrics", health.MetricsHandler)
}

