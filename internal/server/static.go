package server

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/gofiber/fiber/v2"
	"hostberry/internal/i18n"
)

// SetupStaticFiles sirve estáticos desde disco si existe `./website/static`,
// o bien desde el FS embebido si está disponible.
func SetupStaticFiles(app *fiber.App, staticFS embed.FS) {
	if _, err := os.Stat("./website/static"); err == nil {
		app.Static("/static", "./website/static", fiber.Static{
			Compress:  true,
			ByteRange: true,
		})
		i18n.LogTln("logs.static_files_loaded")
		return
	}

	staticSubFS, err := fs.Sub(staticFS, "website/static")
	if err != nil {
		i18n.LogTf("logs.static_files_embed_error", err)
		i18n.LogT("logs.static_files_not_found")
		return
	}

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

		c.Type(filepath.Ext(path))
		return c.SendStream(file, int(stat.Size()))
	})
	i18n.LogTln("logs.static_files_embedded")
}

