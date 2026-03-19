package server

import (
	"fmt"
	"os/exec"
	"time"

	"hostberry/internal/i18n"
	wifiHandlers "hostberry/internal/wifi"
)

// StartWiFiAutoConnect intenta auto-conectar a la última red WiFi conocida.
// Se ejecuta en background para no bloquear el arranque del servidor.
func StartWiFiAutoConnect() {
	go func() {
		i18n.LogTf("logs.wifi_auto_wait")
		time.Sleep(5 * time.Second)

		// Intentar detectar interfaz (menos intentos, más rápido)
		var interfaceName string
		for attempt := 0; attempt < 3; attempt++ {
			interfaceName = wifiHandlers.DetectWiFiInterface()
			if interfaceName != "" {
				// Verificar que la interfaz realmente existe
				cmd := exec.Command("sh", "-c", fmt.Sprintf("ip link show %s 2>/dev/null", interfaceName))
				if err := cmd.Run(); err == nil {
					i18n.LogTf("logs.wifi_interface_detected", interfaceName)
					break
				}
			}
			if attempt < 2 {
				i18n.LogTf("logs.wifi_interface_wait", attempt+1)
				time.Sleep(2 * time.Second)
			}
		}

		if interfaceName != "" {
			i18n.LogTf("logs.wifi_auto_start", interfaceName)
			wifiHandlers.AutoConnectToLastNetwork(interfaceName)
		} else {
			i18n.LogT("logs.wifi_interface_not_found")
		}
	}()
}

