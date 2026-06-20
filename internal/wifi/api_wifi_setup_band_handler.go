package wifi

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"hostberry/internal/auth"
)

// WifiSetupBandHandler fija la banda del AP "hostberry" (2.4 o 5 GHz) durante el asistente inicial.
// En radio única AP+STA, esto determina en qué banda se escanea y se puede conectar la STA, ya que
// el AP y el cliente comparten la misma antena (mismo canal). El frontend del wizard llama a este
// endpoint cuando el usuario elige la banda, antes de escanear las redes.
func WifiSetupBandHandler(c *fiber.Ctx) error {
	var req struct {
		Band string `json:"band"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "error": "Datos inválidos"})
	}

	band := strings.TrimSpace(req.Band)
	if band == "" {
		band = c.Query("band")
	}
	switch band {
	case "2.4", "2.4GHz", "24":
		band = band24GHz
	case "5", "5GHz", "5G":
		band = band5GHz
	default:
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"error":   "Banda inválida; usa \"2.4\" o \"5\"",
		})
	}

	// Durante el wizard: solo guardar preferencia; la banda se aplica al reiniciar al finalizar.
	if auth.IsInitialSetupPending() {
		if err := SaveWizardPreferredBand(band); err != nil {
			return c.Status(500).JSON(fiber.Map{"success": false, "error": err.Error()})
		}
		// La banda elegida también es la banda de escaneo: la radio única puede escanear 5 GHz
		// off-channel aunque el AP siga en 2.4 durante el asistente. El cambio real del AP se aplica
		// al reiniciar tras completar el asistente.
		return c.JSON(fiber.Map{
			"success":       true,
			"deferred":      true,
			"band":          band,
			"scan_band":     band,
			"concurrent_ap": concurrentAPInterfacePresent(),
		})
	}

	// Fuera del wizard: aplicar la banda en caliente si hace falta.
	if !concurrentAPInterfacePresent() {
		_ = SaveWizardPreferredBand(band)
		return c.JSON(fiber.Map{
			"success":       true,
			"band":          band,
			"concurrent_ap": false,
			"channel":       defaultAPChannelForBand(band),
		})
	}

	channel, actualBand, err := SetWizardAPBand(band)
	if err != nil {
		if actualBand != "" && actualBand != band {
			return c.JSON(fiber.Map{
				"success":        false,
				"error":          err.Error(),
				"band":           actualBand,
				"requested_band": band,
				"channel":        channel,
				"concurrent_ap":  true,
			})
		}
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success":        true,
		"band":           actualBand,
		"requested_band": band,
		"channel":        channel,
		"concurrent_ap":  true,
	})
}
