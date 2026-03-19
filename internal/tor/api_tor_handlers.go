package tor

import (
	"github.com/gofiber/fiber/v2"

	"hostberry/internal/database"
	i18n "hostberry/internal/i18n"
	middleware "hostberry/internal/middleware"
	"hostberry/internal/models"
	webtemplates "hostberry/internal/templates"
)

func TorStatusHandler(c *fiber.Ctx) error {
	result := GetTorStatus()
	return c.JSON(result)
}

func TorInstallHandler(c *fiber.Ctx) error {
	return middleware.RunActionWithUser(c, "tor", "Tor instalado correctamente", "instalar Tor", func(user *models.User) map[string]interface{} {
		return InstallTor(user.Username)
	})
}

func TorConfigureHandler(c *fiber.Ctx) error {
	var req struct {
		EnableSocks           bool `json:"enable_socks"`
		SocksPort             int  `json:"socks_port"`
		EnableControlPort     bool `json:"enable_control_port"`
		ControlPort           int  `json:"control_port"`
		EnableHiddenService   bool `json:"enable_hidden_service"`
		EnableTransPort       bool `json:"enable_trans_port"`
		TransPort             int  `json:"trans_port"`
		EnableDNSPort         bool `json:"enable_dns_port"`
		DNSPort               int  `json:"dns_port"`
		ClientOnly            bool `json:"client_only"`
		AutomapHostsOnResolve bool `json:"automap_hosts_on_resolve"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Datos inválidos"})
	}

	user, ok := middleware.GetUser(c)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "No autorizado"})
	}
	userID := user.ID

	if req.SocksPort == 0 {
		req.SocksPort = 9050
	}
	if req.ControlPort == 0 {
		req.ControlPort = 9051
	}
	if req.TransPort == 0 {
		req.TransPort = 9040
	}
	if req.DNSPort == 0 {
		req.DNSPort = 53
	}

	opts := TorConfigOptions{
		User:                  user.Username,
		EnableSocks:           req.EnableSocks,
		SocksPort:             req.SocksPort,
		EnableControlPort:     req.EnableControlPort,
		ControlPort:           req.ControlPort,
		EnableHiddenService:   req.EnableHiddenService,
		EnableTransPort:       req.EnableTransPort,
		TransPort:             req.TransPort,
		EnableDNSPort:         req.EnableDNSPort,
		DNSPort:               req.DNSPort,
		ClientOnly:            req.ClientOnly,
		AutomapHostsOnResolve: req.AutomapHostsOnResolve,
	}

	result := ConfigureTor(opts)
	if success, ok := result["success"].(bool); ok && success {
		database.InsertLog("INFO", database.LogMsg("Tor configurado correctamente", user.Username), "tor", &userID)
		return c.JSON(result)
	}

	if errorMsg, ok := result["error"].(string); ok {
		database.InsertLog("ERROR", database.LogMsgErr("configurar Tor", errorMsg, user.Username), "tor", &userID)
		return c.Status(500).JSON(fiber.Map{"error": errorMsg})
	}

	return c.Status(500).JSON(fiber.Map{"error": "Error desconocido"})
}

func TorEnableHandler(c *fiber.Ctx) error {
	return middleware.RunActionWithUser(c, "tor", "Tor habilitado correctamente", "habilitar Tor", func(user *models.User) map[string]interface{} {
		return EnableTor(user.Username)
	})
}

func TorIptablesEnableHandler(c *fiber.Ctx) error {
	return middleware.RunActionWithUser(c, "tor", "Red torificada correctamente", "torificar red", func(user *models.User) map[string]interface{} {
		return EnableTorIptables(user.Username)
	})
}

func TorIptablesDisableHandler(c *fiber.Ctx) error {
	return middleware.RunActionWithUser(c, "tor", "Torificación de red desactivada correctamente", "desactivar torificación de red", func(user *models.User) map[string]interface{} {
		return DisableTorIptables(user.Username)
	})
}

func TorDisableHandler(c *fiber.Ctx) error {
	return middleware.RunActionWithUser(c, "tor", "Tor deshabilitado correctamente", "deshabilitar Tor", func(user *models.User) map[string]interface{} {
		return DisableTor(user.Username)
	})
}

func TorCircuitHandler(c *fiber.Ctx) error {
	result := GetTorCircuitInfo()
	return c.JSON(result)
}

func TorPageHandler(c *fiber.Ctx) error {
	return webtemplates.RenderTemplate(c, "tor", fiber.Map{
		"Title":     i18n.T(c, "tor.title", "Tor Configuration"),
		"tor_status": GetTorStatus(),
	})
}

