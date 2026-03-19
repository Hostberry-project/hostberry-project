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
)

func WifiToggleHandler(c *fiber.Ctx) error {
	user, ok := middleware.GetUser(c)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "No autorizado"})
	}
	userID := user.ID

	interfaceName := c.Query("interface", constants.DefaultWiFiInterface)
	if err := validateInterfaceName(interfaceName); err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "error": "Nombre de interfaz inválido"})
	}
	rfkillCheck := exec.Command("sh", "-c", "sudo rfkill list wifi 2>/dev/null | grep -i 'soft blocked'")
	rfkillOut, _ := rfkillCheck.Output()
	isBlocked := strings.Contains(strings.ToLower(string(rfkillOut)), "yes")

	result := ToggleWiFi(interfaceName, isBlocked)

	if success, ok := result["success"].(bool); ok && success {
		database.InsertLog("INFO", database.LogMsg("WiFi activado o desactivado correctamente", user.Username), "wifi", &userID)
		return c.JSON(result)
	}

	if errorMsg, ok := result["error"].(string); ok && errorMsg != "" {
		database.InsertLog("ERROR", database.LogMsgErr("cambiar estado WiFi", errorMsg, user.Username), "wifi", &userID)
		return c.Status(500).JSON(fiber.Map{"success": false, "error": errorMsg})
	}

	rfkillOut, rfkillErr := execCommand("rfkill list wifi 2>/dev/null | grep -i 'wifi' | head -1").CombinedOutput()
	if rfkillErr == nil && strings.Contains(strings.ToLower(string(rfkillOut)), "wifi") {
		statusOut, _ := execCommand("rfkill list wifi 2>/dev/null | grep -i 'soft blocked'").CombinedOutput()
		isBlocked := strings.Contains(strings.ToLower(string(statusOut)), "yes")

		var rfkillCmd string
		var wasEnabled bool
		if isBlocked {
			rfkillCmd = "rfkill unblock wifi"
			wasEnabled = false
		} else {
			rfkillCmd = "rfkill block wifi"
			wasEnabled = true
		}

		_, rfkillToggleErr := execCommand(rfkillCmd + " 2>/dev/null").CombinedOutput()
		if rfkillToggleErr == nil {
			if !wasEnabled {
				time.Sleep(1 * time.Second)

				ifaceCmd := exec.Command("sh", "-c", "ip -o link show | awk -F': ' '{print $2}' | grep -E '^wlan|^wl' | head -1")
				ifaceOut, ifaceErr := ifaceCmd.Output()
				if ifaceErr == nil {
					iface := strings.TrimSpace(string(ifaceOut))
					if iface != "" {
						execCommand(fmt.Sprintf("ip link set %s up 2>/dev/null", iface)).Run()
						time.Sleep(1 * time.Second)
					}
				}
			}
			database.InsertLog("INFO", database.LogMsg("WiFi activado o desactivado correctamente (rfkill)", user.Username), "wifi", &userID)
			return c.JSON(fiber.Map{"success": true, "message": "WiFi toggle exitoso"})
		}
	}

	var iface string
	ipOut, ipErr := exec.Command("sh", "-c", "ip -o link show | awk -F': ' '{print $2}' | grep -E '^wlan|^wl' | head -1").Output()
	if ipErr == nil {
		iface = strings.TrimSpace(string(ipOut))
	}

	if iface == "" {
		iwOut, iwErr := execCommand("iwconfig 2>/dev/null | grep -i 'wlan' | head -1 | awk '{print $1}'").CombinedOutput()
		if iwErr == nil {
			iface = strings.TrimSpace(string(iwOut))
		}
	}

	if iface != "" {
		statusOut, _ := exec.Command("sh", "-c", fmt.Sprintf("ip link show %s 2>/dev/null | grep -i 'state'", iface)).CombinedOutput()
		isDown := strings.Contains(strings.ToLower(string(statusOut)), "down") || strings.Contains(strings.ToLower(string(statusOut)), "disabled")

		if isDown {
			execCommand("rfkill unblock wifi 2>/dev/null").Run()
			execCommand(fmt.Sprintf("ip link set %s up 2>/dev/null", iface)).Run()
			execCommand(fmt.Sprintf("ifconfig %s up 2>/dev/null", iface)).Run()
			time.Sleep(1 * time.Second)
			database.InsertLog("INFO", database.LogMsg("WiFi activado en interfaz "+iface, user.Username), "wifi", &userID)
			return c.JSON(fiber.Map{"success": true, "message": fmt.Sprintf("WiFi activado en interfaz %s", iface)})
		} else {
			iwCmd := fmt.Sprintf("ifconfig %s down", iface)
			execCommand(iwCmd + " 2>/dev/null").Run()
			database.InsertLog("INFO", database.LogMsg("WiFi desactivado en interfaz "+iface, user.Username), "wifi", &userID)
			return c.JSON(fiber.Map{"success": true, "message": fmt.Sprintf("WiFi desactivado en interfaz %s", iface)})
		}
	}

	errorMsg := "No se pudo cambiar el estado de WiFi. Verifica que tengas permisos sudo configurados (NOPASSWD) o que rfkill/ip estén disponibles. Para configurar sudo sin contraseña, ejecuta: sudo visudo y agrega: usuario ALL=(ALL) NOPASSWD: /usr/sbin/rfkill, /sbin/ip, /sbin/ifconfig"
	database.InsertLog("ERROR", database.LogMsgErr("cambiar estado WiFi", errorMsg, user.Username), "wifi", &userID)
	return c.Status(500).JSON(fiber.Map{"success": false, "error": errorMsg})
}

