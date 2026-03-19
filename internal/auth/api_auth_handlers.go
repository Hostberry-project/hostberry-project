package auth

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"hostberry/internal/config"
	"hostberry/internal/database"
	"hostberry/internal/i18n"
	"hostberry/internal/models"
	"hostberry/internal/validators"
)

func translateLoginError(c *fiber.Ctx, err error) string {
	var le *models.LoginError
	if errors.As(err, &le) {
		msg := i18n.T(c, le.Key, le.Default)
		if len(le.Args) > 0 {
			msg = strings.ReplaceAll(msg, "{minutes}", fmt.Sprint(le.Args[0]))
			msg = strings.ReplaceAll(msg, "{duration}", fmt.Sprint(le.Args[0]))
		}
		return msg
	}
	return err.Error()
}

func getUserFromLocals(c *fiber.Ctx) (*models.User, bool) {
	u := c.Locals("user")
	if u == nil {
		return nil, false
	}
	user, ok := u.(*models.User)
	return user, ok && user != nil
}

func LoginAPIHandler(c *fiber.Ctx) error {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": i18n.T(c, "errors.invalid_data", "Invalid data"),
		})
	}

	if err := validators.ValidateUsername(req.Username); err != nil {
		return err
	}

	if req.Password == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": i18n.T(c, "auth.password_required", "Password is required"),
		})
	}

	user, token, err := Login(req.Username, req.Password)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{
			"error": translateLoginError(c, err),
		})
	}

	userID := user.ID
	database.InsertLog("INFO", database.LogMsg("Inicio de sesión correcto", user.Username), "auth", &userID)

	// Primer login o credenciales por defecto (admin/admin): forzar cambio de contraseña en first-login
	passwordChangeRequired := user.LoginCount == 1 || (user.Username == "admin" && CheckPassword("admin", user.Password))

	cookieExpiry := time.Duration(config.AppConfig.Security.TokenExpiry) * time.Minute
	secure := false
	// Si la petición ya viene por HTTPS (cabeceras estándar reverse proxy),
	// marcar la cookie como Secure para evitar envío por HTTP plano.
	if c.Secure() || strings.EqualFold(c.Get("X-Forwarded-Proto"), "https") {
		secure = true
	}
	c.Cookie(&fiber.Cookie{
		Name:     "access_token",
		Value:    token,
		Path:     "/",
		HTTPOnly: true,
		SameSite: "Strict",
		MaxAge:   int(cookieExpiry.Seconds()), // Expira al mismo tiempo que el token
		Secure:   secure,
	})

	return c.JSON(fiber.Map{
		"access_token":            token,
		"password_change_required": passwordChangeRequired,
		"user": fiber.Map{
			"id":       user.ID,
			"username": user.Username,
			"email":    user.Email,
		},
	})
}

func LogoutAPIHandler(c *fiber.Ctx) error {
	user, ok := getUserFromLocals(c)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": i18n.T(c, "auth.unauthorized", "Unauthorized")})
	}
	userID := user.ID
	database.InsertLog("INFO", database.LogMsg("Cierre de sesión", user.Username), "auth", &userID)

	secure := false
	if c.Secure() || strings.EqualFold(c.Get("X-Forwarded-Proto"), "https") {
		secure = true
	}
	c.Cookie(&fiber.Cookie{
		Name:     "access_token",
		Value:    "",
		Path:     "/",
		HTTPOnly: true,
		SameSite: "Strict",
		Secure:   secure,
		MaxAge:   -1,
	})

	return c.JSON(fiber.Map{
		"message": i18n.T(c, "auth.logout_success", "Logout successful"),
	})
}

func MeHandler(c *fiber.Ctx) error {
	user, ok := getUserFromLocals(c)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": i18n.T(c, "auth.unauthorized", "Unauthorized")})
	}
	return c.JSON(fiber.Map{
		"id":       user.ID,
		"username": user.Username,
		"email":    user.Email,
		"first_name": user.FirstName,
		"last_name":  user.LastName,
		"role":       user.Role,
		"timezone":   user.Timezone,
	})
}

func ChangePasswordAPIHandler(c *fiber.Ctx) error {
	user, ok := getUserFromLocals(c)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "No autorizado"})
	}

	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": i18n.T(c, "errors.invalid_data", "Invalid data")})
	}
	if req.CurrentPassword == "" || req.NewPassword == "" {
		return c.Status(400).JSON(fiber.Map{"error": i18n.T(c, "auth.passwords_required", "Passwords required")})
	}
	if !CheckPassword(req.CurrentPassword, user.Password) {
		return c.Status(401).JSON(fiber.Map{"error": i18n.T(c, "auth.incorrect_current_password", "Current password is incorrect")})
	}

	hashed, err := HashPassword(req.NewPassword)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": i18n.T(c, "errors.server_error", "Internal server error")})
	}
	user.Password = hashed
	if err := database.DB.Save(user).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": i18n.T(c, "errors.server_error", "Internal server error")})
	}

	userID := user.ID
	database.InsertLog("INFO", database.LogMsg("Contraseña cambiada", user.Username), "auth", &userID)
	return c.JSON(fiber.Map{"message": i18n.T(c, "auth.password_changed", "Password changed successfully")})
}

