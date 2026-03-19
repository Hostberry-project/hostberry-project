package system

import (
	"encoding/json"
	"time"

	"github.com/gofiber/fiber/v2"
	"hostberry/internal/i18n"
	"hostberry/internal/database"
	middleware "hostberry/internal/middleware"
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

func ProfilePageHandler(c *fiber.Ctx) error {
	user, ok := middleware.GetUser(c)
	if !ok {
		return c.Redirect("/login")
	}

	logs, _, _ := database.GetLogs("all", 10, 0)
	type activity struct {
		Action      string
		Timestamp   string
		Description string
		IPAddress   string
	}

	var activities []activity
	for _, l := range logs {
		activities = append(activities, activity{
			Action:      l.Source,
			Timestamp:   l.CreatedAt.Format(time.RFC3339),
			Description: l.Message,
			IPAddress:   "-",
		})
	}

	// settings para render de la página.
	configs, _ := database.GetAllConfigs()
	configsJSON, _ := json.Marshal(configs)

	return webtemplates.RenderTemplate(c, "profile", fiber.Map{
		"Title":              i18n.T(c, "auth.profile", "Profile"),
		"user":               user,
		"recent_activities": activities,
		"settings":          configs,
		"settings_json":     string(configsJSON),
		"last_update":       time.Now().Unix(),
	})
}

func FirstLoginPageHandler(c *fiber.Ctx) error {
	return webtemplates.RenderTemplate(c, "first_login", fiber.Map{
		"Title": i18n.T(c, "auth.first_login", "First Login"),
	})
}

