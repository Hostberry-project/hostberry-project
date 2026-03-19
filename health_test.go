package main

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"hostberry/internal/config"
	"hostberry/internal/health"
)

// helper para crear una app mínima con las rutas de health/metrics.
func newTestApp() *fiber.App {
	config.AppConfig = &config.Config{
		Security: config.SecurityConfig{
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
	app.Get("/health", health.HealthCheckHandler)
	app.Get("/health/ready", health.ReadinessCheckHandler)
	app.Get("/health/live", health.LivenessCheckHandler)
	app.Get("/metrics", health.MetricsHandler)
	app.Get("/api/v1/system/https-info", systemHttpsInfoHandler)

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

	// /metrics debe devolver hostberry_up en el cuerpo
	req := httptest.NewRequest("GET", "/metrics", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("error haciendo request a /metrics: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status inesperado para /metrics: %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "hostberry_up") {
		t.Fatalf("respuesta de /metrics no contiene hostberry_up:\n%s", string(body))
	}

	// /api/v1/system/https-info debe devolver JSON con is_https y host/port
	req2 := httptest.NewRequest("GET", "/api/v1/system/https-info", nil)
	resp2, err := app.Test(req2)
	if err != nil {
		t.Fatalf("error haciendo request a /api/v1/system/https-info: %v", err)
	}
	if resp2.StatusCode != 200 {
		t.Fatalf("status inesperado para /api/v1/system/https-info: %d", resp2.StatusCode)
	}
}

