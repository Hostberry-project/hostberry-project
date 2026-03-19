package main

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/gofiber/fiber/v2"
)

func requestIDMiddleware(c *fiber.Ctx) error {
	requestID := c.Get("X-Request-ID")
	
	if requestID == "" {
		bytes := make([]byte, 16)
		if _, err := rand.Read(bytes); err == nil {
			requestID = hex.EncodeToString(bytes)
		} else {
			requestID = generateSimpleID()
		}
	}

	c.Locals("request_id", requestID)
	
	c.Set("X-Request-ID", requestID)

	return c.Next()
}

func generateSimpleID() string {
	// Fallback cuando rand.Read falla: tiempo + pid para reducir colisiones
	b := make([]byte, 8)
	for i := range b {
		b[i] = byte(time.Now().UnixNano() >> (i * 8))
	}
	return hex.EncodeToString(b)
}
