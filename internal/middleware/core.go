package middleware

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"hostberry/internal/auth"
	"hostberry/internal/config"
	"hostberry/internal/database"
	"hostberry/internal/i18n"
	"hostberry/internal/metrics"
	webtemplates "hostberry/internal/templates"
	"hostberry/internal/models"
)

func isUnsafeMethod(method string) bool {
	switch method {
	case fiber.MethodPost, fiber.MethodPut, fiber.MethodPatch, fiber.MethodDelete:
		return true
	default:
		return false
	}
}

// Permite requests mutantes autenticadas por cookie solo desde mismo origen.
// Mitiga CSRF cuando no se usa Authorization: Bearer.
func isSameOriginForCookieAuth(c *fiber.Ctx) bool {
	origin := strings.TrimSpace(c.Get("Origin"))
	referer := strings.TrimSpace(c.Get("Referer"))

	// Si no hay cabeceras de origen/referer no bloqueamos para no romper clientes legacy.
	if origin == "" && referer == "" {
		return true
	}

	host := strings.ToLower(strings.TrimSpace(c.Hostname()))
	if host == "" {
		return false
	}

	if origin != "" {
		if u, err := url.Parse(origin); err != nil || !strings.EqualFold(u.Hostname(), host) {
			return false
		}
		return true
	}

	if referer != "" {
		if u, err := url.Parse(referer); err != nil || !strings.EqualFold(u.Hostname(), host) {
			return false
		}
		return true
	}

	return true
}