func FirstLoginChangeAPIHandler(c *fiber.Ctx) error {
	tokenString := c.Get("Authorization")
	if tokenString != "" {
		tokenString = strings.TrimPrefix(tokenString, "Bearer ")
	} else {
		tokenString = c.Cookies("access_token")
	}

	if tokenString == "" {
		return c.Status(401).JSON(fiber.Map{
			"error": i18n.T(c, "auth.token_required", "Token required"),
		})
	}

	claims, err := ValidateToken(tokenString)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{
			"error": i18n.T(c, "auth.invalid_token", "Invalid token"),
		})
	}

	var user models.User
	if err := database.DB.Where("id = ? AND is_active = ?", claims.UserID, true).First(&user).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{
			"error": i18n.T(c, "auth.user_not_found", "User not found"),
		})
	}

	if user.LoginCount != 1 {
		return c.Status(403).JSON(fiber.Map{
			"error": i18n.T(c, "auth.first_login_only", "This endpoint is only available on first login"),
		})
	}

	var req struct {
		NewUsername string `json:"new_username"`
		NewPassword string `json:"new_password"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Datos inválidos",
		})
	}

	if req.NewUsername != "" {
		if err := validators.ValidateUsername(req.NewUsername); err != nil {
			return err
		}
		if req.NewUsername != user.Username {
			var existingUser models.User
			if err := database.DB.Where("username = ?", req.NewUsername).First(&existingUser).Error; err == nil {
				return c.Status(400).JSON(fiber.Map{
					"error": "El nombre de usuario ya está en uso",
				})
			}
			user.Username = req.NewUsername
		}
	}

	if req.NewPassword == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "La nueva contraseña es requerida",
		})
	}
	if err := validators.ValidatePassword(req.NewPassword); err != nil {
		return err
	}

	hashed, err := HashPassword(req.NewPassword)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Error hasheando contraseña",
		})
	}
	user.Password = hashed
	user.LoginCount++

	if err := database.DB.Save(&user).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Error guardando credenciales",
		})
	}

	userID := user.ID
	database.InsertLog("INFO", database.LogMsg("Credenciales actualizadas en primer acceso", user.Username), "auth", &userID)

	// Generar nuevo token con las credenciales actualizadas y dejar al usuario logueado
	newToken, err := GenerateToken(&user)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Error generando sesión",
		})
	}

	cookieExpiry := time.Duration(config.AppConfig.Security.TokenExpiry) * time.Minute
	secure := false
	if c.Secure() || strings.EqualFold(c.Get("X-Forwarded-Proto"), "https") {
		secure = true
	}
	c.Cookie(&fiber.Cookie{
		Name:     "access_token",
		Value:    newToken,
		Path:     "/",
		HTTPOnly: true,
		SameSite: "Strict",
		MaxAge:   int(cookieExpiry.Seconds()),
		Secure:   secure,
	})

	return c.JSON(fiber.Map{
		"message":      i18n.T(c, "auth.credentials_updated_redirect", "Credenciales actualizadas. Redirigiendo al dashboard."),
		"access_token": newToken,
		"user": fiber.Map{
			"id":       user.ID,
			"username": user.Username,
			"email":    user.Email,
		},
	})
}

func UpdateProfileAPIHandler(c *fiber.Ctx) error {
	user, ok := getUserFromLocals(c)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "No autorizado"})
	}

	var req struct {
		Email     string `json:"email"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Timezone  string `json:"timezone"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Datos inválidos"})
	}

	user.Email = req.Email
	user.FirstName = req.FirstName
	user.LastName = req.LastName
	if req.Timezone != "" {
		user.Timezone = req.Timezone
	}

	if err := database.DB.Save(user).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Error guardando perfil"})
	}

	userID := user.ID
	database.InsertLog("INFO", database.LogMsg("Perfil actualizado", user.Username), "auth", &userID)
	return c.JSON(fiber.Map{"message": "Perfil actualizado"})
}

func UpdatePreferencesAPIHandler(c *fiber.Ctx) error {
	user, ok := getUserFromLocals(c)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "No autorizado"})
	}

	var req struct {
		EmailNotifications bool `json:"email_notifications"`
		SystemAlerts       bool `json:"system_alerts"`
		SecurityAlerts     bool `json:"security_alerts"`
		ShowActivity       bool `json:"show_activity"`
		DataCollection     bool `json:"data_collection"`
		Analytics          bool `json:"analytics"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Datos inválidos"})
	}

	user.EmailNotifications = req.EmailNotifications
	user.SystemAlerts = req.SystemAlerts
	user.SecurityAlerts = req.SecurityAlerts
	user.ShowActivity = req.ShowActivity
	user.DataCollection = req.DataCollection
	user.Analytics = req.Analytics

	if err := database.DB.Save(user).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Error guardando preferencias"})
	}

	userID := user.ID
	database.InsertLog("INFO", database.LogMsg("Preferencias actualizadas", user.Username), "auth", &userID)
	return c.JSON(fiber.Map{"message": "Preferencias actualizadas"})
}

