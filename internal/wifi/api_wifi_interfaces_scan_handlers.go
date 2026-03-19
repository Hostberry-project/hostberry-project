package wifi

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/gofiber/fiber/v2"
	"hostberry/internal/constants"
)

func detectWiFiInterface() string {
	cmd := exec.Command("sh", "-c", "ip -o link show | awk -F': ' '{print $2}' | grep -E '^wlan|^wl' | head -1")
	out, err := cmd.Output()
	if err == nil {
		iface := strings.TrimSpace(string(out))
		if iface != "" {
			return iface
		}
	}

	return constants.DefaultWiFiInterface
}

// DetectWiFiInterface wrapper exportado para usarlo desde main (setup/auto-conexión).
func DetectWiFiInterface() string {
	return detectWiFiInterface()
}

// WifiInterfacesHandler devuelve una lista simple de interfaces WiFi y su estado.
func WifiInterfacesHandler(c *fiber.Ctx) error {
	var interfaces []fiber.Map

	cmd := exec.Command("sh", "-c", "ip -o link show | awk -F': ' '{print $2}' | grep -E '^wlan|^wl'")
	out, err := cmd.Output()
	if err == nil {
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		for _, ifaceName := range lines {
			ifaceName = strings.TrimSpace(ifaceName)
			if ifaceName != "" {
				stateCmd := exec.Command("sh", "-c", fmt.Sprintf("cat /sys/class/net/%s/operstate 2>/dev/null", ifaceName))
				stateOut, _ := stateCmd.Output()
				state := strings.TrimSpace(string(stateOut))
				if state == "" {
					state = "unknown"
				}

				interfaces = append(interfaces, fiber.Map{
					"name":  ifaceName,
					"type":  "wifi",
					"state": state,
				})
			}
		}
	}

	if len(interfaces) == 0 {
		interfaces = append(interfaces, fiber.Map{
			"name":  constants.DefaultWiFiInterface,
			"type":  "wifi",
			"state": "unknown",
		})
	}

	return c.JSON(fiber.Map{
		"success":    true,
		"interfaces": interfaces,
	})
}

// WifiScanHandler escanea redes WiFi (útil también para el setup wizard).
func WifiScanHandler(c *fiber.Ctx) error {
	// Para el setup wizard puede que no exista sesión/token.
	// Escanear redes no requiere usuario; solo la interfaz a usar.
	interfaceName := c.Query("interface", "")
	if interfaceName == "" {
		var req struct {
			Interface string `json:"interface"`
		}
		if err := c.BodyParser(&req); err == nil {
			interfaceName = req.Interface
		}
	}

	if interfaceName == "" {
		interfaceName = detectWiFiInterface()
	}
	if interfaceName == "" {
		interfaceName = constants.DefaultWiFiInterface
	}

	result := ScanWiFiNetworks(interfaceName)
	if networks, ok := result["networks"]; ok {
		return c.JSON(networks)
	}
	return c.JSON([]fiber.Map{})
}

