package wifi

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gofiber/fiber/v2"
	"hostberry/internal/constants"
)

func listWiFiInterfacesFromSys() []string {
	entries, err := os.ReadDir("/sys/class/net")
	if err != nil {
		return nil
	}

	var ifaces []string
	for _, e := range entries {
		if !e.IsDir() && e.Type().IsRegular() {
			// /sys/class/net/<iface> es un directorio, pero dejamos este guard por robustez.
			continue
		}

		name := e.Name()
		if strings.HasPrefix(name, "wlan") || strings.HasPrefix(name, "wl") {
			if validateInterfaceName(name) == nil {
				ifaces = append(ifaces, name)
			}
		}
	}

	sort.Strings(ifaces)
	return ifaces
}

func detectWiFiInterface() string {
	ifaces := listWiFiInterfacesFromSys()
	if len(ifaces) > 0 {
		return ifaces[0]
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
	for _, ifaceName := range listWiFiInterfacesFromSys() {
		operstatePath := filepath.Join("/sys/class/net", ifaceName, "operstate")
		b, err := os.ReadFile(operstatePath)
		state := "unknown"
		if err == nil {
			if s := strings.TrimSpace(string(b)); s != "" {
				state = s
			}
		}

		interfaces = append(interfaces, fiber.Map{
			"name":  ifaceName,
			"type":  "wifi",
			"state": state,
		})
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

	if err := validateInterfaceName(interfaceName); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Nombre de interfaz inválido"})
	}

	result := ScanWiFiNetworks(interfaceName)
	if success, ok := result["success"].(bool); ok && !success {
		if errMsg, ok := result["error"].(string); ok && errMsg != "" {
			return c.Status(500).JSON(fiber.Map{"success": false, "error": errMsg})
		}
		return c.Status(500).JSON(fiber.Map{"success": false, "error": "Error escaneando redes"})
	}

	if networks, ok := result["networks"]; ok {
		return c.JSON(networks)
	}
	return c.JSON([]fiber.Map{})
}

