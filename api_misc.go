package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v2"
	"hostberry/internal/database"
)

func helpContactHandler(c *fiber.Ctx) error {
	user, ok := GetUser(c)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "No autorizado"})
	}
	userID := user.ID
	InsertLog("INFO", LogMsg("Solicitud de contacto o ayuda recibida", user.Username), "help", &userID)
	return c.JSON(fiber.Map{"success": true})
}

func translationsHandler(c *fiber.Ctx) error {
	lang := c.Params("lang", "en")
	if lang != "en" && lang != "es" {
		lang = "en"
	}
	path := filepath.Clean(filepath.Join("locales", lang+".json"))
	if !strings.HasPrefix(path, "locales"+string(filepath.Separator)) && path != "locales" {
		return c.Status(400).JSON(fiber.Map{"error": "idioma no permitido"})
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	var out interface{}
	if err := json.Unmarshal(b, &out); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "JSON inválido en locales"})
	}
	return c.JSON(out)
}
