package system

import (
	"github.com/gofiber/fiber/v2"

	"hostberry/internal/database"
	middleware "hostberry/internal/middleware"
)

// HelpContactHandler registra la solicitud de contacto/ayuda.
func HelpContactHandler(c *fiber.Ctx) error {
	user, ok := middleware.GetUser(c)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "No autorizado"})
	}
	userID := user.ID
	database.InsertLog("INFO", database.LogMsg("Solicitud de contacto o ayuda recibida", user.Username), "help", &userID)
	return c.JSON(fiber.Map{"success": true})
}

