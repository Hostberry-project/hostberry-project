package vpn

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"hostberry/internal/i18n"
	"hostberry/internal/utils"
	"hostberry/internal/validators"
)

const openvpnClientConfigPath = "/etc/openvpn/client.conf"

// executeCommand proxy para mantener compatibilidad interna con el código original.
func executeCommand(cmd string) (string, error) {
	return utils.ExecuteCommand(cmd)
}

func readRedactedConfigMetadata(path, vpnName string) map[string]interface{} {
	result := map[string]interface{}{
		"success":  true,
		"exists":   false,
		"redacted": true,
		"message":  fmt.Sprintf("La configuración de %s no se devuelve por API por seguridad", vpnName),
	}

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return result
		}
		result["success"] = false
		result["error"] = fmt.Sprintf("Error accediendo a la configuración de %s: %v", vpnName, err)
		return result
	}

	result["exists"] = true
	result["size"] = info.Size()
	return result
}

func getOpenVPNServiceStatus() string {
	if out, err := exec.Command("systemctl", "is-active", "openvpn").Output(); err == nil {
		status := strings.TrimSpace(string(out))
		if status != "" {
			return status
		}
	}
	if err := exec.Command("pgrep", "openvpn").Run(); err == nil {
		return "active"
	}
	return "inactive"
}

func wgShow(args ...string) (string, error) {
	out, err := exec.Command("wg", args...).Output()
	return strings.TrimSpace(string(out)), err
}

func wgInterfaces() []string {
	out, err := wgShow("show", "interfaces")
	if err != nil || out == "" {
		return []string{}
	}
	fields := strings.Fields(out)
	result := make([]string, 0, len(fields))
	for _, iface := range fields {
		if validators.ValidateIfaceName(iface) == nil {
			result = append(result, iface)
		}
	}
	return result
}

func getOpenVPNConfig() map[string]interface{} {
	return readRedactedConfigMetadata(openvpnClientConfigPath, "OpenVPN")
}

func saveOpenVPNConfig(config, user string) map[string]interface{} {
	result := make(map[string]interface{})
	if config == "" {
		result["success"] = false
		result["error"] = "Configuración requerida"
		return result
	}
	if err := validators.ValidateVPNConfig(config); err != nil {
		result["success"] = false
		result["error"] = err.Error()
		return result
	}
	if user == "" {
		user = "unknown"
	}
	if err := os.WriteFile(openvpnClientConfigPath, []byte(config), 0600); err != nil {
		result["success"] = false
		result["error"] = fmt.Sprintf("Error guardando configuración: %v", err)
		return result
	}
	i18n.LogTf("logs.vpn_openvpn_config_saved", user)
	result["success"] = true
	result["message"] = "Configuración OpenVPN guardada"
	return result
}

func getVPNStatus() map[string]interface{} {
	result := make(map[string]interface{})

	openvpnStatus := getOpenVPNServiceStatus()

	result["openvpn"] = map[string]interface{}{
		"active": openvpnStatus == "active",
		"status": openvpnStatus,
	}

	wgOut, _ := wgShow("show")
	wgActive := wgOut != ""

	result["wireguard"] = map[string]interface{}{
		"active":     wgActive,
		"interfaces": []string{},
	}

	if wgActive {
		result["wireguard"] = map[string]interface{}{
			"active":     wgActive,
			"interfaces": wgInterfaces(),
		}
	}

	result["success"] = true
	return result
}

