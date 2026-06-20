package i18n

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// TranslationsHandler devuelve los ficheros JSON de traducciones para el front.
// Endpoint: `/translations/:lang`
func TranslationsHandler(c *fiber.Ctx) error {
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

