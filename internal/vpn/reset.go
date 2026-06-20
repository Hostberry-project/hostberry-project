package vpn

import "os"

// ResetVPNConfig revierte toda la configuración VPN (OpenVPN y WireGuard) aplicada durante el
// asistente: detiene los servicios/túneles y elimina los ficheros de configuración. Se usa al
// reabrir el asistente sin finalizarlo, para volver a un estado limpio.
func ResetVPNConfig() {
	executeCommand("sudo systemctl stop openvpn@client 2>/dev/null || true")
	executeCommand("sudo systemctl stop openvpn 2>/dev/null || true")
	executeCommand("sudo systemctl disable openvpn@client 2>/dev/null || true")
	executeCommand("sudo wg-quick down wg0 2>/dev/null || true")

	_ = os.Remove("/etc/openvpn/client.conf")
	_ = os.Remove("/etc/wireguard/wg0.conf")
}
