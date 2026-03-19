package hostapd

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"hostberry/internal/constants"
	"hostberry/internal/utils"
)

// executeCommand delega al helper seguro en internal/utils.
func executeCommand(cmd string) (string, error) {
	return utils.ExecuteCommand(cmd)
}

// strconvAtoiSafe wrapper para el paquete main -> internal/utils.
func strconvAtoiSafe(s string) (int, error) {
	return utils.StrconvAtoiSafe(s)
}

func HostapdAccessPointsHandler(c *fiber.Ctx) error {
	var aps []fiber.Map

	hostapdActive := false
	hostapdTransmitting := false // Verificar si realmente está transmitiendo

	systemctlOut, _ := exec.Command("sh", "-c", "systemctl is-active hostapd 2>/dev/null").CombinedOutput()
	systemctlStatus := strings.TrimSpace(string(systemctlOut))
	if systemctlStatus == "active" {
		hostapdActive = true
	}

	if !hostapdActive {
		pgrepOut, _ := exec.Command("sh", "-c", "pgrep hostapd > /dev/null 2>&1 && echo active || echo inactive").CombinedOutput()
		pgrepStatus := strings.TrimSpace(string(pgrepOut))
		if pgrepStatus == "active" {
			hostapdActive = true
		}
	}

	if hostapdActive {
		interfaceName := "ap0" // default para modo AP+STA
		if configContent, err := os.ReadFile("/etc/hostapd/hostapd.conf"); err == nil {
			lines := strings.Split(string(configContent), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "interface=") {
					parts := strings.SplitN(line, "=", 2)
					if len(parts) == 2 {
						interfaceName = strings.TrimSpace(parts[1])
						break
					}
				}
			}
		}

		iwOut, _ := exec.Command("sh", "-c", fmt.Sprintf("iw dev %s info 2>/dev/null | grep -i 'type AP' || iwconfig %s 2>/dev/null | grep -i 'mode:master' || echo ''", interfaceName, interfaceName)).CombinedOutput()
		iwStatus := strings.TrimSpace(string(iwOut))
		if iwStatus != "" {
			hostapdTransmitting = true
		}

		if !hostapdTransmitting {
			cliStatusOut, _ := exec.Command("sh", "-c", fmt.Sprintf("hostapd_cli -i %s status 2>/dev/null | grep -i 'state=ENABLED' || echo ''", interfaceName)).CombinedOutput()
			cliStatus := strings.TrimSpace(string(cliStatusOut))
			if cliStatus != "" {
				hostapdTransmitting = true
			}
		}

		if !hostapdTransmitting {
			journalOut, _ := exec.Command("sh", "-c", "sudo journalctl -u hostapd -n 30 --no-pager 2>/dev/null | tail -20").CombinedOutput()
			journalLogs := strings.ToLower(string(journalOut))
			if strings.Contains(journalLogs, "could not configure driver") ||
				strings.Contains(journalLogs, "nl80211: could not") ||
				strings.Contains(journalLogs, "interface") && strings.Contains(journalLogs, "not found") ||
				strings.Contains(journalLogs, "failed to initialize") {
				hostapdTransmitting = false
			}
		}
	}

	configPath := "/etc/hostapd/hostapd.conf"
	config := make(map[string]string)

	if configContent, err := os.ReadFile(configPath); err == nil {
		lines := strings.Split(string(configContent), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				config[key] = value
			}
		}
	}

	if hostapdActive || len(config) > 0 {
		ssid := config["ssid"]
		if ssid == "" {
			ssid = "hostberry" // Valor por defecto (red + portal cautivo)
		}

		interfaceName := config["interface"]
		if interfaceName == "" {
			interfaceName = constants.DefaultWiFiInterface
		}

		channel := config["channel"]
		if channel == "" {
			channel = "6" // Valor por defecto
		}

		security := "WPA2"
		if config["auth_algs"] == "0" {
			security = "Open"
		} else if strings.Contains(config["wpa_key_mgmt"], "SHA256") {
			security = "WPA3"
		} else if config["wpa"] == "2" {
			security = "WPA2"
		}

		clientsCount := 0
		if hostapdActive {
			cliOut, err := exec.Command("sh", "-c", fmt.Sprintf("hostapd_cli -i %s all_sta 2>/dev/null | grep -c '^sta=' || echo 0", interfaceName)).CombinedOutput()
			if err == nil {
				if count, err := strconvAtoiSafe(strings.TrimSpace(string(cliOut))); err == nil {
					clientsCount = count
				}
			}
		}

		actuallyActive := hostapdActive && hostapdTransmitting

		aps = append(aps, fiber.Map{
			"name":      interfaceName,
			"ssid":      ssid,
			"interface": interfaceName,
			"channel":   channel,
			"security":  security,
			"enabled":   actuallyActive, // Solo true si realmente está transmitiendo
			"active":    actuallyActive, // Solo true si realmente está transmitiendo
			"status": func() string {
				if actuallyActive {
					return "active"
				} else if hostapdActive {
					return "error" // Servicio corriendo pero no transmite
				}
				return "inactive"
			}(),
			"transmitting":    hostapdTransmitting, // Nuevo campo para diagnóstico
			"service_running": hostapdActive,       // Servicio corriendo (pero puede no transmitir)
			"clients_count":   clientsCount,
		})
	}

	return c.JSON(aps)
}

func HostapdClientsHandler(c *fiber.Ctx) error {
	var clients []fiber.Map

	hostapdOut, _ := exec.Command("sh", "-c", "systemctl is-active hostapd 2>/dev/null || pgrep hostapd > /dev/null && echo active || echo inactive").CombinedOutput()
	hostapdStatus := strings.TrimSpace(string(hostapdOut))

	if hostapdStatus == "active" {
		cliOut, err := exec.Command("hostapd_cli", "-i", "wlan0", "all_sta").CombinedOutput()
		if err == nil && len(cliOut) > 0 {
			lines := strings.Split(strings.TrimSpace(string(cliOut)), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line != "" && strings.HasPrefix(line, "sta=") {
					mac := strings.TrimPrefix(line, "sta=")
					clients = append(clients, fiber.Map{
						"mac_address": mac,
						"ip_address":  "-",
						"signal":      "-",
						"uptime":      "-",
					})
				}
			}
		}
	}

	return c.JSON(clients)
}

