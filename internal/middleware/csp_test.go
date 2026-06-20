package middleware

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"hostberry/internal/config"
)

func TestSecurityHeadersCSPWithoutUnsafeInline(t *testing.T) {
	prev := config.AppConfig
	t.Cleanup(func() { config.AppConfig = prev })

	config.AppConfig = &config.Config{
		Security: config.SecurityConfig{EnforceHTTPS: false},
	}

	app := fiber.New()
	app.Use(SecurityHeadersMiddleware)
	app.Get("/", func(c *fiber.Ctx) error { return c.SendString("ok") })

	req := httptest.NewRequest("GET", "/", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	csp := resp.Header.Get("Content-Security-Policy")
	if csp == "" {
		t.Fatal("missing Content-Security-Policy header")
	}
	if strings.Contains(csp, "unsafe-inline") {
		t.Fatalf("CSP must not allow unsafe-inline: %q", csp)
	}
	if !strings.Contains(csp, "script-src 'self'") {
		t.Fatalf("expected script-src 'self' in CSP: %q", csp)
	}
}
