package wifi

import (
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
)

func WifiLegacyStatusHandler(c *fiber.Ctx) error {
	var enabled bool = false
	var hardBlocked bool = false
	var softBlocked bool = false

	wifiCheck := execCommand("nmcli -t -f WIFI g 2>/dev/null")
	wifiOut, err := wifiCheck.Output()
	if err == nil {
		wifiState := strings.ToLower(strings.TrimSpace(filterSudoErrors(wifiOut)))
		if strings.Contains(wifiState, "enabled") || strings.Contains(wifiState, "on") {
			enabled = true
		} else if strings.Contains(wifiState, "disabled") || strings.Contains(wifiState, "off") {
			enabled = false
		}
	}

	rfkillOut, _ := execCommand("rfkill list wifi 2>/dev/null").CombinedOutput()
	rfkillStr := strings.ToLower(filterSudoErrors(rfkillOut))
	if strings.Contains(rfkillStr, "hard blocked: yes") {
		hardBlocked = true
		enabled = false
	} else if strings.Contains(rfkillStr, "soft blocked: yes") {
		softBlocked = true
		enabled = false
	} else {
		iwOut, _ := execCommand("iwconfig 2>/dev/null | grep -i 'wlan' | head -1").CombinedOutput()
		cleanIwOut := filterSudoErrors(iwOut)
		if len(cleanIwOut) > 0 {
			iwStatus, _ := execCommand("iwconfig 2>/dev/null | grep -i 'wlan' | head -1 | grep -i 'unassociated'").CombinedOutput()
			cleanIwStatus := filterSudoErrors(iwStatus)
			if len(cleanIwStatus) == 0 {
				enabled = true
			}
		} else {
			ipCheck := exec.Command("sh", "-c", "ip link show | grep -E '^[0-9]+: wlan' | grep -i 'state UP'")
			if ipOut, err := ipCheck.Output(); err == nil && len(ipOut) > 0 {
				enabled = true
			}
		}
	}

	ssid := ""
	connected := false
	iface := "wlan0"

	ipIfaceCmd := exec.Command("sh", "-c", "ip -o link show | awk -F': ' '{print $2}' | grep -E '^wlan|^wl' | head -1")
	if ipIfaceOut, err := ipIfaceCmd.Output(); err == nil {
		if ipIfaceStr := strings.TrimSpace(string(ipIfaceOut)); ipIfaceStr != "" {
			iface = ipIfaceStr
		}
	}

	wpaStatusCmd := exec.Command("sh", "-c", fmt.Sprintf("sudo wpa_cli -i %s status 2>/dev/null", iface))
	wpaStatusOut, wpaErr := wpaStatusCmd.CombinedOutput()
	if wpaErr == nil && len(wpaStatusOut) > 0 {
		wpaStatus := string(wpaStatusOut)
		for _, line := range strings.Split(wpaStatus, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "ssid=") {
				ssid = strings.TrimPrefix(line, "ssid=")
				if ssid != "" {
					if strings.Contains(wpaStatus, "wpa_state=COMPLETED") {
						connected = true
					}
				}
				break
			}
		}
	}

	if !connected || ssid == "" {
		iwLinkCmd := exec.Command("sh", "-c", fmt.Sprintf("iw dev %s link 2>/dev/null", iface))
		iwLinkOut, iwErr := iwLinkCmd.CombinedOutput()
		if iwErr == nil && len(iwLinkOut) > 0 {
			iwLink := string(iwLinkOut)
			for _, line := range strings.Split(iwLink, "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "Connected to ") {
					connected = true
				} else if strings.Contains(line, "SSID:") {
					ssid = strings.TrimSpace(strings.TrimPrefix(line, "SSID:"))
					if ssid != "" {
						connected = true
					}
				}
			}
		}
	}

	if !connected || ssid == "" {
		iwOut, _ := execCommand("iwconfig 2>/dev/null | grep -i 'essid' | grep -v 'off/any' | head -1").CombinedOutput()
		iwStr := filterSudoErrors(iwOut)
		if strings.Contains(iwStr, "ESSID:") {
			parts := strings.Split(iwStr, "ESSID:")
			if len(parts) > 1 {
				ssid = strings.TrimSpace(strings.Trim(parts[1], "\""))
				if ssid != "" && ssid != "off/any" {
					connected = true
				}
			}
		}
	}

	reallyConnected := false
	if connected && ssid != "" {
		// Verificar si wpa_state es COMPLETED (autenticado)
		wpaStateCompleted := false
		if wpaErr == nil && len(wpaStatusOut) > 0 {
			wpaStatus := string(wpaStatusOut)
			if strings.Contains(wpaStatus, "wpa_state=COMPLETED") {
				wpaStateCompleted = true
			}
		}

		ipCheckCmd := exec.Command("sh", "-c", fmt.Sprintf("ip addr show %s 2>/dev/null | grep 'inet ' | awk '{print $2}' | cut -d/ -f1 | head -1", iface))
		ipOut, ipErr := ipCheckCmd.Output()
		if ipErr == nil {
			ip := strings.TrimSpace(string(ipOut))
			if ip != "" && ip != "N/A" && !strings.HasPrefix(ip, "169.254") {
				reallyConnected = true
				log.Printf("WiFi realmente conectado: SSID=%s, IP=%s", ssid, ip)
			} else {
				// Si wpa_state es COMPLETED, considerar conectado aunque no tenga IP aún
				if wpaStateCompleted {
					reallyConnected = true
					log.Printf("WiFi autenticado (wpa_state=COMPLETED) pero sin IP aún: SSID=%s", ssid)
					// Intentar obtener IP si no hay proceso DHCP corriendo
					dhcpCheck := exec.Command("sh", "-c", fmt.Sprintf("ps aux | grep -E '[d]hclient|udhcpc' | grep %s", iface))
					if dhcpOut, _ := dhcpCheck.Output(); len(dhcpOut) == 0 {
						log.Printf("Iniciando DHCP para obtener IP...")
						executeCommand(fmt.Sprintf("sudo dhclient -v %s 2>&1 || sudo udhcpc -i %s -q -n 2>&1 || true", iface, iface))
					} else {
						log.Printf("WiFi está obteniendo IP (DHCP en proceso)")
					}
				} else {
					log.Printf("WiFi tiene SSID pero no está completamente autenticado: SSID=%s, IP=%s", ssid, ip)
				}
			}
		} else if wpaStateCompleted {
			// Si wpa_state es COMPLETED pero no se pudo verificar IP, considerar conectado
			reallyConnected = true
			log.Printf("WiFi autenticado (wpa_state=COMPLETED): SSID=%s", ssid)
		}
	}

	var connectionInfo fiber.Map = nil
	if reallyConnected && ssid != "" {
		connectionInfo = fiber.Map{
			"ssid": ssid,
		}

		iface := "wlan0"
		ipIfaceCmd := exec.Command("sh", "-c", "ip -o link show | awk -F': ' '{print $2}' | grep -E '^wlan|^wl' | head -1")
		if ipIfaceOut, err := ipIfaceCmd.Output(); err == nil {
			if ipIfaceStr := strings.TrimSpace(string(ipIfaceOut)); ipIfaceStr != "" {
				iface = ipIfaceStr
			}
		}

		wpaStatusCmd := exec.Command("sh", "-c", fmt.Sprintf("sudo wpa_cli -i %s status 2>/dev/null", iface))
		wpaStatusOut, wpaErr := wpaStatusCmd.CombinedOutput()
		if wpaErr == nil && len(wpaStatusOut) > 0 {
			wpaStatus := string(wpaStatusOut)
			log.Printf("wpa_cli status output for %s: %s", iface, wpaStatus)
			for _, line := range strings.Split(wpaStatus, "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "signal=") {
					signalStr := strings.TrimPrefix(line, "signal=")
					signalStr = strings.TrimSpace(signalStr)
					if signalStr != "" && signalStr != "0" {
						if signalInt, err := strconv.Atoi(signalStr); err == nil && signalInt != 0 {
							if signalInt > 0 {
								signalInt = -signalInt
							}
							if signalInt >= -100 && signalInt <= -30 {
								signalStr = strconv.Itoa(signalInt)
								connectionInfo["signal"] = signalStr
								log.Printf("Found signal from wpa_cli: %s dBm", signalStr)
							} else {
								log.Printf("Signal out of range from wpa_cli: %d dBm (ignoring)", signalInt)
							}
						}
					}
				} else if strings.HasPrefix(line, "key_mgmt=") {
					keyMgmt := strings.TrimPrefix(line, "key_mgmt=")
					keyMgmt = strings.TrimSpace(keyMgmt)
					if keyMgmt != "" {
						keyMgmtUpper := strings.ToUpper(keyMgmt)
						if strings.Contains(keyMgmtUpper, "WPA3") || strings.Contains(keyMgmtUpper, "SAE") {
							connectionInfo["security"] = "WPA3"
						} else if strings.Contains(keyMgmtUpper, "WPA2") || strings.Contains(keyMgmtUpper, "WPA-PSK") || strings.Contains(keyMgmtUpper, "WPA") {
							connectionInfo["security"] = "WPA2"
						} else if strings.Contains(keyMgmtUpper, "NONE") || keyMgmtUpper == "" {
							connectionInfo["security"] = "Open"
						} else {
							if strings.Contains(keyMgmtUpper, "PSK") {
								connectionInfo["security"] = "WPA2"
							} else {
								connectionInfo["security"] = keyMgmt
							}
						}
						log.Printf("Found security from wpa_cli: %s (key_mgmt=%s)", connectionInfo["security"], keyMgmt)
					}
				} else if strings.HasPrefix(line, "wpa=") {
					wpaStr := strings.TrimPrefix(line, "wpa=")
					wpaStr = strings.TrimSpace(wpaStr)
					if wpaStr == "2" && (connectionInfo["security"] == nil || connectionInfo["security"] == "") {
						connectionInfo["security"] = "WPA2"
						log.Printf("Found security from wpa_cli wpa field: WPA2")
					} else if wpaStr == "1" && (connectionInfo["security"] == nil || connectionInfo["security"] == "") {
						connectionInfo["security"] = "WPA"
						log.Printf("Found security from wpa_cli wpa field: WPA")
					}
				} else if strings.HasPrefix(line, "freq=") {
					freqStr := strings.TrimPrefix(line, "freq=")
					freqStr = strings.TrimSpace(freqStr)
					if freq, err := strconv.Atoi(freqStr); err == nil && freq > 0 {
						var channel int
						if freq >= 2412 && freq <= 2484 {
							channel = (freq-2412)/5 + 1
						} else if freq >= 5000 && freq <= 5825 {
							channel = (freq - 5000) / 5
						} else if freq >= 5955 && freq <= 7115 {
							channel = (freq - 5955) / 5
						}
						if channel > 0 {
							connectionInfo["channel"] = strconv.Itoa(channel)
							log.Printf("Found channel from wpa_cli: %d (from freq %d MHz)", channel, freq)
						} else {
							log.Printf("Could not convert freq %d to channel", freq)
						}
					}
				}
			}
		} else {
			log.Printf("wpa_cli failed or returned empty for %s: %v", iface, wpaErr)
		}

		if connectionInfo["signal"] == nil || connectionInfo["signal"] == "" || connectionInfo["signal"] == "0" ||
			connectionInfo["channel"] == nil || connectionInfo["channel"] == "" ||
			connectionInfo["security"] == nil || connectionInfo["security"] == "" {
			log.Printf("Getting additional info from iw for interface %s", iface)
			iwLinkCmd := exec.Command("sh", "-c", fmt.Sprintf("sudo iw dev %s link 2>/dev/null", iface))
			iwLinkOut, iwErr := iwLinkCmd.CombinedOutput()
			if iwErr != nil || len(iwLinkOut) == 0 {
				iwLinkCmd = exec.Command("sh", "-c", fmt.Sprintf("iw dev %s link 2>/dev/null", iface))
				iwLinkOut, iwErr = iwLinkCmd.CombinedOutput()
			}
			if iwErr == nil && len(iwLinkOut) > 0 {
				iwLink := string(iwLinkOut)
				log.Printf("iw link output for %s: %s", iface, iwLink)
				for _, line := range strings.Split(iwLink, "\n") {
					line = strings.TrimSpace(line)
					if (connectionInfo["signal"] == nil || connectionInfo["signal"] == "" || connectionInfo["signal"] == "0") && strings.Contains(strings.ToLower(line), "signal") {
						parts := strings.Fields(line)
						for i, part := range parts {
							partLower := strings.ToLower(part)
							if (partLower == "signal:" || partLower == "signal") && i+1 < len(parts) {
								signalStr := strings.TrimSpace(parts[i+1])
								signalStr = strings.TrimSuffix(signalStr, "dBm")
								signalStr = strings.TrimSpace(signalStr)
								if signalStr != "" && signalStr != "0" {
									if signalInt, err := strconv.Atoi(signalStr); err == nil && signalInt != 0 {
										if signalInt > 0 {
											signalInt = -signalInt
										}
										if signalInt >= -100 && signalInt <= -30 {
											signalStr = strconv.Itoa(signalInt)
											connectionInfo["signal"] = signalStr
											log.Printf("Found signal from iw: %s dBm", signalStr)
										}
									} else {
										re := regexp.MustCompile(`-?\d+`)
										matches := re.FindString(signalStr)
										if matches != "" {
											if signalInt, err := strconv.Atoi(matches); err == nil {
												if signalInt > 0 {
													signalInt = -signalInt
												}
												if signalInt >= -100 && signalInt <= -30 {
													connectionInfo["signal"] = strconv.Itoa(signalInt)
													log.Printf("Found signal from iw (parsed): %d dBm", signalInt)
												}
											}
										}
									}
								}
								break
							}
						}
					}
					if (connectionInfo["channel"] == nil || connectionInfo["channel"] == "") && strings.Contains(line, "freq:") {
						parts := strings.Fields(line)
						for i, part := range parts {
							if part == "freq:" && i+1 < len(parts) {
								freqStr := strings.TrimSpace(parts[i+1])
								if freq, err := strconv.Atoi(freqStr); err == nil && freq > 0 {
									var channel int
									if freq >= 2412 && freq <= 2484 {
										channel = (freq-2412)/5 + 1
									} else if freq >= 5000 && freq <= 5825 {
										channel = (freq - 5000) / 5
									} else if freq >= 5955 && freq <= 7115 {
										channel = (freq - 5955) / 5
									}
									if channel > 0 {
										connectionInfo["channel"] = strconv.Itoa(channel)
										log.Printf("Found channel from iw: %d (from freq %d)", channel, freq)
									}
								}
								break
							}
						}
					}
					if connectionInfo["security"] == nil || connectionInfo["security"] == "" {
						if strings.Contains(line, "WPA3") || strings.Contains(line, "SAE") {
							connectionInfo["security"] = "WPA3"
							log.Printf("Found security from iw: WPA3")
						} else if strings.Contains(line, "WPA2") || strings.Contains(line, "WPA") {
							connectionInfo["security"] = "WPA2"
							log.Printf("Found security from iw: WPA2")
						}
					}
				}
			} else {
				log.Printf("iw link command failed or returned empty: %v, output: %s", iwErr, string(iwLinkOut))
			}
		}

		if connectionInfo["signal"] == nil || connectionInfo["signal"] == "" || connectionInfo["signal"] == "0" {
			log.Printf("Trying /proc/net/wireless for signal on %s", iface)
			wirelessCmd := exec.Command("sh", "-c", fmt.Sprintf("cat /proc/net/wireless 2>/dev/null | grep %s", iface))
			wirelessOut, wirelessErr := wirelessCmd.Output()
			if wirelessErr == nil && len(wirelessOut) > 0 {
				wirelessLine := strings.TrimSpace(string(wirelessOut))
				log.Printf("/proc/net/wireless output: %s", wirelessLine)
				parts := strings.Fields(wirelessLine)
				if len(parts) >= 3 {
					if signalLevel, err := strconv.Atoi(strings.TrimSuffix(parts[2], ".")); err == nil && signalLevel > 0 {
						signalDbm := signalLevel / 10
						if signalDbm > 0 {
							connectionInfo["signal"] = fmt.Sprintf("-%d", signalDbm)
							log.Printf("Found signal from /proc/net/wireless: -%d dBm", signalDbm)
						}
					}
				}
			}
		}

		if (connectionInfo["signal"] == nil || connectionInfo["signal"] == "" || connectionInfo["signal"] == "0") ||
			(connectionInfo["channel"] == nil || connectionInfo["channel"] == "") {
			log.Printf("Trying iwconfig as last resort for interface %s", iface)
			iwconfigCmd := exec.Command("sh", "-c", fmt.Sprintf("iwconfig %s 2>/dev/null", iface))
			iwconfigOut, iwconfigErr := iwconfigCmd.CombinedOutput()
			if iwconfigErr == nil && len(iwconfigOut) > 0 {
				iwconfigStr := string(iwconfigOut)
				log.Printf("iwconfig output: %s", iwconfigStr)
				if connectionInfo["signal"] == nil || connectionInfo["signal"] == "" || connectionInfo["signal"] == "0" {
					if strings.Contains(iwconfigStr, "Signal level=") {
						parts := strings.Split(iwconfigStr, "Signal level=")
						if len(parts) > 1 {
							fields := strings.Fields(parts[1])
							if len(fields) == 0 {
								// parts[1] vacío o solo espacios, evitar panic
							} else {
								signalPart := fields[0]
								signalStr := strings.TrimSpace(signalPart)
								signalStr = strings.TrimSuffix(signalStr, "dBm")
								signalStr = strings.TrimSpace(signalStr)
								if signalStr != "" && signalStr != "0" {
									connectionInfo["signal"] = signalStr
									log.Printf("Found signal from iwconfig: %s", signalStr)
								}
							}
						}
					}
				}
				if connectionInfo["channel"] == nil || connectionInfo["channel"] == "" {
					if strings.Contains(iwconfigStr, "Channel:") {
						parts := strings.Split(iwconfigStr, "Channel:")
						if len(parts) > 1 {
							channelFields := strings.Fields(parts[1])
							if len(channelFields) > 0 {
								channelStr := strings.TrimSpace(channelFields[0])
								if channelStr != "" {
									connectionInfo["channel"] = channelStr
									log.Printf("Found channel from iwconfig: %s", channelStr)
								}
							}
						}
					}
				}
			}
		}

		if (connectionInfo["signal"] == nil || connectionInfo["signal"] == "" || connectionInfo["signal"] == "0") ||
			(connectionInfo["channel"] == nil || connectionInfo["channel"] == "") {
			log.Printf("Trying iw dev %s station dump as additional method", iface)
			iwStationCmd := exec.Command("sh", "-c", fmt.Sprintf("sudo iw dev %s station dump 2>/dev/null", iface))
			iwStationOut, iwStationErr := iwStationCmd.CombinedOutput()
			if iwStationErr == nil && len(iwStationOut) > 0 {
				iwStationStr := string(iwStationOut)
				log.Printf("iw station dump output: %s", iwStationStr)
				if connectionInfo["signal"] == nil || connectionInfo["signal"] == "" || connectionInfo["signal"] == "0" {
					lines := strings.Split(iwStationStr, "\n")
					for _, line := range lines {
						line = strings.TrimSpace(line)
						if strings.Contains(strings.ToLower(line), "signal") {
							re := regexp.MustCompile(`-?\d+`)
							matches := re.FindAllString(line, -1)
							for _, match := range matches {
								if signalInt, err := strconv.Atoi(match); err == nil {
									if signalInt > 0 {
										signalInt = -signalInt
									}
									if signalInt >= -100 && signalInt <= -30 {
										connectionInfo["signal"] = strconv.Itoa(signalInt)
										log.Printf("Found signal from iw station dump: %d dBm", signalInt)
										break
									}
								}
							}
						}
					}
				}
			}
		}

		if connectionInfo["signal"] == nil || connectionInfo["signal"] == "" || connectionInfo["signal"] == "0" {
			log.Printf("Warning: Could not determine signal strength for %s after all methods", iface)
			delete(connectionInfo, "signal")
		}
		if connectionInfo["channel"] == nil || connectionInfo["channel"] == "" {
			log.Printf("Warning: Could not determine channel for %s after all methods", iface)
			delete(connectionInfo, "channel")
		}
		if connectionInfo["security"] == nil || connectionInfo["security"] == "" {
			log.Printf("Warning: Could not determine security for %s, defaulting to WPA2", iface)
			connectionInfo["security"] = "WPA2" // Valor por defecto común
		}

		if iface != "" {
			ipCmd := exec.Command("sh", "-c", fmt.Sprintf("ip addr show %s 2>/dev/null | grep 'inet ' | awk '{print $2}' | cut -d/ -f1 | head -1", iface))
			ipOut, _ := ipCmd.Output()
			if ipStr := strings.TrimSpace(string(ipOut)); ipStr != "" {
				connectionInfo["ip"] = ipStr
			}

			macCmd := exec.Command("sh", "-c", fmt.Sprintf("cat /sys/class/net/%s/address 2>/dev/null", iface))
			macOut, _ := macCmd.Output()
			if macStr := strings.TrimSpace(string(macOut)); macStr != "" {
				connectionInfo["mac"] = macStr
			}

			speedCmd := exec.Command("sh", "-c", fmt.Sprintf("cat /sys/class/net/%s/speed 2>/dev/null", iface))
			speedOut, _ := speedCmd.Output()
			if speedStr := strings.TrimSpace(string(speedOut)); speedStr != "" && speedStr != "-1" {
				connectionInfo["speed"] = speedStr + " Mbps"
			}
		}
	}

	if !connected && enabled {
		ifaceCmd := execCommand("nmcli -t -f DEVICE,TYPE dev status 2>/dev/null | grep wifi | head -1 | cut -d: -f1")
		if ifaceOut, err := ifaceCmd.Output(); err == nil {
			iface := strings.TrimSpace(string(ifaceOut))
			if iface != "" {
				if connectionInfo == nil {
					connectionInfo = fiber.Map{}
				}
				macCmd := exec.Command("sh", "-c", fmt.Sprintf("cat /sys/class/net/%s/address 2>/dev/null", iface))
				macOut, _ := macCmd.Output()
				if macStr := strings.TrimSpace(string(macOut)); macStr != "" {
					connectionInfo["mac"] = macStr
				}
			}
		}
	}

	// connection_type: "wifi" | "ethernet" | "" para el wizard
	connectionType := ""
	if reallyConnected && ssid != "" {
		connectionType = "wifi"
	} else {
		defaultIfaceCmd := exec.Command("sh", "-c", "ip route show default 2>/dev/null | awk '{print $5}' | head -1")
		if defaultOut, err := defaultIfaceCmd.Output(); err == nil {
			ifaceName := strings.TrimSpace(string(defaultOut))
			if ifaceName != "" && (strings.HasPrefix(ifaceName, "eth") || strings.HasPrefix(ifaceName, "enp") || strings.HasPrefix(ifaceName, "eno") || strings.HasPrefix(ifaceName, "ens")) {
				connectionType = "ethernet"
			}
		}
	}

	return c.JSON(fiber.Map{
		"enabled":            enabled,
		"connected":          reallyConnected,
		"current_connection": ssid,
		"ssid":               ssid,
		"connection_type":    connectionType,
		"hard_blocked":       hardBlocked,
		"soft_blocked":       softBlocked,
		"connection_info":    connectionInfo,
	})
}