func HostapdCreateAp0Handler(c *fiber.Ctx) error {
	phyInterface := "wlan0"

	interfacesResp, _ := executeCommand("ip link show | grep -E '^[0-9]+: wlan' | awk -F: '{print $2}' | awk '{print $1}' | head -1")
	if strings.TrimSpace(interfacesResp) != "" {
		phyInterface = strings.TrimSpace(interfacesResp)
	}

	log.Printf("Creating ap0 interface from %s", phyInterface)

	ap0CheckCmd := "ip link show ap0 2>/dev/null"
	ap0Exists := false
	if out, err := executeCommand(ap0CheckCmd); err == nil && strings.TrimSpace(out) != "" {
		ap0Exists = true
		log.Printf("Interface ap0 already exists")
		return c.JSON(fiber.Map{
			"success": true,
			"message": "Interface ap0 already exists",
			"exists":  true,
		})
	}

	executeCommand(fmt.Sprintf("sudo ip link set %s up 2>/dev/null || true", phyInterface))
	time.Sleep(500 * time.Millisecond)

	phyName := ""
	phyCmd2 := fmt.Sprintf("cat /sys/class/net/%s/phy80211/name 2>/dev/null", phyInterface)
	phyOut2, _ := executeCommand(phyCmd2)
	phyName = strings.TrimSpace(phyOut2)

	if phyName == "" {
		phyCmd := fmt.Sprintf("iw dev %s info 2>/dev/null | grep 'wiphy' | awk '{print $2}'", phyInterface)
		phyOut, _ := executeCommand(phyCmd)
		phyName = strings.TrimSpace(phyOut)
	}

	if phyName == "" {
		phyCmd3 := "iw list 2>/dev/null | grep 'Wiphy' | head -1 | awk '{print $2}'"
		phyOut3, _ := executeCommand(phyCmd3)
		phyName = strings.TrimSpace(phyOut3)
	}

	if phyName == "" {
		phyName = "phy0"
		log.Printf("Warning: Could not detect phy name, using default: %s", phyName)
	}

	log.Printf("Detected phy name: %s for interface %s", phyName, phyInterface)

	delOut, _ := executeCommand("sudo iw dev ap0 del 2>/dev/null || true")
	if delOut != "" {
		log.Printf("Removed existing ap0 interface (if it existed): %s", strings.TrimSpace(delOut))
	}
	time.Sleep(1 * time.Second)

	createApCmd := fmt.Sprintf("sudo iw phy %s interface add ap0 type __ap 2>&1", phyName)
	log.Printf("Executing: %s", createApCmd)
	createOut, createErr := executeCommand(createApCmd)
	if createOut != "" {
		log.Printf("Command output: %s", strings.TrimSpace(createOut))
	}

	if createErr != nil {
		log.Printf("Error creating ap0 with phy %s: %s", phyName, strings.TrimSpace(createOut))
		log.Printf("Trying alternative method: using interface %s directly...", phyInterface)

		createApCmd2 := fmt.Sprintf("sudo iw dev %s interface add ap0 type __ap 2>&1", phyInterface)
		log.Printf("Executing: %s", createApCmd2)
		createOut2, createErr2 := executeCommand(createApCmd2)
		if createOut2 != "" {
			log.Printf("Method 1 output: %s", strings.TrimSpace(createOut2))
		}

		if createErr2 != nil {
			log.Printf("Error with alternative method: %s", strings.TrimSpace(createOut2))
			return c.Status(500).JSON(fiber.Map{
				"success": false,
				"error":   fmt.Sprintf("Failed to create ap0 interface: %s", strings.TrimSpace(createOut2)),
			})
		} else {
			log.Printf("Successfully created ap0 interface using alternative method (from %s)", phyInterface)
			ap0Exists = true
		}
	} else {
		log.Printf("Successfully created ap0 interface using phy %s", phyName)
		ap0Exists = true
	}

	if ap0Exists {
		time.Sleep(2 * time.Second)

		verified := false
		for i := 0; i < 5; i++ {
			verifyCmd := "ip link show ap0 2>/dev/null"
			verifyOut, verifyErr := executeCommand(verifyCmd)
			if verifyErr == nil && strings.TrimSpace(verifyOut) != "" {
				log.Printf("Interface ap0 verified successfully (attempt %d)", i+1)
				verified = true
				break
			}

			if i < 4 {
				log.Printf("Verification attempt %d failed, retrying...", i+1)
				time.Sleep(1 * time.Second)
			}
		}

		if !verified {
			log.Printf("ERROR: Interface ap0 was NOT created successfully after all attempts")
			return c.Status(500).JSON(fiber.Map{
				"success": false,
				"error":   "Failed to verify ap0 interface creation. Please check WiFi hardware and drivers.",
			})
		} else {
			log.Printf("SUCCESS: Interface ap0 created and verified")
			executeCommand("sudo ip link set ap0 up 2>/dev/null || true")
			return c.JSON(fiber.Map{
				"success": true,
				"message": "Interface ap0 created successfully",
				"exists":  true,
			})
		}
	}

	return c.Status(500).JSON(fiber.Map{
		"success": false,
		"error":   "Failed to create ap0 interface",
	})
}

