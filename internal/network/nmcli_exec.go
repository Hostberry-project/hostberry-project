package network

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"time"
)

const nmcliTimeout = 45 * time.Second

// runSudoNmcli ejecuta nmcli con argumentos separados (sin shell) bajo sudo.
func runSudoNmcli(args ...string) (stdoutStderr string, err error) {
	if len(args) == 0 {
		return "", nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), nmcliTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sudo", append([]string{"nmcli"}, args...)...)
	cmd.Env = append(os.Environ(), "SUDO_ASKPASS=/bin/false")
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}
