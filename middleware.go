package main

import (
	"fmt"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v2"
	"hostberry/internal/config"
	"hostberry/internal/models"
)

// Contadores simples de peticiones HTTP por clase de estado.
var (
	httpRequests2xx uint64
	httpRequests4xx uint64
	httpRequests5xx uint64
)

func requireAuth(c *fiber.Ctx) error {
	if c.Method() == fiber.MethodOptions {
		return c.Next()
	}

	path := c.Path()

	if strings.HasPrefix(path, "/static/") {
		return c.Next()
	}
	if path == "/login" || path == "/first-login" || path == "/" {
		return c.Next()
	}
	
	publicPaths := map[string]bool{
		"/api/v1/auth/login":     true,
		"/api/v1/auth/login/":    true, // Con slash final
		"/api/v1/translations":   true,
		"/api/v1/translations/":  true,
		// Permitir que el setup wizard pueda consultar estado/escaneo WiFi incluso
		// cuando el navegador pierde conexión tras cambiar de red.
		"/api/v1/wifi/status":   true,
		"/api/v1/wifi/status/": true,
		"/api/v1/wifi/scan":     true,
		"/api/v1/wifi/scan/":    true,
		// Durante el setup el usuario puede no tener sesión/token aún.
		// Necesitamos permitir ejecutar el "connect" del wizard.
		"/api/v1/wifi/connect":   true,
		"/api/v1/wifi/connect/": true,
		"/api/v1/wifi/disconnect":   true,
		"/api/v1/wifi/disconnect/": true,
		"/health":                 true,
		"/health/":                true,
		"/health/ready":           true,
		"/health/ready/":          true,
		"/health/live":            true,
		"/health/live/":           true,
	}

	normalizedPath := strings.TrimRight(path, "/")
	if normalizedPath == "" {
		normalizedPath = "/"
	}

	if publicPaths[path] || publicPaths[normalizedPath] {
		return c.Next()
	}

	if strings.HasPrefix(path, "/api/v1/translations/") {
		return c.Next()
	}

	var token string

	if strings.HasPrefix(path, "/api/") {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			token = c.Cookies("access_token")
			if token == "" {
				return c.Status(401).JSON(fiber.Map{
					"error": "No autorizado - token requerido",
				})
			}
		} else {
			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				return c.Status(401).JSON(fiber.Map{
					"error": "Formato de token inválido",
				})
			}
			token = parts[1]
		}

	} else {
		token = c.Cookies("access_token")
		if token == "" {
			return c.Redirect("/login")
		}
	}

	claims, err := ValidateToken(token)
	if err != nil {
		if strings.HasPrefix(path, "/api/") {
			return c.Status(401).JSON(fiber.Map{
				"error": "Token inválido o expirado",
			})
		}
		return c.Redirect("/login")
	}

	var user models.User
	if err := db.First(&user, claims.UserID).Error; err != nil {
		LogTf("logs.middleware_user_not_found", claims.UserID, err)
		if strings.HasPrefix(path, "/api/") {
			return c.Status(401).JSON(fiber.Map{
				"error": "Usuario no encontrado. Por favor, inicia sesión nuevamente.",
				"code":   "USER_NOT_FOUND",
			})
		}
		return c.Redirect("/login")
	}

	if !user.IsActive {
		if strings.HasPrefix(path, "/api/") {
			return c.Status(401).JSON(fiber.Map{
				"error": "Usuario inactivo",
			})
		}
		return c.Redirect("/login")
	}

	c.Locals("user", &user)
	c.Locals("user_id", user.ID)

	return c.Next()
}

// GetUser obtiene el usuario autenticado de forma segura (evita panic por type assertion).
// Solo debe usarse en rutas protegidas por requireAuth. Si ok es false, el handler debe responder 401.
func GetUser(c *fiber.Ctx) (*models.User, bool) {
	u := c.Locals("user")
	if u == nil {
		return nil, false
	}
	user, ok := u.(*models.User)
	return user, ok && user != nil
}

