// Package wifisetup gestiona el token opcional para llamar a la API WiFi de configuración
// sin sesión JWT (p. ej. scripts o recuperación con acceso físico/consola).
package wifisetup

import (
	"crypto/subtle"
	"strings"
	"sync"

	"hostberry/internal/config"
	"hostberry/internal/database"
	"hostberry/internal/models"
)

const (
	// HeaderName es la cabecera HTTP aceptada para el token de setup WiFi.
	HeaderName = "X-HostBerry-WiFi-Setup-Token"
)

var (
	mu                 sync.RWMutex
	tokenRaw           []byte
	explicitToken      bool
	setupBypassAllowed = true
)

// Init carga el token desde config (si está definido) o genera uno aleatorio por arranque.
func Init() {
	mu.Lock()
	defer mu.Unlock()
	s := strings.TrimSpace(config.AppConfig.Security.WifiSetupToken)
	explicitToken = s != ""
	if explicitToken {
		tokenRaw = []byte(s)
		return
	}
	tokenRaw = []byte(config.GenerateRandomSecret())
}

// RefreshSetupMode recalcula si el bypass WiFi sin JWT está permitido (llamar tras Init de BD).
func RefreshSetupMode() {
	mu.Lock()
	defer mu.Unlock()
	if explicitToken {
		setupBypassAllowed = true
		return
	}
	if database.DB == nil {
		setupBypassAllowed = true
		return
	}
	var count int64
	if err := database.DB.Model(&models.User{}).Count(&count).Error; err != nil {
		setupBypassAllowed = true
		return
	}
	if count == 0 {
		setupBypassAllowed = true
		return
	}
	var pending int64
	if err := database.DB.Model(&models.User{}).Where("first_login_completed = ?", false).Count(&pending).Error; err != nil {
		setupBypassAllowed = false
		return
	}
	setupBypassAllowed = pending > 0
}

// BypassAllowed indica si la API WiFi puede aceptar el token de setup sin JWT.
func BypassAllowed() bool {
	mu.RLock()
	defer mu.RUnlock()
	return setupBypassAllowed
}

// TokenForDisplay devuelve el token actual para mostrar en logs (no usar en tests de igualdad).
func TokenForDisplay() string {
	mu.RLock()
	defer mu.RUnlock()
	return string(tokenRaw)
}

// Valid comprueba el token proporcionado con comparación en tiempo constante.
func Valid(provided string) bool {
	provided = strings.TrimSpace(provided)
	if provided == "" {
		return false
	}
	mu.RLock()
	defer mu.RUnlock()
	if !setupBypassAllowed {
		return false
	}
	if len(tokenRaw) == 0 {
		return false
	}
	if len(provided) != len(tokenRaw) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(provided), tokenRaw) == 1
}

// ExtractFromRequest obtiene el candidato a token solo desde cabecera.
func ExtractFromRequest(getHeader func(string) string, getQuery func(string) string) string {
	if getHeader != nil {
		if v := strings.TrimSpace(getHeader(HeaderName)); v != "" {
			return v
		}
	}
	return ""
}

// DisableSetupBypass desactiva el bypass tras completar la configuración inicial.
func DisableSetupBypass() {
	mu.Lock()
	defer mu.Unlock()
	if explicitToken {
		return
	}
	setupBypassAllowed = false
	tokenRaw = nil
}
