package auth

import (
	"encoding/json"

	"github.com/gofiber/fiber/v2"
	"hostberry/internal/database"
	"hostberry/internal/i18n"
	"hostberry/internal/models"
	webtemplates "hostberry/internal/templates"
)

func IndexPageHandler(c *fiber.Ctx) error {
	token := c.Cookies("access_token")
	if token != "" {
		claims, err := ValidateToken(token)
		if err == nil {
			var user models.User
			if err := database.DB.First(&user, claims.UserID).Error; err == nil && user.IsActive {
				return c.Redirect("/dashboard")
			}
		}
	}

	return c.Redirect("/login")
}

func DashboardPageHandler(c *fiber.Ctx) error {
	return webtemplates.RenderTemplate(c, "dashboard", fiber.Map{
		"Title": i18n.T(c, "dashboard.title", "Dashboard"),
	})
}

func LoginPageHandler(c *fiber.Ctx) error {
	return webtemplates.RenderTemplate(c, "login", fiber.Map{
		"Title":                        i18n.T(c, "auth.login", "Login"),
		"ShowDefaultCredentialsNotice": IsDefaultAdminCredentialsInUse(),
	})
}

func SettingsPageHandler(c *fiber.Ctx) error {
	configs, _ := database.GetAllConfigs()

	if _, exists := configs["max_login_attempts"]; !exists || configs["max_login_attempts"] == "" {
		configs["max_login_attempts"] = "3"
	}
	if _, exists := configs["session_timeout"]; !exists || configs["session_timeout"] == "" {
		configs["session_timeout"] = "60"
	}
	if _, exists := configs["cache_enabled"]; !exists || configs["cache_enabled"] == "" {
		configs["cache_enabled"] = "true"
	}
	if _, exists := configs["cache_size"]; !exists || configs["cache_size"] == "" {
		configs["cache_size"] = "75"
	}

	configsJSON, _ := json.Marshal(configs)

	return webtemplates.RenderTemplate(c, "settings", fiber.Map{
		"Title":         i18n.T(c, "navigation.settings", "Settings"),
		"settings":      configs,
		"settings_json": string(configsJSON),
	})
}

