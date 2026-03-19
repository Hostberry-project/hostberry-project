package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"hostberry/internal/auth"
	"hostberry/internal/config"
	"hostberry/internal/database"
	"hostberry/internal/i18n"
	middleware "hostberry/internal/middleware"
	"hostberry/internal/models"
	"hostberry/internal/tor"
	"hostberry/internal/validators"
	webtemplates "hostberry/internal/templates"
	"hostberry/internal/wifi"
	"hostberry/internal/vpn"
	network "hostberry/internal/network"
)

func translateLoginError(c *fiber.Ctx, err error) string {
	var le *models.LoginError
	if errors.As(err, &le) {
		msg := i18n.T(c, le.Key, le.Default)
		if len(le.Args) > 0 {
			msg = strings.ReplaceAll(msg, "{minutes}", fmt.Sprint(le.Args[0]))
			msg = strings.ReplaceAll(msg, "{duration}", fmt.Sprint(le.Args[0]))
		}
		return msg
	}
	return err.Error()
}

func loginAPIHandler(c *fiber.Ctx) error {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": i18n.T(c, "errors.invalid_data", "Invalid data"),
		})
	}

	if err := validators.ValidateUsername(req.Username); err != nil {
		return err
	}

	if req.Password == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": i18n.T(c, "auth.password_required", "Password is required"),
		})
	}

	user, token, err := auth.Login(req.Username, req.Password)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{
			"error": translateLoginError(c, err),
		})
	}

	userID := user.ID
	database.InsertLog("INFO", database.LogMsg("Inicio de sesión correcto", user.Username), "auth", &userID)

	// Primer login o credenciales por defecto (admin/admin): forzar cambio de contraseña en first-login
	passwordChangeRequired := user.LoginCount == 1 || (user.Username == "admin" && auth.CheckPassword("admin", user.Password))

	cookieExpiry := time.Duration(config.AppConfig.Security.TokenExpiry) * time.Minute
	secure := false
	// Si la petición ya viene por HTTPS (cabeceras estándar reverse proxy),
	// marcar la cookie como Secure para evitar envío por HTTP plano.
	if c.Secure() || strings.EqualFold(c.Get("X-Forwarded-Proto"), "https") {
		secure = true
	}
	c.Cookie(&fiber.Cookie{
		Name:     "access_token",
		Value:    token,
		Path:     "/",
		HTTPOnly: true,
		SameSite: "Lax",
		MaxAge:   int(cookieExpiry.Seconds()), // Expira al mismo tiempo que el token
		Secure:   secure,
	})

	return c.JSON(fiber.Map{
		"access_token":            token,
		"password_change_required": passwordChangeRequired,
		"user": fiber.Map{
			"id":       user.ID,
			"username": user.Username,
			"email":    user.Email,
		},
	})
}

func logoutAPIHandler(c *fiber.Ctx) error {
	user, ok := middleware.GetUser(c)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": i18n.T(c, "auth.unauthorized", "Unauthorized")})
	}
	userID := user.ID
	database.InsertLog("INFO", database.LogMsg("Cierre de sesión", user.Username), "auth", &userID)

	c.Cookie(&fiber.Cookie{
		Name:     "access_token",
		Value:    "",
		Path:     "/",
		HTTPOnly: true,
		MaxAge:   -1,
	})

	return c.JSON(fiber.Map{
		"message": i18n.T(c, "auth.logout_success", "Logout successful"),
	})
}

func meHandler(c *fiber.Ctx) error {
	user, ok := middleware.GetUser(c)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": i18n.T(c, "auth.unauthorized", "Unauthorized")})
	}
	return c.JSON(fiber.Map{
		"id":       user.ID,
		"username": user.Username,
		"email":    user.Email,
		"first_name": user.FirstName,
		"last_name":  user.LastName,
		"role":       user.Role,
		"timezone":   user.Timezone,
	})
}

func changePasswordAPIHandler(c *fiber.Ctx) error {
	user, ok := middleware.GetUser(c)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "No autorizado"})
	}

	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": i18n.T(c, "errors.invalid_data", "Invalid data")})
	}
	if req.CurrentPassword == "" || req.NewPassword == "" {
		return c.Status(400).JSON(fiber.Map{"error": i18n.T(c, "auth.passwords_required", "Passwords required")})
	}
	if !auth.CheckPassword(req.CurrentPassword, user.Password) {
		return c.Status(401).JSON(fiber.Map{"error": i18n.T(c, "auth.incorrect_current_password", "Current password is incorrect")})
	}

	hashed, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": i18n.T(c, "errors.server_error", "Internal server error")})
	}
	user.Password = hashed
	if err := database.DB.Save(user).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": i18n.T(c, "errors.server_error", "Internal server error")})
	}

	userID := user.ID
	database.InsertLog("INFO", database.LogMsg("Contraseña cambiada", user.Username), "auth", &userID)
	return c.JSON(fiber.Map{"message": i18n.T(c, "auth.password_changed", "Password changed successfully")})
}

