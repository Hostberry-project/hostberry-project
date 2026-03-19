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

func WifiSoftwareSwitchHandler(c *fiber.Ctx) error {
	user, ok := middleware.GetUser(c)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "No autorizado"})
	}
	userID := user.ID

	rfkillCheck := exec.Command("sh", "-c", "command -v rfkill 2>/dev/null")
	if rfkillCheck.Run() != nil {
		errorMsg := "rfkill no está disponible en el sistema"
		database.InsertLog("ERROR", database.LogMsgErr("cambiar conmutador de software WiFi", errorMsg, user.Username), "wifi", &userID)
		return c.Status(500).JSON(fiber.Map{"success": false, "error": errorMsg})
	}

	statusOut, _ := execCommand("rfkill list wifi 2>/dev/null | grep -i 'soft blocked'").CombinedOutput()
	statusStr := strings.ToLower(string(statusOut))
	isBlocked := strings.Contains(statusStr, "yes")

	var cmd string
	var action string
	if isBlocked {
		cmd = "rfkill unblock wifi"
		action = "desbloqueado"
	} else {
		cmd = "rfkill block wifi"
		action = "bloqueado"
	}

	output, err := execCommand(cmd + " 2>&1").CombinedOutput()
	if err != nil {
		errorMsg := fmt.Sprintf("Error ejecutando rfkill: %s", string(output))
		database.InsertLog("ERROR", database.LogMsgErr("cambiar conmutador de software WiFi", errorMsg, user.Username), "wifi", &userID)
		return c.Status(500).JSON(fiber.Map{"success": false, "error": errorMsg})
	}

	time.Sleep(1 * time.Second)

	newStatusOut, _ := execCommand("rfkill list wifi 2>/dev/null | grep -i 'soft blocked'").CombinedOutput()
	newStatusStr := strings.ToLower(string(newStatusOut))
	newIsBlocked := strings.Contains(newStatusStr, "yes")

	if isBlocked == newIsBlocked {
		errorMsg := "El switch de software no cambió de estado"
		database.InsertLog("WARN", database.LogMsgWarn("el conmutador de software WiFi no cambió: "+errorMsg, user.Username), "wifi", &userID)
		return c.Status(500).JSON(fiber.Map{"success": false, "error": errorMsg})
	}

	message := fmt.Sprintf("Switch de software %s exitosamente", action)
	database.InsertLog("INFO", database.LogMsg("Conmutador de software WiFi "+action+" correctamente", user.Username), "wifi", &userID)
	return c.JSON(fiber.Map{
		"success": true,
		"message": message,
		"blocked": newIsBlocked,
	})
}

