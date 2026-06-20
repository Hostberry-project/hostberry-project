package server

import (
	"os"
	"path/filepath"

	"github.com/gofiber/fiber/v2"
)

// OpenAPIHandler sirve la especificación OpenAPI estática.
func OpenAPIHandler(c *fiber.Ctx) error {
	candidates := []string{
		"docs/openapi.yaml",
		"/opt/hostberry/docs/openapi.yaml",
	}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "docs", "openapi.yaml"))
	}
	for _, p := range candidates {
		data, err := os.ReadFile(p)
		if err == nil {
			c.Set("Content-Type", "application/yaml; charset=utf-8")
			return c.Send(data)
		}
	}
	return c.Status(404).JSON(fiber.Map{"error": "openapi.yaml no encontrado"})
}
