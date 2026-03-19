package wifi

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"hostberry/internal/constants"
	"hostberry/internal/database"
	middleware "hostberry/internal/middleware"
	"hostberry/internal/validators"
)

func WifiConfigHandler(c *fiber.Ctx) error {
	user, ok := middleware.GetUser(c)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "No autorizado"})
	}
	userID := user.ID

	var req struct {
		SSID     string `json:"ssid"`
		Password string `json:"password"`
		Security string `json:"security"`
		Region   string `json:"region"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Datos inválidos"})
	}

	if req.Region != "" {
		if len(req.Region) != 2 {
			return c.Status(400).JSON(fiber.Map{"error": "Código de región inválido. Debe ser de 2 letras (ej: US, ES, GB)"})
		}

		req.Region = strings.ToUpper(req.Region)

		iwCheck := exec.Command("sh", "-c", "command -v iw 2>/dev/null")
		if iwCheck.Run() == nil {
			cmd := exec.Command("sh", "-c", fmt.Sprintf("sudo iw reg set %s 2>&1", req.Region))
			out, err := cmd.CombinedOutput()
			output := strings.TrimSpace(string(out))

			if err == nil {
				verifyCmd := exec.Command("sh", "-c", "iw reg get 2>&1")
				verifyOut, _ := verifyCmd.CombinedOutput()
				verifyOutput := strings.TrimSpace(string(verifyOut))

				if strings.Contains(verifyOutput, req.Region) || output == "" {
					database.InsertLog("INFO", database.LogMsg("Región WiFi cambiada a "+req.Region, user.Username), "wifi", &userID)
					return c.JSON(fiber.Map{"success": true, "message": "Región WiFi cambiada exitosamente a " + req.Region})
				}
			}

			crdaCmd := exec.Command("sh", "-c", fmt.Sprintf("echo 'REGDOMAIN=%s' | sudo tee /etc/default/crda >/dev/null 2>&1", req.Region))
			if crdaCmd.Run() == nil {
				database.InsertLog("INFO", database.LogMsg("Región WiFi configurada a "+req.Region+" (crda)", user.Username), "wifi", &userID)
				exec.Command("sh", "-c", "sudo nmcli radio wifi off 2>/dev/null").Run()
				time.Sleep(1 * time.Second)
				exec.Command("sh", "-c", "sudo nmcli radio wifi on 2>/dev/null").Run()
				return c.JSON(fiber.Map{"success": true, "message": "Región WiFi configurada exitosamente. WiFi reiniciado para aplicar cambios."})
			}

			regdomCmd := exec.Command("sh", "-c", fmt.Sprintf("echo '%s' | sudo tee /etc/conf.d/wireless-regdom >/dev/null 2>&1", req.Region))
			if regdomCmd.Run() == nil {
				database.InsertLog("INFO", database.LogMsg("Región WiFi configurada a "+req.Region+" (wireless-regdom)", user.Username), "wifi", &userID)
				return c.JSON(fiber.Map{"success": true, "message": "Región WiFi configurada. Reinicia WiFi o el sistema para aplicar cambios."})
			}
		}

		crdaCmd2 := exec.Command("sh", "-c", fmt.Sprintf("echo 'REGDOMAIN=%s' | sudo tee /etc/default/crda >/dev/null 2>&1", req.Region))
		if crdaCmd2.Run() == nil {
			database.InsertLog("INFO", database.LogMsg("Región WiFi configurada a "+req.Region, user.Username), "wifi", &userID)
			return c.JSON(fiber.Map{"success": true, "message": "Región WiFi configurada. Reinicia WiFi para aplicar cambios."})
		}

		errorMsg := fmt.Sprintf("No se pudo cambiar la región WiFi automáticamente. Verifica que 'iw' esté instalado (sudo apt-get install iw) y que tengas permisos sudo configurados. Puedes configurarlo manualmente ejecutando: sudo iw reg set %s", req.Region)
		database.InsertLog("ERROR", database.LogMsgErr("cambiar región WiFi a "+req.Region, errorMsg, user.Username), "wifi", &userID)
		return c.Status(500).JSON(fiber.Map{"error": errorMsg})
	}

	if req.SSID != "" {
		if err := validators.ValidateSSID(req.SSID); err != nil {
			return err
		}
		if len(req.Password) > 128 {
			return c.Status(400).JSON(fiber.Map{
				"error": "La contraseña no puede tener más de 128 caracteres",
			})
		}

		country := req.Region
		if country == "" {
			country = constants.DefaultCountryCode
		}

		result := ConnectWiFi(req.SSID, req.Password, constants.DefaultWiFiInterface, country, user.Username)
		if success, ok := result["success"].(bool); ok && success {
			database.InsertLog("INFO", database.LogMsg("Conexión WiFi a "+req.SSID+" correcta", user.Username), "wifi", &userID)
			return c.JSON(result)
		}

		errorMsg := "Error desconocido"
		if errorMsgVal, ok := result["error"].(string); ok && errorMsgVal != "" {
			errorMsg = errorMsgVal
		}
		database.InsertLog("ERROR", database.LogMsgErr("conectar WiFi a "+req.SSID, errorMsg, user.Username), "wifi", &userID)
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"error":   errorMsg,
			"message": fmt.Sprintf("Error conectando a %s", req.SSID),
		})
	}

	return c.Status(400).JSON(fiber.Map{"error": "Se requiere ssid o region"})
}

func WifiStatusHandler(c *fiber.Ctx) error {
	return WifiLegacyStatusHandler(c)
}

