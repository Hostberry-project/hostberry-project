package wifi

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"hostberry/internal/database"
	middleware "hostberry/internal/middleware"
)

func WifiUnblockHandler(c *fiber.Ctx) error {
	user, ok := middleware.GetUser(c)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "No autorizado"})
	}
	userID := user.ID

	success := false
	method := ""
	var lastError error

	rfkillCheck := exec.Command("sh", "-c", "command -v rfkill 2>/dev/null")
	if rfkillCheck.Run() == nil {
		rfkillOut, rfkillErr := execCommand("rfkill list wifi 2>/dev/null | grep -i 'wifi' | head -1").CombinedOutput()
		if rfkillErr == nil && strings.Contains(strings.ToLower(string(rfkillOut)), "wifi") {
			rfkillCmd := "rfkill unblock wifi"
			rfkillOutSudo, rfkillUnblockErr := execCommand(rfkillCmd + " 2>&1").CombinedOutput()
			if rfkillUnblockErr == nil {
				success = true
				method = "rfkill (con sudo)"
			} else {
				lastError = fmt.Errorf("rfkill error: %s", string(rfkillOutSudo))
			}
		}
	}

	if !success {
		nmcliCheck := exec.Command("sh", "-c", "command -v nmcli 2>/dev/null")
		if nmcliCheck.Run() == nil {
			nmcliCmd := "nmcli radio wifi on"
			nmcliOut, nmcliErr := execCommand(nmcliCmd + " 2>&1").CombinedOutput()
			if nmcliErr == nil {
				success = true
				method = "nmcli (con sudo)"
			} else {
				if lastError == nil {
					lastError = fmt.Errorf("nmcli error: %s", string(nmcliOut))
				}
			}
		}
	}

	if success && method == "rfkill (con sudo)" {
		nmcliCheck := exec.Command("sh", "-c", "command -v nmcli 2>/dev/null")
		if nmcliCheck.Run() == nil {
			execCommand("nmcli radio wifi on 2>/dev/null").Run()
		}
	}

	if success {
		time.Sleep(1 * time.Second)

		database.InsertLog("INFO", database.LogMsg("WiFi desbloqueado correctamente", user.Username), "wifi", &userID)
		return c.JSON(fiber.Map{"success": true, "message": "WiFi desbloqueado exitosamente"})
	}

	errorDetails := "No se pudo desbloquear WiFi."
	if lastError != nil {
		errorDetails += fmt.Sprintf(" Último error: %v", lastError)
	}

	availableCmds := []string{}
	if exec.Command("sh", "-c", "command -v rfkill 2>/dev/null").Run() == nil {
		availableCmds = append(availableCmds, "rfkill")
	}
	if exec.Command("sh", "-c", "command -v nmcli 2>/dev/null").Run() == nil {
		availableCmds = append(availableCmds, "nmcli")
	}

	if len(availableCmds) == 0 {
		errorDetails += " No se encontraron comandos rfkill ni nmcli instalados."
	} else {
		errorDetails += fmt.Sprintf(" Comandos disponibles: %s. Verifica permisos sudo (NOPASSWD) ejecutando: sudo fix_wifi_permissions.sh", strings.Join(availableCmds, ", "))
	}

	database.InsertLog("ERROR", database.LogMsgErr("desbloquear WiFi", errorDetails, user.Username), "wifi", &userID)
	return c.Status(500).JSON(fiber.Map{"error": errorDetails})
}