func firstLoginChangeAPIHandler(c *fiber.Ctx) error {
	tokenString := c.Get("Authorization")
	if tokenString != "" {
		tokenString = strings.TrimPrefix(tokenString, "Bearer ")
	} else {
		tokenString = c.Cookies("access_token")
	}

	if tokenString == "" {
		return c.Status(401).JSON(fiber.Map{
			"error": i18n.T(c, "auth.token_required", "Token required"),
		})
	}

	claims, err := auth.ValidateToken(tokenString)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{
			"error": i18n.T(c, "auth.invalid_token", "Invalid token"),
		})
	}

 var user models.User
	if err := database.DB.Where("id = ? AND is_active = ?", claims.UserID, true).First(&user).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{
			"error": i18n.T(c, "auth.user_not_found", "User not found"),
		})
	}

	if user.LoginCount != 1 {
		return c.Status(403).JSON(fiber.Map{
			"error": i18n.T(c, "auth.first_login_only", "This endpoint is only available on first login"),
		})
	}

	var req struct {
		NewUsername string `json:"new_username"`
		NewPassword string `json:"new_password"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Datos inválidos",
		})
	}

	if req.NewUsername != "" {
		if err := validators.ValidateUsername(req.NewUsername); err != nil {
			return err
		}
		if req.NewUsername != user.Username {
			var existingUser models.User
			if err := database.DB.Where("username = ?", req.NewUsername).First(&existingUser).Error; err == nil {
				return c.Status(400).JSON(fiber.Map{
					"error": "El nombre de usuario ya está en uso",
				})
			}
			user.Username = req.NewUsername
		}
	}

	if req.NewPassword == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "La nueva contraseña es requerida",
		})
	}
	if err := validators.ValidatePassword(req.NewPassword); err != nil {
		return err
	}

	hashed, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Error hasheando contraseña",
		})
	}
	user.Password = hashed

	user.LoginCount++

	if err := database.DB.Save(&user).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Error guardando credenciales",
		})
	}

	userID := user.ID
	database.InsertLog("INFO", database.LogMsg("Credenciales actualizadas en primer acceso", user.Username), "auth", &userID)

	// Generar nuevo token con las credenciales actualizadas y dejar al usuario logueado
	newToken, err := auth.GenerateToken(&user)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Error generando sesión",
		})
	}
	cookieExpiry := time.Duration(config.AppConfig.Security.TokenExpiry) * time.Minute
	secure := false
	if c.Secure() || strings.EqualFold(c.Get("X-Forwarded-Proto"), "https") {
		secure = true
	}
	c.Cookie(&fiber.Cookie{
		Name:     "access_token",
		Value:    newToken,
		Path:     "/",
		HTTPOnly: true,
		SameSite: "Lax",
		MaxAge:   int(cookieExpiry.Seconds()),
		Secure:   secure,
	})

	return c.JSON(fiber.Map{
		"message":      i18n.T(c, "auth.credentials_updated_redirect", "Credenciales actualizadas. Redirigiendo al dashboard."),
		"access_token": newToken,
		"user": fiber.Map{
			"id":       user.ID,
			"username": user.Username,
			"email":    user.Email,
		},
	})
}

func updateProfileAPIHandler(c *fiber.Ctx) error {
	user, ok := middleware.GetUser(c)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "No autorizado"})
	}

	var req struct {
		Email     string `json:"email"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Timezone  string `json:"timezone"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Datos inválidos"})
	}

	user.Email = req.Email
	user.FirstName = req.FirstName
	user.LastName = req.LastName
	if req.Timezone != "" {
		user.Timezone = req.Timezone
	}

	if err := database.DB.Save(user).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Error guardando perfil"})
	}

	userID := user.ID
		database.InsertLog("INFO", database.LogMsg("Perfil actualizado", user.Username), "auth", &userID)
	return c.JSON(fiber.Map{"message": "Perfil actualizado"})
}

func updatePreferencesAPIHandler(c *fiber.Ctx) error {
	user, ok := middleware.GetUser(c)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "No autorizado"})
	}

	var req struct {
		EmailNotifications bool `json:"email_notifications"`
		SystemAlerts       bool `json:"system_alerts"`
		SecurityAlerts     bool `json:"security_alerts"`
		ShowActivity       bool `json:"show_activity"`
		DataCollection     bool `json:"data_collection"`
		Analytics          bool `json:"analytics"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Datos inválidos"})
	}

	user.EmailNotifications = req.EmailNotifications
	user.SystemAlerts = req.SystemAlerts
	user.SecurityAlerts = req.SecurityAlerts
	user.ShowActivity = req.ShowActivity
	user.DataCollection = req.DataCollection
	user.Analytics = req.Analytics

	if err := database.DB.Save(user).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Error guardando preferencias"})
	}

	userID := user.ID
		database.InsertLog("INFO", database.LogMsg("Preferencias actualizadas", user.Username), "auth", &userID)
	return c.JSON(fiber.Map{"message": "Preferencias actualizadas"})
}

