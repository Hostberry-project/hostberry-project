package network

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"hostberry/internal/config"
	"hostberry/internal/i18n"
)

func NetworkInterfacesHandler(c *fiber.Ctx) error {
	result := GetNetworkInterfaces()
	if result != nil {
		if interfaces, ok := result["interfaces"]; ok {
			if interfacesArray, ok := interfaces.([]map[string]interface{}); ok && len(interfacesArray) > 0 {
				if config.AppConfig.Server.Debug {
					i18n.LogTf("logs.handlers_interfaces_count", len(interfacesArray))
				}
				return c.JSON(result)
			}
		}
	}

	interfaces := []map[string]interface{}{}

	cmd := exec.Command("sh", "-c", "ip -o link show | awk -F': ' '{print $2}'")
	output, err := cmd.Output()
	if err != nil {
		i18n.LogTf("logs.handlers_interfaces_error", err)
		return c.JSON(fiber.Map{"interfaces": interfaces})
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	i18n.LogTf("logs.handlers_interfaces_found", lines)
	for _, ifaceName := range lines {
		ifaceName = strings.TrimSpace(ifaceName)
		if ifaceName == "" || ifaceName == "lo" {
			continue // Saltar loopback
		}

		ifaceCheckCmd := exec.Command("sh", "-c", fmt.Sprintf("ip link show %s 2>/dev/null", ifaceName))
		if ifaceCheckErr := ifaceCheckCmd.Run(); ifaceCheckErr != nil {
			i18n.LogTf("logs.handlers_interface_skip", ifaceName)
			continue
		}

		i18n.LogTf("logs.handlers_interface_processing", ifaceName)

		iface := map[string]interface{}{
			"name": ifaceName,
			"ip":   "N/A",
			"mac":  "N/A",
			"state": "unknown",
		}

		stateCmd := exec.Command("sh", "-c", fmt.Sprintf("cat /sys/class/net/%s/operstate 2>/dev/null", ifaceName))
		if stateOut, err := stateCmd.Output(); err == nil {
			state := strings.TrimSpace(string(stateOut))
			if state == "" {
				ipStateCmd := exec.Command("sh", "-c", fmt.Sprintf("ip link show %s 2>/dev/null | grep -o 'state [A-Z]*' | awk '{print $2}'", ifaceName))
				if ipStateOut, ipStateErr := ipStateCmd.Output(); ipStateErr == nil {
					state = strings.TrimSpace(string(ipStateOut))
				}
				if state == "" {
					state = "unknown"
				}
			}
			iface["state"] = state
		} else {
			ipStateCmd := exec.Command("sh", "-c", fmt.Sprintf("ip link show %s 2>/dev/null | grep -o 'state [A-Z]*' | awk '{print $2}'", ifaceName))
			if ipStateOut, ipStateErr := ipStateCmd.Output(); ipStateErr == nil {
				state := strings.TrimSpace(string(ipStateOut))
				if state != "" {
					iface["state"] = state
				}
			}
		}

		if ifaceName == "ap0" {
			i18n.LogTf("logs.handlers_ap0_found", iface["state"])
			if iface["state"] == "down" || iface["state"] == "unknown" {
				i18n.LogT("logs.handlers_ap0_down")
				activateCmd := exec.Command("sh", "-c", "sudo ip link set ap0 up 2>/dev/null")
				if activateErr := activateCmd.Run(); activateErr == nil {
					time.Sleep(500 * time.Millisecond)
					stateCmd2 := exec.Command("sh", "-c", "cat /sys/class/net/ap0/operstate 2>/dev/null")
					if stateOut2, err2 := stateCmd2.Output(); err2 == nil {
						newState := strings.TrimSpace(string(stateOut2))
						if newState != "" {
							iface["state"] = newState
							i18n.LogTf("logs.handlers_ap0_activated", newState)
						}
					}
				}
			}
		}

		if strings.HasPrefix(ifaceName, "wlan") {
			wpaStatusCmd := exec.Command("sh", "-c", fmt.Sprintf("sudo wpa_cli -i %s status 2>/dev/null | grep 'wpa_state=' | cut -d= -f2", ifaceName))
			if wpaStateOut, err := wpaStatusCmd.Output(); err == nil {
				wpaState := strings.TrimSpace(string(wpaStateOut))
				if wpaState == "COMPLETED" {
					iface["wpa_state"] = "COMPLETED"
				} else if wpaState == "ASSOCIATING" || wpaState == "ASSOCIATED" || wpaState == "4WAY_HANDSHAKE" || wpaState == "GROUP_HANDSHAKE" {
					iface["wpa_state"] = wpaState
					iface["state"] = "connecting"
				} else {
					iface["wpa_state"] = wpaState
					if iface["state"] == "up" {
						iface["state"] = "down"
					}
				}
			}
		}

		ipCmd := exec.Command("sh", "-c", fmt.Sprintf("ip addr show %s 2>/dev/null | grep 'inet ' | awk '{print $2}' | head -1", ifaceName))
		if ipOut, err := ipCmd.Output(); err == nil {
			ipLine := strings.TrimSpace(string(ipOut))
			if ipLine != "" {
				parts := strings.Split(ipLine, "/")
				iface["ip"] = parts[0]
				if len(parts) > 1 {
					iface["netmask"] = parts[1]
				}
			}
		}

		if (iface["ip"] == "N/A" || iface["ip"] == "") {
			ipCmdSudo := exec.Command("sh", "-c", fmt.Sprintf("sudo ip addr show %s 2>/dev/null | grep 'inet ' | awk '{print $2}' | head -1", ifaceName))
			if ipOutSudo, err := ipCmdSudo.Output(); err == nil {
				ipLineSudo := strings.TrimSpace(string(ipOutSudo))
				if ipLineSudo != "" {
					parts := strings.Split(ipLineSudo, "/")
					iface["ip"] = parts[0]
					if len(parts) > 1 {
						iface["netmask"] = parts[1]
					}
				}
			}
		}

		if iface["ip"] == "N/A" || iface["ip"] == "" {
			ifconfigCmd := exec.Command("sh", "-c", fmt.Sprintf("ifconfig %s 2>/dev/null | grep 'inet ' | awk '{print $2}' | head -1", ifaceName))
			if ifconfigOut, err := ifconfigCmd.Output(); err == nil {
				ifconfigLine := strings.TrimSpace(string(ifconfigOut))
				if ifconfigLine != "" {
					ifconfigLine = strings.TrimPrefix(ifconfigLine, "addr:")
					iface["ip"] = ifconfigLine
				}
			}
		}

		if iface["ip"] == "N/A" || iface["ip"] == "" {
			hostnameCmd := exec.Command("sh", "-c", "hostname -I 2>/dev/null | awk '{print $1}'")
			if hostnameOut, err := hostnameCmd.Output(); err == nil {
				hostnameIP := strings.TrimSpace(string(hostnameOut))
				if hostnameIP != "" {
					checkCmd := exec.Command("sh", "-c", fmt.Sprintf("ip addr show %s 2>/dev/null | grep -q '%s' && echo '%s'", ifaceName, hostnameIP, hostnameIP))
					if checkOut, err := checkCmd.Output(); err == nil {
						checkIP := strings.TrimSpace(string(checkOut))
						if checkIP != "" {
							iface["ip"] = checkIP
						}
					}
				}
			}
		}

		if (iface["state"] == "up" || iface["state"] == "connected" || iface["state"] == "connecting") && (iface["ip"] == "N/A" || iface["ip"] == "") {
			dhcpCheck := exec.Command("sh", "-c", fmt.Sprintf("ps aux | grep -E '[d]hclient|udhcpc' | grep %s", ifaceName))
			if dhcpOut, err := dhcpCheck.Output(); err == nil {
				dhcpLine := strings.TrimSpace(string(dhcpOut))
				if dhcpLine != "" {
					iface["ip"] = "Obtaining IP..."
				}
			}
		}

		gatewayCmd := exec.Command("sh", "-c", fmt.Sprintf("ip route | grep %s | grep default | awk '{print $3}' | head -1", ifaceName))
		if gatewayOut, err := gatewayCmd.Output(); err == nil {
			gateway := strings.TrimSpace(string(gatewayOut))
			if gateway != "" {
				iface["gateway"] = gateway
			}
		}

		if _, hasGateway := iface["gateway"]; !hasGateway {
			defaultGatewayCmd := exec.Command("sh", "-c", "ip route | grep default | awk '{print $3}' | head -1")
			if defaultGatewayOut, err := defaultGatewayCmd.Output(); err == nil {
				defaultGateway := strings.TrimSpace(string(defaultGatewayOut))
				if defaultGateway != "" {
					iface["gateway"] = defaultGateway
				}
			}
		}

		isAPMode := false
		if iface["ip"] != "N/A" && iface["ip"] != "" && iface["ip"] != "Obtaining IP..." {
			ipStr, ok := iface["ip"].(string)
			if !ok {
				ipStr = fmt.Sprintf("%v", iface["ip"])
			}
			gatewayStr := ""
			if iface["gateway"] != nil {
				if gw, ok := iface["gateway"].(string); ok {
					gatewayStr = gw
				} else {
					gatewayStr = fmt.Sprintf("%v", iface["gateway"])
				}
			}
			if ipStr == "192.168.4.1" || (strings.HasPrefix(ipStr, "192.168.4.") && (gatewayStr == "" || gatewayStr == "192.168.4.1")) {
				hostapdCheck := exec.Command("sh", "-c", "systemctl is-active hostapd 2>/dev/null || pgrep hostapd > /dev/null && echo active || echo inactive")
				if hostapdOut, err := hostapdCheck.Output(); err == nil {
					if strings.TrimSpace(string(hostapdOut)) == "active" {
						isAPMode = true
						iface["ap_mode"] = true
					}
				}
			}
		}

		if strings.HasPrefix(ifaceName, "wlan") {
			if isAPMode {
				iface["connected"] = false
				iface["state"] = "ap_mode"
				iface["internet_connected"] = false
			} else if wpaState, hasWpaState := iface["wpa_state"]; hasWpaState && wpaState == "COMPLETED" {
				if iface["ip"] == "N/A" || iface["ip"] == "" || iface["ip"] == "Obtaining IP..." {
					iface["connected"] = false
					iface["state"] = "connecting"
					iface["internet_connected"] = false
				} else {
					iface["connected"] = true
					iface["state"] = "connected"
					hasInternet := false
					ipStr, ok := iface["ip"].(string)
					if !ok {
						ipStr = fmt.Sprintf("%v", iface["ip"])
					}

					gatewayStr := ""
					if iface["gateway"] != nil {
						if gw, ok := iface["gateway"].(string); ok {
							gatewayStr = gw
						} else {
							gatewayStr = fmt.Sprintf("%v", iface["gateway"])
						}
					}

					if gatewayStr == "" {
						defaultGatewayCmd := exec.Command("sh", "-c", "ip route | grep default | awk '{print $3}' | head -1")
						if defaultGatewayOut, err := defaultGatewayCmd.Output(); err == nil {
							defaultGateway := strings.TrimSpace(string(defaultGatewayOut))
							if defaultGateway != "" {
								gatewayStr = defaultGateway
								iface["gateway"] = defaultGateway
							}
						}
					}

					if !strings.HasPrefix(ipStr, "192.168.4.") && gatewayStr != "" && gatewayStr != "192.168.4.1" {
						hasInternet = true
					} else if strings.HasPrefix(ipStr, "192.168.4.") {
						hasInternet = false
					} else {
						pingCmd := exec.Command("sh", "-c", fmt.Sprintf("timeout 2 ping -c 1 -W 1 8.8.8.8 > /dev/null 2>&1 && echo 'ok' || echo 'fail'"))
						if pingOut, err := pingCmd.Output(); err == nil {
							if strings.TrimSpace(string(pingOut)) == "ok" {
								hasInternet = true
							}
						}
						if !hasInternet && !strings.HasPrefix(ipStr, "192.168.4.") && ipStr != "" {
							hasInternet = true
						}
					}
					iface["internet_connected"] = hasInternet
				}
			} else if wpaState, hasWpaState := iface["wpa_state"]; hasWpaState && (wpaState == "ASSOCIATING" || wpaState == "ASSOCIATED" || wpaState == "4WAY_HANDSHAKE" || wpaState == "GROUP_HANDSHAKE") {
				iface["connected"] = false
				iface["state"] = "connecting"
				iface["internet_connected"] = false
			} else {
				iface["connected"] = false
				iface["internet_connected"] = false
				if iface["state"] != "down" {
					iface["state"] = "down"
				}
			}
		} else {
			if iface["ip"] != "N/A" && iface["ip"] != "" && iface["ip"] != "Obtaining IP..." {
				iface["connected"] = true
				if iface["state"] == "up" {
					iface["state"] = "connected"
				}
				hasInternet := false
				ipStr, ok := iface["ip"].(string)
				if !ok {
					ipStr = fmt.Sprintf("%v", iface["ip"])
				}

				gatewayStr := ""
				if iface["gateway"] != nil {
					if gw, ok := iface["gateway"].(string); ok {
						gatewayStr = gw
					} else {
						gatewayStr = fmt.Sprintf("%v", iface["gateway"])
					}
				}

				if gatewayStr == "" {
					defaultGatewayCmd := exec.Command("sh", "-c", "ip route | grep default | awk '{print $3}' | head -1")
					if defaultGatewayOut, err := defaultGatewayCmd.Output(); err == nil {
						defaultGateway := strings.TrimSpace(string(defaultGatewayOut))
						if defaultGateway != "" {
							gatewayStr = defaultGateway
							iface["gateway"] = defaultGateway
						}
					}
				}

				if strings.HasPrefix(ipStr, "192.168.4.") {
					hasInternet = false
				} else if !strings.HasPrefix(ipStr, "192.168.4.") && ipStr != "" {
					hasInternet = true

					if gatewayStr != "" && gatewayStr != "192.168.4.1" {
						pingCmd := exec.Command("sh", "-c", fmt.Sprintf("timeout 2 ping -c 1 -W 1 8.8.8.8 > /dev/null 2>&1 && echo 'ok' || echo 'fail'"))
						if pingOut, err := pingCmd.Output(); err == nil {
							if strings.TrimSpace(string(pingOut)) == "ok" {
								hasInternet = true
							} else {
								hasInternet = true
							}
						}
					}
				} else {
					hasInternet = false
				}
				iface["internet_connected"] = hasInternet
			} else {
				iface["connected"] = false
				iface["internet_connected"] = false
			}
		}

		macCmd := exec.Command("sh", "-c", fmt.Sprintf("cat /sys/class/net/%s/address 2>/dev/null", ifaceName))
		if macOut, err := macCmd.Output(); err == nil {
			mac := strings.TrimSpace(string(macOut))
			if mac != "" {
				iface["mac"] = mac
			}
		}

		interfaces = append(interfaces, iface)
	}

	i18n.LogTf("logs.handlers_fallback_interfaces", len(interfaces))
	return c.JSON(fiber.Map{"interfaces": interfaces})
}

