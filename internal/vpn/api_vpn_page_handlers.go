package vpn

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"hostberry/internal/i18n"
	webtemplates "hostberry/internal/templates"
)

func VpnPageHandler(c *fiber.Ctx) error {
	return webtemplates.RenderTemplate(c, "vpn", fiber.Map{
		"Title":        i18n.T(c, "vpn.overview", "VPN Overview"),
		"vpn_stats":    fiber.Map{},
		"vpn_status":   fiber.Map{},
		"vpn_config":   fiber.Map{},
		"vpn_security": fiber.Map{},
		"last_update":  time.Now().Unix(),
	})
}

