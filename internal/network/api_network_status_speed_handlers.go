package network

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"hostberry/internal/config"
	"hostberry/internal/i18n"
	webtemplates "hostberry/internal/templates"
)

func NetworkStatusHandler(c *fiber.Ctx) error {
	result := GetNetworkStatus()
	return c.JSON(result)
}

// libreSpeedCLIPath nombres posibles del binario LibreSpeed (speedtest-cli)
var librespeedCLIPath = []string{"librespeed-cli", "librespeed-cli-go", "/usr/bin/librespeed-cli", "/usr/local/bin/librespeed-cli"}

func NetworkSpeedtestHandler(c *fiber.Ctx) error {
	_ = config.AppConfig // Asegura dependencia (y mantiene compatibilidad con el resto del paquete).
	ctx, cancel := context.WithTimeout(c.Context(), 120*time.Second)
	defer cancel()

	var bin string
	for _, p := range librespeedCLIPath {
		if path, err := exec.LookPath(p); err == nil {
			bin = path
			break
		}
	}
	if bin == "" {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"error":   "LibreSpeed CLI no instalado. Instálalo desde https://github.com/librespeed/speedtest-cli",
		})
	}

	cmd := exec.CommandContext(ctx, bin, "--json", "--telemetry-level", "disabled")
	out, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return c.Status(408).JSON(fiber.Map{"success": false, "error": "Timeout del test de velocidad"})
		}
		return c.JSON(fiber.Map{
			"success": false,
			"error":   strings.TrimSpace(string(out)),
		})
	}

	// La salida puede tener líneas de log y una línea JSON; buscar la línea que empieza con '{'.
	lines := strings.Split(string(out), "\n")
	var raw []byte
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(line, "{") {
			raw = []byte(line)
			break
		}
	}
	if len(raw) == 0 {
		raw = out
	}

	var result struct {
		Timestamp     string  `json:"timestamp"`
		Ping          float64 `json:"ping"`
		Jitter        float64 `json:"jitter"`
		Download      float64 `json:"download"`
		Upload        float64 `json:"upload"`
		BytesSent     int64   `json:"bytes_sent"`
		BytesReceived int64   `json:"bytes_received"`
		Server        struct {
			Name string `json:"name"`
			URL  string `json:"url"`
		} `json:"server"`
		Client struct {
			IP      string `json:"ip"`
			Org     string `json:"org"`
			Country string `json:"country"`
			City    string `json:"city"`
		} `json:"client"`
	}

	if err := json.Unmarshal(raw, &result); err != nil {
		return c.JSON(fiber.Map{
			"success": false,
			"error":   "No se pudo interpretar la salida de LibreSpeed: " + err.Error(),
		})
	}

	// Pequeña traza para debug.
	if config.AppConfig.Server.Debug {
		i18n.LogTf("logs.network_speedtest_ok", result.Ping, result.Download)
	}

	return c.JSON(fiber.Map{
		"success":         true,
		"ping_ms":         result.Ping,
		"jitter_ms":       result.Jitter,
		"download_mbps":  result.Download,
		"upload_mbps":    result.Upload,
		"bytes_sent":      result.BytesSent,
		"bytes_received":  result.BytesReceived,
		"server_name":     result.Server.Name,
		"server_url":      result.Server.URL,
		"client_ip":       result.Client.IP,
		"client_org":      result.Client.Org,
		"client_country":  result.Client.Country,
		"client_city":     result.Client.City,
		"timestamp":       result.Timestamp,
	})
}

func NetworkPageHandler(c *fiber.Ctx) error {
	return webtemplates.RenderTemplate(c, "network", fiber.Map{
		"Title": i18n.T(c, "network.title", "Network Management"),
	})
}

