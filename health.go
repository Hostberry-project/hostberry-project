package main

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"hostberry/internal/constants"
	"hostberry/internal/database"
	"hostberry/internal/i18n"
	"hostberry/internal/metrics"
)

type HealthCheckResponse struct {
	Status    string            `json:"status"`
	Timestamp time.Time         `json:"timestamp"`
	Version   string            `json:"version"`
	Services  map[string]string `json:"services"`
}

func healthCheckHandler(c *fiber.Ctx) error {
	response := HealthCheckResponse{
		Status:    "healthy",
		Timestamp: time.Now(),
		Version:   "2.0.0",
		Services:  make(map[string]string),
	}

	if database.DB != nil {
		sqlDB, err := database.DB.DB()
		if err == nil {
			if err := sqlDB.Ping(); err == nil {
				response.Services["database"] = "healthy"
			} else {
				response.Services["database"] = "unhealthy"
				response.Status = "degraded"
			}
		} else {
			response.Services["database"] = "unhealthy"
			response.Status = "degraded"
		}
	} else {
		response.Services["database"] = "not_configured"
		response.Status = "degraded"
	}


	if i18n.Ready() {
		response.Services["i18n"] = "healthy"
	} else {
		response.Services["i18n"] = "unhealthy"
		response.Status = "degraded"
	}

	statusCode := 200
	if response.Status == "degraded" {
		statusCode = 503
	}

	return c.Status(statusCode).JSON(response)
}

func readinessCheckHandler(c *fiber.Ctx) error {
	if db == nil {
		return c.Status(503).JSON(fiber.Map{
			"status":  "not_ready",
			"message": "Database not initialized",
		})
	}

	sqlDB, err := db.DB()
	if err != nil {
		return c.Status(503).JSON(fiber.Map{
			"status":  "not_ready",
			"message": "Database connection error",
		})
	}

	if err := sqlDB.Ping(); err != nil {
		return c.Status(503).JSON(fiber.Map{
			"status":  "not_ready",
			"message": "Database ping failed",
		})
	}

	return c.JSON(fiber.Map{
		"status":  "ready",
		"message": "Application is ready",
	})
}

func livenessCheckHandler(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"status":  "alive",
		"message": "Application is running",
	})
}

