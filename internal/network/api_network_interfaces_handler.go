package network

import (
	

	"github.com/gofiber/fiber/v2"
)

func NetworkInterfacesHandler(c *fiber.Ctx) error {
	// `GetNetworkInterfaces()` ya implementa la recopilación completa sin `sh -c`
	// (ver `internal/network/network.go`), y el front espera `data.interfaces`.
	result := GetNetworkInterfaces()
	if result == nil {
		return c.JSON(fiber.Map{"interfaces": []map[string]interface{}{}, "success": false})
	}
	// Mantener compatibilidad: devolvemos siempre un objeto con la clave `interfaces`.
	return c.JSON(fiber.Map{
		"interfaces": result["interfaces"],
		// Si existe, mantenemos `success`/`count` para clientes nuevos.
		"success": result["success"],
		"count":   result["count"],
	})
}
