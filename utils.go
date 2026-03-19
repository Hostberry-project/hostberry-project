package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"hostberry/internal/auth"
	"hostberry/internal/config"
	"hostberry/internal/database"
	"hostberry/internal/i18n"
	"hostberry/internal/models"
	"hostberry/internal/validators"
)

const (
	defaultCommandTimeout = 30 * time.Second
	cacheTTL              = 5 * time.Second
)

type cachedResult struct {
	output    string
	err       error
	timestamp time.Time
}

var (
	commandCache  = make(map[string]*cachedResult)
	cacheMutex    sync.RWMutex
	sudoAvailable *bool
)

func generateSecureAdminPassword() (string, error) {
	randomBytes := make([]byte, 12)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}
	// Cumple requisitos: mayúscula, minúscula, número y carácter especial.
	return fmt.Sprintf("Hb!%s9aA", hex.EncodeToString(randomBytes)), nil
}

func createDefaultAdmin() {
	var count int64
	if err := database.DB.Model(&models.User{}).Count(&count).Error; err != nil {
		if config.AppConfig.Server.Debug {
			i18n.LogTf("logs.utils_count_error", err)
		}
		return
	}

	if config.AppConfig.Server.Debug {
		i18n.LogTf("logs.utils_users_count", count)
	}

	if count == 0 {
		if config.AppConfig.Server.Debug {
			i18n.LogTln("logs.utils_creating_admin")
		}

		adminPassword := strings.TrimSpace(os.Getenv("HOSTBERRY_DEFAULT_ADMIN_PASSWORD"))
		useBootstrap := false
		if adminPassword == "" {
			adminPassword = "admin"
			useBootstrap = true
			log.Printf("SECURITY: Primer arranque. Usuario admin creado con contraseña por defecto. Cámbiala en Ajustes tras el primer acceso.")
		} else if err := validators.ValidatePassword(adminPassword); err != nil {
			i18n.LogTf("logs.utils_admin_error", fmt.Errorf("HOSTBERRY_DEFAULT_ADMIN_PASSWORD inválida: %w", err))
			return
		}

		var admin *models.User
		var err error
		if useBootstrap {
			admin, err = auth.RegisterBootstrap("admin", adminPassword, "admin@hostberry.local")
		} else {
			admin, err = auth.Register("admin", adminPassword, "admin@hostberry.local")
		}
		if err != nil {
			i18n.LogTf("logs.utils_admin_error", err)
		} else {
			if config.AppConfig.Server.Debug {
				i18n.LogT("logs.utils_admin_success")
			}
			_ = admin
		}
	}
}

func executeCommand(cmd string) (string, error) {
	return executeCommandWithTimeout(cmd, defaultCommandTimeout)
}

func executeCommandWithTimeout(cmd string, timeout time.Duration) (string, error) {
	cacheKey := cmd + "|" + timeout.String()

	cacheMutex.RLock()
	if cached, exists := commandCache[cacheKey]; exists {
		if time.Since(cached.timestamp) < cacheTTL {
			cacheMutex.RUnlock()
			return cached.output, cached.err
		}
	}
	cacheMutex.RUnlock()

	allowedCommands := []string{
		"hostname", "hostnamectl", "uname", "cat", "grep", "awk", "sed", "cut", "head", "tail",
		"top", "free", "df", "nproc",
		"iwlist", "nmcli", "iw",
		"ip", "wg", "wg-quick", "systemctl", "pgrep",
		"sudo", "sh", "reboot", "shutdown", "poweroff",
		"rfkill", "ifconfig", "iwconfig",
		"hostapd", "hostapd_cli", "dnsmasq", "iptables", "iptables-save", "netfilter-persistent", "sysctl", "tee", "cp", "mkdir", "echo", "chmod", "bash", "cat",
		"dhclient", "udhcpc", "wpa_supplicant", "wpa_cli", "pkill", "killall",
	}

	noSudoCommands := []string{
		"hostname", "uname", "cat", "grep", "awk", "sed", "cut", "head", "tail",
		"free", "df", "nproc", "pgrep",
	}

	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return "", nil
	}

	commandIndex := 0
	hasSudo := false
	if len(parts) > 1 && parts[0] == "sudo" {
		commandIndex = 1
		hasSudo = true
	}

	if commandIndex >= len(parts) {
		return "", exec.ErrNotFound
	}

	command := parts[commandIndex]
	allowed := false
	for _, allowedCmd := range allowedCommands {
		if command == allowedCmd {
			allowed = true
			break
		}
	}

	if !allowed {
		return "", exec.ErrNotFound
	}

	needsSudo := true
	for _, noSudoCmd := range noSudoCommands {
		if command == noSudoCmd {
			needsSudo = false
			break
		}
	}

	if !needsSudo && hasSudo {
		cmd = strings.Join(parts[1:], " ")
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	baseCmd := execCommand(cmd)
	cmdObj := exec.CommandContext(ctx, baseCmd.Path)
	cmdObj.Args = baseCmd.Args
	cmdObj.Env = append(os.Environ(),
		"SUDO_ASKPASS=/bin/false",
		"SUDO_LOG_FILE=",
		"HOSTNAME="+getHostname(),
	)

	out, err := cmdObj.CombinedOutput()
	outputStr := filterSudoErrorString(string(out))

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			err = exec.ErrNotFound
		}
		if outputStr == "" {
			cacheMutex.Lock()
			commandCache[cacheKey] = &cachedResult{output: "", err: err, timestamp: time.Now()}
			cacheMutex.Unlock()
			return "", err
		}
	}

	result := strings.TrimSpace(outputStr)

	cacheMutex.Lock()
	commandCache[cacheKey] = &cachedResult{output: result, err: err, timestamp: time.Now()}
	if len(commandCache) > 100 {
		for k := range commandCache {
			if time.Since(commandCache[k].timestamp) > cacheTTL*2 {
				delete(commandCache, k)
			}
		}
	}
	cacheMutex.Unlock()

	return result, err
}

