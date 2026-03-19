package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"hostberry/internal/constants"
)

const WpaSupplicantConfigDir = "/etc/wpa_supplicant"
const WpaSupplicantAltConfigDir = "/var/lib/hostberry/wpa_supplicant"

// WpaSocketDirs: directorios donde wpa_supplicant puede crear el socket (evita repetir la lista).
var WpaSocketDirs = []string{"/run/wpa_supplicant", "/var/run/wpa_supplicant", "/tmp/wpa_supplicant"}

var activeRunDir string

func getRunDir() string {
	if activeRunDir != "" {
		return activeRunDir
	}
	candidates := WpaSocketDirs
	for _, dir := range candidates {
		if _, err := os.Stat(dir); err == nil {
			testFile := fmt.Sprintf("%s/.test_write", dir)
			if err := os.WriteFile(testFile, []byte("test"), 0644); err == nil {
				os.Remove(testFile)
				activeRunDir = dir
				LogTf("logs.socket_dir_selected", activeRunDir)
				return activeRunDir
			} else {
				LogTf("logs.socket_dir_not_writable", dir, err)
			}
		} else {
			if err := os.MkdirAll(dir, 0755); err == nil {
				testFile := fmt.Sprintf("%s/.test_write", dir)
				if err := os.WriteFile(testFile, []byte("test"), 0644); err == nil {
					os.Remove(testFile)
					activeRunDir = dir
					LogTf("logs.socket_dir_created", activeRunDir)
					return activeRunDir
				}
			}
		}
	}
	activeRunDir = "/tmp/wpa_supplicant"
	os.MkdirAll(activeRunDir, 0755)
	LogTf("logs.socket_dir_default", activeRunDir)
	return activeRunDir
}

func ensureWpaSupplicantDirs() error {
	if _, err := os.Stat(WpaSupplicantConfigDir); os.IsNotExist(err) {
		LogTf("logs.wpa_config_dir_creating", WpaSupplicantConfigDir)
		cmd := exec.Command("sudo", "mkdir", "-p", WpaSupplicantConfigDir)
		if out, err := cmd.CombinedOutput(); err != nil {
			LogTf("logs.wpa_config_dir_error", WpaSupplicantConfigDir, err, string(out))
		}
	}
	exec.Command("sudo", "chmod", "755", WpaSupplicantConfigDir).Run()
	exec.Command("sudo", "chown", "root:netdev", WpaSupplicantConfigDir).Run()

	runDirCandidates := WpaSocketDirs
	var createdDir string

	for _, dir := range runDirCandidates {
		if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
			LogTf("logs.socket_dir_exists", dir)
			createdDir = dir
			break
		}

		LogTf("logs.socket_dir_creating", dir)
		cmd := exec.Command("sudo", "mkdir", "-p", dir)
		if out, err := cmd.CombinedOutput(); err != nil {
			LogTf("logs.socket_dir_create_error", dir, err, string(out))
			continue
		}

		if _, err := os.Stat(dir); err == nil {
			LogTf("logs.socket_dir_created_ok", dir)
			createdDir = dir
			break
		}
	}

	if createdDir == "" {
		createdDir = "/tmp/wpa_supplicant"
		os.MkdirAll(createdDir, 0775)
		LogTf("logs.socket_dir_temp", createdDir)
	}

	exec.Command("sudo", "chmod", "775", createdDir).Run()
	exec.Command("sudo", "chown", "root:netdev", createdDir).Run()

	activeRunDir = createdDir
	LogTf("logs.socket_dir_active", activeRunDir)

	return nil
}

func stopWpaSupplicant(interfaceName string) {
	LogTf("logs.wpa_stopping", interfaceName)

	executeCommand(fmt.Sprintf("sudo pkill -f 'wpa_supplicant.*-i.*%s' 2>/dev/null || true", interfaceName))
	executeCommand(fmt.Sprintf("sudo pkill -f 'wpa_supplicant.*%s' 2>/dev/null || true", interfaceName))
	
	executeCommand("sudo killall wpa_supplicant 2>/dev/null || true")

	for i := 0; i < 5; i++ {
		checkCmd := exec.Command("sh", "-c", fmt.Sprintf("pgrep -f 'wpa_supplicant.*%s'", interfaceName))
		if out, _ := checkCmd.Output(); strings.TrimSpace(string(out)) == "" {
			break
		}
		time.Sleep(500 * time.Millisecond)
		if i == 4 {
			LogT("logs.wpa_force_kill")
			executeCommand(fmt.Sprintf("sudo pkill -9 -f 'wpa_supplicant.*%s' 2>/dev/null || true", interfaceName))
			executeCommand("sudo killall -9 wpa_supplicant 2>/dev/null || true")
		}
	}

	for _, dir := range WpaSocketDirs {
		executeCommand(fmt.Sprintf("sudo rm -f %s/%s 2>/dev/null || true", dir, interfaceName))
	}
}