func connectVPN(config, vpnType, user string) map[string]interface{} {
	result := make(map[string]interface{})

	if config == "" {
		result["success"] = false
		result["error"] = "Configuración requerida"
		return result
	}
	if vpnType == "wireguard" {
		if err := validators.ValidateWireGuardConfig(config); err != nil {
			result["success"] = false
			result["error"] = err.Error()
			return result
		}
	} else {
		if err := validators.ValidateVPNConfig(config); err != nil {
			result["success"] = false
			result["error"] = err.Error()
			return result
		}
	}

	if vpnType == "" {
		vpnType = "openvpn"
	}
	if user == "" {
		user = "unknown"
	}

	i18n.LogTf("logs.vpn_connecting", vpnType, user)

	if vpnType == "openvpn" {
		configFile := "/etc/openvpn/client.conf"
		if err := os.WriteFile(configFile, []byte(config), 0600); err != nil {
			result["success"] = false
			result["error"] = fmt.Sprintf("Error guardando configuración: %v", err)
			return result
		}

		cmd := "sudo systemctl start openvpn@client"
		if out, err := executeCommand(cmd); err != nil {
			result["success"] = false
			result["error"] = err.Error()
			if out != "" {
				result["error"] = strings.TrimSpace(out)
			}
		} else {
			result["success"] = true
			result["message"] = "OpenVPN iniciado"
		}
	} else if vpnType == "wireguard" {
		configFile := "/etc/wireguard/wg0.conf"
		if err := os.WriteFile(configFile, []byte(config), 0600); err != nil {
			result["success"] = false
			result["error"] = fmt.Sprintf("Error guardando configuración: %v", err)
			return result
		}

		cmd := "sudo wg-quick up wg0"
		if out, err := executeCommand(cmd); err != nil {
			result["success"] = false
			result["error"] = err.Error()
			if out != "" {
				result["error"] = strings.TrimSpace(out)
			}
		} else {
			result["success"] = true
			result["message"] = "WireGuard activado"
		}
	} else {
		result["success"] = false
		result["error"] = fmt.Sprintf("Tipo de VPN no soportado: %s", vpnType)
	}

	return result
}

func getWireGuardStatus() map[string]interface{} {
	result := make(map[string]interface{})

	wgOut, _ := wgShow("show")
	wgActive := wgOut != ""

	result["active"] = wgActive
	result["interfaces"] = []map[string]interface{}{}

	if wgActive {
		interfaceList := []map[string]interface{}{}
		for _, iface := range wgInterfaces() {
			interfaceInfo := map[string]interface{}{
				"name": iface,
			}
			if detailsOut, err := wgShow("show", iface); err == nil {
				interfaceInfo["details"] = detailsOut
			} else {
				interfaceInfo["details"] = ""
			}
			interfaceList = append(interfaceList, interfaceInfo)
		}
		result["interfaces"] = interfaceList
	} else {
		result["message"] = "WireGuard no está activo"
	}

	result["success"] = true
	return result
}

func configureWireGuard(config, user string) map[string]interface{} {
	result := make(map[string]interface{})

	if err := validators.ValidateWireGuardConfig(config); err != nil {
		result["success"] = false
		result["error"] = err.Error()
		return result
	}

	if user == "" {
		user = "unknown"
	}

	i18n.LogTf("logs.vpn_wireguard_config", user)

	configFile := "/etc/wireguard/wg0.conf"
	if err := os.WriteFile(configFile, []byte(config), 0600); err != nil {
		result["success"] = false
		result["error"] = fmt.Sprintf("Error guardando configuración: %v", err)
		result["message"] = "Error guardando configuración"
		i18n.LogTf("logs.vpn_wireguard_error", err)
		return result
	}

	if wgStatusOut, err := wgShow("show", "wg0"); err == nil && wgStatusOut != "" {
		executeCommand("sudo wg-quick down wg0 2>/dev/null")
		executeCommand("sudo wg-quick up wg0 2>/dev/null")
		result["message"] = "WireGuard reiniciado con nueva configuración"
	} else {
		result["message"] = "Configuración guardada (no activa)"
	}

	result["success"] = true
	i18n.LogT("logs.vpn_wireguard_success")
	return result
}

// ---- wrappers exportados ----

func GetOpenVPNConfig() map[string]interface{} { return getOpenVPNConfig() }
func GetWireGuardConfigMetadata() map[string]interface{} {
	return readRedactedConfigMetadata("/etc/wireguard/wg0.conf", "WireGuard")
}
func SaveOpenVPNConfig(config, user string) map[string]interface{} {
	return saveOpenVPNConfig(config, user)
}
func GetVPNStatus() map[string]interface{} { return getVPNStatus() }
func ConnectVPN(config, vpnType, user string) map[string]interface{} {
	return connectVPN(config, vpnType, user)
}
func GetWireGuardStatus() map[string]interface{} { return getWireGuardStatus() }
func ConfigureWireGuard(config, user string) map[string]interface{} {
	return configureWireGuard(config, user)
}

