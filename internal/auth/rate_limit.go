package auth

import (
	"encoding/json"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"hostberry/internal/i18n"
)

type authRateLimiter struct {
	requests map[string][]time.Time
	mu       sync.Mutex
	maxReqs  int
	window   time.Duration
}

func newAuthRateLimiter(maxReqs int, window time.Duration) *authRateLimiter {
	rl := &authRateLimiter{
		requests: make(map[string][]time.Time),
		maxReqs:  maxReqs,
		window:   window,
	}
	go rl.cleanup()
	return rl
}

func (rl *authRateLimiter) AllowWithRetry(key string) (bool, time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)
	validReqs := make([]time.Time, 0, len(rl.requests[key]))
	for _, reqTime := range rl.requests[key] {
		if reqTime.After(cutoff) {
			validReqs = append(validReqs, reqTime)
		}
	}
	rl.requests[key] = validReqs
	if len(rl.requests[key]) >= rl.maxReqs {
		oldest := rl.requests[key][0]
		retryAfter := oldest.Add(rl.window).Sub(now)
		if retryAfter < 0 {
			retryAfter = 0
		}
		return false, retryAfter
	}
	rl.requests[key] = append(rl.requests[key], now)
	return true, 0
}

func (rl *authRateLimiter) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		cutoff := now.Add(-rl.window)
		for key, reqs := range rl.requests {
			validReqs := reqs[:0]
			for _, reqTime := range reqs {
				if reqTime.After(cutoff) {
					validReqs = append(validReqs, reqTime)
				}
			}
			if len(validReqs) == 0 {
				delete(rl.requests, key)
				continue
			}
			rl.requests[key] = append([]time.Time(nil), validReqs...)
		}
		rl.mu.Unlock()
	}
}

var (
	loginIPRateLimiter       = newAuthRateLimiter(20, 10*time.Minute)
	loginUsernameRateLimiter = newAuthRateLimiter(5, 10*time.Minute)
	firstLoginIPRateLimiter  = newAuthRateLimiter(10, 10*time.Minute)
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
