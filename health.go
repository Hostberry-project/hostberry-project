package main

import (
	"fmt"
	"runtime"
	"time"

	"github.com/gofiber/fiber/v2"
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

	if db != nil {
		sqlDB, err := db.DB()
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


	if i18nManager != nil {
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
		"hostberry_unix_time_seconds " + fmt.Sprintf("%d", now) + "\n"

	c.Set(fiber.HeaderContentType, "text/plain; charset=utf-8")
	return c.SendString(body)
}
