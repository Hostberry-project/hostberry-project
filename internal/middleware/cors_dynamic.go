package middleware

import (
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"hostberry/internal/config"
)

// CorsOriginMatchesRequest indica si el navegador puede usar credenciales desde ese Origin
// frente al Host de la petición y la lista extra configurada.
func CorsOriginMatchesRequest(hostHeader string, serverPort int, extraOrigins []string, origin string) bool {
	origin = strings.TrimSpace(origin)
	if origin == "" {
		return false
	}
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}
	oHost := strings.ToLower(u.Hostname())
	oPort := u.Port()
	if oPort == "" {
		if strings.EqualFold(u.Scheme, "https") {
			oPort = "443"
		} else {
			oPort = "80"
		}
	}

	defPort := strconv.Itoa(serverPort)
	hostHeader = strings.TrimSpace(hostHeader)
	reqHost, reqPort, err := net.SplitHostPort(hostHeader)
	if err != nil {
		reqHost = strings.ToLower(hostHeader)
		reqPort = defPort
	} else {
		reqHost = strings.ToLower(reqHost)
		if reqPort == "" {
			reqPort = defPort
		}
	}

	if oHost == reqHost && normalizePort(oPort, defPort) == normalizePort(reqPort, defPort) {
		return true
	}

	if (oHost == "localhost" || oHost == "127.0.0.1") && normalizePort(oPort, defPort) == defPort {
		return true
	}

	for _, ex := range extraOrigins {
		if strings.TrimSpace(ex) == origin {
			return true
		}
	}
	return false
}

func normalizePort(p, def string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return def
	}
	return p
}

// DynamicCORSWithCredentials aplica CORS sin usar "*" con credenciales (evita combinación inválida/peligrosa).
// Refleja Origin solo si coincide con el Host de la petición, localhost/127.0.0.1 al puerto del servidor,
// o security.cors_allow_origins en config.yaml.
func DynamicCORSWithCredentials() fiber.Handler {
	const allowMethods = "GET,POST,PUT,DELETE,PATCH,OPTIONS,HEAD"
	allowHeaders := "Content-Type,Authorization,X-HostBerry-WiFi-Setup-Token"

	return func(c *fiber.Ctx) error {
		if config.AppConfig == nil {
			return c.Next()
		}

		origin := strings.TrimSpace(c.Get(fiber.HeaderOrigin))
		port := config.AppConfig.Server.Port
		if port <= 0 {
			port = 8000
		}

		var allowOrigin string
		if origin != "" && CorsOriginMatchesRequest(c.Host(), port, config.AppConfig.Security.CORSAllowOrigins, origin) {
			allowOrigin = origin
		}

		if c.Method() != fiber.MethodOptions {
			c.Vary(fiber.HeaderOrigin)
			if allowOrigin != "" {
				c.Set(fiber.HeaderAccessControlAllowOrigin, allowOrigin)
				c.Set(fiber.HeaderAccessControlAllowCredentials, "true")
			}
			return c.Next()
		}

		// Preflight
		c.Vary(fiber.HeaderOrigin)
		c.Vary(fiber.HeaderAccessControlRequestMethod)
		c.Vary(fiber.HeaderAccessControlRequestHeaders)
		if allowOrigin != "" {
			c.Set(fiber.HeaderAccessControlAllowOrigin, allowOrigin)
			c.Set(fiber.HeaderAccessControlAllowCredentials, "true")
		}
		c.Set(fiber.HeaderAccessControlAllowMethods, allowMethods)
		if h := c.Get(fiber.HeaderAccessControlRequestHeaders); h != "" {
			c.Set(fiber.HeaderAccessControlAllowHeaders, h)
		} else {
			c.Set(fiber.HeaderAccessControlAllowHeaders, allowHeaders)
		}
		c.Set(fiber.HeaderAccessControlMaxAge, "3600")
		return c.SendStatus(fiber.StatusNoContent)
	}
}