func startWpaSupplicant(interfaceName, configPath, runDir string) error {
	if runDir == "" {
		runDir = "/run/wpa_supplicant"
	}
	LogTf("logs.wpa_starting", interfaceName, configPath, runDir)

	executeCommand(fmt.Sprintf("sudo mkdir -p %s 2>/dev/null || true", runDir))
	executeCommand(fmt.Sprintf("sudo chmod 775 %s 2>/dev/null || true", runDir))
	executeCommand(fmt.Sprintf("sudo chown root:netdev %s 2>/dev/null || true", runDir))
	executeCommand(fmt.Sprintf("sudo rm -f %s/%s 2>/dev/null || true", runDir, interfaceName))

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return fmt.Errorf("archivo de configuración no existe: %s", configPath)
	}

	wpaSupplicantPath := ""
	possiblePaths := []string{
		"/usr/sbin/wpa_supplicant",
		"/sbin/wpa_supplicant",
		"/usr/bin/wpa_supplicant",
		"/bin/wpa_supplicant",
	}
	
	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			wpaSupplicantPath = path
			break
		}
	}
	
	if wpaSupplicantPath == "" {
		whichCmd := exec.Command("sh", "-c", "which wpa_supplicant 2>/dev/null")
		if whichOut, err := whichCmd.Output(); err == nil {
			wpaSupplicantPath = strings.TrimSpace(string(whichOut))
		}
	}
	
	if wpaSupplicantPath == "" {
		return fmt.Errorf("wpa_supplicant no se encontró en el sistema. Instala el paquete wpa_supplicant")
	}
	
	LogTf("logs.wpa_path", wpaSupplicantPath)
	
	if fi, err := os.Stat(wpaSupplicantPath); err != nil || fi.Mode()&0111 == 0 {
		return fmt.Errorf("wpa_supplicant no es ejecutable en %s", wpaSupplicantPath)
	}
	
	tryStart := func(driver string) (out []byte, runErr error) {
		args := []string{wpaSupplicantPath, "-B", "-i", interfaceName, "-c", configPath}
		if driver != "" {
			args = append(args, "-D", driver)
		}
		if runDir != "" {
			args = append(args, "-C", runDir)
		}
		cmd := exec.Command("sudo", args...)
		cmd.Env = append(os.Environ(), "SUDO_ASKPASS=/bin/false")
		out, runErr = cmd.CombinedOutput()
		return out, runErr
	}

	tryDrivers := []string{"nl80211,wext", "wext", "nl80211", ""}
	var lastErr error
	var lastOut string

	for _, driver := range tryDrivers {
		executeCommand(fmt.Sprintf("sudo rm -f %s/%s 2>/dev/null || true", runDir, interfaceName))
		startOut, startErr := tryStart(driver)
		outStr := string(startOut)
		if startErr != nil {
			lastOut = outStr
			lastErr = startErr
			LogTf("logs.wpa_start_error", startErr, outStr)
			if strings.Contains(outStr, "not found") || strings.Contains(outStr, "No such file") {
				return fmt.Errorf("wpa_supplicant no se encontró en %s. Instala el paquete wpa_supplicant (apt install wpasupplicant)", wpaSupplicantPath)
			}
			if strings.Contains(outStr, "ctrl_iface exists") || strings.Contains(outStr, "cannot override it") {
				LogT("logs.wpa_socket_in_use")
				executeCommand(fmt.Sprintf("sudo rm -f %s/%s 2>/dev/null || true", runDir, interfaceName))
				startOut, startErr = tryStart(driver)
				if startErr != nil {
					lastOut = string(startOut)
					lastErr = fmt.Errorf("error iniciando wpa_supplicant tras limpiar socket: %v, output: %s", startErr, string(startOut))
					continue
				}
				outStr = string(startOut)
				startErr = nil // éxito del retry, continuar con comprobación de pid
			} else {
				// Error de driver u otro: probar siguiente driver antes de devolver error
				driverHint := ""
				if driver != "" {
					driverHint = " (driver " + driver + ")"
				}
				lastErr = fmt.Errorf("error iniciando wpa_supplicant%v: %v. %s", driverHint, startErr, strings.TrimSpace(outStr))
				if strings.Contains(strings.ToLower(outStr), "driver") || strings.Contains(strings.ToLower(outStr), "nl80211") || strings.Contains(strings.ToLower(outStr), "wext") || strings.Contains(outStr, "Could not configure") {
					continue
				}
				return lastErr
			}
		}
		LogTf("logs.wpa_command_executed", strings.TrimSpace(outStr))
		time.Sleep(2 * time.Second)

		pidFound := false
		var pid string
		pidCmd := exec.Command("sh", "-c", fmt.Sprintf("pgrep -f 'wpa_supplicant.*%s'", interfaceName))
		if pidOut, err := pidCmd.Output(); err == nil {
			pid = strings.TrimSpace(string(pidOut))
			if pid != "" {
				pidFound = true
			}
		}
		if !pidFound {
			pidCmd2 := exec.Command("sh", "-c", fmt.Sprintf("pgrep -f '%s.*%s'", wpaSupplicantPath, interfaceName))
			if pidOut2, err2 := pidCmd2.Output(); err2 == nil {
				pid = strings.TrimSpace(string(pidOut2))
				if pid != "" {
					pidFound = true
				}
			}
		}
		if !pidFound {
			psCmd := exec.Command("sh", "-c", fmt.Sprintf("ps aux | grep '[w]pa_supplicant.*%s' | awk '{print $2}' | head -1", interfaceName))
			if psOut, err := psCmd.Output(); err == nil {
				pid = strings.TrimSpace(string(psOut))
				if pid != "" {
					pidFound = true
				}
			}
		}
		if pidFound {
			LogTf("logs.wpa_running", pid)
			return nil
		}
		executeCommand(fmt.Sprintf("sudo pkill -f 'wpa_supplicant.*%s' 2>/dev/null || true", interfaceName))
		lastErr = fmt.Errorf("wpa_supplicant se ejecutó pero se detuvo de inmediato. Comprueba la interfaz %s y los logs (journalctl -u hostberry)", interfaceName)
		lastOut = outStr
	}

	if lastErr != nil {
		LogT("logs.wpa_not_running")
		dmesgCmd := exec.Command("sh", "-c", "dmesg | tail -20 | grep -i wpa 2>/dev/null || echo 'No hay mensajes de wpa en dmesg'")
		if dmesgOut, err := dmesgCmd.Output(); err == nil {
			LogTf("logs.wpa_dmesg", string(dmesgOut))
		}
		if lastOut != "" {
			return fmt.Errorf("%v. Salida: %s", lastErr, strings.TrimSpace(lastOut))
		}
		return lastErr
	}
	return nil
}

