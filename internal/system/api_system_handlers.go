package system

import (
	"os/exec"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"hostberry/internal/database"
	middleware "hostberry/internal/middleware"
)

func SystemStatsHandler(c *fiber.Ctx) error {
	result := GetSystemStats()
	return c.JSON(result)
}

func SystemInfoHandler(c *fiber.Ctx) error {
	result := GetSystemInfo()
	return c.JSON(result)
}

func SystemLogsHandler(c *fiber.Ctx) error {
	level := c.Query("level", "all")
	limitStr := c.Query("limit", "20")
	offsetStr := c.Query("offset", "0")

	switch level {
	case "all", "INFO", "WARNING", "ERROR", "DEBUG":
	default:
		level = "all"
	}

	limit, _ := strconv.Atoi(limitStr)
	offset, _ := strconv.Atoi(offsetStr)

	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 || offset > 10000 {
		offset = 0
	}

	logs, total, err := database.GetLogs(level, limit, offset)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"logs":  logs,
		"total": total,
		"limit": limit,
		"offset": offset,
	})
}

func SystemServicesHandler(c *fiber.Ctx) error {
	services := make(map[string]interface{})

	wgOut, _ := exec.Command("wg", "show").CombinedOutput()
	wgActive := strings.TrimSpace(string(wgOut)) != ""
	services["wireguard"] = map[string]interface{}{
		"status": wgActive,
		"active": wgActive,
	}

	openvpnOut, _ := exec.Command("sh", "-c", "systemctl is-active openvpn 2>/dev/null || pgrep openvpn > /dev/null && echo active || echo inactive").CombinedOutput()
	openvpnStatus := strings.TrimSpace(string(openvpnOut))
	openvpnActive := openvpnStatus == "active"
	services["openvpn"] = map[string]interface{}{
		"status": openvpnStatus,
		"active": openvpnActive,
	}

	pgrepOut, _ := exec.Command("sh", "-c", "pgrep hostapd > /dev/null 2>&1 && echo active || echo inactive").CombinedOutput()
	pgrepStatus := strings.TrimSpace(string(pgrepOut))

	hostapdOut, _ := exec.Command("sh", "-c", "systemctl is-active hostapd 2>/dev/null || echo inactive").CombinedOutput()
	hostapdStatus := strings.TrimSpace(string(hostapdOut))

	hostapdEnabledOut, _ := exec.Command("sh", "-c", "systemctl is-enabled hostapd 2>/dev/null || echo disabled").CombinedOutput()
	hostapdEnabledStatus := strings.TrimSpace(string(hostapdEnabledOut))
	hostapdEnabled := hostapdEnabledStatus == "enabled"

	hostapdActive := hostapdStatus == "active" || pgrepStatus == "active"
	if hostapdStatus == "inactive" && pgrepStatus == "active" {
		hostapdStatus = "active"
	}

	services["hostapd"] = map[string]interface{}{
		"status":  hostapdStatus,
		"active":  hostapdActive,
		"enabled": hostapdEnabled,
	}

	dnsmasqOut, _ := exec.Command("sh", "-c", "systemctl is-active dnsmasq 2>/dev/null || echo inactive").CombinedOutput()
	dnsmasqStatus := strings.TrimSpace(string(dnsmasqOut))
	piholeOut, _ := exec.Command("sh", "-c", "systemctl is-active pihole-FTL 2>/dev/null || echo inactive").CombinedOutput()
	piholeStatus := strings.TrimSpace(string(piholeOut))
	adblockActive := dnsmasqStatus == "active" || piholeStatus == "active"
	services["adblock"] = map[string]interface{}{
		"status": adblockActive,
		"active": adblockActive,
		"type": func() string {
			if dnsmasqStatus == "active" {
				return "dnsmasq"
			}
			if piholeStatus == "active" {
				return "pihole"
			}
			return "none"
		}(),
	}

	return c.JSON(fiber.Map{
		"services": services,
	})
}

func SystemRestartHandler(c *fiber.Ctx) error {
	user, ok := middleware.GetUser(c)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "No autorizado"})
	}
	userID := user.ID

	result := SystemRestart(user.Username)
	if success, ok := result["success"].(bool); ok && success {
		database.InsertLog("INFO", database.LogMsg("Sistema reiniciado correctamente", user.Username), "system", &userID)
		return c.JSON(result)
	}

	if errMsg, ok := result["error"].(string); ok {
		database.InsertLog("ERROR", database.LogMsgErr("reiniciar sistema", errMsg, user.Username), "system", &userID)
		return c.Status(500).JSON(fiber.Map{"error": errMsg})
	}

	return c.JSON(result)
}

func SystemShutdownHandler(c *fiber.Ctx) error {
	user, ok := middleware.GetUser(c)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "No autorizado"})
	}
	userID := user.ID

	result := SystemShutdown(user.Username)
	if success, ok := result["success"].(bool); ok && success {
		database.InsertLog("INFO", database.LogMsg("Sistema apagado correctamente", user.Username), "system", &userID)
		return c.JSON(result)
	}

	if err, ok := result["error"].(string); ok {
		database.InsertLog("ERROR", database.LogMsgErr("apagar sistema", err, user.Username), "system", &userID)
		return c.Status(500).JSON(fiber.Map{"error": err})
	}

	return c.Status(500).JSON(fiber.Map{"error": "Error desconocido"})
}

