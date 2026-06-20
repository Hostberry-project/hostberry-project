package captiveportal

import "strings"

// ProbePaths son rutas que los SO móviles consultan para detectar portal cautivo.
// Deben coincidir con captivePortalAllowedPath en middleware.
var ProbePaths = []string{
	"/generate_204",
	"/gen_204",
	"/generate_205",
	"/library/test/success.html",
	"/success.txt",
	"/ncsi.txt",
	"/connecttest.txt",
	"/canonical.html",
	"/redirect",
	"/hotspot-detect.html",
	"/hotspotdetect.html",
	"/check_network_status.txt",
	"/mobile/status.php",
	"/kindle-wifi/wifistub.html",
	"/kindle-wifi/wifiredirect.html",
}

// APIPath es el endpoint JSON (RFC 8908) anunciado por DHCP option 114.
const APIPath = "/api/captive-portal"

// IsProbePath indica si la ruta es una comprobación de portal cautivo del SO.
func IsProbePath(path string) bool {
	path = strings.TrimSuffix(path, "/")
	if path == "" {
		path = "/"
	}
	for _, p := range ProbePaths {
		if path == p {
			return true
		}
	}
	if strings.HasPrefix(path, "/kindle-wifi/") {
		return true
	}
	return false
}

// IsAllowedWebPath indica si un cliente AP puede acceder sin redirección del middleware.
func IsAllowedWebPath(path string) bool {
	if path == "/login" || path == "/first-login" || path == "/portal" || path == "/setup-wizard" {
		return true
	}
	if strings.HasPrefix(path, "/setup-wizard/") {
		return true
	}
	if path == APIPath {
		return true
	}
	return IsProbePath(path)
}