// metricsHandler expone métricas sencillas en texto plano (formato tipo Prometheus).
// Está pensado para ser consumido por Prometheus o por scripts de monitoreo.
func metricsHandler(c *fiber.Ctx) error {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// NOTA: no exponemos información sensible (ni usuarios, ni tokens, etc.).
	now := time.Now().Unix()

	// Lectura atómica de contadores HTTP
	req2xx := metrics.Load2xx()
	req4xx := metrics.Load4xx()
	req5xx := metrics.Load5xx()

	// Estado de servicios del sistema (hostapd/dnsmasq) y WiFi
	hostapdUp := serviceIsActive("hostapd")
	dnsmasqUp := serviceIsActive("dnsmasq")
	wifiIfaceUp := wifiInterfaceUp()

	body := "" +
		"# HELP hostberry_up 1 si la aplicación está respondiendo.\n" +
		"# TYPE hostberry_up gauge\n" +
		"hostberry_up 1\n\n" +
		"# HELP hostberry_build_info Información básica de la build (versión estática y runtime).\n" +
		"# TYPE hostberry_build_info gauge\n" +
		"hostberry_build_info{version=\"2.0.0\",go_version=\"" + runtime.Version() + "\"} 1\n\n" +
		"# HELP hostberry_mem_bytes Uso de memoria total reportado por Go (bytes).\n" +
		"# TYPE hostberry_mem_bytes gauge\n" +
		"hostberry_mem_bytes " + fmt.Sprintf("%d", m.Alloc) + "\n\n" +
		"# HELP hostberry_goroutines Número de goroutines actuales.\n" +
		"# TYPE hostberry_goroutines gauge\n" +
		"hostberry_goroutines " + fmt.Sprintf("%d", runtime.NumGoroutine()) + "\n\n" +
		"# HELP hostberry_unix_time_seconds Marca de tiempo UNIX del último scrape.\n" +
		"# TYPE hostberry_unix_time_seconds gauge\n" +
		"hostberry_unix_time_seconds " + fmt.Sprintf("%d", now) + "\n\n" +
		"# HELP hostberry_http_requests_total Número total de peticiones HTTP por clase de código.\n" +
		"# TYPE hostberry_http_requests_total counter\n" +
		"hostberry_http_requests_total{code_class=\"2xx\"} " + fmt.Sprintf("%d", req2xx) + "\n" +
		"hostberry_http_requests_total{code_class=\"4xx\"} " + fmt.Sprintf("%d", req4xx) + "\n" +
		"hostberry_http_requests_total{code_class=\"5xx\"} " + fmt.Sprintf("%d", req5xx) + "\n\n" +
		"# HELP hostberry_service_up Estado de servicios del sistema (1=activo, 0=no activo).\n" +
		"# TYPE hostberry_service_up gauge\n" +
		"hostberry_service_up{service=\"hostapd\"} " + fmt.Sprintf("%d", hostapdUp) + "\n" +
		"hostberry_service_up{service=\"dnsmasq\"} " + fmt.Sprintf("%d", dnsmasqUp) + "\n\n" +
		"# HELP hostberry_wifi_interface_up Indica si la interfaz WiFi principal está activa (1=UP,0=no).\n" +
		"# TYPE hostberry_wifi_interface_up gauge\n" +
		"hostberry_wifi_interface_up{interface=\"" + constants.DefaultWiFiInterface + "\"} " + fmt.Sprintf("%d", wifiIfaceUp) + "\n"

	c.Set(fiber.HeaderContentType, "text/plain; charset=utf-8")
	return c.SendString(body)
}

// serviceIsActive devuelve 1 si systemd reporta el servicio como "active", 0 en caso contrario.
func serviceIsActive(name string) int {
	cmd := exec.Command("systemctl", "is-active", name)
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	state := strings.TrimSpace(string(out))
	if state == "active" {
		return 1
	}
	return 0
}

// wifiInterfaceUp devuelve 1 si la interfaz WiFi principal está UP, 0 en caso contrario.
func wifiInterfaceUp() int {
	if constants.DefaultWiFiInterface == "" {
		return 0
	}
	cmd := exec.Command("sh", "-c", fmt.Sprintf("ip link show %s 2>/dev/null | grep -q 'state UP'", constants.DefaultWiFiInterface))
	if err := cmd.Run(); err == nil {
		return 1
	}
	return 0
}

// metricsSummaryHandler devuelve un resumen JSON de las métricas para uso interno del panel.
// Es más simple de consumir desde JS que el texto plano de /metrics.
func metricsSummaryHandler(c *fiber.Ctx) error {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	req2xx := metrics.Load2xx()
	req4xx := metrics.Load4xx()
	req5xx := metrics.Load5xx()

	hostapdUp := serviceIsActive("hostapd")
	dnsmasqUp := serviceIsActive("dnsmasq")
	wifiIfaceUp := wifiInterfaceUp()

	return c.JSON(fiber.Map{
		"up":           true,
		"version":      "2.0.0",
		"go_version":   runtime.Version(),
		"unix_time":    time.Now().Unix(),
		"mem_bytes":    m.Alloc,
		"goroutines":   runtime.NumGoroutine(),
		"http_2xx":     req2xx,
		"http_4xx":     req4xx,
		"http_5xx":     req5xx,
		"hostapd_up":   hostapdUp == 1,
		"dnsmasq_up":   dnsmasqUp == 1,
		"wifi_iface":   constants.DefaultWiFiInterface,
		"wifi_up":      wifiIfaceUp == 1,
	})
}
