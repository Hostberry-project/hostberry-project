package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"hostberry/internal/constants"
)

// scanWiFiNetworks escanea redes WiFi con iw y devuelve un mapa con "success", "networks" y "error".
func scanWiFiNetworks(interfaceName string) map[string]interface{} {
	result := make(map[string]interface{})
	networks := []map[string]interface{}{}
	if interfaceName == "" {
		interfaceName = constants.DefaultWiFiInterface
	}
	executeCommand(fmt.Sprintf("sudo ip link set %s up 2>/dev/null || true", interfaceName))
	time.Sleep(1 * time.Second)
	scanCmd := exec.Command("sh", "-c", fmt.Sprintf("sudo iw dev %s scan 2>/dev/null", interfaceName))
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

// toggleWiFi habilita o deshabilita la interfaz WiFi (rfkill + ip link).
func toggleWiFi(interfaceName string, enable bool) map[string]interface{} {
	result := make(map[string]interface{})
	if interfaceName == "" {
		interfaceName = constants.DefaultWiFiInterface
	}
	if enable {
		executeCommand("sudo rfkill unblock wifi 2>/dev/null || true")
		executeCommand(fmt.Sprintf("sudo ip link set %s up 2>/dev/null || true", interfaceName))
		result["success"] = true
		result["message"] = "WiFi habilitado"
		result["enabled"] = true
	} else {
		executeCommand("sudo rfkill block wifi 2>/dev/null || true")
		executeCommand(fmt.Sprintf("sudo ip link set %s down 2>/dev/null || true", interfaceName))
		result["success"] = true
		result["message"] = "WiFi deshabilitado"
		result["enabled"] = false
	}
	return result
}

// connectWiFi conecta a una red WiFi; usa helpers de wifi_helpers (startWpaSupplicant, waitForWpaCliConnection, etc.).
func connectWiFi(ssid, password, interfaceName, country, user string) map[string]interface{} {
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

	// Asegurar que la interfaz está levantada y el WiFi desbloqueado antes de iniciar wpa_supplicant.
	executeCommand("sudo rfkill unblock wifi 2>/dev/null || true")
	executeCommand(fmt.Sprintf("sudo ip link set %s up 2>/dev/null || true", interfaceName))

	safeSSID := regexp.MustCompile(`[^a-zA-Z0-9_-]`).ReplaceAllString(ssid, "_")
	wpaConfigPath := fmt.Sprintf("%s/wpa_supplicant-%s.conf", WpaSupplicantConfigDir, safeSSID)

	// Crear directorio de config si no existe
	if _, err := os.Stat(WpaSupplicantConfigDir); os.IsNotExist(err) {
		executeCommand(fmt.Sprintf("sudo mkdir -p %s 2>/dev/null || true", WpaSupplicantConfigDir))
		executeCommand(fmt.Sprintf("sudo chmod 755 %s 2>/dev/null || true", WpaSupplicantConfigDir))
		executeCommand(fmt.Sprintf("sudo chown root:netdev %s 2>/dev/null || true", WpaSupplicantConfigDir))
	}

	var networkBlock string
	// Detectar tipo de seguridad (WPA3 vs WPA2) a partir del escaneo interno
	securityType := ""
	if password != "" {
		if scanRes := scanWiFiNetworks(interfaceName); scanRes != nil {
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
		// WPA2/PSK (o tipo desconocido): delegar en wpa_passphrase
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
	executeCommand(fmt.Sprintf("sudo mkdir -p %s 2>/dev/null || true", runDir))
	executeCommand(fmt.Sprintf("sudo chmod 775 %s 2>/dev/null || true", runDir))
	executeCommand(fmt.Sprintf("sudo chown root:netdev %s 2>/dev/null || true", runDir))
	activeRunDir = runDir

	configContent := fmt.Sprintf("ctrl_interface=DIR=%s GROUP=netdev\nupdate_config=1\ncountry=%s\n\n%s", runDir, country, networkBlock)
	tmpPath := fmt.Sprintf("/tmp/wpa_supplicant_%s_%d.conf", safeSSID, time.Now().Unix())
	if err := os.WriteFile(tmpPath, []byte(configContent), 0600); err != nil {
		result["error"] = fmt.Sprintf("Error escribiendo config: %v", err)
		return result
	}
	defer os.Remove(tmpPath)
	executeCommand(fmt.Sprintf("sudo cp %s %s 2>/dev/null || true", tmpPath, wpaConfigPath))
	executeCommand(fmt.Sprintf("sudo chmod 600 %s 2>/dev/null || true", wpaConfigPath))
	executeCommand(fmt.Sprintf("sudo chown root:root %s 2>/dev/null || true", wpaConfigPath))

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

	// Intentar diferenciar entre contraseña incorrecta y otros problemas
	if statusOut, _ := runWpaCli("status"); statusOut != "" {
		// Algunos drivers indican fallo de autenticación explícito
		if strings.Contains(statusOut, "AUTH_FAILED") || strings.Contains(statusOut, "WRONG_KEY") {
			result["error"] = "La contraseña WiFi parece incorrecta. Comprueba la clave e inténtalo de nuevo."
			return result
		}
		// Si estamos repetidamente en 4-Way Handshake sin completar, suele ser problema de clave
		if strings.Contains(statusOut, "4WAY_HANDSHAKE") {
			result["error"] = "No se pudo completar la autenticación WPA. Verifica la contraseña y el tipo de seguridad (WPA2/WPA3)."
			return result
		}
	}

	result["error"] = "Tiempo de espera agotado. Comprueba la contraseña y la cobertura de la red e inténtalo de nuevo."
	return result
}

// autoConnectToLastNetwork intenta conectarse automáticamente a la última red WiFi conectada
func autoConnectToLastNetwork(interfaceName string) {
	if interfaceName == "" {
		interfaceName = constants.DefaultWiFiInterface
	}

	i18n.LogTf("logs.wifi_auto_connect_start", interfaceName)

	// Verificar que la interfaz existe antes de continuar
	cmd := exec.Command("sh", "-c", fmt.Sprintf("ip link show %s 2>/dev/null", interfaceName))
	if err := cmd.Run(); err != nil {
		i18n.LogTf("logs.wifi_interface_not_exists", interfaceName)
		return
	}

	// Paso 1: Activar software switch y WiFi en paralelo (más rápido)
	LogT("logs.wifi_activating")
	executeCommand("sudo rfkill unblock wifi 2>/dev/null || true")
	executeCommand(fmt.Sprintf("sudo ip link set %s up 2>/dev/null || true", interfaceName))
	time.Sleep(1 * time.Second) // Reducido de 3 a 1 segundo

	// Paso 3: Intentar obtener la última red desde wpa_cli (más confiable)
	LogT("logs.wifi_searching_network")
	
	// Primero intentar con wpa_cli si wpa_supplicant está corriendo
	socketDirs := WpaSocketDirs
	var workingSocketDir string
	var useGlobalSocket bool

	// Buscar socket de interfaz específica
	for _, dir := range socketDirs {
		socketPath := fmt.Sprintf("%s/%s", dir, interfaceName)
		if _, err := os.Stat(socketPath); err == nil {
			workingSocketDir = dir
			i18n.LogTf("logs.wifi_socket_interface_found", socketPath)
			break
		}
	}
	
	// Si no hay socket de interfaz, intentar con socket global
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
		// wpa_supplicant está corriendo, intentar reconectar a la red activa (método más rápido)
		LogT("logs.wifi_wpa_running")
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

		// Obtener estado actual (verificación rápida)
		statusOut, err := runWpaCli("status")
		if err == nil {
			// Verificar si ya está conectado (verificación inmediata)
			if strings.Contains(statusOut, "wpa_state=COMPLETED") {
				LogT("logs.wifi_already_connected")
				// Iniciar DHCP en segundo plano si no tiene IP
				go func() {
					ipCmd := exec.Command("sh", "-c", fmt.Sprintf("ip addr show %s 2>/dev/null | grep 'inet ' | awk '{print $2}' | cut -d/ -f1 | head -1", interfaceName))
					if ipOut, _ := ipCmd.Output(); ipOut == nil || strings.TrimSpace(string(ipOut)) == "" {
						executeCommand(fmt.Sprintf("sudo dhclient -v %s 2>&1 || sudo udhcpc -i %s -q -n 2>&1 || true", interfaceName, interfaceName))
					}
				}()
				return
			}

			// Reconexión rápida: habilitar y reconectar directamente
			listOut, _ := runWpaCli("list_networks")
			if listOut != "" {
				lines := strings.Split(listOut, "\n")
				if len(lines) > 1 {
					// Buscar red CURRENT o primera habilitada
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
						// Hacer todo en secuencia rápida
						runWpaCli("enable_network", netID)
						runWpaCli("select_network", netID)
						runWpaCli("reconnect")
						
						// Verificar conexión más rápido (menos espera)
						for attempt := 0; attempt < 5; attempt++ {
							time.Sleep(1 * time.Second) // Reducido de 2 a 1 segundo
							statusOut2, _ := runWpaCli("status")
							if strings.Contains(statusOut2, "wpa_state=COMPLETED") {
								LogT("logs.wifi_reconnected")
								// Iniciar DHCP en segundo plano
								go func() {
									executeCommand(fmt.Sprintf("sudo dhclient -v %s 2>&1 || sudo udhcpc -i %s -q -n 2>&1 || true", interfaceName, interfaceName))
								}()
								return
							}
						}
						LogT("logs.wifi_reconnect_started")
					}
				}
			}
		} else {
			// Error obteniendo estado - no traducir, es debug interno
		}
	} else {
		LogT("logs.wifi_wpa_not_running")
	}

	// Paso 4: Si no hay wpa_supplicant corriendo o no se pudo reconectar,
	// buscar el último archivo de configuración y conectarse usando connectWiFi
	LogT("logs.wifi_searching_config")
	ssid, _, err := getLastConnectedNetwork(interfaceName)
	if err != nil {
		i18n.LogTf("logs.wifi_config_not_found", err)
		LogT("logs.wifi_trying_other_way")
		
		// Último intento: buscar cualquier archivo de configuración reciente
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
				// Leer archivo usando sudo cat
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
			LogT("logs.wifi_no_network_found")
			return
		}
	} else {
		// Última red encontrada - no traducir, es debug interno
	}

	// Paso 5: Usar el archivo de configuración existente para iniciar wpa_supplicant
	// Paso 5: Conectándose usando archivo de configuración existente...
	
	// Buscar el archivo de configuración para esta red
	safeSSID := regexp.MustCompile(`[^a-zA-Z0-9_-]`).ReplaceAllString(ssid, "_")
	wpaConfigPath := fmt.Sprintf("%s/wpa_supplicant-%s.conf", WpaSupplicantConfigDir, safeSSID)
	
	if _, err := os.Stat(wpaConfigPath); os.IsNotExist(err) {
		// Intentar directorio alternativo
		wpaConfigPath = fmt.Sprintf("%s/wpa_supplicant-%s.conf", WpaSupplicantAltConfigDir, safeSSID)
		if _, err := os.Stat(wpaConfigPath); os.IsNotExist(err) {
			// Buscar cualquier archivo que contenga este SSID
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
						// Verificar que el archivo existe y es legible
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
				// No se encontró archivo de configuración - no traducir, es debug interno
				// Último recurso: usar connectWiFi
				country := constants.DefaultCountryCode
				result := connectWiFi(ssid, "", interfaceName, country, "system")
				if success, ok := result["success"].(bool); ok && success {
					i18n.LogTf("logs.wifi_auto_success", ssid)
					return
				} else {
					errorMsg := "Error desconocido"
					if err, ok := result["error"].(string); ok && err != "" {
						errorMsg = err
					}
					i18n.LogTf("logs.wifi_auto_error", errorMsg)
					return
				}
			}
		}
	}
	
	// Usando archivo de configuración - no traducir, es debug interno
	
	// Detener wpa_supplicant existente si está corriendo (más rápido)
	stopWpaSupplicant(interfaceName)
	time.Sleep(1 * time.Second) // Reducido de 2 a 1 segundo
	
	// Iniciar wpa_supplicant con el archivo de configuración existente
	runDir := getRunDir()
	LogT("logs.wifi_starting_wpa")
	if err := startWpaSupplicant(interfaceName, wpaConfigPath, runDir); err != nil {
		i18n.LogTf("logs.wifi_wpa_start_error", err)
		LogT("logs.wifi_trying_connect")
		country := constants.DefaultCountryCode
		result := connectWiFi(ssid, "", interfaceName, country, "system")
		if success, ok := result["success"].(bool); ok && success {
			i18n.LogTf("logs.wifi_auto_success", ssid)
		} else {
			errStr, _ := result["error"].(string)
			i18n.LogTf("logs.wifi_auto_error", errStr)
		}
		return
	}
	
	// Esperar menos tiempo para que wpa_supplicant se inicie
	time.Sleep(2 * time.Second) // Reducido de 3 a 2 segundos
	
	// Verificar conexión usando wpa_cli (menos intentos, más rápido)
	socketDir, err := waitForWpaCliConnection(interfaceName, 5) // Reducido de 10 a 5 intentos
	if err != nil {
		// No se pudo conectar a wpa_cli - no traducir, es debug interno
		return
	}
	
	runWpaCli := func(args ...string) (string, error) {
		base := []string{"wpa_cli", "-i", interfaceName, "-p", socketDir}
		cmd := exec.Command("sudo", append(base, args...)...)
		out, err := cmd.CombinedOutput()
		return strings.TrimSpace(string(out)), err
	}
	
	// Habilitar y seleccionar la red (todo en secuencia rápida)
	LogT("logs.wifi_enabling_network")
	runWpaCli("enable_network", "0")
	runWpaCli("select_network", "0")
	runWpaCli("reconnect")
	
	// Verificar conexión más rápido (menos espera, menos intentos)
	LogT("logs.wifi_waiting_auth")
	connected := false
	for attempt := 0; attempt < 8; attempt++ { // Reducido de 15 a 8 intentos
		time.Sleep(1 * time.Second) // Reducido de 2 a 1 segundo
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
		// Continuar de todas formas, puede que se conecte después
	}
	
	// Solicitar IP con DHCP en segundo plano (no bloquea)
	log.Printf("📡 Solicitando IP con DHCP...")
	go func() {
		executeCommand(fmt.Sprintf("sudo pkill -f 'dhclient.*%s|udhcpc.*%s' 2>/dev/null || true", interfaceName, interfaceName))
		time.Sleep(300 * time.Millisecond)
		executeCommand(fmt.Sprintf("sudo dhclient -v %s 2>&1 || sudo udhcpc -i %s -q -n 2>&1 || true", interfaceName, interfaceName))
	}()
	
	// Verificar IP rápidamente (solo unos pocos intentos)
	var ip string
	for ipAttempt := 0; ipAttempt < 5; ipAttempt++ { // Reducido de 15 a 5 intentos
		time.Sleep(1 * time.Second) // Reducido de 2 a 1 segundo
		
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