func networkStatusHandler(c *fiber.Ctx) error {
	result := network.GetNetworkStatus()
	return c.JSON(result)
}

// librespeedCLIPath nombres posibles del binario LibreSpeed (speedtest-cli)
var librespeedCLIPath = []string{"librespeed-cli", "librespeed-cli-go", "/usr/bin/librespeed-cli", "/usr/local/bin/librespeed-cli"}

func networkSpeedtestHandler(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.Context(), 120*time.Second)
	defer cancel()
	var bin string
	for _, p := range librespeedCLIPath {
		if path, err := exec.LookPath(p); err == nil {
			bin = path
			break
		}
	}
	if bin == "" {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"error":   "LibreSpeed CLI no instalado. Instálalo desde https://github.com/librespeed/speedtest-cli",
		})
	}
	cmd := exec.CommandContext(ctx, bin, "--json", "--telemetry-level", "disabled")
	out, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return c.Status(408).JSON(fiber.Map{"success": false, "error": "Timeout del test de velocidad"})
		}
		return c.JSON(fiber.Map{
			"success": false,
			"error":   strings.TrimSpace(string(out)),
		})
	}
	// La salida puede tener líneas de log y una línea JSON; buscar la línea que empieza con {
	lines := strings.Split(string(out), "\n")
	var raw []byte
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(line, "{") {
			raw = []byte(line)
			break
		}
	}
	if len(raw) == 0 {
		raw = out
	}
	var result struct {
		Timestamp      string  `json:"timestamp"`
		Ping           float64 `json:"ping"`
		Jitter         float64 `json:"jitter"`
		Download       float64 `json:"download"`
		Upload         float64 `json:"upload"`
		BytesSent      int64   `json:"bytes_sent"`
		BytesReceived  int64   `json:"bytes_received"`
		Server         struct {
			Name string `json:"name"`
			URL  string `json:"url"`
		} `json:"server"`
		Client struct {
			IP       string `json:"ip"`
			Org      string `json:"org"`
			Country  string `json:"country"`
			City     string `json:"city"`
		} `json:"client"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return c.JSON(fiber.Map{"success": false, "error": "No se pudo interpretar la salida de LibreSpeed: " + err.Error()})
	}
	return c.JSON(fiber.Map{
		"success":         true,
		"ping_ms":         result.Ping,
		"jitter_ms":       result.Jitter,
		"download_mbps":   result.Download,
		"upload_mbps":     result.Upload,
		"bytes_sent":      result.BytesSent,
		"bytes_received":  result.BytesReceived,
		"server_name":     result.Server.Name,
		"server_url":      result.Server.URL,
		"client_ip":       result.Client.IP,
		"client_org":      result.Client.Org,
		"client_country":  result.Client.Country,
		"client_city":     result.Client.City,
		"timestamp":       result.Timestamp,
	})
}

func networkInterfacesHandler(c *fiber.Ctx) error {
	result := network.GetNetworkInterfaces()
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

func wifiConnectHandler(c *fiber.Ctx) error {
	return wifi.WifiConnectHandler(c)
}

func vpnStatusHandler(c *fiber.Ctx) error {
	return vpn.VpnStatusHandler(c)
}

func vpnConnectHandler(c *fiber.Ctx) error {
	return vpn.VpnConnectHandler(c)
}

func networkPageHandler(c *fiber.Ctx) error {
	return webtemplates.RenderTemplate(c, "network", fiber.Map{
		"Title": i18n.T(c, "network.title", "Network Management"),
	})
}

func wifiPageHandler(c *fiber.Ctx) error {
	return webtemplates.RenderTemplate(c, "wifi", fiber.Map{
		"Title":         i18n.T(c, "wifi.overview", "WiFi Overview"),
		"wifi_stats":    fiber.Map{},
		"wifi_status":   fiber.Map{},
		"wifi_config":   fiber.Map{},
		"guest_network": fiber.Map{},
		"last_update":   time.Now().Unix(),
	})
}

func wifiScanPageHandler(c *fiber.Ctx) error {
	return webtemplates.RenderTemplate(c, "wifi_scan", fiber.Map{
		"Title": i18n.T(c, "wifi.scan", "WiFi Scan"),
	})
}

func vpnPageHandler(c *fiber.Ctx) error {
	return webtemplates.RenderTemplate(c, "vpn", fiber.Map{
		"Title":        i18n.T(c, "vpn.overview", "VPN Overview"),
		"vpn_stats":    fiber.Map{},
		"vpn_status":   fiber.Map{},
		"vpn_config":   fiber.Map{},
		"vpn_security": fiber.Map{},
		"last_update":  time.Now().Unix(),
	})
}

func wireguardPageHandler(c *fiber.Ctx) error {
	return webtemplates.RenderTemplate(c, "wireguard", fiber.Map{
		"Title":            i18n.T(c, "wireguard.overview", "WireGuard Overview"),
		"wireguard_stats":  fiber.Map{},
		"wireguard_status": fiber.Map{},
		"wireguard_config": fiber.Map{},
		"last_update":      time.Now().Unix(),
	})
}

func torPageHandler(c *fiber.Ctx) error {
	return webtemplates.RenderTemplate(c, "tor", fiber.Map{
		"Title": i18n.T(c, "tor.title", "Tor Configuration"),
		"tor_status": tor.GetTorStatus(),
	})
}

func adblockPageHandler(c *fiber.Ctx) error {
	return webtemplates.RenderTemplate(c, "adblock", fiber.Map{
		"Title": i18n.T(c, "adblock.overview", "AdBlock (Blocky)"),
	})
}

func hostapdPageHandler(c *fiber.Ctx) error {
	return webtemplates.RenderTemplate(c, "hostapd", fiber.Map{
		"Title":          i18n.T(c, "hostapd.overview", "Hotspot Overview"),
		"hostapd_stats":  fiber.Map{},
		"hostapd_status": fiber.Map{},
		"hostapd_config": fiber.Map{},
		"last_update":    time.Now().Unix(),
	})
}

func profilePageHandler(c *fiber.Ctx) error {
	user, ok := middleware.GetUser(c)
	if !ok {
		return c.Redirect("/login")
	}
	logs, _, _ := database.GetLogs("all", 10, 0)
	type activity struct {
		Action      string
		Timestamp   string
		Description string
		IPAddress   string
	}
	var activities []activity
	for _, l := range logs {
		activities = append(activities, activity{
			Action:      l.Source,
			Timestamp:   l.CreatedAt.Format(time.RFC3339),
			Description: l.Message,
			IPAddress:   "-",
		})
	}

	configs, _ := database.GetAllConfigs()
	configsJSON, _ := json.Marshal(configs)
	return webtemplates.RenderTemplate(c, "profile", fiber.Map{
		"Title": i18n.T(c, "auth.profile", "Profile"),
		"user":  user,
		"recent_activities": activities,
		"settings":          configs,
		"settings_json":     string(configsJSON),
		"last_update":       time.Now().Unix(),
	})
}

func systemPageHandler(c *fiber.Ctx) error {
	return webtemplates.RenderTemplate(c, "system", fiber.Map{
		"Title": i18n.T(c, "system.title", "System Manager"),
	})
}

func monitoringPageHandler(c *fiber.Ctx) error {
	return webtemplates.RenderTemplate(c, "monitoring", fiber.Map{
		"Title": i18n.T(c, "monitoring.title", "Monitoring"),
	})
}

func updatePageHandler(c *fiber.Ctx) error {
	return webtemplates.RenderTemplate(c, "update", fiber.Map{
		"Title": i18n.T(c, "update.title", "Updates"),
	})
}

func firstLoginPageHandler(c *fiber.Ctx) error {
	return webtemplates.RenderTemplate(c, "first_login", fiber.Map{
		"Title": i18n.T(c, "auth.first_login", "First Login"),
	})
}

func setupWizardPageHandler(c *fiber.Ctx) error {
	return webtemplates.RenderTemplate(c, "setup_wizard", fiber.Map{
		"Title":      i18n.T(c, "setup_wizard.title", "Configuración inicial"),
		"last_update": time.Now().Unix(),
	})
}

func setupWizardVpnPageHandler(c *fiber.Ctx) error {
	return webtemplates.RenderTemplate(c, "setup_wizard_vpn", fiber.Map{
		"Title":      i18n.T(c, "setup_wizard.security_vpn", "VPN"),
		"last_update": time.Now().Unix(),
	})
}

func setupWizardWireguardPageHandler(c *fiber.Ctx) error {
	return webtemplates.RenderTemplate(c, "setup_wizard_wireguard", fiber.Map{
		"Title":      i18n.T(c, "setup_wizard.security_wireguard", "WireGuard"),
		"last_update": time.Now().Unix(),
	})
}

func setupWizardTorPageHandler(c *fiber.Ctx) error {
	return webtemplates.RenderTemplate(c, "setup_wizard_tor", fiber.Map{
		"Title":      i18n.T(c, "setup_wizard.security_tor", "Tor"),
		"last_update": time.Now().Unix(),
	})
}
