package system

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"hostberry/internal/config"
	"hostberry/internal/database"
	"hostberry/internal/i18n"
	middleware "hostberry/internal/middleware"
)

var allowedSystemConfigKeys = map[string]struct{}{
	"language":           {},
	"theme":              {},
	"timezone":           {},
	"dhcp_enabled":       {},
	"dhcp_interface":     {},
	"dhcp_range_start":   {},
	"dhcp_range_end":     {},
	"dhcp_gateway":       {},
	"dhcp_lease_time":    {},
	"dns_server":         {},
	"max_login_attempts": {},
	"session_timeout":    {},
	"cache_enabled":      {},
	"cache_size":         {},
	"compression_enabled": {},
	"email_notifications": {},
	"email_address":       {},
	"smtp_host":           {},
	"smtp_port":           {},
	"smtp_user":           {},
	"smtp_password":       {},
	"smtp_from":           {},
	"smtp_tls":            {},
	"system_alerts":       {},
}

// SystemConfigHandler guarda la configuración del sistema recibida desde la UI.
func SystemConfigHandler(c *fiber.Ctx) error {
	var req map[string]interface{}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Datos inválidos",
		})
	}

	user, ok := middleware.GetUser(c)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "No autorizado"})
	}
	userID := user.ID

	updatedKeys := []string{}
	errors := []string{}

	for key, value := range req {
		if _, ok := allowedSystemConfigKeys[key]; !ok {
			errors = append(errors, fmt.Sprintf("Clave no permitida: %s", key))
			continue
		}

		var valueStr string
		switch v := value.(type) {
		case string:
			valueStr = v
		case float64:
			valueStr = fmt.Sprintf("%v", v) // Para números JSON
		case bool:
			valueStr = fmt.Sprintf("%v", v)
		case nil:
			continue
		default:
			valueStr = fmt.Sprintf("%v", v)
		}

		// No sobreescribir secretos con cadena vacía cuando la UI deja el campo en blanco
		// para indicar "mantener valor actual".
		if key == "smtp_password" && strings.TrimSpace(valueStr) == "" {
			continue
		}

		if err := database.SetConfig(key, valueStr); err != nil {
			i18n.LogTf("logs.config_save_error", key, err)
			errors = append(errors, fmt.Sprintf("Error guardando %s", key))
		}

		if key == "timezone" && valueStr != "" {
			tz := strings.TrimSpace(valueStr)

			// Validaciones defensivas contra inyecciones de ruta/comando.
			if strings.Contains(tz, "..") || strings.Contains(tz, ";") || strings.HasPrefix(tz, "/") {
				errors = append(errors, "Zona horaria inválida")
				continue
			}

			zonePath := filepath.Clean(filepath.Join("/usr/share/zoneinfo", tz))
			if !strings.HasPrefix(zonePath, "/usr/share/zoneinfo") {
				errors = append(errors, "Zona horaria inválida")
				continue
			}
			if _, err := os.Stat(zonePath); os.IsNotExist(err) {
				errors = append(errors, "Zona horaria no encontrada")
				continue
			}

			cmd := exec.Command("sudo", "/usr/local/sbin/hostberry-safe/set-timezone", tz)
			output, err := cmd.CombinedOutput()
			if err != nil {
				combined := strings.TrimSpace(string(output))
				i18n.LogTf("logs.config_timezone_error", err, combined)

				baseMsg := "No se pudo aplicar la zona horaria al sistema"
				if combined != "" {
					if strings.Contains(strings.ToLower(combined), "sudo") &&
						(strings.Contains(strings.ToLower(combined), "password") || strings.Contains(strings.ToLower(combined), "required")) {
						errors = append(errors, "Permisos insuficientes (sudo requerido)")
					} else {
						errors = append(errors, fmt.Sprintf("%s: %s", baseMsg, combined[:min(len(combined), 200)]))
					}
				} else {
					errors = append(errors, fmt.Sprintf("%s (rc=%v)", baseMsg, err))
				}
			} else {
				i18n.LogTf("logs.config_timezone_success", tz)
			}
		}

		if key == "session_timeout" {
			if timeout, err := strconv.Atoi(valueStr); err == nil && timeout > 0 {
				config.AppConfig.Security.TokenExpiry = timeout
				i18n.LogTf("logs.config_session_timeout", timeout)
			}
		}

		if key == "max_login_attempts" {
			i18n.LogTf("logs.config_max_login", valueStr)
		}

		if key == "cache_enabled" {
			i18n.LogTf("logs.config_cache_enabled", valueStr)
		}

		if key == "compression_enabled" {
			i18n.LogTf("logs.config_compression", valueStr)
		}

		updatedKeys = append(updatedKeys, key)
	}

	response := fiber.Map{
		"message":      "Configuración guardada",
		"updated_keys": updatedKeys,
	}

	if len(errors) > 0 {
		response["errors"] = errors
		response["message"] = fmt.Sprintf("Configuración guardada con advertencias (Algunos errores: %s)", strings.Join(errors, ", "))
	} else {
		response["message"] = "Configuración actualizada exitosamente"
	}

	database.InsertLog("INFO", database.LogMsg("Configuración del sistema actualizada", user.Username), "system", &userID)
	return c.JSON(response)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

