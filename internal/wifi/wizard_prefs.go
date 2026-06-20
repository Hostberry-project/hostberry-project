package wifi

import (
	"strings"

	"hostberry/internal/database"
)

const WizardPreferredBandConfigKey = "wizard_preferred_wifi_band"

// SaveWizardPreferredBand guarda la banda WiFi upstream elegida en el asistente (2.4 o 5).
func SaveWizardPreferredBand(band string) error {
	band = strings.TrimSpace(band)
	if band != band24GHz && band != band5GHz {
		return nil
	}
	return database.SetConfig(WizardPreferredBandConfigKey, band)
}

// GetWizardPreferredBand devuelve la banda guardada en el wizard o "" si no hay preferencia.
func GetWizardPreferredBand() string {
	val, err := database.GetConfig(WizardPreferredBandConfigKey)
	if err != nil {
		return ""
	}
	switch strings.TrimSpace(val) {
	case band24GHz, band5GHz:
		return strings.TrimSpace(val)
	default:
		return ""
	}
}

// PersistPreferredBandForReboot escribe hostapd.conf para la banda elegida antes del reinicio
// post-wizard. El arranque en frío aplica el canal sin cortar clientes del asistente.
func PersistPreferredBandForReboot(band string) error {
	if band != band24GHz && band != band5GHz {
		return nil
	}
	ch := defaultAPChannelForBand(band)
	mode := "g"
	if band == band5GHz {
		mode = "a"
	}
	return writeHostapdChannelMode(ch, mode)
}
