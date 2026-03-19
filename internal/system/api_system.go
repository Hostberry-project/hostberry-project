package system

import (
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"hostberry/internal/config"
	"hostberry/internal/database"
	"hostberry/internal/utils"
)

// strconvAtoiSafe wrapper para mantener el comportamiento anterior.
func strconvAtoiSafe(s string) (int, error) {
	return utils.StrconvAtoiSafe(s)
}

func systemActivityHandler(c *fiber.Ctx) error {
	limitStr := c.Query("limit", "10")
	limit := 10
	if v, err := strconvAtoiSafe(limitStr); err == nil && v > 0 && v <= 100 {
		limit = v
	}

	logs, _, err := database.GetLogs("all", limit, 0)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	var activities []fiber.Map
	for _, l := range logs {
		activities = append(activities, fiber.Map{
			"timestamp": l.CreatedAt.Format(time.RFC3339),
			"level":     l.Level,
			"message":   l.Message,
			"source":    l.Source,
		})
	}

	return c.JSON(activities)
}

func SystemActivityHandler(c *fiber.Ctx) error {
	return systemActivityHandler(c)
}

func SystemNetworkHandler(c *fiber.Ctx) error {
	out, err := os.ReadFile("/proc/net/dev")
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"raw": string(out)})
}

func SystemUpdatesHandler(c *fiber.Ctx) error {
	commands := []string{
		"apt list --upgradable 2>/dev/null | tail -n +2 | cut -d/ -f1",
		"apt-get -s upgrade 2>/dev/null | awk '/^Inst /{print $2}'",
	}

	pkgSet := make(map[string]struct{})
	for _, cmdText := range commands {
		out, err := exec.Command("sh", "-c", cmdText).Output()
		if err != nil {
			continue
		}
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			pkg := strings.TrimSpace(line)
			if pkg == "" {
				continue
			}
			pkgSet[pkg] = struct{}{}
		}
		if len(pkgSet) > 0 {
			break
		}
	}

	updates := make([]string, 0, len(pkgSet))
	for pkg := range pkgSet {
		updates = append(updates, pkg)
	}
	sort.Strings(updates)

	return c.JSON(fiber.Map{
		"success":           true,
		"updates_available":  len(updates) > 0,
		"update_count":      len(updates),
		"updates":           updates,
		"available":         len(updates) > 0, // compatibilidad con clientes antiguos
	})
}

func SystemBackupHandler(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"success": false, "message": "Backup no implementado aún"})
}

// systemHttpsInfoHandler devuelve información básica sobre el estado HTTP/HTTPS actual.
// Útil para mostrar en la página System una guía de configuración TLS.
func SystemHttpsInfoHandler(c *fiber.Ctx) error {
	isHttps := c.Secure() || strings.EqualFold(c.Get("X-Forwarded-Proto"), "https")

	return c.JSON(fiber.Map{
		"is_https":        isHttps,
		"host":            config.AppConfig.Server.Host,
		"port":            config.AppConfig.Server.Port,
		"tls_cert_file":   config.AppConfig.Server.TLSCertFile,
		"tls_key_file":    config.AppConfig.Server.TLSKeyFile,
	})
}
