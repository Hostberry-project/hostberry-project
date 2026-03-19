package wifi

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"hostberry/internal/i18n"
	webtemplates "hostberry/internal/templates"
)

func WifiPageHandler(c *fiber.Ctx) error {
	return webtemplates.RenderTemplate(c, "wifi", fiber.Map{
		"Title":         i18n.T(c, "wifi.overview", "WiFi Overview"),
		"wifi_stats":    fiber.Map{},
		"wifi_status":   fiber.Map{},
		"wifi_config":   fiber.Map{},
		"guest_network": fiber.Map{},
		"last_update":   time.Now().Unix(),
	})
}

func WifiScanPageHandler(c *fiber.Ctx) error {
	return webtemplates.RenderTemplate(c, "wifi_scan", fiber.Map{
		"Title": i18n.T(c, "wifi.scan", "WiFi Scan"),
	})
}

