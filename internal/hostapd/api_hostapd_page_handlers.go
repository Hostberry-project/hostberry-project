package hostapd

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"hostberry/internal/i18n"
	webtemplates "hostberry/internal/templates"
)

func HostapdPageHandler(c *fiber.Ctx) error {
	return webtemplates.RenderTemplate(c, "hostapd", fiber.Map{
		"Title":          i18n.T(c, "hostapd.overview", "Hotspot Overview"),
		"hostapd_stats":  fiber.Map{},
		"hostapd_status": fiber.Map{},
		"hostapd_config": fiber.Map{},
		"last_update":    time.Now().Unix(),
	})
}

