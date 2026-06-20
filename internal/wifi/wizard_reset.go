package wifi


// ResetWizardNetworkState revierte la configuración de red SENSIBLE aplicada durante el
// asistente inicial cuando este se reabre sin haberse finalizado:
//   - Borra las redes WiFi (STA) conocidas y desconecta la actual.
//   - Olvida la red WiFi pendiente del asistente.
//
// IMPORTANTE: NO se borra la configuración del punto de acceso HostBerry (SSID/clave/banda
// elegidos en el paso 3). Es la red propia del dispositivo, elegida explícitamente por el
// usuario, no un riesgo de seguridad; debe conservarse aunque el wizard se reabra sin
// finalizar (de lo contrario el usuario percibe que "no guarda la configuración").
func ResetWizardNetworkState() {
	iface := DetectWiFiInterface()
	if iface == "" {
		iface = "wlan0"
	}
	socketDir := findWorkingWpaSupplicantSocket(iface)
	if socketDir == "" && len(WpaSocketDirs) > 0 {
		socketDir = WpaSocketDirs[0]
	}

	// 1. Olvidar redes WiFi STA (desconectar + remove_network all + perfiles NM).
	clearLeftoverWiFiNetworks(iface, socketDir)

	// 2. Olvidar la red WiFi pendiente del asistente (STA), conservando el AP HostBerry.
	ClearWizardPendingWiFi()

	// 3. Reconstruir/mantener el AP de configuración a partir de los perfiles GUARDADOS
	//    (o por defecto si el usuario aún no los configuró) para seguir emitiendo el AP.
	EnsureWizardAPBroadcasting("")
	EnsureDualBandHostapd("", true)
}