// RequireAuth protege rutas: valida token/JWT y carga el usuario en Locals.
func RequireAuth(c *fiber.Ctx) error {
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
		"/api/v1/auth/login/":    true,
		"/api/v1/translations":   true,
		"/api/v1/translations/":  true,
		"/api/v1/wifi/status":   true,
		"/api/v1/wifi/status/": true,
		"/api/v1/wifi/scan":     true,
		"/api/v1/wifi/scan/":    true,
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
			if isUnsafeMethod(c.Method()) && !isSameOriginForCookieAuth(c) {
				return c.Status(403).JSON(fiber.Map{
					"error": "Origen no permitido para autenticación por cookie",
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

	claims, err := auth.ValidateToken(token)
	if err != nil {
		if strings.HasPrefix(path, "/api/") {
			return c.Status(401).JSON(fiber.Map{
				"error": "Token inválido o expirado",
			})
		}
		return c.Redirect("/login")
	}

	var user models.User
	if err := database.DB.First(&user, claims.UserID).Error; err != nil {
		i18n.LogTf("logs.middleware_user_not_found", claims.UserID, err)
		if strings.HasPrefix(path, "/api/") {
			return c.Status(401).JSON(fiber.Map{
				"error": "Usuario no encontrado. Por favor, inicia sesión nuevamente.",
				"code":  "USER_NOT_FOUND",
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
// Solo debe usarse en rutas protegidas por RequireAuth.
func GetUser(c *fiber.Ctx) (*models.User, bool) {
	u := c.Locals("user")
	if u == nil {
		return nil, false
	}
	user, ok := u.(*models.User)
	return user, ok && user != nil
}

// RequireAdmin asegura que el usuario autenticado tenga rol "admin".
func RequireAdmin(c *fiber.Ctx) error {
	user, ok := GetUser(c)
	if !ok || user == nil {
		if strings.HasPrefix(c.Path(), "/api/") {
			return c.Status(401).JSON(fiber.Map{"error": "No autorizado"})
		}
		return c.Redirect("/login")
	}
	role := strings.ToLower(strings.TrimSpace(user.Role))
	if role != "admin" {
		userID := user.ID
		database.InsertLog(
			"WARN",
			database.LogMsgWarn(
				"acceso denegado a función de administrador (ruta "+c.Method()+" "+c.Path()+", rol actual: "+user.Role+")",
				user.Username,
			),
			"auth",
			&userID,
		)
		if strings.HasPrefix(c.Path(), "/api/") {
			return c.Status(403).JSON(fiber.Map{"error": "Permisos insuficientes (se requiere rol admin)"})
		}
		return c.Redirect("/dashboard")
	}
	return c.Next()
}

// RunActionWithUser exige usuario autenticado, ejecuta action(user) y devuelve JSON según result["success"]/result["error"].
func RunActionWithUser(c *fiber.Ctx, source, successAction, errorActionPrefix string, action func(*models.User) map[string]interface{}) error {
	user, ok := GetUser(c)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "No autorizado"})
	}
	userID := user.ID
	result := action(user)

	if success, ok := result["success"].(bool); ok && success {
		database.InsertLog("INFO", database.LogMsg(successAction, user.Username), source, &userID)
		return c.JSON(result)
	}
	if errorMsg, ok := result["error"].(string); ok {
		database.InsertLog("ERROR", database.LogMsgErr(errorActionPrefix, errorMsg, user.Username), source, &userID)
		return c.Status(500).JSON(fiber.Map{"error": errorMsg})
	}
	return c.Status(500).JSON(fiber.Map{"error": "Error desconocido"})
}

func LoggingMiddleware(c *fiber.Ctx) error {
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

	// Contadores de métricas HTTP
	if status >= 200 && status < 300 {
		metrics.Add2xx()
	} else if status >= 400 && status < 500 {
		metrics.Add4xx()
	} else if status >= 500 {
		metrics.Add5xx()
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

	if config.AppConfig.Server.Debug || status >= 400 {
		go func() {
			msg := fmt.Sprintf("%s %s %s desde %s en %s (HTTP %d)", statusEmoji, method, path, ip, durationStr, status)
			database.InsertLog("INFO", msg, "http", userIDPtr)
		}()
	}

	return err
}

func ErrorHandler(c *fiber.Ctx, err error) error {
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
		if config.AppConfig.Server.Debug {
			i18n.LogTf("logs.middleware_error", method, path, err)
		}
		go func() {
			database.InsertLog("ERROR", database.LogMsgErr("procesar petición "+path, errMsg, ""), "http", userIDPtr)
		}()
	}

	if strings.HasPrefix(c.Path(), "/api/") {
		resp := fiber.Map{
			"error":   message,
			"path":    c.Path(),
			"method":  c.Method(),
		}
		if config.AppConfig.Server.Debug {
			resp["details"] = err.Error()
		}
		return c.Status(code).JSON(resp)
	}

	renderDetails := ""
	if config.AppConfig.Server.Debug {
		renderDetails = err.Error()
	}

	if renderErr := webtemplates.RenderTemplate(c, "error", fiber.Map{
		"Title":   "Error",
		"Code":    code,
		"Message": message,
		"Details": renderDetails,
	}); renderErr != nil {
		i18n.LogTf("logs.middleware_render_error", renderErr)
		return c.Status(code).SendString(fmt.Sprintf(
			"<html><body><h1>Error %d</h1><p>%s</p></body></html>",
			code, message,
		))
	}

	return nil
}

// SecurityHeadersMiddleware añade cabeceras de seguridad básicas.
func SecurityHeadersMiddleware(c *fiber.Ctx) error {
	if c.Get("X-Content-Type-Options") == "" {
		c.Set("X-Content-Type-Options", "nosniff")
	}
	if c.Get("X-Frame-Options") == "" {
		c.Set("X-Frame-Options", "SAMEORIGIN")
	}
	if c.Get("X-XSS-Protection") == "" {
		c.Set("X-XSS-Protection", "1; mode=block")
	}
	if c.Get("Referrer-Policy") == "" {
		c.Set("Referrer-Policy", "strict-origin-when-cross-origin")
	}
	if c.Get("Permissions-Policy") == "" {
		c.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=(), usb=(), payment=()")
	}
	if c.Get("Cross-Origin-Opener-Policy") == "" {
		c.Set("Cross-Origin-Opener-Policy", "same-origin")
	}
	if c.Get("Cross-Origin-Resource-Policy") == "" {
		c.Set("Cross-Origin-Resource-Policy", "same-origin")
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

// EnforceHTTPSMiddleware redirige HTTP -> HTTPS cuando:
// - Security.EnforceHTTPS es true
// - Se detecta que hay TLS directo o un proxy que marca X-Forwarded-Proto=https.
func EnforceHTTPSMiddleware(c *fiber.Ctx) error {
	if !config.AppConfig.Security.EnforceHTTPS {
		return c.Next()
	}

	// Ya es HTTPS: no hacer nada.
	if c.Secure() || strings.EqualFold(c.Get("X-Forwarded-Proto"), "https") {
		return c.Next()
	}

	// No intentes forzar HTTPS en health/métricas para no romper sondas locales.
	path := c.Path()
	if strings.HasPrefix(path, "/health") || path == "/metrics" {
		return c.Next()
	}

	host := c.Hostname()
	if host == "" {
		host = fmt.Sprintf("%s:%d", config.AppConfig.Server.Host, config.AppConfig.Server.Port)
	}

	u := url.URL{
		Scheme:   "https",
		Host:     host,
		Path:     c.Path(),
		RawQuery: c.Context().URI().QueryArgs().String(),
	}
	return c.Redirect(u.String(), fiber.StatusPermanentRedirect)
}

