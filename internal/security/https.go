package security

import (
	"net"
	"strings"

	"github.com/gofiber/fiber/v2"
	"hostberry/internal/config"
)

var defaultTrustedProxies = []string{
	"127.0.0.1",
	"::1",
	"localhost",
}

// IsHTTPSRequest indica si la petición debe tratarse como HTTPS.
// Solo confía en X-Forwarded-Proto si el cliente directo es un proxy de confianza.
func IsHTTPSRequest(c *fiber.Ctx) bool {
	if c.Secure() {
		return true
	}
	if !strings.EqualFold(strings.TrimSpace(c.Get("X-Forwarded-Proto")), "https") {
		return false
	}
	return IsTrustedProxyIP(c.IP())
}

// IsTrustedProxyIP comprueba si la IP pertenece a la lista de proxies de confianza.
func IsTrustedProxyIP(ip string) bool {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return false
	}
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	if parsed.IsLoopback() {
		return true
	}
	if config.AppConfig != nil {
		for _, trusted := range config.AppConfig.Security.TrustedProxyIPs {
			trusted = strings.TrimSpace(trusted)
			if trusted == "" {
				continue
			}
			if strings.EqualFold(trusted, ip) {
				return true
			}
			if _, cidr, err := net.ParseCIDR(trusted); err == nil && cidr.Contains(parsed) {
				return true
			}
			if t := net.ParseIP(trusted); t != nil && t.Equal(parsed) {
				return true
			}
		}
	}
	for _, trusted := range defaultTrustedProxies {
		if strings.EqualFold(trusted, ip) {
			return true
		}
	}
	return false
}
