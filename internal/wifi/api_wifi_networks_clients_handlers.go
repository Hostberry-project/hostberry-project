package wifi

import (
	"github.com/gofiber/fiber/v2"
	"hostberry/internal/constants"
)

func WifiNetworksHandler(c *fiber.Ctx) error {
	interfaceName := c.Query("interface", constants.DefaultWiFiInterface)
	if err := validateInterfaceName(interfaceName); err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "error": "Nombre de interfaz inválido"})
	}
	result := ScanWiFiNetworks(interfaceName)
	if success, ok := result["success"].(bool); ok && !success {
		if errMsg, ok := result["error"].(string); ok && errMsg != "" {
			return c.Status(500).JSON(fiber.Map{"success": false, "error": errMsg})
		}
		return c.Status(500).JSON(fiber.Map{"success": false, "error": "Error escaneando redes"})
	}

	if networks, ok := result["networks"]; ok {
		return c.JSON(networks)
	}
	return c.JSON([]fiber.Map{})
}

func WifiClientsHandler(c *fiber.Ctx) error {
	return c.JSON([]fiber.Map{})
}

