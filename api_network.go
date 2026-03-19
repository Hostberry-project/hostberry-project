package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"hostberry/internal/config"
	"hostberry/internal/i18n"
)

func networkRoutingHandler(c *fiber.Ctx) error {
	out, err := exec.Command("sh", "-c", "ip route 2>/dev/null").CombinedOutput()
	if err != nil {
		i18n.LogTf("logs.api_route_error", err, string(out))
		return c.Status(500).JSON(fiber.Map{"error": strings.TrimSpace(string(out))})
	}
	var routes []fiber.Map
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	i18n.LogTf("logs.api_processing_routes", len(lines))

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 1 {
			continue
		}
		route := fiber.Map{"raw": line}
		route["destination"] = parts[0]

		for i := 0; i < len(parts)-1; i++ {
			if parts[i] == "via" && i+1 < len(parts) {
				route["gateway"] = parts[i+1]
			}
			if parts[i] == "dev" && i+1 < len(parts) {
				route["interface"] = parts[i+1]
			}
			if parts[i] == "metric" && i+1 < len(parts) {
				route["metric"] = parts[i+1]
			}
		}

		if _, hasGateway := route["gateway"]; !hasGateway {
			route["gateway"] = "*"
		}

		if _, hasInterface := route["interface"]; !hasInterface {
			route["interface"] = "-"
		}

		if _, hasMetric := route["metric"]; !hasMetric {
			route["metric"] = "0"
		}

		routes = append(routes, route)
	}

	i18n.LogTf("logs.api_returning_routes", len(routes))
	return c.JSON(routes)
}

func networkFirewallToggleHandler(c *fiber.Ctx) error {
	return c.Status(501).JSON(fiber.Map{"error": "Firewall toggle no implementado"})
}

