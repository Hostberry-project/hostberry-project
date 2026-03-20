package main

import (
	"embed"

	"hostberry/internal/config"
	"hostberry/internal/database"
	"hostberry/internal/i18n"
	"hostberry/internal/utils"
	"hostberry/internal/wifisetup"
	"hostberry/internal/wifi"
	server "hostberry/internal/server"
)

// Embebemos templates y estáticos para reducir lecturas a disco en Raspberry Pi.
// Esto acelera la carga inicial y evita depender de rutas en filesystem.
//go:embed website/templates
var templatesFS embed.FS

//go:embed website/static
var staticFS embed.FS

func main() {
	if err := config.Load(); err != nil {
		i18n.LogTfatal("logs.config_load_error", err)
	}
	config.Normalize(i18n.LogTf)
	wifisetup.Init()

	if err := i18n.Init("locales"); err != nil {
		i18n.LogTf("logs.i18n_init_warning", err)
	}

	if err := database.Init(); err != nil {
		i18n.LogTfatal("logs.db_init_error", err)
	}

	// Establecer idioma de logs desde configuración del sistema (después de inicializar BD).
	if configs, err := database.GetAllConfigs(); err == nil {
		if lang, ok := configs["log_language"]; ok && lang != "" {
			i18n.SetLogLanguage(lang)
		}
	}

	i18n.LogTln("logs.checking_admin")
	utils.CreateDefaultAdmin()

	// No mostrar el token en claro: solo mostramos el mecanismo (cabecera/query) habilitado para recovery o automatización.
	i18n.LogTf("logs.wifi_setup_token_info", wifisetup.HeaderName)

	wifi.StartWiFiAutoConnectDaemon()

	app := server.CreateApp(templatesFS, staticFS)
	server.SetupRoutes(app)
	server.Start(app)
}

