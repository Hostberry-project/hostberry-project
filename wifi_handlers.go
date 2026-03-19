package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)



// autoConnectToLastNetwork intenta conectarse automáticamente a la última red WiFi conectada
func autoConnectToLastNetwork(interfaceName string) {
	if interfaceName == "" {
		interfaceName = DefaultWiFiInterface
	}

	LogTf("logs.wifi_auto_connect_start", interfaceName)

	// Verificar que la interfaz existe antes de continuar
	cmd := exec.Command("sh", "-c", fmt.Sprintf("ip link show %s 2>/dev/null", interfaceName))
	if err := cmd.Run(); err != nil {
		LogTf("logs.wifi_interface_not_exists", interfaceName)
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
			LogTf("logs.wifi_socket_interface_found", socketPath)
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
				LogTf("logs.wifi_socket_global_found", globalSocket)
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
						LogTf("logs.wifi_reconnecting", netID)
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
		LogTf("logs.wifi_config_not_found", err)
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
							LogTf("logs.wifi_network_found_file", lastFile.Name(), ssid)
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
							LogTf("logs.wifi_config_file_found", wpaConfigPath)
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
				country := DefaultCountryCode
				result := connectWiFi(ssid, "", interfaceName, country, "system")
				if success, ok := result["success"].(bool); ok && success {
					LogTf("logs.wifi_auto_success", ssid)
					return
				} else {
					errorMsg := "Error desconocido"
					if err, ok := result["error"].(string); ok && err != "" {
						errorMsg = err
					}
					LogTf("logs.wifi_auto_error", errorMsg)
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
		LogTf("logs.wifi_wpa_start_error", err)
		LogT("logs.wifi_trying_connect")
		country := DefaultCountryCode
		result := connectWiFi(ssid, "", interfaceName, country, "system")
		if success, ok := result["success"].(bool); ok && success {
			LogTf("logs.wifi_auto_success", ssid)
		} else {
			errStr, _ := result["error"].(string)
			LogTf("logs.wifi_auto_error", errStr)
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
			LogTf("logs.wifi_authenticated", ssid)
			break
		}
		if attempt%3 == 0 && attempt > 0 {
			LogTf("logs.wifi_status_attempt3", statusOut, attempt+1)
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
