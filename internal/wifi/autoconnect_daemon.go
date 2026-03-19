package wifi

import (
	"os/exec"
	"time"

	"hostberry/internal/i18n"
)

// StartWiFiAutoConnectDaemon lanza en background el auto-setup/conexión WiFi.
func StartWiFiAutoConnectDaemon() {
	go func() {
		i18n.LogTf("logs.wifi_auto_wait")
		time.Sleep(5 * time.Second)

		var interfaceName string
		for attempt := 0; attempt < 3; attempt++ {
			interfaceName = DetectWiFiInterface()
			if interfaceName != "" {
				// Verificar que la interfaz realmente existe (evita sh -c).
				if err := exec.Command("ip", "link", "show", interfaceName).Run(); err == nil {
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
			AutoConnectToLastNetwork(interfaceName)
		} else {
			i18n.LogT("logs.wifi_interface_not_found")
		}
	}()
}

