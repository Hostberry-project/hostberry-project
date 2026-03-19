package wifi

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"hostberry/internal/constants"
	"hostberry/internal/i18n"
	"hostberry/internal/utils"
)

var interfaceNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,32}$`)

func validateInterfaceName(interfaceName string) error {
	interfaceName = strings.TrimSpace(interfaceName)
	if interfaceName == "" {
		return fmt.Errorf("interfaz WiFi requerida")
	}
	if len(interfaceName) > 32 {
		return fmt.Errorf("nombre de interfaz inválido")
	}
	if !interfaceNameRegex.MatchString(interfaceName) {
		return fmt.Errorf("nombre de interfaz inválido")
	}
	return nil
}

func runSudoSilently(args ...string) error {
	cmd := exec.Command("sudo", args...)
	cmd.Env = append(os.Environ(), "SUDO_ASKPASS=/bin/false")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run()
}

// ScanWiFiNetworks escanea redes WiFi con iw y devuelve un mapa con "success", "networks" y "error".
func ScanWiFiNetworks(interfaceName string) map[string]interface{} {
	result := make(map[string]interface{})
	networks := []map[string]interface{}{}
	if interfaceName == "" {
		interfaceName = constants.DefaultWiFiInterface
	}

	if err := validateInterfaceName(interfaceName); err != nil {
		result["success"] = false
		result["error"] = "Nombre de interfaz inválido"
		result["networks"] = networks
		return result
	}

	_ = runSudoSilently("ip", "link", "set", interfaceName, "up")
	time.Sleep(1 * time.Second)

	scanCmd := exec.Command("sudo", "iw", "dev", interfaceName, "scan")
	scanCmd.Env = append(os.Environ(), "SUDO_ASKPASS=/bin/false")
	scanCmd.Stderr = io.Discard
	scanOut, err := scanCmd.Output()
	if err != nil {
		i18n.LogTf("logs.wifi_scan_error", err)
		result["success"] = false
		result["error"] = fmt.Sprintf("Error escaneando redes: %v", err)
		result["networks"] = networks
		return result
	}

	lines := strings.Split(string(scanOut), "\n")
	currentNetwork := make(map[string]interface{})
	seenNetworks := make(map[string]bool)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "BSS ") {
			if len(currentNetwork) > 0 {
				if ssid, ok := currentNetwork["ssid"].(string); ok && ssid != "" {
					if !seenNetworks[ssid] {
						seenNetworks[ssid] = true
						networks = append(networks, currentNetwork)
					}
				}
			}
			currentNetwork = make(map[string]interface{})
			currentNetwork["security"] = "Open"
			currentNetwork["signal"] = 0
		} else if strings.HasPrefix(line, "SSID:") {
			ssid := strings.TrimSpace(strings.TrimPrefix(line, "SSID:"))
			if ssid != "" {
				currentNetwork["ssid"] = ssid
			}
		} else if strings.Contains(line, "signal:") {
			re := regexp.MustCompile(`signal:\s*(-?\d+\.?\d*)\s*dBm?`)
			if matches := re.FindStringSubmatch(line); len(matches) > 1 {
				if n, e := parseFloat(matches[1]); e == nil {
					if n > 0 {
						n = -n
					}
					if n >= -100 && n <= -30 {
						currentNetwork["signal"] = int(n)
					}
				}
			}
		} else if strings.Contains(line, "freq:") {
			re := regexp.MustCompile(`freq:\s*(\d+)`)
			if matches := re.FindStringSubmatch(line); len(matches) > 1 {
				if freq, e := parseInt(matches[1]); e == nil {
					channel := freqToChannel(freq)
					if channel > 0 {
						currentNetwork["channel"] = channel
					}
				}
			}
		} else if strings.Contains(line, "RSN:") {
			if strings.Contains(line, "WPA3") || strings.Contains(line, "SAE") {
				currentNetwork["security"] = "WPA3"
			} else {
				currentNetwork["security"] = "WPA2"
			}
		} else if strings.Contains(line, "WPA:") {
			currentNetwork["security"] = "WPA2"
		}
	}

	if len(currentNetwork) > 0 {
		if ssid, ok := currentNetwork["ssid"].(string); ok && ssid != "" && !seenNetworks[ssid] {
			networks = append(networks, currentNetwork)
		}
	}

	result["success"] = true
	result["networks"] = networks
	result["count"] = len(networks)
	return result
}

func parseFloat(s string) (float64, error) {
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	return f, err
}

func parseInt(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}

func freqToChannel(freq int) int {
	if freq >= 2412 && freq <= 2484 {
		return (freq-2412)/5 + 1
	}
	if freq >= 5000 && freq <= 5825 {
		return (freq - 5000) / 5
	}
	if freq >= 5955 && freq <= 7115 {
		return (freq - 5955) / 5
	}
	return 0
}

// ToggleWiFi habilita o deshabilita la interfaz WiFi (rfkill + ip link).
func ToggleWiFi(interfaceName string, enable bool) map[string]interface{} {
	result := make(map[string]interface{})
	if interfaceName == "" {
		interfaceName = constants.DefaultWiFiInterface
	}

	if err := validateInterfaceName(interfaceName); err != nil {
		result["success"] = false
		result["error"] = "Nombre de interfaz inválido"
		result["enabled"] = false
		return result
	}

	if enable {
		if err := runSudoSilently("rfkill", "unblock", "wifi"); err != nil {
			result["success"] = false
			result["error"] = "No se pudo desbloquear WiFi (rfkill)"
			result["enabled"] = false
			return result
		}
		if err := runSudoSilently("ip", "link", "set", interfaceName, "up"); err != nil {
			result["success"] = false
			result["error"] = "No se pudo activar la interfaz WiFi (ip link)"
			result["enabled"] = false
			return result
		}

		result["success"] = true
		result["message"] = "WiFi habilitado"
		result["enabled"] = true
	} else {
		if err := runSudoSilently("rfkill", "block", "wifi"); err != nil {
			result["success"] = false
			result["error"] = "No se pudo bloquear WiFi (rfkill)"
			result["enabled"] = false
			return result
		}
		if err := runSudoSilently("ip", "link", "set", interfaceName, "down"); err != nil {
			result["success"] = false
			result["error"] = "No se pudo desactivar la interfaz WiFi (ip link)"
			result["enabled"] = false
			return result
		}

		result["success"] = true
		result["message"] = "WiFi deshabilitado"
		result["enabled"] = false
	}
	return result
}

// ConnectWiFi conecta a una red WiFi; usa helpers de wpa_supplicant (startWpaSupplicant, waitForWpaCliConnection, etc.).
func ConnectWiFi(ssid, password, interfaceName, country, user string) map[string]interface{} {
	result := make(map[string]interface{})
	result["success"] = false
	result["error"] = ""
	if ssid == "" {
		result["error"] = "SSID requerido"
		return result
	}
	if interfaceName == "" {
		interfaceName = constants.DefaultWiFiInterface
	}

	if err := validateInterfaceName(interfaceName); err != nil {
		result["error"] = "Nombre de interfaz inválido"
		return result
	}

	_ = runSudoSilently("rfkill", "unblock", "wifi")
	_ = runSudoSilently("ip", "link", "set", interfaceName, "up")

	safeSSID := regexp.MustCompile(`[^a-zA-Z0-9_-]`).ReplaceAllString(ssid, "_")
	wpaConfigPath := fmt.Sprintf("%s/wpa_supplicant-%s.conf", WpaSupplicantConfigDir, safeSSID)

	// Crear directorio de config si no existe
	if _, err := os.Stat(WpaSupplicantConfigDir); os.IsNotExist(err) {
		utils.ExecuteCommand(fmt.Sprintf("sudo mkdir -p %s 2>/dev/null || true", WpaSupplicantConfigDir))
		utils.ExecuteCommand(fmt.Sprintf("sudo chmod 755 %s 2>/dev/null || true", WpaSupplicantConfigDir))
		utils.ExecuteCommand(fmt.Sprintf("sudo chown root:netdev %s 2>/dev/null || true", WpaSupplicantConfigDir))
	}

	var networkBlock string

	// Detectar tipo de seguridad (WPA3 vs WPA2) a partir del escaneo interno
	securityType := ""
	if password != "" {
		if scanRes := ScanWiFiNetworks(interfaceName); scanRes != nil {
			if nets, ok := scanRes["networks"].([]map[string]interface{}); ok {
				for _, net := range nets {
					if v, ok := net["ssid"].(string); ok && v == ssid {
						if sec, ok := net["security"].(string); ok {
							securityType = sec
						}
						break
					}
				}
			}
		}
	}

	escape := func(s string) string {
		s = strings.ReplaceAll(s, "\\", "\\\\")
		return strings.ReplaceAll(s, "\"", "\\\"")
	}

	if password != "" && strings.EqualFold(securityType, "WPA3") {
		// WPA3 Personal (SAE)
		networkBlock = fmt.Sprintf("network={\n\tssid=\"%s\"\n\tkey_mgmt=SAE\n\tsae_password=\"%s\"\n}", escape(ssid), escape(password))
	} else if password != "" {
		// WPA2/PSK
		cmd := exec.Command("wpa_passphrase", ssid, password)
		cmd.Env = append(os.Environ(), "LANG=C")
		out, err := cmd.Output()
		if err != nil || !strings.Contains(string(out), "network=") {
			result["error"] = "Error al generar la clave PSK. Verifica el SSID y la contraseña."
			return result
		}
		networkBlock = strings.TrimSpace(string(out))
	} else {
		networkBlock = fmt.Sprintf("network={\n\tssid=\"%s\"\n\tkey_mgmt=NONE\n}", strings.ReplaceAll(ssid, "\\", "\\\\"))
	}

	runDir := "/var/run/wpa_supplicant"
	if _, err := os.Stat("/var/run/wpa_supplicant"); os.IsNotExist(err) {
		runDir = "/run/wpa_supplicant"
	}

	utils.ExecuteCommand(fmt.Sprintf("sudo mkdir -p %s 2>/dev/null || true", runDir))
	utils.ExecuteCommand(fmt.Sprintf("sudo chmod 775 %s 2>/dev/null || true", runDir))
	utils.ExecuteCommand(fmt.Sprintf("sudo chown root:netdev %s 2>/dev/null || true", runDir))
	activeRunDir = runDir

	configContent := fmt.Sprintf("ctrl_interface=DIR=%s GROUP=netdev\nupdate_config=1\ncountry=%s\n\n%s", runDir, country, networkBlock)
	tmpPath := fmt.Sprintf("/tmp/wpa_supplicant_%s_%d.conf", safeSSID, time.Now().Unix())
	if err := os.WriteFile(tmpPath, []byte(configContent), 0600); err != nil {
		result["error"] = fmt.Sprintf("Error escribiendo config: %v", err)
		return result
	}
	defer os.Remove(tmpPath)

	utils.ExecuteCommand(fmt.Sprintf("sudo cp %s %s 2>/dev/null || true", tmpPath, wpaConfigPath))
	utils.ExecuteCommand(fmt.Sprintf("sudo chmod 600 %s 2>/dev/null || true", wpaConfigPath))
	utils.ExecuteCommand(fmt.Sprintf("sudo chown root:root %s 2>/dev/null || true", wpaConfigPath))

	stopWpaSupplicant(interfaceName)
	time.Sleep(1 * time.Second)

	if err := startWpaSupplicant(interfaceName, wpaConfigPath, runDir); err != nil {
		result["error"] = err.Error()
		return result
	}

	socketDir, err := waitForWpaCliConnection(interfaceName, 10)
	if err != nil {
		result["error"] = "wpa_cli no puede comunicarse con wpa_supplicant. Verifica permisos del socket."
		return result
	}

	runWpaCli := func(args ...string) (string, error) {
		base := []string{"wpa_cli", "-i", interfaceName, "-p", socketDir}
		cmd := exec.Command("sudo", append(base, args...)...)
		out, err := cmd.CombinedOutput()
		return strings.TrimSpace(string(out)), err
	}

	runWpaCli("list_networks")
	netIDOut, netIDErr := runWpaCli("add_network")
	if netIDErr != nil || netIDOut == "" {
		result["error"] = fmt.Sprintf("Error agregando red: %v", netIDErr)
		return result
	}

	netID := strings.TrimSpace(netIDOut)
	if _, err := runWpaCli("set_network", netID, "ssid", fmt.Sprintf("\"%s\"", escape(ssid))); err != nil {
		result["error"] = "Error configurando SSID"
		return result
	}

	if password != "" {
		if _, err := runWpaCli("set_network", netID, "psk", fmt.Sprintf("\"%s\"", escape(password))); err != nil {
			result["error"] = "Error configurando PSK"
			return result
		}
	} else {
		runWpaCli("set_network", netID, "key_mgmt", "NONE")
	}

	runWpaCli("enable_network", netID)
	runWpaCli("select_network", netID)
	runWpaCli("reconnect")

	// Esperar conexión
	for i := 0; i < 25; i++ {
		time.Sleep(1 * time.Second)
		statusOut, _ := runWpaCli("status")
		if strings.Contains(statusOut, "wpa_state=COMPLETED") {
			result["success"] = true
			result["message"] = "Conectado"
			return result
		}
	}

	// Diferenciar entre contraseña incorrecta y otros problemas
	if statusOut, _ := runWpaCli("status"); statusOut != "" {
		if strings.Contains(statusOut, "AUTH_FAILED") || strings.Contains(statusOut, "WRONG_KEY") {
			result["error"] = "La contraseña WiFi parece incorrecta. Comprueba la clave e inténtalo de nuevo."
			return result
		}
		if strings.Contains(statusOut, "4WAY_HANDSHAKE") {
			result["error"] = "No se pudo completar la autenticación WPA. Verifica la contraseña y el tipo de seguridad (WPA2/WPA3)."
			return result
		}
	}

	result["error"] = "Tiempo de espera agotado. Comprueba la contraseña y la cobertura de la red e inténtalo de nuevo."
	return result
}

// AutoConnectToLastNetwork intenta conectarse automáticamente a la última red WiFi conectada
func AutoConnectToLastNetwork(interfaceName string) {
	if interfaceName == "" {
		interfaceName = constants.DefaultWiFiInterface
	}

	i18n.LogTf("logs.wifi_auto_connect_start", interfaceName)

	cmd := exec.Command("sh", "-c", fmt.Sprintf("ip link show %s 2>/dev/null", interfaceName))
	if err := cmd.Run(); err != nil {
		i18n.LogTf("logs.wifi_interface_not_exists", interfaceName)
		return
	}

	// Activar switch y WiFi
	i18n.LogT("logs.wifi_activating")
	utils.ExecuteCommand("sudo rfkill unblock wifi 2>/dev/null || true")
	utils.ExecuteCommand(fmt.Sprintf("sudo ip link set %s up 2>/dev/null || true", interfaceName))
	time.Sleep(1 * time.Second)

	// Intentar reconectar con wpa_cli si wpa_supplicant está corriendo
	i18n.LogT("logs.wifi_searching_network")

	socketDirs := WpaSocketDirs
	var workingSocketDir string
	var useGlobalSocket bool

	for _, dir := range socketDirs {
		socketPath := fmt.Sprintf("%s/%s", dir, interfaceName)
		if _, err := os.Stat(socketPath); err == nil {
			workingSocketDir = dir
			i18n.LogTf("logs.wifi_socket_interface_found", socketPath)
			break
		}
	}

	if workingSocketDir == "" {
		for _, dir := range socketDirs {
			globalSocket := fmt.Sprintf("%s/global", dir)
			if _, err := os.Stat(globalSocket); err == nil {
				workingSocketDir = dir
				useGlobalSocket = true
				i18n.LogTf("logs.wifi_socket_global_found", globalSocket)
				break
			}
		}
	}

	if workingSocketDir != "" {
		i18n.LogT("logs.wifi_wpa_running")

		runWpaCli := func(args ...string) (string, error) {
			var base []string
			if useGlobalSocket {
				base = []string{"wpa_cli", "-g", workingSocketDir, "-i", interfaceName}
			} else {
				base = []string{"wpa_cli", "-i", interfaceName, "-p", workingSocketDir}
			}
			cmd := exec.Command("sudo", append(base, args...)...)
			out, err := cmd.CombinedOutput()
			return strings.TrimSpace(string(out)), err
		}

		statusOut, err := runWpaCli("status")
		if err == nil {
			if strings.Contains(statusOut, "wpa_state=COMPLETED") {
				i18n.LogT("logs.wifi_already_connected")
				go func() {
					ipCmd := exec.Command("sh", "-c", fmt.Sprintf("ip addr show %s 2>/dev/null | grep 'inet ' | awk '{print $2}' | cut -d/ -f1 | head -1", interfaceName))
					if ipOut, _ := ipCmd.Output(); strings.TrimSpace(string(ipOut)) == "" {
						utils.ExecuteCommand(fmt.Sprintf("sudo dhclient -v %s 2>&1 || sudo udhcpc -i %s -q -n 2>&1 || true", interfaceName, interfaceName))
					}
				}()
				return
			}

			listOut, _ := runWpaCli("list_networks")
			if listOut != "" {
				lines := strings.Split(listOut, "\n")
				if len(lines) > 1 {
					var netID string
					for i, line := range lines {
						if i == 0 {
							continue
						}
						fields := strings.Fields(line)
						if len(fields) >= 1 {
							if len(fields) >= 4 && fields[3] == "[CURRENT]" {
								netID = fields[0]
								break
							} else if netID == "" && len(fields) >= 2 {
								netID = fields[0]
							}
						}
					}

					if netID != "" {
						i18n.LogTf("logs.wifi_reconnecting", netID)
						runWpaCli("enable_network", netID)
						runWpaCli("select_network", netID)
						runWpaCli("reconnect")

						for attempt := 0; attempt < 5; attempt++ {
							time.Sleep(1 * time.Second)
							statusOut2, _ := runWpaCli("status")
							if strings.Contains(statusOut2, "wpa_state=COMPLETED") {
								i18n.LogT("logs.wifi_reconnected")
								go func() {
									utils.ExecuteCommand(fmt.Sprintf("sudo dhclient -v %s 2>&1 || sudo udhcpc -i %s -q -n 2>&1 || true", interfaceName, interfaceName))
								}()
								return
							}
						}
						i18n.LogT("logs.wifi_reconnect_started")
					}
				}
			}
		}
	} else {
		i18n.LogT("logs.wifi_wpa_not_running")
	}

	// Si no hay wpa_supplicant corriendo o no se pudo reconectar,
	// buscar el último archivo de configuración y conectarse usando ConnectWiFi
	i18n.LogT("logs.wifi_searching_config")
	ssid, _, err := getLastConnectedNetwork(interfaceName)
	if err != nil {
		i18n.LogTf("logs.wifi_config_not_found", err)
		i18n.LogT("logs.wifi_trying_other_way")

		// Último recurso: buscar cualquier config reciente
		configDirs := []string{WpaSupplicantConfigDir, WpaSupplicantAltConfigDir}
		for _, configDir := range configDirs {
			configFiles, err := os.ReadDir(configDir)
			if err != nil {
				continue
			}
			var lastFile os.DirEntry
			var lastTime time.Time
			for _, file := range configFiles {
				if strings.HasPrefix(file.Name(), "wpa_supplicant-") && strings.HasSuffix(file.Name(), ".conf") {
					info, err := file.Info()
					if err == nil && info.ModTime().After(lastTime) {
						lastTime = info.ModTime()
						lastFile = file
					}
				}
			}
			if lastFile != nil {
				configPath := fmt.Sprintf("%s/%s", configDir, lastFile.Name())
				cmd := exec.Command("sudo", "cat", configPath)
				cmd.Env = append(os.Environ(), "SUDO_ASKPASS=/bin/false")
				contentBytes, err := cmd.CombinedOutput()
				if err != nil {
					continue
				}
				lines := strings.Split(string(contentBytes), "\n")
				for _, line := range lines {
					if strings.HasPrefix(strings.TrimSpace(line), "ssid=") {
						ssid = strings.Trim(strings.TrimPrefix(strings.TrimSpace(line), "ssid="), "\"")
						if ssid != "" {
							i18n.LogTf("logs.wifi_network_found_file", lastFile.Name(), ssid)
							break
						}
					}
				}
				if ssid != "" {
					break
				}
			}
		}

		if ssid == "" {
			i18n.LogT("logs.wifi_no_network_found")
			return
		}
	}

	safeSSID := regexp.MustCompile(`[^a-zA-Z0-9_-]`).ReplaceAllString(ssid, "_")
	wpaConfigPath := fmt.Sprintf("%s/wpa_supplicant-%s.conf", WpaSupplicantConfigDir, safeSSID)

	if _, err := os.Stat(wpaConfigPath); os.IsNotExist(err) {
		wpaConfigPath = fmt.Sprintf("%s/wpa_supplicant-%s.conf", WpaSupplicantAltConfigDir, safeSSID)
		if _, err := os.Stat(wpaConfigPath); os.IsNotExist(err) {
			configDirs := []string{WpaSupplicantConfigDir, WpaSupplicantAltConfigDir}
			found := false
			for _, configDir := range configDirs {
				configFiles, err := os.ReadDir(configDir)
				if err != nil {
					continue
				}
				for _, file := range configFiles {
					if strings.Contains(file.Name(), safeSSID) && strings.HasSuffix(file.Name(), ".conf") {
						wpaConfigPath = fmt.Sprintf("%s/%s", configDir, file.Name())
						cmd := exec.Command("sudo", "test", "-r", wpaConfigPath)
						cmd.Env = append(os.Environ(), "SUDO_ASKPASS=/bin/false")
						if err := cmd.Run(); err == nil {
							found = true
							i18n.LogTf("logs.wifi_config_file_found", wpaConfigPath)
							break
						}
					}
				}
				if found {
					break
				}
			}
			if !found {
				country := constants.DefaultCountryCode
				result := ConnectWiFi(ssid, "", interfaceName, country, "system")
				if success, ok := result["success"].(bool); ok && success {
					i18n.LogTf("logs.wifi_auto_success", ssid)
					return
				}
				errorMsg := "Error desconocido"
				if errStr, ok := result["error"].(string); ok && errStr != "" {
					errorMsg = errStr
				}
				i18n.LogTf("logs.wifi_auto_error", errorMsg)
				return
			}
		}
	}

	stopWpaSupplicant(interfaceName)
	time.Sleep(1 * time.Second)

	runDir := getRunDir()
	i18n.LogT("logs.wifi_starting_wpa")
	if err := startWpaSupplicant(interfaceName, wpaConfigPath, runDir); err != nil {
		i18n.LogTf("logs.wifi_wpa_start_error", err)
		i18n.LogT("logs.wifi_trying_connect")
		country := constants.DefaultCountryCode
		result := ConnectWiFi(ssid, "", interfaceName, country, "system")
		if success, ok := result["success"].(bool); ok && success {
			i18n.LogTf("logs.wifi_auto_success", ssid)
		} else {
			errStr, _ := result["error"].(string)
			i18n.LogTf("logs.wifi_auto_error", errStr)
		}
		return
	}

	time.Sleep(2 * time.Second)

	socketDir, err := waitForWpaCliConnection(interfaceName, 5)
	if err != nil {
		return
	}

	runWpaCli := func(args ...string) (string, error) {
		base := []string{"wpa_cli", "-i", interfaceName, "-p", socketDir}
		cmd := exec.Command("sudo", append(base, args...)...)
		out, err := cmd.CombinedOutput()
		return strings.TrimSpace(string(out)), err
	}

	i18n.LogT("logs.wifi_enabling_network")
	runWpaCli("enable_network", "0")
	runWpaCli("select_network", "0")
	runWpaCli("reconnect")

	i18n.LogT("logs.wifi_waiting_auth")
	connected := false
	for attempt := 0; attempt < 8; attempt++ {
		time.Sleep(1 * time.Second)
		statusOut, _ := runWpaCli("status")
		if strings.Contains(statusOut, "wpa_state=COMPLETED") {
			connected = true
			i18n.LogTf("logs.wifi_authenticated", ssid)
			break
		}
		if attempt%3 == 0 && attempt > 0 {
			i18n.LogTf("logs.wifi_status_attempt3", statusOut, attempt+1)
		}
	}

	if !connected {
		log.Printf("⚠️  No se pudo autenticar después de 8 segundos. Estado final:")
		statusOut, _ := runWpaCli("status")
		log.Printf("Estado: %s", statusOut)
	}

	log.Printf("📡 Solicitando IP con DHCP...")
	go func() {
		utils.ExecuteCommand(fmt.Sprintf("sudo pkill -f 'dhclient.*%s|udhcpc.*%s' 2>/dev/null || true", interfaceName, interfaceName))
		time.Sleep(300 * time.Millisecond)
		utils.ExecuteCommand(fmt.Sprintf("sudo dhclient -v %s 2>&1 || sudo udhcpc -i %s -q -n 2>&1 || true", interfaceName, interfaceName))
	}()

	var ip string
	for ipAttempt := 0; ipAttempt < 5; ipAttempt++ {
		time.Sleep(1 * time.Second)
		ipCmd := exec.Command("sh", "-c", fmt.Sprintf("ip addr show %s 2>/dev/null | grep 'inet ' | awk '{print $2}' | cut -d/ -f1 | head -1", interfaceName))
		ipOut, _ := ipCmd.Output()
		ip = strings.TrimSpace(string(ipOut))
		if ip != "" && ip != "N/A" && !strings.HasPrefix(ip, "169.254") {
			log.Printf("✅✅ Autoconexión completa: %s (IP: %s)", ssid, ip)
			return
		}
	}

	if connected {
		log.Printf("✅ Autoconexión exitosa a %s (IP se asignará en segundo plano)", ssid)
	} else {
		log.Printf("⚠️  Autoconexión iniciada, puede tardar unos segundos más en completarse")
	}
}

