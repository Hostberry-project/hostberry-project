package auth

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"hostberry/internal/config"
	"hostberry/internal/security"
)

func setAccessTokenCookie(c *fiber.Ctx, token string) {
	cookieExpiry := time.Duration(config.AppConfig.Security.TokenExpiry) * time.Minute
	c.Cookie(&fiber.Cookie{
		Name:     "access_token",
		Value:    token,
		Path:     "/",
		HTTPOnly: true,
		SameSite: "Strict",
		MaxAge:   int(cookieExpiry.Seconds()),
		Secure:   security.IsHTTPSRequest(c),
	})
}

func clearAccessTokenCookie(c *fiber.Ctx) {
	c.Cookie(&fiber.Cookie{
		Name:     "access_token",
		Value:    "",
		Path:     "/",
		HTTPOnly: true,
		SameSite: "Strict",
		Secure:   security.IsHTTPSRequest(c),
		MaxAge:   -1,
	})
}

func finalizeCredentialChange() {
	security.RemoveInstallCredentialsFile()
}
