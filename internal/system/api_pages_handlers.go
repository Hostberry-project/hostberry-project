package system

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"hostberry/internal/i18n"
	webtemplates "hostberry/internal/templates"
)

func SystemPageHandler(c *fiber.Ctx) error {
	return webtemplates.RenderTemplate(c, "system", fiber.Map{
		"Title": i18n.T(c, "system.title", "System Manager"),
	})
}

func MonitoringPageHandler(c *fiber.Ctx) error {
	return webtemplates.RenderTemplate(c, "monitoring", fiber.Map{
		"Title": i18n.T(c, "monitoring.title", "Monitoring"),
	})
}

func UpdatePageHandler(c *fiber.Ctx) error {
	return webtemplates.RenderTemplate(c, "update", fiber.Map{
		"Title": i18n.T(c, "update.title", "Updates"),
	})
}

func SetupWizardPageHandler(c *fiber.Ctx) error {
	return webtemplates.RenderTemplate(c, "setup_wizard", fiber.Map{
		"Title":       i18n.T(c, "setup_wizard.title", "Configuración inicial"),
		"last_update": time.Now().Unix(),
	})
}

func SetupWizardVpnPageHandler(c *fiber.Ctx) error {
	return webtemplates.RenderTemplate(c, "setup_wizard_vpn", fiber.Map{
		"Title":       i18n.T(c, "setup_wizard.security_vpn", "VPN"),
		"last_update": time.Now().Unix(),
	})
}

func SetupWizardWireguardPageHandler(c *fiber.Ctx) error {
	return webtemplates.RenderTemplate(c, "setup_wizard_wireguard", fiber.Map{
		"Title":       i18n.T(c, "setup_wizard.security_wireguard", "WireGuard"),
		"last_update": time.Now().Unix(),
	})
}

func SetupWizardTorPageHandler(c *fiber.Ctx) error {
	return webtemplates.RenderTemplate(c, "setup_wizard_tor", fiber.Map{
		"Title":       i18n.T(c, "setup_wizard.security_tor", "Tor"),
		"last_update": time.Now().Unix(),
	})
}

