package system

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"hostberry/internal/auth"
	"hostberry/internal/database"
	"hostberry/internal/hostapd"
	"hostberry/internal/i18n"
	middleware "hostberry/internal/middleware"
	"hostberry/internal/wifi"
)

// SetupWizardCompleteAPIHandler marca el wizard como completado y devuelve la ruta siguiente.
func SetupWizardCompleteAPIHandler(c *fiber.Ctx) error {
	user, ok := middleware.GetUser(c)
	if !ok || user == nil {
		return c.Status(401).JSON(fiber.Map{
			"error": i18n.T(c, "auth.unauthorized", "Unauthorized"),
		})
	}

	reboot := false
	if !user.SetupWizardCompleted {
		user.SetupWizardCompleted = true
		if err := database.DB.Save(user).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{
				"error": i18n.T(c, "errors.general_error_message", "An unexpected error occurred"),
			})
		}
		_ = database.SetConfig("setup_wizard_completed_by_api", "1")
		// La configuración aplicada queda confirmada: ya no debe revertirse al reabrir.
		ClearWizardDirty()
		if err := hostapd.ApplySingleClientAPLimit(false); err != nil {
			return c.Status(500).JSON(fiber.Map{
				"error": i18n.T(c, "setup_wizard.release_ap_limit_failed", "Setup saved but the access point could not allow more clients. Check hostapd and dnsmasq."),
			})
		}
		// Desactivar el portal cautivo y habilitar el reparto de Internet del AP. Se persiste en
		// disco (config dnsmasq + servicio captivo deshabilitado + sysctl), de modo que tras el
		// reinicio el AP "hostberry" da Internet y el panel queda accesible en hostberry.local.
		hostapd.ApplyNormalAPMode()
		if band := wifi.GetWizardPreferredBand(); band != "" {
			_ = wifi.PersistPreferredBandForReboot(band)
		}
		// Aplicar la red WiFi elegida en el asistente: se deja lista en wpa_supplicant para que el
		// equipo se conecte al arrancar tras el reinicio (la conexión se difiere a "Finalizar").
		_ = wifi.PersistPendingWiFiForReboot()
		wifi.ClearWizardPendingWiFi()
		// Devolver wlan0 a NetworkManager: durante el asistente estuvo bajo un supplicant dedicado
		// (sin autoscan). Al reiniciar, NM debe gestionarlo de nuevo para la autoconexión normal.
		wifi.StopSetupModeSupplicant()
		reboot = true
		username := user.Username
		go func() {
			time.Sleep(2 * time.Second)
			SystemRestart(username)
		}()
	}

	redirect := "/first-login"
	if !auth.IsPasswordChangeRequired(user) {
		redirect = "/dashboard"
	}

	return c.JSON(fiber.Map{
		"message":  i18n.T(c, "setup_wizard.completed_redirect", "Setup complete. Continue to secure your account."),
		"redirect": redirect,
		"reboot":   reboot,
	})
}
