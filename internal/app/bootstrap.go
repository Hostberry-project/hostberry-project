// Package app centraliza el arranque de HostBerry (facilita tests e inyección futura).
package app

import (
	"embed"

	"hostberry/internal/auth"
	"hostberry/internal/config"
	"hostberry/internal/database"
	"hostberry/internal/hostapd"
	"hostberry/internal/i18n"
	"hostberry/internal/logging"
	"hostberry/internal/utils"
	"hostberry/internal/wifisetup"
	"hostberry/internal/wifi"
	server "hostberry/internal/server"

	"github.com/gofiber/fiber/v2"
)

// Bootstrap inicializa subsistemas y devuelve la aplicación Fiber configurada.
func Bootstrap(templatesFS, staticFS embed.FS) (*fiber.App, error) {
	if err := config.Load(); err != nil {
		return nil, err
	}
	config.Normalize(i18n.LogTf)

	if cfg := config.AppConfig; cfg != nil {
		_ = logging.Init(logging.Config{
			Level:      cfg.Logging.Level,
			File:       cfg.Logging.File,
			MaxSizeMB:  cfg.Logging.MaxSize,
			MaxBackups: cfg.Logging.MaxBackups,
		})
	}

	wifisetup.Init()
	if err := i18n.Init("locales"); err != nil {
		i18n.LogTf("logs.i18n_init_warning", err)
	}
	if err := database.Init(); err != nil {
		return nil, err
	}
	wifisetup.RefreshSetupMode()

	if configs, err := database.GetAllConfigs(); err == nil {
		if lang, ok := configs["log_language"]; ok && lang != "" {
			i18n.SetLogLanguage(lang)
		}
	}

	i18n.LogTln("logs.checking_admin")
	utils.CreateDefaultAdmin()
	if auth.IsInitialSetupPending() {
		if err := hostapd.ApplySingleClientAPLimit(true); err != nil {
			i18n.LogTf("logs.setup_single_client_limit_warning", err)
		} else {
			i18n.LogTln("logs.setup_single_client_limit_active")
		}
		// Radio única: durante el asistente, wlan0 sale de NetworkManager y queda bajo un
		// wpa_supplicant dedicado sin autoscan, para que los escaneos no tiren al cliente del portal.
		wifi.StartSetupModeSupplicant()
		wifi.EnsureWizardAPBroadcasting("")
		wifi.EnsureDualBandHostapd("", true)
		hostapd.EnsureAPRunningForSetup()
		// Restaurar el portal cautivo por si un asistente anterior lo desmontó (modo normal).
		// Es idempotente: en una instalación nueva no cambia nada.
		go hostapd.ApplyCaptivePortalMode()
		// Calentar la caché de escaneo para que el primer "Buscar redes" del wizard sea rápido.
		wifi.StartScanPrefetch()
	} else {
		// Setup completado: devolver wlan0 a NetworkManager por si quedó el modo setup de un
		// asistente anterior (idempotente; permite la autoconexión normal tras el reinicio).
		wifi.StopSetupModeSupplicant()
		// Setup completado: el AP "hostberry" pasa a modo normal (sin portal cautivo) y comparte
		// Internet del uplink (Ethernet o WiFi STA). Las reglas iptables no persisten al reiniciar,
		// así que se reaplican en cada arranque. Se hace en segundo plano para no retrasar el panel.
		go hostapd.ApplyNormalAPMode()
	}
	i18n.LogTf("logs.wifi_setup_token_info", wifisetup.HeaderName)
	wifi.StartWiFiAutoConnectDaemon()

	app := server.CreateApp(templatesFS, staticFS)
	server.SetupRoutes(app)
	return app, nil
}
