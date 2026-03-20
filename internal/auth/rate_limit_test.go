package auth

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

func TestLoginRateLimitMiddlewareLimitsByUsername(t *testing.T) {
	loginIPRateLimiter = newAuthRateLimiter(10, time.Minute)
	loginUsernameRateLimiter = newAuthRateLimiter(2, time.Minute)

	app := fiber.New()
	app.Post("/login", LoginRateLimitMiddleware, func(c *fiber.Ctx) error {
		return c.SendStatus(200)
	})

	body := []byte(`{"username":"admin","password":"secret"}`)
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test failed: %v", err)
		}
		if resp.StatusCode != 200 {
			t.Fatalf("unexpected status on allowed request: %d", resp.StatusCode)
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	if resp.StatusCode != 429 {
		t.Fatalf("expected 429, got %d", resp.StatusCode)
	}
	if resp.Header.Get("Retry-After") == "" {
		t.Fatal("expected Retry-After header")
	}
}

func TestFirstLoginRateLimitMiddlewareLimitsByIP(t *testing.T) {
	firstLoginIPRateLimiter = newAuthRateLimiter(2, time.Minute)

	app := fiber.New()
	app.Post("/first-login/change", FirstLoginRateLimitMiddleware, func(c *fiber.Ctx) error {
		return c.SendStatus(200)
	})

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/first-login/change", bytes.NewReader([]byte(`{}`)))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test failed: %v", err)
		}
		if resp.StatusCode != 200 {
			t.Fatalf("unexpected status on allowed request: %d", resp.StatusCode)
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/first-login/change", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	if resp.StatusCode != 429 {
		t.Fatalf("expected 429, got %d", resp.StatusCode)
	}
}
