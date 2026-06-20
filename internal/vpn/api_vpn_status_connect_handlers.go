package vpn

import (
	"github.com/gofiber/fiber/v2"
	"hostberry/internal/database"
	middleware "hostberry/internal/middleware"
)

// VpnStatusHandler devuelve el estado general de OpenVPN/WireGuard.
func VpnStatusHandler(c *fiber.Ctx) error {
	result := GetVPNStatus()
	return c.JSON(result)
}

// VpnConnectHandler conecta a VPN (OpenVPN/WireGuard según configuración remota).
func VpnConnectHandler(c *fiber.Ctx) error {
	var req struct {
		Config string `json:"config"`
		Type   string `json:"type"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Datos inválidos"})
	}
	user, ok := middleware.GetUser(c)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "No autorizado"})
	}
	userID := user.ID
	result := ConnectVPN(req.Config, req.Type, user.Username)
	if success, ok := result["success"].(bool); ok && success {
		database.InsertLog("INFO", database.LogMsg("Conexión VPN ("+req.Type+") correcta", user.Username), "vpn", &userID)
		return c.JSON(result)
	}
	if errorMsg, ok := result["error"].(string); ok {
		database.InsertLog("ERROR", database.LogMsgErr("conectar VPN ("+req.Type+")", errorMsg, user.Username), "vpn", &userID)
		return c.Status(500).JSON(fiber.Map{"error": errorMsg})
	}
	return c.Status(500).JSON(fiber.Map{"error": "Error desconocido"})
}

