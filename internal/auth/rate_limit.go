package auth

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"hostberry/internal/i18n"
	middleware "hostberry/internal/middleware"
)

var (
	loginIPRateLimiter        = middleware.NewRateLimiter(20, 10*time.Minute)
	loginUsernameRateLimiter  = middleware.NewRateLimiter(5, 10*time.Minute)
	firstLoginIPRateLimiter   = middleware.NewRateLimiter(10, 10*time.Minute)
)

func loginRateLimitKeyUsername(c *fiber.Ctx) string {
	var payload struct {
		Username string `json:"username"`
	}
	if err := json.Unmarshal(c.Body(), &payload); err != nil {
		return ""
	}
	username := strings.ToLower(strings.TrimSpace(payload.Username))
	if username == "" {
		return ""
	}
	return username
}

func writeRateLimitExceeded(c *fiber.Ctx, retryAfter time.Duration) error {
	seconds := int(retryAfter.Round(time.Second).Seconds())
	if seconds < 1 {
		seconds = 1
	}
	c.Set("Retry-After", strconv.Itoa(seconds))
	return c.Status(429).JSON(fiber.Map{
		"error": i18n.T(c, "auth.too_many_attempts", "Demasiados intentos. Intenta nuevamente más tarde"),
	})
}

func LoginRateLimitMiddleware(c *fiber.Ctx) error {
	if allowed, retryAfter := loginIPRateLimiter.AllowWithRetry("login:ip:" + c.IP()); !allowed {
		return writeRateLimitExceeded(c, retryAfter)
	}
	if username := loginRateLimitKeyUsername(c); username != "" {
		if allowed, retryAfter := loginUsernameRateLimiter.AllowWithRetry("login:user:" + username); !allowed {
			return writeRateLimitExceeded(c, retryAfter)
		}
	}
	return c.Next()
}

func FirstLoginRateLimitMiddleware(c *fiber.Ctx) error {
	if allowed, retryAfter := firstLoginIPRateLimiter.AllowWithRetry("first-login:ip:" + c.IP()); !allowed {
		return writeRateLimitExceeded(c, retryAfter)
	}
	return c.Next()
}