func HostapdToggleHandler(c *fiber.Ctx) error {
	log.Printf("HostAPD toggle request received")

	hostapdOut, _ := exec.Command("sh", "-c", "systemctl is-active hostapd 2>/dev/null || pgrep hostapd > /dev/null && echo active || echo inactive").CombinedOutput()
	hostapdStatus := strings.TrimSpace(string(hostapdOut))
	isActive := hostapdStatus == "active"

	log.Printf("Current HostAPD status: %s (isActive: %v)", hostapdStatus, isActive)

	var cmdStr string
	var enableCmd string
	var action string

	if isActive {
		action = "disable"
		executeCommand("sudo systemctl stop dnsmasq 2>/dev/null || true")
		cmdStr = "sudo systemctl stop hostapd"
		enableCmd = "sudo systemctl disable hostapd 2>/dev/null || true"

	} else {
		action = "enable"

		configPath := "/etc/hostapd/hostapd.conf"
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			log.Printf("HostAPD configuration file not found: %s", configPath)
			return c.Status(400).JSON(fiber.Map{
				"error":          "HostAPD configuration not found. Please configure HostAPD first using the configuration form below.",
				"success":        false,
				"config_missing": true,
				"config_path":    configPath,
			})
		}

		configContent, err := os.ReadFile(configPath)
		if err != nil || len(configContent) == 0 {
			log.Printf("HostAPD configuration file is empty or unreadable: %s", configPath)
			return c.Status(400).JSON(fiber.Map{
				"error":          "HostAPD configuration file is empty or invalid. Please configure HostAPD first using the configuration form below.",
				"success":        false,
				"config_missing": true,
				"config_path":    configPath,
			})
		}

		ap0CheckCmd := "ip link show ap0 2>/dev/null"
		ap0Exists := false
		if out, err := executeCommand(ap0CheckCmd); err == nil && strings.TrimSpace(out) != "" {
			ap0Exists = true
			log.Printf("Interface ap0 already exists")
		} else {
			log.Printf("Interface ap0 does not exist, creating it...")
			phyInterface := "wlan0"
			if configContent, err := os.ReadFile(configPath); err == nil {
				lines := strings.Split(string(configContent), "\n")
				for _, line := range lines {
					line = strings.TrimSpace(line)
					if strings.HasPrefix(line, "interface=") {
						break
					}
				}
			}

			executeCommand(fmt.Sprintf("sudo ip link set %s up 2>/dev/null || true", phyInterface))
			time.Sleep(500 * time.Millisecond)

			phyName := ""

			phyCmd := fmt.Sprintf("iw dev %s info 2>/dev/null | grep 'wiphy' | awk '{print $2}'", phyInterface)
			phyOut, _ := executeCommand(phyCmd)
			phyName = strings.TrimSpace(phyOut)

			if phyName == "" {
				phyCmd2 := fmt.Sprintf("cat /sys/class/net/%s/phy80211/name 2>/dev/null", phyInterface)
				phyOut2, _ := executeCommand(phyCmd2)
				phyName = strings.TrimSpace(phyOut2)
			}

			if phyName == "" {
				phyCmd3 := "iw list 2>/dev/null | grep -A 1 'Wiphy' | tail -1 | awk '{print $2}'"
				phyOut3, _ := executeCommand(phyCmd3)
				phyName = strings.TrimSpace(phyOut3)
			}

			if phyName == "" {
				phyName = "phy0"
				log.Printf("Warning: Could not detect phy name, using default: %s", phyName)
			}

			log.Printf("Creating ap0 interface using phy %s from interface %s...", phyName, phyInterface)

			executeCommand("sudo iw dev ap0 del 2>/dev/null || true")
			time.Sleep(1 * time.Second)

			createApCmd := fmt.Sprintf("sudo iw phy %s interface add ap0 type __ap", phyName)
			createOut, createErr := executeCommand(createApCmd)
			if createErr != nil {
				log.Printf("Warning: Could not create ap0 interface with phy %s: %s", phyName, strings.TrimSpace(createOut))
				createApCmd2 := fmt.Sprintf("sudo iw dev %s interface add ap0 type __ap", phyInterface)
				createOut2, createErr2 := executeCommand(createApCmd2)
				if createErr2 != nil {
					log.Printf("Warning: Alternative method also failed: %s", strings.TrimSpace(createOut2))
				} else {
					log.Printf("Successfully created ap0 interface using alternative method (from %s)", phyInterface)
					ap0Exists = true
				}
			} else {
				log.Printf("Successfully created ap0 interface using phy %s", phyName)
				ap0Exists = true
			}

			if ap0Exists {
				verifyCmd := "ip link show ap0 2>/dev/null"
				verifyOut, verifyErr := executeCommand(verifyCmd)
				if verifyErr == nil && strings.TrimSpace(verifyOut) != "" {
					log.Printf("Interface ap0 verified: %s", strings.TrimSpace(verifyOut))
					executeCommand("sudo ip link set ap0 up 2>/dev/null || true")
					log.Printf("Activated ap0 interface")
				} else {
					log.Printf("Warning: ap0 was created but verification failed")
				}
			}
		}

		maskedCheck, _ := exec.Command("sh", "-c", "systemctl is-enabled hostapd 2>&1").CombinedOutput()
		maskedStatus := strings.TrimSpace(string(maskedCheck))
		if strings.Contains(maskedStatus, "masked") {
			log.Printf("HostAPD service is masked, unmasking...")
			executeCommand("sudo systemctl unmask hostapd 2>/dev/null || true")
		}

		configLines := strings.Split(string(configContent), "\n")
		var interfaceName, gatewayIP string
		for _, line := range configLines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			if strings.HasPrefix(line, "interface=") {
				interfaceName = strings.TrimPrefix(line, "interface=")
			}
		}

		if interfaceName == "" {
			log.Printf("HostAPD configuration file missing interface setting: %s", configPath)
			return c.Status(400).JSON(fiber.Map{
				"error":          "HostAPD configuration file is missing required 'interface' setting. Please configure HostAPD first using the configuration form below.",
				"success":        false,
				"config_missing": true,
				"config_path":    configPath,
			})
		}

		if interfaceName != "" {
			gatewayIP = "192.168.4.1"

			ipCheckCmd := fmt.Sprintf("ip addr show %s 2>/dev/null | grep 'inet ' | awk '{print $2}' | cut -d/ -f1", interfaceName)
			ipOut, _ := exec.Command("sh", "-c", ipCheckCmd).CombinedOutput()
			currentIP := strings.TrimSpace(string(ipOut))

			if currentIP == "" {
				log.Printf("Configuring IP %s on interface %s", gatewayIP, interfaceName)
				ipCmd := fmt.Sprintf("sudo ip addr add %s/24 dev %s 2>/dev/null || sudo ip addr replace %s/24 dev %s", gatewayIP, interfaceName, gatewayIP, interfaceName)
				if out, err := executeCommand(ipCmd); err != nil {
					log.Printf("Warning: Error setting IP on interface: %s", strings.TrimSpace(out))
				}

				if out, err := executeCommand(fmt.Sprintf("sudo ip link set %s up", interfaceName)); err != nil {
					log.Printf("Warning: Error bringing interface up: %s", strings.TrimSpace(out))
				}
			}
		}

		executeCommand("sudo systemctl unmask hostapd 2>/dev/null || true")
		executeCommand("sudo systemctl unmask dnsmasq 2>/dev/null || true")

		executeCommand("sudo systemctl daemon-reload 2>/dev/null || true")

		enableCmd = "sudo systemctl enable hostapd 2>/dev/null || true"
		executeCommand("sudo systemctl enable dnsmasq 2>/dev/null || true")

		executeCommand(fmt.Sprintf("sudo chmod 644 %s 2>/dev/null || true", configPath))

		cmdStr = "sudo systemctl start hostapd"
		executeCommand("sudo systemctl start dnsmasq 2>/dev/null || true")
	}

	log.Printf("Action: %s, Command: %s", action, cmdStr)

	if enableCmd != "" {
		if out, err := executeCommand(enableCmd); err != nil {
			log.Printf("Warning: Error enabling/disabling hostapd: %s", strings.TrimSpace(out))
		} else {
			log.Printf("Enable/disable command executed successfully: %s", strings.TrimSpace(out))
		}
	}

	out, err := executeCommand(cmdStr)
	if err != nil {
		log.Printf("Error executing %s command: %s", action, strings.TrimSpace(out))

		var errorDetails string
		if action == "enable" {
			journalOut, _ := exec.Command("sh", "-c", "sudo journalctl -u hostapd -n 20 --no-pager 2>/dev/null | tail -10").CombinedOutput()
			journalLogs := strings.TrimSpace(string(journalOut))
			if journalLogs != "" {
				lines := strings.Split(journalLogs, "\n")
				errorLines := []string{}
				for _, line := range lines {
					line = strings.TrimSpace(line)
					if line != "" && (strings.Contains(strings.ToLower(line), "error") ||
						strings.Contains(strings.ToLower(line), "failed") ||
						strings.Contains(strings.ToLower(line), "fail")) {
						errorLines = append(errorLines, line)
					}
				}
				if len(errorLines) > 0 {
					errorDetails = fmt.Sprintf(" Recent errors: %s", strings.Join(errorLines, "; "))
				} else {
					errorDetails = fmt.Sprintf(" Last logs: %s", strings.Join(lines[len(lines)-3:], "; "))
				}
			} else {
				statusOut, _ := exec.Command("sh", "-c", "sudo systemctl status hostapd --no-pager 2>/dev/null | head -15").CombinedOutput()
				statusInfo := strings.TrimSpace(string(statusOut))
				if statusInfo != "" {
					errorDetails = fmt.Sprintf(" Service status: %s", statusInfo)
				}
			}
		}

		return c.Status(500).JSON(fiber.Map{
			"error":   fmt.Sprintf("Failed to %s hostapd: %s%s", action, strings.TrimSpace(out), errorDetails),
			"success": false,
		})
	}

	log.Printf("HostAPD %s command executed. Output: %s", action, strings.TrimSpace(out))

	if action == "enable" {
		time.Sleep(1500 * time.Millisecond) // Más tiempo para que hostapd inicie
	} else {
		time.Sleep(500 * time.Millisecond)
	}

	hostapdOut2, _ := exec.Command("sh", "-c", "systemctl is-active hostapd 2>/dev/null || pgrep hostapd > /dev/null && echo active || echo inactive").CombinedOutput()
	hostapdStatus2 := strings.TrimSpace(string(hostapdOut2))
	actuallyActive := hostapdStatus2 == "active"

	if action == "enable" && !actuallyActive {
		log.Printf("HostAPD failed to start. Checking logs...")
		enabledOut, _ := exec.Command("sh", "-c", "systemctl is-enabled hostapd 2>/dev/null || echo disabled").CombinedOutput()
		enabledStatus := strings.TrimSpace(string(enabledOut))

		journalOut, _ := exec.Command("sh", "-c", "sudo journalctl -u hostapd -n 15 --no-pager 2>/dev/null | tail -8").CombinedOutput()
		journalLogs := strings.TrimSpace(string(journalOut))

		statusOut, _ := exec.Command("sh", "-c", "sudo systemctl status hostapd --no-pager 2>/dev/null | head -20").CombinedOutput()
		statusInfo := strings.TrimSpace(string(statusOut))

		var errorMsg string
		if journalLogs != "" {
			lines := strings.Split(journalLogs, "\n")
			errorLines := []string{}
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line != "" {
					lowerLine := strings.ToLower(line)
					if strings.Contains(lowerLine, "error") || strings.Contains(lowerLine, "failed") ||
						strings.Contains(lowerLine, "fail") || strings.Contains(lowerLine, "cannot") {
						errorLines = append(errorLines, line)
					}
				}
			}
			if len(errorLines) > 0 {
				maxLines := 3
				if len(errorLines) < maxLines {
					maxLines = len(errorLines)
				}
				errorMsg = strings.Join(errorLines[:maxLines], "; ")
			} else if len(lines) > 0 {
				maxLines := 3
				if len(lines) < maxLines {
					maxLines = len(lines)
				}
				errorMsg = strings.Join(lines[len(lines)-maxLines:], "; ")
			}
		}

		if errorMsg == "" && statusInfo != "" {
			statusLines := strings.Split(statusInfo, "\n")
			for _, line := range statusLines {
				if strings.Contains(strings.ToLower(line), "active:") ||
					strings.Contains(strings.ToLower(line), "failed") ||
					strings.Contains(strings.ToLower(line), "error") {
					errorMsg = strings.TrimSpace(line)
					break
				}
			}
		}

		if errorMsg != "" {
			return c.Status(500).JSON(fiber.Map{
				"error":   fmt.Sprintf("Failed to enable HostAPD. Service status: %s (enabled: %s). %s", hostapdStatus2, enabledStatus, errorMsg),
				"success": false,
				"status":  hostapdStatus2,
				"enabled": false,
			})
		} else {
			return c.Status(500).JSON(fiber.Map{
				"error":   fmt.Sprintf("Failed to enable HostAPD. Service status: %s (enabled: %s). Check configuration and logs.", hostapdStatus2, enabledStatus),
				"success": false,
				"status":  hostapdStatus2,
				"enabled": false,
			})
		}
	}

	log.Printf("HostAPD status after %s: %s (actuallyActive: %v)", action, hostapdStatus2, actuallyActive)

	return c.JSON(fiber.Map{
		"success": true,
		"output":  strings.TrimSpace(out),
		"enabled": actuallyActive,
		"action":  action,
		"status":  hostapdStatus2,
	})
}