func filterSudoErrors(output []byte) string {
	return filterSudoErrorString(string(output))
}

func filterSudoErrorString(output string) string {
	lines := strings.Split(output, "\n")
	var cleanLines []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" &&
			!strings.Contains(line, "sudo: unable to open log file") &&
			!strings.Contains(line, "Read-only file system") &&
			!strings.Contains(line, "sudo: unable to stat") &&
			!strings.Contains(line, "sudo: unable to resolve host") &&
			!strings.Contains(line, "Name or service not known") {
			cleanLines = append(cleanLines, line)
		}
	}
	return strings.Join(cleanLines, "\n")
}

func getHostname() string {
	hostname := os.Getenv("HOSTNAME")
	if hostname == "" {
		if h, err := exec.Command("hostname").Output(); err == nil {
			hostname = strings.TrimSpace(string(h))
		}
	}
	return hostname
}

func canUseSudo() bool {
	if sudoAvailable != nil {
		return *sudoAvailable
	}

	result := false
	defer func() {
		sudoAvailable = &result
	}()

	if os.Geteuid() == 0 {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	sudoCheck := exec.CommandContext(ctx, "sh", "-c", "command -v sudo 2>/dev/null")
	if sudoCheck.Run() != nil {
		return false
	}

	testCmd := exec.CommandContext(ctx, "sh", "-c", "sudo -n true 2>&1")
	output, err := testCmd.CombinedOutput()
	outputStr := strings.ToLower(string(output))

	if err == nil {
		result = true
		return true
	}

	if strings.Contains(outputStr, "no new privileges") {
		result = false
		return false
	}

	if strings.Contains(outputStr, "password") || strings.Contains(outputStr, "a password is required") {
		result = true
		return true
	}

	return false
}

func execCommand(cmd string) *exec.Cmd {
	cmd = strings.TrimSpace(cmd)
	cmd = strings.TrimPrefix(cmd, "sudo ")

	if os.Geteuid() == 0 {
		return exec.Command("sh", "-c", cmd)
	}

	if canUseSudo() {
		cmd = "sudo " + cmd
	}

	return exec.Command("sh", "-c", cmd)
}

func clearCommandCache() {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()
	commandCache = make(map[string]*cachedResult)
}

// strconvAtoiSafe parsea un string como entero sin signo (solo dígitos), sin usar strconv.Atoi.
func strconvAtoiSafe(s string) (int, error) {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("invalid int")
		}
		n = n*10 + int(r-'0')
	}
	return n, nil
}

func mapActiveStatus(status string) string {
	status = strings.ToLower(strings.TrimSpace(status))
	if status == "active" {
		return "connected"
	}
	return "disconnected"
}

func mapBoolStatus(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	if v == "true" || v == "1" || v == "yes" {
		return "connected"
	}
	return "disconnected"
}
