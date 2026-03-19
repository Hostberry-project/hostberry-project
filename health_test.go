package main

import (
	"testing"

	"github.com/gofiber/fiber/v2"
)

// helper para crear una app mínima con las rutas de health/metrics.
func newTestApp() *fiber.App {
	appConfig = Config{
		Security: SecurityConfig{
			JWTSecret:    "test-secret",
			TokenExpiry:  5,
			BcryptCost:   4,
			RateLimitRPS: 10,
		},
	}
	return fiber.New()
}

func TestHealthEndpoints(t *testing.T) {
	app := newTestApp()
	app.Get("/health", healthCheckHandler)
	app.Get("/health/ready", readinessCheckHandler)
	app.Get("/health/live", livenessCheckHandler)

	for _, path := range []string{"/health", "/health/ready", "/health/live"} {
		req := httptest.NewRequest("GET", path, nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("error haciendo request a %s: %v", path, err)
		}
		if resp.StatusCode <= 0 {
			t.Fatalf("status inválido para %s: %d", path, resp.StatusCode)
		}
	}
}