func HostapdRestartHandler(c *fiber.Ctx) error {
	out1, err1 := executeCommand("sudo systemctl stop hostapd")

	time.Sleep(500 * time.Millisecond)

	out2, err2 := executeCommand("sudo systemctl start hostapd")

	if err1 != nil || err2 != nil {
		return c.Status(500).JSON(fiber.Map{
			"error":   "Error reiniciando HostAPD",
			"stop":    strings.TrimSpace(out1),
			"start":   strings.TrimSpace(out2),
			"stopOk":  err1 == nil,
			"startOk": err2 == nil,
			"success": false,
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"output":  "HostAPD restarted successfully",
	})
}

func HostapdDiagnosticsHandler(c *fiber.Ctx) error {
	diagnostics := make(map[string]interface{})

	systemctlOut, _ := exec.Command("sh", "-c", "systemctl is-active hostapd 2>/dev/null").CombinedOutput()
	systemctlStatus := strings.TrimSpace(string(systemctlOut))
	pgrepOut, _ := exec.Command("sh", "-c", "pgrep hostapd > /dev/null 2>&1 && echo active || echo inactive").CombinedOutput()
	pgrepStatus := strings.TrimSpace(string(pgrepOut))

	serviceRunning := systemctlStatus == "active" || pgrepStatus == "active"
	diagnostics["service_running"] = serviceRunning
	diagnostics["systemctl_status"] = systemctlStatus
	diagnostics["process_running"] = pgrepStatus == "active"

	interfaceName := "wlan0"
	configPath := "/etc/hostapd/hostapd.conf"
	if configContent, err := os.ReadFile(configPath); err == nil {
		lines := strings.Split(string(configContent), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "interface=") {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					interfaceName = strings.TrimSpace(parts[1])
					break
				}
			}
		}
	}
	diagnostics["interface"] = interfaceName

	iwOut, _ := exec.Command("sh", "-c", fmt.Sprintf("iw dev %s info 2>/dev/null | grep -i 'type AP' || iwconfig %s 2>/dev/null | grep -i 'mode:master' || echo ''", interfaceName, interfaceName)).CombinedOutput()
	iwStatus := strings.TrimSpace(string(iwOut))
	transmitting := iwStatus != ""

	if !transmitting && serviceRunning {
		cliStatusOut, _ := exec.Command("sh", "-c", fmt.Sprintf("hostapd_cli -i %s status 2>/dev/null | grep -i 'state=ENABLED' || echo ''", interfaceName)).CombinedOutput()
		cliStatus := strings.TrimSpace(string(cliStatusOut))
		if cliStatus != "" {
			transmitting = true
		}
	}

	diagnostics["transmitting"] = transmitting
	diagnostics["interface_in_ap_mode"] = iwStatus != ""

	journalOut, _ := exec.Command("sh", "-c", "sudo journalctl -u hostapd -n 50 --no-pager 2>/dev/null | tail -30").CombinedOutput()
	journalLogs := string(journalOut)
	diagnostics["recent_logs"] = journalLogs

	errors := []string{}
	journalLower := strings.ToLower(journalLogs)
	if strings.Contains(journalLower, "could not configure driver") {
		errors = append(errors, "Driver configuration error")
	}
	if strings.Contains(journalLower, "nl80211: could not") {
		errors = append(errors, "nl80211 driver error")
	}
	if strings.Contains(journalLower, "interface") && strings.Contains(journalLower, "not found") {
		errors = append(errors, "Interface not found")
	}
	if strings.Contains(journalLower, "failed to initialize") {
		errors = append(errors, "Initialization failed")
	}
	if strings.Contains(journalLower, "could not set channel") {
		errors = append(errors, "Channel configuration error")
	}

	diagnostics["errors"] = errors
	diagnostics["has_errors"] = len(errors) > 0

	ipOut, _ := exec.Command("sh", "-c", fmt.Sprintf("ip addr show %s 2>/dev/null | grep -i 'state UP' || echo ''", interfaceName)).CombinedOutput()
	interfaceUp := strings.Contains(strings.ToLower(string(ipOut)), "state up")
	diagnostics["interface_up"] = interfaceUp

	dnsmasqOut, _ := exec.Command("sh", "-c", "systemctl is-active dnsmasq 2>/dev/null || echo inactive").CombinedOutput()
	dnsmasqStatus := strings.TrimSpace(string(dnsmasqOut))
	diagnostics["dnsmasq_running"] = dnsmasqStatus == "active"

	diagnostics["status"] = func() string {
		if !serviceRunning {
			return "service_stopped"
		}
		if !transmitting {
			return "service_running_not_transmitting"
		}
		return "ok"
	}()

	return c.JSON(diagnostics)
}

func HostapdGetConfigHandler(c *fiber.Ctx) error {
	configPath := "/etc/hostapd/hostapd.conf"

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return c.JSON(fiber.Map{
			"success": false,
			"error":   "Configuration file not found",
			"config":  nil,
		})
	}

	configContent, err := os.ReadFile(configPath)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"error":   fmt.Sprintf("Error reading config file: %v", err),
			"config":  nil,
		})
	}

	config := make(map[string]string)
	lines := strings.Split(string(configContent), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			config[key] = value
		}
	}

	interfaceForDisplay := config["interface"]
	if interfaceForDisplay == "ap0" {
		interfaceForDisplay = "wlan0"
	}

	result := fiber.Map{
		"success": true,
		"config": fiber.Map{
			"interface": interfaceForDisplay, // Mostrar interfaz física al usuario
			"ssid":      config["ssid"],
			"channel":   config["channel"],
			"password":  config["wpa_passphrase"], // Devolver la contraseña para que el usuario pueda verla/editarla
		},
	}

	if config["auth_algs"] == "0" {
		result["config"].(fiber.Map)["security"] = "open"
	} else if strings.Contains(config["wpa_key_mgmt"], "SHA256") {
		result["config"].(fiber.Map)["security"] = "wpa3"
	} else if config["wpa"] == "2" {
		result["config"].(fiber.Map)["security"] = "wpa2"
	} else {
		result["config"].(fiber.Map)["security"] = "wpa2" // Por defecto
	}

	dnsmasqPath := "/etc/dnsmasq.conf"
	if dnsmasqContent, err := os.ReadFile(dnsmasqPath); err == nil {
		dnsmasqLines := strings.Split(string(dnsmasqContent), "\n")
		for _, line := range dnsmasqLines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "dhcp-option=3,") {
				gateway := strings.TrimPrefix(line, "dhcp-option=3,")
				result["config"].(fiber.Map)["gateway"] = gateway
			} else if strings.HasPrefix(line, "dhcp-range=") {
				rangeStr := strings.TrimPrefix(line, "dhcp-range=")
				parts := strings.Split(rangeStr, ",")
				if len(parts) >= 2 {
					result["config"].(fiber.Map)["dhcp_range_start"] = parts[0]
					result["config"].(fiber.Map)["dhcp_range_end"] = parts[1]
					if len(parts) >= 4 {
						result["config"].(fiber.Map)["lease_time"] = parts[3]
					}
				}
			}
		}
	}

	configMap := result["config"].(fiber.Map)
	if configMap["gateway"] == nil || configMap["gateway"] == "" {
		configMap["gateway"] = "192.168.4.1"
	}
	if configMap["dhcp_range_start"] == nil || configMap["dhcp_range_start"] == "" {
		configMap["dhcp_range_start"] = "192.168.4.2"
	}
	if configMap["dhcp_range_end"] == nil || configMap["dhcp_range_end"] == "" {
		configMap["dhcp_range_end"] = "192.168.4.254"
	}
	if configMap["lease_time"] == nil || configMap["lease_time"] == "" {
		configMap["lease_time"] = "12h"
	}
	if configMap["channel"] == nil || configMap["channel"] == "" {
		configMap["channel"] = "6"
	}

	countryCode := config["country_code"]
	if countryCode == "" {
		countryCode = config["country"] // Algunas configuraciones usan "country" en lugar de "country_code"
	}
	if countryCode == "" {
		countryCode = constants.DefaultCountryCode
	}
	configMap["country"] = countryCode

	return c.JSON(result)
}

