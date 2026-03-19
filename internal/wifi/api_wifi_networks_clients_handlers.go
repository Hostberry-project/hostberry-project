package wifi

import (
	"github.com/gofiber/fiber/v2"
	"hostberry/internal/constants"
)

func WifiNetworksHandler(c *fiber.Ctx) error {
	interfaceName := c.Query("interface", constants.DefaultWiFiInterface)
	result := ScanWiFiNetworks(interfaceName)
	if networks, ok := result["networks"]; ok {
		return c.JSON(networks)
	}
	return c.JSON([]fiber.Map{})
}

func WifiClientsHandler(c *fiber.Ctx) error {
	return c.JSON([]fiber.Map{})
}

