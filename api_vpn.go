package main

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
	middleware "hostberry/internal/middleware"
)

func vpnConnectionsHandler(c *fiber.Ctx) error {
	result := getVPNStatus()

	var conns []fiber.Map
	if ov, ok := result["openvpn"].(map[string]interface{}); ok {
		status := fmt.Sprintf("%v", ov["status"])
		conns = append(conns, fiber.Map{"name": "openvpn", "type": "openvpn", "status": mapActiveStatus(status), "bandwidth": "-"})
	}
	if wg, ok := result["wireguard"].(map[string]interface{}); ok {
		active := fmt.Sprintf("%v", wg["active"])
		conns = append(conns, fiber.Map{"name": "wireguard", "type": "wireguard", "status": mapBoolStatus(active), "bandwidth": "-"})
	}
	return c.JSON(conns)
}

func vpnServersHandler(c *fiber.Ctx) error { return c.JSON([]fiber.Map{}) }
func vpnClientsHandler(c *fiber.Ctx) error { return c.JSON([]fiber.Map{}) }
func vpnToggleHandler(c *fiber.Ctx) error {
	return c.Status(501).JSON(fiber.Map{"error": "VPN toggle no implementado"})
}
func vpnGetConfigHandler(c *fiber.Ctx) error {
	result := getOpenVPNConfig()
	return c.JSON(result)
}

func vpnConfigHandler(c *fiber.Ctx) error {
	var req struct {
		Config string `json:"config"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Datos inválidos"})
	}
	user, ok := middleware.GetUser(c)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "No autorizado"})
	}
	result := saveOpenVPNConfig(req.Config, user.Username)
	if success, ok := result["success"].(bool); ok && success {
		return c.JSON(result)
	}
	if errorMsg, ok := result["error"].(string); ok {
		return c.Status(400).JSON(fiber.Map{"error": errorMsg})
	}
	return c.Status(500).JSON(fiber.Map{"error": "Error desconocido"})
}
func vpnConnectionToggleHandler(c *fiber.Ctx) error {
	return c.Status(501).JSON(fiber.Map{"error": "VPN connection toggle no implementado"})
}
func vpnCertificatesGenerateHandler(c *fiber.Ctx) error {
	return c.Status(501).JSON(fiber.Map{"error": "VPN certificates no implementado"})
}