func HostapdConfigHandler(c *fiber.Ctx) error {
	var req struct {
		Interface      string `json:"interface"`
		SSID           string `json:"ssid"`
		Password       string `json:"password"`
		Channel        int    `json:"channel"`
		Security       string `json:"security"`
		Gateway        string `json:"gateway"`
		DHCPRangeStart string `json:"dhcp_range_start"`
		DHCPRangeEnd   string `json:"dhcp_range_end"`
		LeaseTime      string `json:"lease_time"`
		Country        string `json:"country"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error":   "Invalid request body",
			"success": false,
		})
	}

	if req.Interface == "" || req.SSID == "" || req.Channel < 1 || req.Channel > 13 {
		return c.Status(400).JSON(fiber.Map{
			"error":   "Missing required fields: interface, ssid, channel",
			"success": false,
		})
	}

	if req.Gateway == "" {
		req.Gateway = "192.168.4.1"
	}
	if req.DHCPRangeStart == "" {
		req.DHCPRangeStart = "192.168.4.2"
	}
	if req.DHCPRangeEnd == "" {
		req.DHCPRangeEnd = "192.168.4.254"
	}
	if req.LeaseTime == "" {
		req.LeaseTime = "12h"
	}
	if req.Country == "" {
		req.Country = constants.DefaultCountryCode
	}

	if len(req.Country) != 2 {
		req.Country = "US"
	}
	req.Country = strings.ToUpper(req.Country)

	if req.Security != "wpa2" && req.Security != "wpa3" && req.Security != "open" {
		req.Security = "wpa2"
	}

	apInterface := "ap0"
	phyInterface := req.Interface // wlan0 o la interfaz física

	log.Printf("Configuring AP+STA mode: creating virtual interface %s from %s", apInterface, phyInterface)

	executeCommand(fmt.Sprintf("sudo ip link set %s up 2>/dev/null || true", phyInterface))
	time.Sleep(500 * time.Millisecond)

	phyName := ""
	phyCmd2 := fmt.Sprintf("cat /sys/class/net/%s/phy80211/name 2>/dev/null", phyInterface)
	phyOut2, _ := executeCommand(phyCmd2)
	phyName = strings.TrimSpace(phyOut2)

	if phyName == "" {
		phyCmd := fmt.Sprintf("iw dev %s info 2>/dev/null | grep 'wiphy' | awk '{print $2}'", phyInterface)
		phyOut, _ := executeCommand(phyCmd)
		phyName = strings.TrimSpace(phyOut)
	}

	if phyName == "" {
		phyCmd3 := fmt.Sprintf("iw phy | grep -B 5 '%s' | grep 'Wiphy' | awk '{print $2}' | head -1", phyInterface)
		phyOut3, _ := executeCommand(phyCmd3)
		phyName = strings.TrimSpace(phyOut3)
	}

	if phyName == "" {
		phyCmd4 := "iw list 2>/dev/null | grep 'Wiphy' | head -1 | awk '{print $2}'"
		phyOut4, _ := executeCommand(phyCmd4)
		phyName = strings.TrimSpace(phyOut4)
	}

	if phyName == "" {
		if strings.HasPrefix(phyInterface, "wlan") {
			if numStr := strings.TrimPrefix(phyInterface, "wlan"); numStr != "" {
				phyName = "phy" + numStr
				log.Printf("Trying phy name based on interface number: %s", phyName)
			}
		}
	}

	if phyName == "" {
		phyName = "phy0"
		log.Printf("Warning: Could not detect phy name, using default: %s", phyName)
	}

	log.Printf("Detected phy name: %s for interface %s", phyName, phyInterface)

	macAddress := ""
	macCmd := exec.Command("sh", "-c", fmt.Sprintf("cat /sys/class/net/%s/address 2>/dev/null", phyInterface))
	if macOut, err := macCmd.Output(); err == nil {
		macAddress = strings.TrimSpace(string(macOut))
	}
	if macAddress == "" {
		log.Printf("Warning: Could not get MAC address for %s", phyInterface)
		macAddress = "00:00:00:00:00:00" // Valor por defecto
	}

	log.Printf("Using phy: %s (MAC: %s) for virtual interface creation from %s", phyName, macAddress, phyInterface)

	if apInterface == "ap0" {
		log.Printf("Creating udev rule for automatic ap0 interface creation (TheWalrus method - Raspberry Pi 3 B+)")
		udevRulePath := "/etc/udev/rules.d/70-persistent-net.rules"

		checkCmd := exec.Command("sh", "-c", fmt.Sprintf("grep -q 'ap0' %s 2>/dev/null && echo 'exists' || echo 'not_exists'", udevRulePath))
		checkOut, _ := checkCmd.Output()
		if strings.TrimSpace(string(checkOut)) != "exists" {
			udevRuleContent := fmt.Sprintf(`# Regla para crear interfaz virtual ap0 automáticamente (método TheWalrus - Raspberry Pi 3 B+)
SUBSYSTEM=="ieee80211", ACTION=="add|change", ATTR{macaddress}=="%s", KERNEL=="%s", \
RUN+="/sbin/iw phy %s interface add ap0 type __ap", \
RUN+="/bin/ip link set ap0 address %s"
`, macAddress, phyName, phyName, macAddress)

			tmpUdevFile := "/tmp/70-persistent-net.rules.tmp"
			if err := os.WriteFile(tmpUdevFile, []byte(udevRuleContent), 0644); err == nil {
				if _, err := os.Stat(udevRulePath); err == nil {
					existingContent, _ := os.ReadFile(udevRulePath)
					combinedContent := string(existingContent) + "\n" + udevRuleContent
					os.WriteFile(tmpUdevFile, []byte(combinedContent), 0644)
				}
				executeCommand(fmt.Sprintf("sudo cp %s %s && sudo chmod 644 %s", tmpUdevFile, udevRulePath, udevRulePath))
				os.Remove(tmpUdevFile)
				log.Printf("Created udev rule for automatic ap0 creation (TheWalrus method - Raspberry Pi 3 B+)")
				executeCommand("sudo udevadm control --reload-rules 2>/dev/null || true")
				executeCommand("sudo udevadm trigger 2>/dev/null || true")
			} else {
				log.Printf("Warning: Could not create udev rule: %v", err)
			}
		} else {
			log.Printf("udev rule for ap0 already exists")
		}
	}

	checkApCmd := fmt.Sprintf("ip link show %s 2>/dev/null", apInterface)
	apExists := false
	checkOut, checkErr := executeCommand(checkApCmd)
	if checkErr == nil && strings.TrimSpace(checkOut) != "" {
		apExists = true
		log.Printf("Interface %s already exists, reusing it", apInterface)
		executeCommand(fmt.Sprintf("sudo ip link set %s up 2>/dev/null || true", apInterface))
	}

	if !apExists {
		log.Printf("Interface %s does not exist, creating it...", apInterface)

		delOut, delErr := executeCommand(fmt.Sprintf("sudo iw dev %s del 2>/dev/null || true", apInterface))
		if delErr == nil {
			log.Printf("Removed existing %s interface (if it existed): %s", apInterface, strings.TrimSpace(delOut))
		}
		time.Sleep(1 * time.Second)

		log.Printf("Creating virtual interface %s using phy %s...", apInterface, phyName)

		phyExistsCmd := fmt.Sprintf("iw phy %s info 2>/dev/null", phyName)
		phyExistsOut, phyExistsErr := executeCommand(phyExistsCmd)
		if phyExistsErr != nil || strings.TrimSpace(phyExistsOut) == "" {
			log.Printf("Warning: phy %s may not exist, output: %s", phyName, strings.TrimSpace(phyExistsOut))
		} else {
			log.Printf("phy %s exists and is accessible", phyName)
		}

		phyCheckCmd := fmt.Sprintf("iw phy %s info 2>/dev/null | grep -i 'AP'", phyName)
		phyCheckOut, _ := executeCommand(phyCheckCmd)
		if strings.TrimSpace(phyCheckOut) == "" {
			log.Printf("Warning: phy %s may not support AP mode, but attempting anyway", phyName)
		} else {
			log.Printf("phy %s supports AP mode: %s", phyName, strings.TrimSpace(phyCheckOut))
		}

		iwInfoCmd := fmt.Sprintf("iw dev %s info 2>/dev/null", phyInterface)
		iwInfoOut, _ := executeCommand(iwInfoCmd)
		if strings.Contains(iwInfoOut, "type AP") {
			log.Printf("Warning: Physical interface %s is in AP mode, changing to managed first", phyInterface)
			executeCommand(fmt.Sprintf("sudo iw dev %s set type managed 2>/dev/null", phyInterface))
			time.Sleep(1 * time.Second)
		}

		executeCommand(fmt.Sprintf("sudo ip link set %s up 2>/dev/null || true", phyInterface))
		time.Sleep(500 * time.Millisecond)

		createApCmd := fmt.Sprintf("sudo iw phy %s interface add %s type __ap 2>&1", phyName, apInterface)
		log.Printf("Executing: %s", createApCmd)
		createOut, createErr := executeCommand(createApCmd)
		if createOut != "" {
			log.Printf("Command output: %s", strings.TrimSpace(createOut))
		}

		if createErr != nil {
			log.Printf("Error creating virtual interface %s with phy %s: %s", apInterface, phyName, strings.TrimSpace(createOut))
			log.Printf("Error details: %v", createErr)
			log.Printf("Trying alternative method 1: using interface %s directly...", phyInterface)

			createApCmd2 := fmt.Sprintf("sudo iw dev %s interface add %s type __ap 2>&1", phyInterface, apInterface)
			log.Printf("Executing: %s", createApCmd2)
			createOut2, createErr2 := executeCommand(createApCmd2)
			if createOut2 != "" {
				log.Printf("Method 1 output: %s", strings.TrimSpace(createOut2))
			}

			if createErr2 != nil {
				log.Printf("Error with alternative method 1: %s", strings.TrimSpace(createOut2))
				log.Printf("Trying alternative method 2: using iw phy without sudo...")

				createApCmd3 := fmt.Sprintf("iw phy %s interface add %s type __ap 2>&1", phyName, apInterface)
				log.Printf("Executing: %s", createApCmd3)
				createOut3, createErr3 := executeCommand(createApCmd3)
				if createOut3 != "" {
					log.Printf("Method 2 output: %s", strings.TrimSpace(createOut3))
				}

				if createErr3 != nil {
					log.Printf("Error with alternative method 2: %s", strings.TrimSpace(createOut3))
					log.Printf("Trying alternative method 3: using mac80211_hwsim if available...")

					phyListCmd := "iw phy 2>/dev/null | grep 'Wiphy' | awk '{print $2}'"
					phyListOut, _ := executeCommand(phyListCmd)
					log.Printf("Available phys: %s", strings.TrimSpace(phyListOut))
					altPhyName := strings.TrimSpace(phyListOut)
					if altPhyName != "" && altPhyName != phyName {
						phyLines := strings.Split(altPhyName, "\n")
						if len(phyLines) > 0 {
							altPhyName = strings.TrimSpace(phyLines[0])
						}
						if altPhyName != "" && altPhyName != phyName {
							log.Printf("Trying with alternative phy: %s", altPhyName)
							createApCmd4 := fmt.Sprintf("sudo iw phy %s interface add %s type __ap 2>&1", altPhyName, apInterface)
							log.Printf("Executing: %s", createApCmd4)
							createOut4, createErr4 := executeCommand(createApCmd4)
							if createOut4 != "" {
								log.Printf("Method 3 output: %s", strings.TrimSpace(createOut4))
							}
							if createErr4 == nil {
								log.Printf("Successfully created interface %s using alternative phy %s", apInterface, altPhyName)
								apExists = true
								phyName = altPhyName // Actualizar phyName para uso posterior
							} else {
								log.Printf("Error with alternative phy: %s", strings.TrimSpace(createOut4))
								apInterface = phyInterface
								log.Printf("Falling back to using physical interface %s directly (non-concurrent mode)", apInterface)
							}
						} else {
							apInterface = phyInterface
							log.Printf("Falling back to using physical interface %s directly (non-concurrent mode)", apInterface)
						}
					} else {
						apInterface = phyInterface
						log.Printf("Falling back to using physical interface %s directly (non-concurrent mode)", apInterface)
					}
				} else {
					log.Printf("Successfully created interface %s using method 2 (without sudo)", apInterface)
					apExists = true
				}
			} else {
				log.Printf("Successfully created interface %s using alternative method 1 (from %s)", apInterface, phyInterface)
				apExists = true
			}
		} else {
			log.Printf("Successfully created interface %s using phy %s", apInterface, phyName)
			apExists = true
		}

		if apExists && apInterface == "ap0" {
			time.Sleep(2 * time.Second)

			verified := false
			for i := 0; i < 5; i++ {
				verifyCmd := fmt.Sprintf("ip link show %s 2>/dev/null", apInterface)
				verifyOut, verifyErr := executeCommand(verifyCmd)
				if verifyErr == nil && strings.TrimSpace(verifyOut) != "" {
					log.Printf("Interface %s verified successfully with ip link (attempt %d)", apInterface, i+1)
					verified = true
					break
				}

				lsCmd := fmt.Sprintf("ls /sys/class/net/ 2>/dev/null | grep -q '^%s$' && echo 'exists'", apInterface)
				lsOut, _ := executeCommand(lsCmd)
				if strings.TrimSpace(lsOut) == "exists" {
					log.Printf("Interface %s verified successfully with ls /sys/class/net (attempt %d)", apInterface, i+1)
					verified = true
					break
				}

				iwListCmd := fmt.Sprintf("iw dev 2>/dev/null | grep -q 'Interface %s' && echo 'exists'", apInterface)
				iwListOut, _ := executeCommand(iwListCmd)
				if strings.TrimSpace(iwListOut) == "exists" {
					log.Printf("Interface %s verified successfully with iw dev (attempt %d)", apInterface, i+1)
					verified = true
					break
				}

				if i < 4 {
					log.Printf("Verification attempt %d failed, retrying...", i+1)
					time.Sleep(1 * time.Second)
				}
			}

			if !verified {
				log.Printf("ERROR: Interface %s was NOT created successfully after all attempts", apInterface)
				log.Printf("Diagnostics:")
				log.Printf("  - phy name: %s", phyName)
				log.Printf("  - physical interface: %s", phyInterface)
				log.Printf("  - MAC address: %s", macAddress)

				log.Printf("Attempting manual creation as last resort...")
				manualCmd := fmt.Sprintf("sudo iw phy %s interface add %s type __ap 2>&1; sleep 1; ip link show %s 2>&1", phyName, apInterface, apInterface)
				manualOut, _ := executeCommand(manualCmd)
				log.Printf("Manual creation result: %s", strings.TrimSpace(manualOut))
			} else {
				log.Printf("SUCCESS: Interface %s created and verified", apInterface)
			}
		}
	}

	ipCmd := fmt.Sprintf("sudo ip addr add %s/24 dev %s 2>/dev/null || sudo ip addr replace %s/24 dev %s", req.Gateway, apInterface, req.Gateway, apInterface)
	if out, err := executeCommand(ipCmd); err != nil {
		log.Printf("Warning: Error setting IP on interface %s: %s", apInterface, strings.TrimSpace(out))
	}

	if out, err := executeCommand(fmt.Sprintf("sudo ip link set %s up", apInterface)); err != nil {
		log.Printf("Warning: Error bringing interface %s up: %s", apInterface, strings.TrimSpace(out))
	} else {
		log.Printf("Successfully created and activated virtual interface %s", apInterface)
		checkCmd := fmt.Sprintf("ip link show %s", apInterface)
		if checkOut, checkErr := executeCommand(checkCmd); checkErr == nil {
			log.Printf("Interface %s verified: %s", apInterface, strings.TrimSpace(checkOut))
		}
	}

	configPath := "/etc/hostapd/hostapd.conf"

	executeCommand("sudo mkdir -p /etc/hostapd 2>/dev/null || true")

	configContent := fmt.Sprintf(`interface=%s
driver=nl80211
ssid=%s
hw_mode=g
channel=%d
country_code=%s
`, apInterface, req.SSID, req.Channel, req.Country)

	if req.Security == "open" {
		configContent += "auth_algs=0\n"
	} else if req.Security == "wpa2" {
		if req.Password == "" {
			return c.Status(400).JSON(fiber.Map{
				"error":   "Password required for WPA2/WPA3",
				"success": false,
			})
		}
		configContent += fmt.Sprintf(`wpa=2
wpa_passphrase=%s
wpa_key_mgmt=WPA-PSK
wpa_pairwise=TKIP
rsn_pairwise=CCMP
`, req.Password)
	} else if req.Security == "wpa3" {
		if req.Password == "" {
			return c.Status(400).JSON(fiber.Map{
				"error":   "Password required for WPA2/WPA3",
				"success": false,
			})
		}
		configContent += fmt.Sprintf(`wpa=2
wpa_passphrase=%s
wpa_key_mgmt=WPA-PSK-SHA256
wpa_pairwise=CCMP
rsn_pairwise=CCMP
`, req.Password)
	}

	tmpFile := "/tmp/hostapd.conf.tmp"
	log.Printf("Creating temporary config file: %s", tmpFile)
	if err := os.WriteFile(tmpFile, []byte(configContent), 0644); err != nil {
		log.Printf("Error creating temporary config file: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error":   fmt.Sprintf("Error creating temporary config file: %v", err),
			"success": false,
		})
	}

	if _, err := os.Stat(tmpFile); os.IsNotExist(err) {
		log.Printf("Temporary file was not created: %s", tmpFile)
		return c.Status(500).JSON(fiber.Map{
			"error":   "Failed to create temporary config file",
			"success": false,
		})
	}

	log.Printf("Temporary file created successfully, size: %d bytes", len(configContent))

	if _, err := os.Stat(tmpFile); os.IsNotExist(err) {
		log.Printf("Temporary file does not exist before copy: %s", tmpFile)
		return c.Status(500).JSON(fiber.Map{
			"error":   "Temporary file was not created or was deleted",
			"success": false,
		})
	}

	if fileInfo, err := os.Stat(tmpFile); err == nil {
		log.Printf("Temporary file exists, size: %d bytes, mode: %v", fileInfo.Size(), fileInfo.Mode())
	} else {
		log.Printf("Cannot stat temporary file: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error":   fmt.Sprintf("Cannot access temporary file: %v", err),
			"success": false,
		})
	}

	log.Printf("Ensuring /etc/hostapd directory exists...")
	if out, err := executeCommand("sudo mkdir -p /etc/hostapd"); err != nil {
		log.Printf("Warning: Error creating /etc/hostapd directory: %v, output: %s", err, out)
	}
	if out, err := executeCommand("sudo chmod 755 /etc/hostapd"); err != nil {
		log.Printf("Warning: Error setting permissions on /etc/hostapd: %v, output: %s", err, out)
	}

	if _, err := os.Stat("/etc/hostapd"); os.IsNotExist(err) {
		log.Printf("Error: /etc/hostapd directory does not exist after creation attempt")
		return c.Status(500).JSON(fiber.Map{
			"error":   "Failed to create /etc/hostapd directory. Please run: sudo mkdir -p /etc/hostapd && sudo chmod 755 /etc/hostapd",
			"success": false,
		})
	}

	os.Chmod(tmpFile, 0644)

	cmdStr := fmt.Sprintf("sudo cp %s %s", tmpFile, configPath)
	log.Printf("Executing: %s", cmdStr)
	out, err := executeCommand(cmdStr)
	if err != nil {
		log.Printf("Error copying config file: %v, output: '%s'", err, out)
		os.Remove(tmpFile) // Limpiar archivo temporal
		errorMsg := strings.TrimSpace(out)
		if errorMsg == "" {
			errorMsg = err.Error()
		}
		return c.Status(500).JSON(fiber.Map{
			"error":   fmt.Sprintf("Error saving hostapd configuration: %s. Please check sudo permissions for cp command.", errorMsg),
			"success": false,
		})
	}
	log.Printf("File copied successfully, output: '%s'", strings.TrimSpace(out))

	chmodCmd := fmt.Sprintf("sudo chmod 644 %s", configPath)
	log.Printf("Setting permissions: %s", chmodCmd)
	if out, err := executeCommand(chmodCmd); err != nil {
		log.Printf("Warning: Error setting permissions: %v, output: %s", err, strings.TrimSpace(out))
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		os.Remove(tmpFile)
		log.Printf("Config file was not created at: %s", configPath)
		return c.Status(500).JSON(fiber.Map{
			"error":   fmt.Sprintf("Config file was not created at %s", configPath),
			"success": false,
		})
	}

	log.Printf("HostAPD config file created successfully at: %s", configPath)

	os.Remove(tmpFile)

	dnsmasqConfigPath := "/etc/dnsmasq.conf"
	executeCommand(fmt.Sprintf("sudo cp %s %s.backup 2>/dev/null || true", dnsmasqConfigPath, dnsmasqConfigPath))

	dnsmasqContent := fmt.Sprintf(`# Configuración de dnsmasq para modo AP+STA según método TheWalrus (Raspberry Pi 3 B+)
# Solo servir DHCP en ap0, no en wlan0 (que es STA)
interface=%s
no-dhcp-interface=%s
bind-interfaces
server=8.8.8.8
server=8.8.4.4
domain-needed
bogus-priv
dhcp-range=%s,%s,255.255.255.0,%s
dhcp-option=3,%s
dhcp-option=6,%s
`, apInterface, phyInterface, req.DHCPRangeStart, req.DHCPRangeEnd, req.LeaseTime, req.Gateway, req.Gateway)

	tmpDnsmasqFile := "/tmp/dnsmasq.conf.tmp"
	if err := os.WriteFile(tmpDnsmasqFile, []byte(dnsmasqContent), 0644); err != nil {
		log.Printf("Error creating temporary dnsmasq config file: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error":   fmt.Sprintf("Error creating temporary dnsmasq config file: %v", err),
			"success": false,
		})
	}

	os.Chmod(tmpDnsmasqFile, 0644)
	cmdStr2 := fmt.Sprintf("sudo cp %s %s && sudo chmod 644 %s", tmpDnsmasqFile, dnsmasqConfigPath, dnsmasqConfigPath)
	if out, err := executeCommand(cmdStr2); err != nil {
		os.Remove(tmpDnsmasqFile) // Limpiar archivo temporal
		log.Printf("Error writing dnsmasq config file: %s, output: %s", err, strings.TrimSpace(out))
		return c.Status(500).JSON(fiber.Map{
			"error":   fmt.Sprintf("Error saving dnsmasq configuration: %s", strings.TrimSpace(out)),
			"success": false,
		})
	}

	os.Remove(tmpDnsmasqFile)

	executeCommand("echo 1 | sudo tee /proc/sys/net/ipv4/ip_forward > /dev/null")
	executeCommand("sudo sysctl -w net.ipv4.ip_forward=1 > /dev/null 2>&1")

	sysctlCheckCmd := "grep -q '^net.ipv4.ip_forward=1' /etc/sysctl.conf || echo 'net.ipv4.ip_forward=1' | sudo tee -a /etc/sysctl.conf > /dev/null"
	executeCommand(sysctlCheckCmd)

	mainInterface := "eth0"
	if out, _ := executeCommand("ip route | grep default | awk '{print $5}' | head -1"); strings.TrimSpace(out) != "" {
		mainInterface = strings.TrimSpace(out)
	}

	apIPBegin := req.Gateway
	if lastDot := strings.LastIndex(req.Gateway, "."); lastDot > 0 {
		apIPBegin = req.Gateway[:lastDot]
	}

	if mainInterface != "" && mainInterface != apInterface {
		executeCommand(fmt.Sprintf("sudo iptables -t nat -D POSTROUTING -s %s.0/24 ! -d %s.0/24 -j MASQUERADE 2>/dev/null || true", apIPBegin, apIPBegin))
		executeCommand(fmt.Sprintf("sudo iptables -t nat -D POSTROUTING -o %s -j MASQUERADE 2>/dev/null || true", mainInterface))
		executeCommand(fmt.Sprintf("sudo iptables -D FORWARD -i %s -o %s -j ACCEPT 2>/dev/null || true", apInterface, mainInterface))
		executeCommand(fmt.Sprintf("sudo iptables -D FORWARD -i %s -o %s -m state --state RELATED,ESTABLISHED -j ACCEPT 2>/dev/null || true", mainInterface, apInterface))

		executeCommand(fmt.Sprintf("sudo iptables -t nat -A POSTROUTING -s %s.0/24 ! -d %s.0/24 -j MASQUERADE", apIPBegin, apIPBegin))
		executeCommand(fmt.Sprintf("sudo iptables -t nat -A POSTROUTING -o %s -j MASQUERADE", mainInterface))
		executeCommand(fmt.Sprintf("sudo iptables -A FORWARD -i %s -o %s -j ACCEPT", apInterface, mainInterface))
		executeCommand(fmt.Sprintf("sudo iptables -A FORWARD -i %s -o %s -m state --state RELATED,ESTABLISHED -j ACCEPT", mainInterface, apInterface))
	}

	overrideDir := "/etc/systemd/system/hostapd.service.d"
	executeCommand(fmt.Sprintf("sudo mkdir -p %s 2>/dev/null || true", overrideDir))
	overrideContent := fmt.Sprintf(`[Service]
ExecStart=
ExecStart=/usr/sbin/hostapd -B -P /run/hostapd.pid %s
PIDFile=/run/hostapd.pid
Type=forking
`, configPath)
	tmpOverrideFile := "/tmp/hostapd-override.conf.tmp"
	if err := os.WriteFile(tmpOverrideFile, []byte(overrideContent), 0644); err != nil {
		log.Printf("Warning: Error creating temporary override file: %v", err)
	} else {
		overridePath := fmt.Sprintf("%s/override.conf", overrideDir)
		os.Chmod(tmpOverrideFile, 0644)
		cmdStr3 := fmt.Sprintf("sudo cp %s %s && sudo chmod 644 %s", tmpOverrideFile, overridePath, overridePath)
		if out, err := executeCommand(cmdStr3); err != nil {
			log.Printf("Warning: Error writing override file: %s, output: %s", err, strings.TrimSpace(out))
		} else {
			log.Printf("Override file created successfully")
		}
		os.Remove(tmpOverrideFile)
	}

	manageAp0Script := `#!/bin/bash
# check if hostapd service success to start or not
# in our case, it cannot start when /var/run/hostapd/ap0 exist
# so we have to delete it
echo 'Check if hostapd.service is hang cause ap0 exist...'
hostapd_is_running=$(systemctl is-active hostapd 2>/dev/null | grep -c "active")
if test 1 -ne "${hostapd_is_running}"; then
    rm -rf /var/run/hostapd/ap0 || echo "ap0 interface does not exist, the failure is elsewhere"
    # También limpiar el PID file si existe
    rm -f /run/hostapd.pid || true
fi
# Asegurar que ap0 existe antes de iniciar hostapd
if ! ip link show ap0 > /dev/null 2>&1; then
    # Intentar crear ap0 si no existe
    phy=$(iw dev wlan0 info 2>/dev/null | grep wiphy | awk '{print $2}')
    if [ -n "$phy" ]; then
        iw phy $phy interface add ap0 type __ap 2>/dev/null || true
        sleep 1
    fi
fi
`
	manageAp0Path := "/bin/manage-ap0-iface.sh"
	tmpManageAp0 := "/tmp/manage-ap0-iface.sh.tmp"
	if err := os.WriteFile(tmpManageAp0, []byte(manageAp0Script), 0755); err == nil {
		executeCommand(fmt.Sprintf("sudo cp %s %s && sudo chmod +x %s", tmpManageAp0, manageAp0Path, manageAp0Path))
		os.Remove(tmpManageAp0)
		log.Printf("Created manage-ap0-iface.sh script")
	}

	overridePath := fmt.Sprintf("%s/override.conf", overrideDir)
	overrideContentWithPreStart := fmt.Sprintf(`[Service]
ExecStart=
ExecStartPre=/bin/manage-ap0-iface.sh
ExecStart=/usr/sbin/hostapd -B -P /run/hostapd.pid %s
PIDFile=/run/hostapd.pid
Type=forking
TimeoutStartSec=30
TimeoutStopSec=10
`, configPath)
	tmpOverrideFile2 := "/tmp/hostapd-override.conf.tmp"
	if err := os.WriteFile(tmpOverrideFile2, []byte(overrideContentWithPreStart), 0644); err == nil {
		cmdStr4 := fmt.Sprintf("sudo cp %s %s && sudo chmod 644 %s", tmpOverrideFile2, overridePath, overridePath)
		if out, err := executeCommand(cmdStr4); err != nil {
			log.Printf("Warning: Error updating override file with pre-start: %s, output: %s", err, strings.TrimSpace(out))
		} else {
			log.Printf("Override file updated with pre-start script and PID file configuration")
		}
		os.Remove(tmpOverrideFile2)
	}

	executeCommand("sudo systemctl daemon-reload")

	apIPBeginForScript := req.Gateway
	if lastDot := strings.LastIndex(req.Gateway, "."); lastDot > 0 {
		apIPBeginForScript = req.Gateway[:lastDot]
	}
	rpiWifiScript := fmt.Sprintf(`#!/bin/bash
echo 'Starting Wifi AP and STA client...'
/usr/sbin/ifdown --force %s 2>/dev/null || true
/usr/sbin/ifdown --force %s 2>/dev/null || true
/usr/sbin/ifup --force %s 2>/dev/null || true
/usr/sbin/ifup --force %s 2>/dev/null || true
/usr/sbin/sysctl -w net.ipv4.ip_forward=1 > /dev/null 2>&1
/usr/sbin/iptables -t nat -A POSTROUTING -s %s.0/24 ! -d %s.0/24 -j MASQUERADE 2>/dev/null || true
/usr/bin/systemctl restart dnsmasq 2>/dev/null || true
echo 'WPA Supplicant reconfigure in 5sec...'
/usr/bin/sleep 5
wpa_cli -i %s reconfigure 2>/dev/null || true
`, phyInterface, apInterface, apInterface, phyInterface, apIPBeginForScript, apIPBeginForScript, phyInterface)
	rpiWifiPath := "/bin/rpi-wifi.sh"
	tmpRpiWifi := "/tmp/rpi-wifi.sh.tmp"
	if err := os.WriteFile(tmpRpiWifi, []byte(rpiWifiScript), 0755); err == nil {
		executeCommand(fmt.Sprintf("sudo cp %s %s && sudo chmod +x %s", tmpRpiWifi, rpiWifiPath, rpiWifiPath))
		os.Remove(tmpRpiWifi)
		log.Printf("Created rpi-wifi.sh script")
	}

	executeCommand("sudo systemctl enable hostapd 2>/dev/null || true")
	executeCommand("sudo systemctl enable dnsmasq 2>/dev/null || true")

	if out, err := executeCommand("sudo systemctl restart dnsmasq"); err != nil {
		log.Printf("Warning: Error restarting dnsmasq: %s", strings.TrimSpace(out))
	}

	if apInterface == "ap0" {
		ap0CheckCmd := "ip link show ap0 2>/dev/null"
		ap0CheckOut, ap0CheckErr := executeCommand(ap0CheckCmd)
		if ap0CheckErr != nil || strings.TrimSpace(ap0CheckOut) == "" {
			log.Printf("Warning: ap0 interface does not exist, attempting to create it before starting hostapd")
			createAp0Cmd := fmt.Sprintf("sudo iw phy %s interface add ap0 type __ap 2>&1", phyName)
			createAp0Out, _ := executeCommand(createAp0Cmd)
			if createAp0Out != "" {
				log.Printf("ap0 creation attempt: %s", strings.TrimSpace(createAp0Out))
			}
			time.Sleep(1 * time.Second)
			ap0CheckOut2, _ := executeCommand(ap0CheckCmd)
			if strings.TrimSpace(ap0CheckOut2) == "" {
				log.Printf("ERROR: ap0 interface still does not exist after creation attempt")
				return c.Status(500).JSON(fiber.Map{
					"error":   "Failed to create ap0 interface. Please check WiFi hardware and drivers.",
					"success": false,
				})
			}
		}
	}

	executeCommand("sudo rm -rf /var/run/hostapd/ap0 2>/dev/null || true")
	executeCommand("sudo rm -f /run/hostapd.pid 2>/dev/null || true")

	if out, err := executeCommand("sudo systemctl restart hostapd"); err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error":   fmt.Sprintf("Configuration saved but failed to restart hostapd: %s", strings.TrimSpace(out)),
			"success": false,
		})
	}

	time.Sleep(2 * time.Second)
	hostapdStatusCmd := "systemctl is-active hostapd 2>/dev/null"
	hostapdStatusOut, _ := executeCommand(hostapdStatusCmd)
	if strings.TrimSpace(hostapdStatusOut) != "active" {
		log.Printf("Warning: hostapd service may not be active after restart. Status: %s", strings.TrimSpace(hostapdStatusOut))
		pgrepOut, _ := executeCommand("pgrep hostapd 2>/dev/null && echo 'running' || echo 'not running'")
		log.Printf("hostapd process check: %s", strings.TrimSpace(pgrepOut))
	}

	// Asegurar que ap0 tenga la IP del gateway y reiniciar dnsmasq para que los clientes reciban IP por DHCP
	executeCommand("sudo ip addr add 192.168.4.1/24 dev ap0 2>/dev/null || true")
	if out, err := executeCommand("sudo systemctl restart dnsmasq 2>/dev/null"); err != nil {
		log.Printf("Warning: dnsmasq restart after hostapd config: %s", strings.TrimSpace(out))
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Configuration saved and services restarted",
	})
}

