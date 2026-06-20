package security

import (
	"os"
	"path/filepath"
)

const installCredentialsName = "INSTALL_CREDENTIALS.txt"

// InstallCredentialsCandidates devuelve rutas posibles del fichero de credenciales iniciales.
func InstallCredentialsCandidates() []string {
	candidates := []string{
		filepath.Join("/opt/hostberry", installCredentialsName),
	}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), installCredentialsName))
	}
	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(wd, installCredentialsName))
	}
	return candidates
}

// RemoveInstallCredentialsFile elimina el fichero de credenciales generado en la instalación.
func RemoveInstallCredentialsFile() {
	for _, path := range InstallCredentialsCandidates() {
		if path == "" {
			continue
		}
		_ = os.Remove(path)
	}
}
