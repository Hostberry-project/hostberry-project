package middleware

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"hostberry/internal/i18n"
)

var sensitiveActionLimiter = NewRateLimiter(5, time.Minute)

// SensitiveActionRateLimit limita acciones críticas por IP y usuario autenticado.
func SensitiveActionRateLimit(action string) fiber.Handler {
	action = strings.TrimSpace(action)
	if action == "" {
		action = "sensitive"
	}
	return func(c *fiber.Ctx) error {
		key := fmt.Sprintf("%s:ip:%s", action, c.IP())
		if user, ok := GetUser(c); ok && user != nil {
			key = fmt.Sprintf("%s:user:%d", action, user.ID)
		}
		allowed, retryAfter := sensitiveActionLimiter.AllowWithRetry(key)
		if allowed {
			return c.Next()
		}
		seconds := int(retryAfter.Round(time.Second).Seconds())
		if seconds < 1 {
			seconds = 1
		}
		c.Set("Retry-After", strconv.Itoa(seconds))
		return c.Status(429).JSON(fiber.Map{
			"error": i18n.T(c, "errors.rate_limit_sensitive", "Demasiadas acciones sensibles. Espera un momento e inténtalo de nuevo."),
		})
	}
}
