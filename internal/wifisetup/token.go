// Package wifisetup gestiona el token opcional para llamar a la API WiFi de configuración
// sin sesión JWT (p. ej. scripts o recuperación con acceso físico/consola).
package wifisetup

import (
	"crypto/subtle"
	"strings"
	"sync"

	"hostberry/internal/config"
)

const (
	// HeaderName es la cabecera HTTP aceptada junto a wifi_setup_token (query).
	HeaderName = "X-HostBerry-WiFi-Setup-Token"
	// QueryParam es el nombre del parámetro de consulta alternativo.
	QueryParam = "wifi_setup_token"
)

var (
	mu       sync.RWMutex
	tokenRaw []byte
)

// Init carga el token desde config (si está definido) o genera uno aleatorio por arranque.
func Init() {
	mu.Lock()
	defer mu.Unlock()
	s := strings.TrimSpace(config.AppConfig.Security.WifiSetupToken)
	if s != "" {
		tokenRaw = []byte(s)
		return
	}
	tokenRaw = []byte(config.GenerateRandomSecret())
}

// TokenForDisplay devuelve el token actual para mostrar en logs (no usar en tests de igualdad).
func TokenForDisplay() string {
	mu.RLock()
	defer mu.RUnlock()
	return string(tokenRaw)
}

// Valid comprueba el token proporcionado (cabecera o query) con comparación en tiempo constante.
func Valid(provided string) bool {
	provided = strings.TrimSpace(provided)
	if provided == "" {
		return false
	}
	mu.RLock()
	defer mu.RUnlock()
	if len(tokenRaw) == 0 {
		return false
	}
	// subtle.ConstantTimeCompare exige misma longitud
	if len(provided) != len(tokenRaw) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(provided), tokenRaw) == 1
}

// ExtractFromRequest obtiene el candidato a token de cabecera o query.
func ExtractFromRequest(getHeader func(string) string, getQuery func(string, string) string) string {
	if getHeader != nil {
		if v := strings.TrimSpace(getHeader(HeaderName)); v != "" {
			return v
		}
	}
	if getQuery != nil {
		if v := strings.TrimSpace(getQuery(QueryParam, "")); v != "" {
			return v
		}
	}
	return ""
}