func waitForWpaCliConnection(interfaceName string, maxAttempts int) (string, error) {
	LogTf("logs.wpa_cli_waiting", interfaceName)

	socketDirs := []string{}
	if activeRunDir != "" {
		socketDirs = append(socketDirs, activeRunDir)
	}
	socketDirs = append(socketDirs, WpaSocketDirs...)
	seen := map[string]bool{}
	uniqueDirs := []string{}
	for _, dir := range socketDirs {
		if dir == "" || seen[dir] {
			continue
		}
		seen[dir] = true
		uniqueDirs = append(uniqueDirs, dir)
	}

	var workingSocketDir string
	var lastPingOutput string
	var lastStatusOutput string
	var lastPingErr error
	var lastStatusErr error

	runWpaCli := func(args ...string) (string, error) {
		cmd := exec.Command("sudo", args...)
		out, err := cmd.CombinedOutput()
		return strings.TrimSpace(string(out)), err
	}
	for attempt := 0; attempt < maxAttempts; attempt++ {
		workingSocketDir = ""
		for _, dir := range uniqueDirs {
			socketPath := fmt.Sprintf("%s/%s", dir, interfaceName)
			if _, err := os.Stat(socketPath); err == nil {
				LogTf("logs.wpa_socket_found", socketPath)
				workingSocketDir = dir
				executeCommand(fmt.Sprintf("sudo chmod 660 %s 2>/dev/null || true", socketPath))
				executeCommand(fmt.Sprintf("sudo chown root:netdev %s 2>/dev/null || true", socketPath))
			}

			pingOut, pingErr := runWpaCli("wpa_cli", "-i", interfaceName, "-p", dir, "ping")
			lastPingOutput = pingOut
			lastPingErr = pingErr
			if lastPingOutput != "" {
				LogTf("logs.wpa_cli_ping", dir, lastPingOutput)
			}
			if pingErr != nil && lastPingOutput != "" {
				LogTf("logs.wpa_cli_ping_error", dir, pingErr)
			}
			if strings.Contains(lastPingOutput, "PONG") {
				LogTf("logs.wpa_cli_responded", dir)
				return dir, nil
			}

			statusOut, statusErr := runWpaCli("wpa_cli", "-i", interfaceName, "-p", dir, "status")
			lastStatusOutput = statusOut
			lastStatusErr = statusErr
			if lastStatusOutput != "" {
				LogTf("logs.wpa_cli_status", dir, lastStatusOutput)
			}
			if statusErr != nil && lastStatusOutput != "" {
				LogTf("logs.wpa_cli_status_error", dir, statusErr)
			}
			if strings.Contains(lastStatusOutput, "wpa_state=") {
				LogTf("logs.wpa_cli_status_valid", dir)
				return dir, nil
			}

			globalSocket := fmt.Sprintf("%s/global", dir)
			if _, err := os.Stat(globalSocket); err == nil {
				globalPingOut, globalPingErr := runWpaCli("wpa_cli", "-g", dir, "-i", interfaceName, "ping")
				if strings.TrimSpace(globalPingOut) != "" {
					LogTf("logs.wpa_cli_global_ping", dir, strings.TrimSpace(globalPingOut))
				}
				if globalPingErr == nil && strings.Contains(globalPingOut, "PONG") {
					LogTf("logs.wpa_cli_global_responded", dir)
					return dir, nil
				}
				globalStatusOut, globalStatusErr := runWpaCli("wpa_cli", "-g", dir, "-i", interfaceName, "status")
				if strings.TrimSpace(globalStatusOut) != "" {
					LogTf("logs.wpa_cli_global_status", dir, strings.TrimSpace(globalStatusOut))
				}
				if globalStatusErr == nil && strings.Contains(globalStatusOut, "wpa_state=") {
					LogTf("logs.wpa_cli_global_status_valid", dir)
					return dir, nil
				}
			}
		}

		if workingSocketDir != "" {
			LogTf("logs.wpa_cli_attempt", attempt+1, maxAttempts, workingSocketDir)
		} else {
			LogTf("logs.wpa_cli_socket_not_found", attempt+1, maxAttempts)
		}

		time.Sleep(1 * time.Second)
	}

	if lastPingOutput != "" || lastStatusOutput != "" {
		return "", fmt.Errorf("wpa_cli no pudo comunicarse con wpa_supplicant después de %d intentos (último ping: %s, error: %v; último status: %s, error: %v)", maxAttempts, lastPingOutput, lastPingErr, lastStatusOutput, lastStatusErr)
	}
	return "", fmt.Errorf("wpa_cli no pudo comunicarse con wpa_supplicant después de %d intentos", maxAttempts)
}