// requireAdmin asegura que el usuario autenticado tenga rol "admin".
// Si no es admin, devuelve 403 (para APIs) o redirige a /dashboard para páginas.
func requireAdmin(c *fiber.Ctx) error {
	user, ok := GetUser(c)
	if !ok || user == nil {
		if strings.HasPrefix(c.Path(), "/api/") {
			return c.Status(401).JSON(fiber.Map{"error": "No autorizado"})
		}
		return c.Redirect("/login")
	}
	role := strings.ToLower(strings.TrimSpace(user.Role))
	if role != "admin" {
		// Registrar intento de acceso sin permisos a rutas solo-admin
		userID := user.ID
		InsertLog("WARN", LogMsgWarn("acceso denegado a función de administrador (ruta "+c.Method()+" "+c.Path()+", rol actual: "+user.Role+")", user.Username), "auth", &userID)
		if strings.HasPrefix(c.Path(), "/api/") {
			return c.Status(403).JSON(fiber.Map{"error": "Permisos insuficientes (se requiere rol admin)"})
		}
		return c.Redirect("/dashboard")
	}
	return c.Next()
}

// RunActionWithUser exige usuario autenticado, ejecuta action(user) y devuelve JSON según result["success"]/result["error"].
// successAction y errorActionPrefix se usan con LogMsg/LogMsgErr para mensajes unificados y legibles.
func RunActionWithUser(c *fiber.Ctx, source, successAction, errorActionPrefix string, action func(*User) map[string]interface{}) error {
	user, ok := GetUser(c)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "No autorizado"})
	}
	userID := user.ID
	result := action(user)
	if success, ok := result["success"].(bool); ok && success {
		InsertLog("INFO", LogMsg(successAction, user.Username), source, &userID)
		return c.JSON(result)
	}
	if errorMsg, ok := result["error"].(string); ok {
		InsertLog("ERROR", LogMsgErr(errorActionPrefix, errorMsg, user.Username), source, &userID)
		return c.Status(500).JSON(fiber.Map{"error": errorMsg})
	}
	return c.Status(500).JSON(fiber.Map{"error": "Error desconocido"})
}


func loggingMiddleware(c *fiber.Ctx) error {
	start := time.Now()

	err := c.Next()

	duration := time.Since(start)

	path := c.Path()
	if strings.HasPrefix(path, "/static/") {
		return err
	}

	method := c.Method()
	ip := c.IP()
	status := c.Response().StatusCode()

	// Actualizar contadores de métricas HTTP por clase de código.
	if status >= 200 && status < 300 {
		atomic.AddUint64(&httpRequests2xx, 1)
	} else if status >= 400 && status < 500 {
		atomic.AddUint64(&httpRequests4xx, 1)
	} else if status >= 500 {
		atomic.AddUint64(&httpRequests5xx, 1)
	}

	userID := c.Locals("user_id")
	var userIDPtr *int
	if userID != nil {
		if id, ok := userID.(int); ok {
			userIDPtr = &id
		}
	}

	statusEmoji := "✅"
	if status >= 400 && status < 500 {
		statusEmoji = "⚠️"
	} else if status >= 500 {
		statusEmoji = "❌"
	}

	durationStr := duration.String()
	if duration < time.Millisecond {
		durationStr = fmt.Sprintf("%.0fµs", float64(duration.Nanoseconds())/1000)
	} else if duration < time.Second {
		durationStr = fmt.Sprintf("%.2fms", float64(duration.Nanoseconds())/1000000)
	} else {
		durationStr = fmt.Sprintf("%.2fs", duration.Seconds())
	}

	if appConfig.Server.Debug || status >= 400 {
		go func() {
			// Mensaje de log más legible:
			// Ejemplo: "✅ GET /wifi/status desde 192.168.1.10 en 12.3ms (HTTP 200)"
			msg := fmt.Sprintf("%s %s %s desde %s en %s (HTTP %d)", statusEmoji, method, path, ip, durationStr, status)
			InsertLog(
				"INFO",
				msg,
				"http",
				userIDPtr,
			)
		}()
	}

	return err
}

func errorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	message := "Error interno del servidor"

	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
		message = e.Message
	}

	method := c.Method()
	path := c.Path()
	errMsg := err.Error()

	userID := c.Locals("user_id")
	var userIDPtr *int
	if userID != nil {
		if id, ok := userID.(int); ok {
			userIDPtr = &id
		}
	}

	if code >= 500 {
		if appConfig.Server.Debug {
			LogTf("logs.middleware_error", method, path, err)
		}
		go func() {
			InsertLog("ERROR", LogMsgErr("procesar petición "+path, errMsg, ""), "http", userIDPtr)
		}()
	}

	if strings.HasPrefix(c.Path(), "/api/") {
		resp := fiber.Map{
			"error":   message,
			"path":    c.Path(),
			"method":  c.Method(),
		}
		if appConfig.Server.Debug {
			resp["details"] = err.Error()
		}
		return c.Status(code).JSON(resp)
	}

	renderDetails := ""
	if appConfig.Server.Debug {
		renderDetails = err.Error()
	}
	if renderErr := renderTemplate(c, "error", fiber.Map{
		"Title":   "Error",
		"Code":    code,
		"Message": message,
		"Details": renderDetails,
	}); renderErr != nil {
		LogTf("logs.middleware_render_error", renderErr)
		return c.Status(code).SendString(fmt.Sprintf(
			"<html><body><h1>Error %d</h1><p>%s</p></body></html>",
			code, message,
		))
	}
	return nil
}

// securityHeadersMiddleware añade cabeceras de seguridad básicas.
// Las cabeceras se fijan antes de c.Next() para que se envíen con la respuesta.
func securityHeadersMiddleware(c *fiber.Ctx) error {
	if c.Get("X-Content-Type-Options") == "" {
		c.Set("X-Content-Type-Options", "nosniff")
	}
	if c.Get("X-Frame-Options") == "" {
		c.Set("X-Frame-Options", "SAMEORIGIN")
	}
	if c.Get("Referrer-Policy") == "" {
		c.Set("Referrer-Policy", "strict-origin-when-cross-origin")
	}
	isHTTPS := c.Secure() || strings.EqualFold(c.Get("X-Forwarded-Proto"), "https")
	if isHTTPS && c.Get("Strict-Transport-Security") == "" {
		c.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
	}
	if c.Get("Content-Security-Policy") == "" {
		c.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; script-src 'self'; font-src 'self' data:; connect-src 'self'")
	}
	return c.Next()
}

// enforceHTTPSMiddleware redirige HTTP -> HTTPS cuando:
// - Security.EnforceHTTPS es true
// - Se detecta que hay TLS directo o un proxy que marca X-Forwarded-Proto=https.
func enforceHTTPSMiddleware(c *fiber.Ctx) error {
	if !appConfig.Security.EnforceHTTPS {
		return c.Next()
	}

	// Ya es HTTPS: no hacer nada.
	if c.Secure() || strings.EqualFold(c.Get("X-Forwarded-Proto"), "https") {
		return c.Next()
	}

	// No intentes forzar HTTPS en salud/métricas para no romper sondas locales.
	path := c.Path()
	if strings.HasPrefix(path, "/health") || path == "/metrics" {
		return c.Next()
	}

	// Construir URL HTTPS manteniendo host, path y query.
	host := c.Hostname()
	if host == "" {
		host = fmt.Sprintf("%s:%d", appConfig.Server.Host, appConfig.Server.Port)
	}
	u := url.URL{
		Scheme:   "https",
		Host:     host,
		Path:     c.Path(),
		RawQuery: c.Context().URI().QueryArgs().String(),
	}
	return c.Redirect(u.String(), fiber.StatusPermanentRedirect)
}
