package security

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"hostberry/internal/config"
)

func TestIsTrustedProxyIP_loopback(t *testing.T) {
	if !IsTrustedProxyIP("127.0.0.1") {
		t.Fatal("expected 127.0.0.1 trusted")
	}
	if !IsTrustedProxyIP("::1") {
		t.Fatal("expected ::1 trusted")
	}
	if IsTrustedProxyIP("8.8.8.8") {
		t.Fatal("unexpected 8.8.8.8 trusted without config")
	}
}

func TestIsHTTPSRequest_directTLS(t *testing.T) {
	app := fiber.New()
	app.Get("/", func(c *fiber.Ctx) error {
		if !IsHTTPSRequest(c) {
			return c.SendString("http")
		}
		return c.SendString("https")
	})
	req := httptest.NewRequest("GET", "https://example.com/", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestIsHTTPSRequest_forwardedUntrusted(t *testing.T) {
	config.AppConfig = &config.Config{}
	app := fiber.New()
	app.Get("/", func(c *fiber.Ctx) error {
		if IsHTTPSRequest(c) {
			return c.SendString("https")
		}
		return c.SendString("http")
	})
	req := httptest.NewRequest("GET", "http://example.com/", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	resp, _ := app.Test(req)
	body := make([]byte, 8)
	_, _ = resp.Body.Read(body)
	if string(body) == "https" {
		t.Fatal("should not trust X-Forwarded-Proto from untrusted IP")
	}
}
