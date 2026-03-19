package server

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/gofiber/fiber/v2"
	"hostberry/internal/i18n"
)

// SetupStaticFiles sirve estáticos desde disco si existe `./website/static`,
// o bien desde el FS embebido si está disponible.
func SetupStaticFiles(app *fiber.App, staticFS embed.FS) {
	// Preferimos servir desde embed para mejorar rendimiento en SD (Raspberry Pi).
	staticSubFS, err := fs.Sub(staticFS, "website/static")
	if err == nil {
		// Validación mínima: comprobamos al menos un archivo conocido.
		if f, openErr := staticSubFS.Open("hostberry.png"); openErr == nil {
			_ = f.Close()
			app.Get("/static/*", func(c *fiber.Ctx) error {
				path := c.Params("*")
				file, err := staticSubFS.Open(path)
				if err != nil {
					return c.Status(404).SendString("Not found")
				}
				defer file.Close()

				stat, err := file.Stat()
				if err != nil {
					return c.Status(500).SendString("Error reading file")
				}

				// Cache-control para acelerar carga de recursos en el navegador.
				// (Los assets no parecen versionados; usamos una expiración moderada.)
				c.Set("Cache-Control", "public, max-age=3600")
				c.Type(filepath.Ext(path))
				return c.SendStream(file, int(stat.Size()))
			})
			i18n.LogTln("logs.static_files_loaded_embed")
			return
		}
	}

	// Fallback a disco (útil para desarrollo o si no se embebió estático).
	if _, err := os.Stat("./website/static"); err == nil {
		app.Static("/static", "./website/static", fiber.Static{
			Compress:      true,
			ByteRange:    true,
			CacheDuration: 1 * time.Hour,
			MaxAge:        3600,
		})
		i18n.LogTln("logs.static_files_loaded_disk")
		return
	}

	i18n.LogT("logs.static_files_not_found")
}