func networkConfigHandler(c *fiber.Ctx) error {
	if c.Method() == "GET" {
		config := fiber.Map{
			"hostname": "",
			"gateway":  "",
			"dns1":     "",
			"dns2":     "",
		}

		hostnameCmd := exec.Command("sh", "-c", "hostnamectl --static 2>/dev/null || hostname 2>/dev/null || echo ''")
		if hostnameOut, err := hostnameCmd.Output(); err == nil {
			config["hostname"] = strings.TrimSpace(string(hostnameOut))
		}

		gatewayCmd := exec.Command("sh", "-c", "ip route | grep default | awk '{print $3}' | head -1")
		if gatewayOut, err := gatewayCmd.Output(); err == nil {
			gateway := strings.TrimSpace(string(gatewayOut))
			if gateway != "" {
				config["gateway"] = gateway
			}
		}

		dnsCmd := exec.Command("sh", "-c", "nmcli -t -f IP4.DNS connection show $(nmcli -t -f NAME connection show --active | head -1) 2>/dev/null | head -2")
		if dnsOut, err := dnsCmd.Output(); err == nil {
			dnsLines := strings.Split(strings.TrimSpace(string(dnsOut)), "\n")
			for i, dns := range dnsLines {
				dns = strings.TrimSpace(dns)
				if dns != "" && strings.Contains(dns, ":") {
					parts := strings.Split(dns, ":")
					if len(parts) > 1 {
						dns = parts[len(parts)-1]
					}
					if i == 0 {
						config["dns1"] = dns
					} else if i == 1 {
						config["dns2"] = dns
					}
				} else if dns != "" && !strings.Contains(dns, ":") {
					if i == 0 {
						config["dns1"] = dns
					} else if i == 1 {
						config["dns2"] = dns
					}
				}
			}
		}

		if config["dns1"] == "" {
			resolveCmd := exec.Command("sh", "-c", "resolvectl dns 2>/dev/null | grep -E '^[0-9]' | awk '{print $2}' | head -2")
			if resolveOut, err := resolveCmd.Output(); err == nil {
				resolveLines := strings.Split(strings.TrimSpace(string(resolveOut)), "\n")
				for i, dns := range resolveLines {
					dns = strings.TrimSpace(dns)
					if dns != "" {
						if i == 0 {
							config["dns1"] = dns
						} else if i == 1 {
							config["dns2"] = dns
						}
					}
				}
			}
		}

		if config["dns1"] == "" {
			resolvCmd := exec.Command("sh", "-c", "grep '^nameserver' /etc/resolv.conf 2>/dev/null | awk '{print $2}' | head -2")
			if resolvOut, err := resolvCmd.Output(); err == nil {
				resolvLines := strings.Split(strings.TrimSpace(string(resolvOut)), "\n")
				for i, dns := range resolvLines {
					dns = strings.TrimSpace(dns)
					if dns != "" && dns != "127.0.0.1" && dns != "127.0.0.53" {
						if i == 0 {
							config["dns1"] = dns
						} else if i == 1 {
							config["dns2"] = dns
						}
					}
				}
			}
		}

		return c.JSON(config)
	}

	var req struct {
		Hostname string `json:"hostname"`
		DNS1     string `json:"dns1"`
		DNS2     string `json:"dns2"`
		Gateway  string `json:"gateway"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request body",
		})
	}

	errors := []string{}
	applied := []string{}

	if req.Hostname != "" {
		if len(req.Hostname) > 64 || len(req.Hostname) < 1 {
			errors = append(errors, "Hostname must be between 1 and 64 characters")
		} else {
			hostnameRegex := regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-\.]*[a-zA-Z0-9])?$`)
			if !hostnameRegex.MatchString(req.Hostname) {
				errors = append(errors, "Hostname contains invalid characters. Use only letters, numbers, hyphens and dots. Cannot start or end with hyphen or dot.")
			} else {
				hostnameApplied := false
				var lastError error
				var lastOutput string

				if !hostnameApplied {
					cmd := fmt.Sprintf("sudo hostnamectl set-hostname %s", req.Hostname)
					if out, err := executeCommand(cmd); err == nil {
						time.Sleep(500 * time.Millisecond)
						verifyCmd := exec.Command("sh", "-c", "hostnamectl --static 2>/dev/null || hostname 2>/dev/null || echo ''")
						if verifyOut, err := verifyCmd.Output(); err == nil {
							currentHostname := strings.TrimSpace(string(verifyOut))
							if currentHostname == req.Hostname {
								hostnameApplied = true
								i18n.LogTf("logs.api_hostname_set", req.Hostname)
								_ = out
							} else {
								i18n.LogTf("logs.api_hostname_verify_failed", req.Hostname, currentHostname)
								lastError = fmt.Errorf("verification failed: got %s", currentHostname)
								lastOutput = out
							}
						} else {
							i18n.LogTf("logs.api_hostname_verify_error", err)
							lastError = err
							lastOutput = out
						}
					} else {
						i18n.LogTf("logs.api_hostnamectl_failed", err, out)
						lastError = err
						lastOutput = out
					}
				}

				if !hostnameApplied {
					hostnameFile := "/etc/hostname"
					tmpHostname := "/tmp/hostname_hostberry_" + fmt.Sprintf("%d", time.Now().Unix())
					if wErr := os.WriteFile(tmpHostname, []byte(req.Hostname+"\n"), 0644); wErr == nil {
						defer os.Remove(tmpHostname)
						cpCmd := exec.Command("sh", "-c", fmt.Sprintf("sudo cp -f %s %s", tmpHostname, hostnameFile))
						cpOut, cpErr := cpCmd.CombinedOutput()
						out := strings.TrimSpace(string(cpOut))
						if cpErr == nil {
							if content, err := os.ReadFile(hostnameFile); err == nil {
								writtenHostname := strings.TrimSpace(string(content))
								if writtenHostname == req.Hostname {
									applyCmdStr := fmt.Sprintf("sudo hostname %s", req.Hostname)
									if applyOut, applyErr := executeCommand(applyCmdStr); applyErr == nil {
										verifyCmd := exec.Command("hostname")
										if verifyOut, err := verifyCmd.Output(); err == nil {
											currentHostname := strings.TrimSpace(string(verifyOut))
											if currentHostname == req.Hostname {
												hostnameApplied = true
												i18n.LogTf("logs.api_hostname_file_set", req.Hostname)
												_ = applyOut
											} else {
												i18n.LogTf("logs.api_hostname_file_verify_failed", req.Hostname, currentHostname)
												lastError = fmt.Errorf("verification failed: got %s", currentHostname)
												lastOutput = applyOut
											}
										}
									} else {
										i18n.LogTf("logs.api_hostname_apply_failed", applyErr, applyOut)
										lastError = applyErr
										lastOutput = applyOut
									}
								} else {
									i18n.LogTf("logs.api_hostname_write_mismatch", req.Hostname, writtenHostname)
									lastError = fmt.Errorf("written hostname mismatch")
									lastOutput = out
								}
							} else {
								i18n.LogTf("logs.api_hostname_read_failed", err)
								lastError = err
								lastOutput = out
							}
						} else {
							i18n.LogTf("logs.api_hostname_write_failed", cpErr, out)
							lastError = cpErr
							lastOutput = out
						}
					}
				}

				if !hostnameApplied {
					cmd := fmt.Sprintf("sudo hostname %s", req.Hostname)
					if out, err := executeCommand(cmd); err == nil {
						time.Sleep(200 * time.Millisecond)
						verifyCmd := exec.Command("hostname")
						if verifyOut, err := verifyCmd.Output(); err == nil {
							currentHostname := strings.TrimSpace(string(verifyOut))
							if currentHostname == req.Hostname {
								hostnameApplied = true
								i18n.LogTf("logs.api_hostname_temp_set", req.Hostname)
								_ = out
							} else {
								i18n.LogTf("logs.api_hostname_temp_verify_failed", req.Hostname, currentHostname)
								lastError = fmt.Errorf("verification failed: got %s", currentHostname)
								lastOutput = out
							}
						} else {
							i18n.LogTf("logs.api_hostname_verify_error2", err)
							lastError = err
							lastOutput = out
						}
					} else {
						i18n.LogTf("logs.api_hostname_cmd_failed", err, out)
						lastError = err
						lastOutput = out
					}
				}

				if hostnameApplied {
					hostsFile := "/etc/hosts"
					tmpFile := "/tmp/hosts_hostberry_" + fmt.Sprintf("%d", time.Now().Unix())

					i18n.LogTf("logs.api_hosts_creating", req.Hostname)

					newContent := "# See `man hosts` for details.\n"
					newContent += "#\n"
					newContent += "# By default, systemd-resolved or libnss-myhostname will resolve\n"
					newContent += "# localhost and the system hostname if they're not specified here.\n"

					if req.Hostname != "" {
						newContent += fmt.Sprintf("127.0.0.1\tlocalhost\t%s\n", req.Hostname)
					} else {
						newContent += "127.0.0.1\tlocalhost\n"
					}

					newContent += "::1\t\tlocalhost\n"

					if err := os.WriteFile(tmpFile, []byte(newContent), 0644); err != nil {
						i18n.LogTf("logs.api_hosts_temp_error", err)
					} else {
						log.Printf("Created new hosts file in /tmp: %s", tmpFile)
						log.Printf("File content:\n%s", newContent)

						if _, err := os.Stat(tmpFile); os.IsNotExist(err) {
							log.Printf("Error: Temp file does not exist after creation")
						} else {
							log.Printf("Temp file verified: exists and readable")

							copySuccess := false
							cpPath := "/bin/cp"
							if _, err := os.Stat(cpPath); os.IsNotExist(err) {
								cpPath = "/usr/bin/cp"
							}

							log.Printf("Attempting to copy with cp: %s -f %s %s", cpPath, tmpFile, hostsFile)
							cpCmd := exec.Command("sudo", cpPath, "-f", tmpFile, hostsFile)
							cpCmd.Env = append(os.Environ(), "SUDO_ASKPASS=/bin/false")
							cpOut, cpErr := cpCmd.CombinedOutput()

							time.Sleep(200 * time.Millisecond)
							if content, err := os.ReadFile(hostsFile); err == nil {
								if strings.Contains(string(content), req.Hostname) {
									log.Printf("Successfully copied with cp - hostname found in /etc/hosts")
									copySuccess = true
								} else {
									if cpErr != nil {
										log.Printf("Error with cp: %v, output: %s", cpErr, string(cpOut))
									} else {
										log.Printf("cp command executed but hostname not found in /etc/hosts")
									}
									log.Printf("Current /etc/hosts content:\n%s", string(content))

									if strings.Contains(string(cpOut), "Read-only file system") || strings.Contains(string(cpOut), "Read-only") {
										log.Printf("Warning: /etc/hosts appears to be on a read-only file system")
										log.Printf("Attempting to remount /etc as read-write...")
										remountCmd := exec.Command("sudo", "mount", "-o", "remount,rw", "/")
										remountCmd.Env = append(os.Environ(), "SUDO_ASKPASS=/bin/false")
										if remountOut, remountErr := remountCmd.CombinedOutput(); remountErr != nil {
											log.Printf("Could not remount / as read-write: %v, output: %s", remountErr, string(remountOut))
										} else {
											log.Printf("Successfully remounted / as read-write")
											cpCmd2 := exec.Command("sudo", cpPath, "-f", tmpFile, hostsFile)
											cpCmd2.Env = append(os.Environ(), "SUDO_ASKPASS=/bin/false")
											if _, cpErr2 := cpCmd2.CombinedOutput(); cpErr2 == nil {
												time.Sleep(200 * time.Millisecond)
												if content2, err2 := os.ReadFile(hostsFile); err2 == nil {
													if strings.Contains(string(content2), req.Hostname) {
														log.Printf("Successfully copied after remount - hostname found in /etc/hosts")
														copySuccess = true
													}
												}
											}
										}
									}
								}
							} else {
								if cpErr != nil {
									log.Printf("Error with cp: %v, output: %s", cpErr, string(cpOut))
								} else {
									log.Printf("cp command executed but could not read /etc/hosts: %v", err)
								}
							}

							if !copySuccess {
								log.Printf("Trying alternative method: cat with tee")
								catCmd := exec.Command("sh", "-c", fmt.Sprintf("sudo cat %s | sudo tee %s > /dev/null", tmpFile, hostsFile))
								catCmd.Env = append(os.Environ(), "SUDO_ASKPASS=/bin/false")
								if catOut, catErr := catCmd.CombinedOutput(); catErr != nil {
									log.Printf("Error with cat/tee: %v, output: %s", catErr, string(catOut))
								} else {
									log.Printf("cat/tee command executed, output: %s", string(catOut))
									time.Sleep(100 * time.Millisecond)
									if content, err := os.ReadFile(hostsFile); err == nil {
										if strings.Contains(string(content), req.Hostname) {
											log.Printf("Successfully copied with cat/tee - hostname found in /etc/hosts")
											copySuccess = true
										} else {
											log.Printf("cat/tee executed but hostname not found in /etc/hosts")
											log.Printf("Current /etc/hosts content:\n%s", string(content))
										}
									}
								}
							}

							if !copySuccess {
								log.Printf("Trying sh -c method with direct redirection")
								writeCmd := exec.Command("sudo", "sh", "-c", fmt.Sprintf("cat %s > %s", tmpFile, hostsFile))
								writeCmd.Env = append(os.Environ(), "SUDO_ASKPASS=/bin/false")
								if writeOut, writeErr := writeCmd.CombinedOutput(); writeErr != nil {
									log.Printf("Error with sh -c: %v, output: %s", writeErr, string(writeOut))
								} else {
									log.Printf("sh -c command executed, output: %s", string(writeOut))
									time.Sleep(100 * time.Millisecond)
									if content, err := os.ReadFile(hostsFile); err == nil {
										if strings.Contains(string(content), req.Hostname) {
											log.Printf("Successfully copied with sh -c - hostname found in /etc/hosts")
											copySuccess = true
										} else {
											log.Printf("sh -c executed but hostname not found in /etc/hosts")
											log.Printf("Current /etc/hosts content:\n%s", string(content))
										}
									}
								}
							}

							if copySuccess {
								chmodPath := "/bin/chmod"
								if _, err := os.Stat(chmodPath); os.IsNotExist(err) {
									chmodPath = "/usr/bin/chmod"
								}
								chmodCmd := fmt.Sprintf("sudo %s 644 %s", chmodPath, hostsFile)
								if out, err := executeCommand(chmodCmd); err != nil {
									log.Printf("Warning: Could not set permissions: %v, output: %s", err, out)
								} else {
									log.Printf("Permissions set correctly")
								}

								if content, err := os.ReadFile(hostsFile); err == nil {
									if strings.Contains(string(content), req.Hostname) {
										log.Printf("Final verification: hostname %s successfully updated in /etc/hosts", req.Hostname)
									} else {
										log.Printf("Final verification failed: hostname not found")
										log.Printf("Final /etc/hosts content:\n%s", string(content))
									}
								}
							} else {
								log.Printf("Error: All copy methods failed. /etc/hosts was not updated.")
							}
						}

						os.Remove(tmpFile)
					}

					applied = append(applied, fmt.Sprintf("Hostname set to %s and /etc/hosts updated", req.Hostname))
				} else {
					errorMsg := fmt.Sprintf("Failed to set hostname: tried hostnamectl, /etc/hostname, and hostname command")
					if lastError != nil {
						errorMsg += fmt.Sprintf(" (last error: %v", lastError)
						if lastOutput != "" {
							errorMsg += fmt.Sprintf(", output: %s", strings.TrimSpace(lastOutput))
						}
						errorMsg += ")"
					}
					errors = append(errors, errorMsg)
					if config.AppConfig.Server.Debug {
						log.Printf("All hostname setting methods failed for hostname: %s. Last error: %v, Last output: %s", req.Hostname, lastError, lastOutput)
					}
				}
			}
		}
	}

	dnsServers := []string{}
	if req.DNS1 != "" {
		ipRegex := regexp.MustCompile(`^([0-9]{1,3}\.){3}[0-9]{1,3}$`)
		if !ipRegex.MatchString(req.DNS1) {
			errors = append(errors, "Invalid DNS1 format")
		} else {
			dnsServers = append(dnsServers, req.DNS1)
		}
	}

	if req.DNS2 != "" {
		ipRegex := regexp.MustCompile(`^([0-9]{1,3}\.){3}[0-9]{1,3}$`)
		if !ipRegex.MatchString(req.DNS2) {
			errors = append(errors, "Invalid DNS2 format")
		} else {
			dnsServers = append(dnsServers, req.DNS2)
		}
	}

	if len(dnsServers) > 0 {
		dnsApplied := false
		dnsStr := strings.Join(dnsServers, " ")

		if !dnsApplied {
			connCmd := exec.Command("sh", "-c", "nmcli -t -f NAME connection show --active 2>/dev/null | head -1")
			if connOut, err := connCmd.Output(); err == nil {
				connName := strings.TrimSpace(string(connOut))
				if connName != "" {
					cmd := fmt.Sprintf("sudo nmcli connection modify '%s' ipv4.dns '%s' 2>&1", connName, dnsStr)
					if out, err := executeCommand(cmd); err == nil {
						applyCmd := fmt.Sprintf("sudo nmcli connection up '%s' 2>&1", connName)
						executeCommand(applyCmd)
						applied = append(applied, fmt.Sprintf("DNS set to %s (via NetworkManager)", strings.Join(dnsServers, ", ")))
						dnsApplied = true
						log.Printf("DNS configured via nmcli: %s", dnsStr)
					} else {
						log.Printf("nmcli DNS configuration failed: %v, output: %s", err, out)
					}
				}
			}
		}

		if !dnsApplied {
			resolvedConf := "/etc/systemd/resolved.conf"
			if _, err := os.Stat(resolvedConf); err == nil {
				content, err := os.ReadFile(resolvedConf)
				if err == nil {
					lines := strings.Split(string(content), "\n")
					updated := false
					newLines := []string{}

					for _, line := range lines {
						trimmed := strings.TrimSpace(line)
						if strings.HasPrefix(trimmed, "DNS=") {
							newLines = append(newLines, "DNS="+strings.Join(dnsServers, " "))
							updated = true
						} else if strings.HasPrefix(trimmed, "#DNS=") && !updated {
							newLines = append(newLines, "DNS="+strings.Join(dnsServers, " "))
							updated = true
						} else {
							newLines = append(newLines, line)
						}
					}

					if !updated {
						newLines = append(newLines, "DNS="+strings.Join(dnsServers, " "))
					}

					newContent := strings.Join(newLines, "\n")
					writeCmd := exec.Command("sudo", "sh", "-c", fmt.Sprintf("cat > %s", resolvedConf))
					writeCmd.Stdin = strings.NewReader(newContent)
					if err := writeCmd.Run(); err == nil {
						executeCommand("sudo systemctl restart systemd-resolved 2>&1")
						applied = append(applied, fmt.Sprintf("DNS set to %s (via systemd-resolved)", strings.Join(dnsServers, ", ")))
						dnsApplied = true
						log.Printf("DNS configured via systemd-resolved: %s", dnsStr)
					}
				}
			}
		}

		if !dnsApplied {
			resolvConf := "/etc/resolv.conf"
			executeCommand(fmt.Sprintf("sudo cp %s %s.backup 2>/dev/null || true", resolvConf, resolvConf))

			content, err := os.ReadFile(resolvConf)
			if err == nil {
				lines := strings.Split(string(content), "\n")
				newLines := []string{}

				for _, line := range lines {
					trimmed := strings.TrimSpace(line)
					if !strings.HasPrefix(trimmed, "nameserver") {
						newLines = append(newLines, line)
					}
				}

				for _, dns := range dnsServers {
					newLines = append(newLines, "nameserver "+dns)
				}

				newContent := strings.Join(newLines, "\n")
				writeCmd := exec.Command("sudo", "sh", "-c", fmt.Sprintf("cat > %s", resolvConf))
				writeCmd.Stdin = strings.NewReader(newContent)
				if err := writeCmd.Run(); err == nil {
					applied = append(applied, fmt.Sprintf("DNS set to %s (via /etc/resolv.conf)", strings.Join(dnsServers, ", ")))
					dnsApplied = true
					log.Printf("DNS configured via /etc/resolv.conf: %s", dnsStr)
				} else {
					log.Printf("Failed to write /etc/resolv.conf: %v", err)
				}
			}
		}

		if !dnsApplied {
			errors = append(errors, fmt.Sprintf("Failed to set DNS: tried NetworkManager, systemd-resolved, and /etc/resolv.conf"))
		}
	}

	if req.Gateway != "" {
		ipRegex := regexp.MustCompile(`^([0-9]{1,3}\.){3}[0-9]{1,3}$`)
		if !ipRegex.MatchString(req.Gateway) {
			errors = append(errors, "Invalid Gateway format")
		} else {
			gatewayApplied := false

			if !gatewayApplied {
				connCmd := exec.Command("sh", "-c", "nmcli -t -f NAME connection show --active 2>/dev/null | head -1")
				if connOut, err := connCmd.Output(); err == nil {
					connName := strings.TrimSpace(string(connOut))
					if connName != "" {
						cmd := fmt.Sprintf("sudo nmcli connection modify '%s' ipv4.gateway %s 2>&1", connName, req.Gateway)
						if out, err := executeCommand(cmd); err == nil {
							applyCmd := fmt.Sprintf("sudo nmcli connection up '%s' 2>&1", connName)
							if _, err := executeCommand(applyCmd); err == nil {
								applied = append(applied, fmt.Sprintf("Gateway set to %s (via NetworkManager)", req.Gateway))
								gatewayApplied = true
								log.Printf("Gateway configured via nmcli: %s", req.Gateway)
							} else {
								log.Printf("Failed to apply gateway via nmcli: %v", err)
							}
						} else {
							log.Printf("nmcli gateway configuration failed: %v, output: %s", err, out)
						}
					}
				}
			}

			if !gatewayApplied {
				ifaceCmd := exec.Command("sh", "-c", "ip route | grep default | awk '{print $5}' | head -1")
				iface := ""
				if ifaceOut, err := ifaceCmd.Output(); err == nil {
					iface = strings.TrimSpace(string(ifaceOut))
				}

				if iface != "" {
					executeCommand("sudo ip route del default 2>/dev/null || true")
					cmd := fmt.Sprintf("sudo ip route add default via %s dev %s 2>&1", req.Gateway, iface)
					if out, err := executeCommand(cmd); err == nil {
						applied = append(applied, fmt.Sprintf("Gateway set to %s (via ip route)", req.Gateway))
						gatewayApplied = true
						log.Printf("Gateway configured via ip route: %s on %s", req.Gateway, iface)
					} else {
						log.Printf("ip route gateway configuration failed: %v, output: %s", err, out)
					}
				} else {
					executeCommand("sudo ip route del default 2>/dev/null || true")
					cmd := fmt.Sprintf("sudo ip route add default via %s 2>&1", req.Gateway)
					if out, err := executeCommand(cmd); err == nil {
						applied = append(applied, fmt.Sprintf("Gateway set to %s (via ip route)", req.Gateway))
						gatewayApplied = true
						log.Printf("Gateway configured via ip route: %s", req.Gateway)
					} else {
						log.Printf("ip route gateway configuration failed: %v, output: %s", err, out)
					}
				}
			}

			if !gatewayApplied {
				ifaceCmd := exec.Command("sh", "-c", "route -n | grep '^0.0.0.0' | awk '{print $8}' | head -1")
				iface := ""
				if ifaceOut, err := ifaceCmd.Output(); err == nil {
					iface = strings.TrimSpace(string(ifaceOut))
				}

				if iface != "" {
					executeCommand("sudo route del default 2>/dev/null || true")
					cmd := fmt.Sprintf("sudo route add default gw %s %s 2>&1", req.Gateway, iface)
					if out, err := executeCommand(cmd); err == nil {
						applied = append(applied, fmt.Sprintf("Gateway set to %s (via route)", req.Gateway))
						gatewayApplied = true
						log.Printf("Gateway configured via route: %s on %s", req.Gateway, iface)
					} else {
						log.Printf("route gateway configuration failed: %v, output: %s", err, out)
					}
				}
			}

			if !gatewayApplied {
				errors = append(errors, fmt.Sprintf("Failed to set gateway: tried NetworkManager, ip route, and route command"))
			}
		}
	}

	if len(errors) > 0 {
		errorMsg := strings.Join(errors, "; ")
		if len(applied) > 0 {
			errorMsg += " (Some settings were applied: " + strings.Join(applied, ", ") + ")"
		}
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"error":   errorMsg,
		})
	}

	message := "Configuration applied successfully"
	if len(applied) > 0 {
		message = strings.Join(applied, "; ")
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": message,
	})
}