// getLastConnectedNetwork obtiene la última red WiFi conectada desde los archivos de configuración
func getLastConnectedNetwork(interfaceName string) (string, string, error) {
	if interfaceName == "" {
		interfaceName = constants.DefaultWiFiInterface
	}

	// Primero intentar obtener desde wpa_cli si wpa_supplicant está corriendo
	socketDirs := WpaSocketDirs
	for _, dir := range socketDirs {
		// Intentar socket de interfaz
		socketPath := fmt.Sprintf("%s/%s", dir, interfaceName)
		if _, err := os.Stat(socketPath); err == nil {
			runWpaCli := func(args ...string) (string, error) {
				base := []string{"wpa_cli", "-i", interfaceName, "-p", dir}
				cmd := exec.Command("sudo", append(base, args...)...)
				out, err := cmd.CombinedOutput()
				return strings.TrimSpace(string(out)), err
			}
			
			listOut, err := runWpaCli("list_networks")
			if err == nil && listOut != "" {
				lines := strings.Split(listOut, "\n")
				if len(lines) > 1 {
					// Buscar la red activa o la primera habilitada
					for i, line := range lines {
						if i == 0 {
							continue // Saltar encabezado
						}
						fields := strings.Fields(line)
						if len(fields) >= 2 {
							ssid := strings.Trim(fields[1], "\"")
							if ssid != "" && ssid != "--" {
								// Verificar si está habilitada o es CURRENT
								if len(fields) >= 4 && (fields[3] == "[CURRENT]" || fields[2] == "[ENABLED]") {
									log.Printf("Red encontrada en wpa_cli: %s", ssid)
									return ssid, "", nil
								}
							}
						}
					}
					// Si no hay red activa, tomar la primera
					if len(lines) > 1 {
						firstLine := lines[1]
						fields := strings.Fields(firstLine)
						if len(fields) >= 2 {
							ssid := strings.Trim(fields[1], "\"")
							if ssid != "" && ssid != "--" {
								LogTf("logs.wifi_first_network_cli", ssid)
								return ssid, "", nil
							}
						}
					}
				}
			}
		}
		
		// Intentar socket global
		globalSocket := fmt.Sprintf("%s/global", dir)
		if _, err := os.Stat(globalSocket); err == nil {
			runWpaCli := func(args ...string) (string, error) {
				base := []string{"wpa_cli", "-g", dir, "-i", interfaceName}
				cmd := exec.Command("sudo", append(base, args...)...)
				out, err := cmd.CombinedOutput()
				return strings.TrimSpace(string(out)), err
			}
			
			listOut, err := runWpaCli("list_networks")
			if err == nil && listOut != "" {
				lines := strings.Split(listOut, "\n")
				if len(lines) > 1 {
					// Buscar la red activa o la primera habilitada
					for i, line := range lines {
						if i == 0 {
							continue
						}
						fields := strings.Fields(line)
						if len(fields) >= 2 {
							ssid := strings.Trim(fields[1], "\"")
							if ssid != "" && ssid != "--" {
								if len(fields) >= 4 && (fields[3] == "[CURRENT]" || fields[2] == "[ENABLED]") {
									LogTf("logs.wifi_network_found_global", ssid)
									return ssid, "", nil
								}
							}
						}
					}
					// Si no hay red activa, tomar la primera
					if len(lines) > 1 {
						firstLine := lines[1]
						fields := strings.Fields(firstLine)
						if len(fields) >= 2 {
							ssid := strings.Trim(fields[1], "\"")
							if ssid != "" && ssid != "--" {
								LogTf("logs.wifi_first_network_global", ssid)
								return ssid, "", nil
							}
						}
					}
				}
			}
		}
	}

	// Si no se encontró en wpa_cli, buscar en archivos de configuración
	// Buscar en ambos directorios
	configDirs := []string{WpaSupplicantConfigDir, WpaSupplicantAltConfigDir}
	var lastConfigFile os.DirEntry
	var lastModTime time.Time
	var foundConfigDir string

	for _, configDir := range configDirs {
		configFiles, err := os.ReadDir(configDir)
		if err != nil {
			continue
		}

		// Buscar el archivo de configuración más reciente
		for _, file := range configFiles {
			if strings.HasPrefix(file.Name(), "wpa_supplicant-") && strings.HasSuffix(file.Name(), ".conf") {
				info, err := file.Info()
				if err != nil {
					continue
				}
				if info.ModTime().After(lastModTime) {
					lastModTime = info.ModTime()
					lastConfigFile = file
					foundConfigDir = configDir
				}
			}
		}
	}

	if lastConfigFile == nil {
		return "", "", fmt.Errorf("no se encontró ninguna red guardada")
	}

	configPath := fmt.Sprintf("%s/%s", foundConfigDir, lastConfigFile.Name())
	
	// Leer archivo usando sudo cat (los archivos tienen permisos 600 y pertenecen a root)
	cmd := exec.Command("sudo", "cat", configPath)
	cmd.Env = append(os.Environ(), "SUDO_ASKPASS=/bin/false")
	configContentBytes, err := cmd.CombinedOutput()
	if err != nil {
		return "", "", fmt.Errorf("error leyendo archivo de configuración: %v", err)
	}
	configContent := string(configContentBytes)

	// Extraer SSID del archivo de configuración
	ssid := ""
	lines := strings.Split(string(configContent), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "ssid=") {
			ssid = strings.Trim(strings.TrimPrefix(line, "ssid="), "\"")
			break
		}
	}

	if ssid == "" {
		return "", "", fmt.Errorf("no se pudo extraer SSID del archivo de configuración")
	}

	LogTf("logs.wifi_ssid_found_config", ssid)
	// No retornamos password porque no podemos obtenerla del archivo (está hasheada)
	// La contraseña ya está en el archivo de configuración, solo necesitamos el SSID
	return ssid, "", nil
}

