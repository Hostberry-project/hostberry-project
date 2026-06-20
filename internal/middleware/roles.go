package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"hostberry/internal/auth"
	"hostberry/internal/database"
)

// RequireRole exige uno de los roles indicados.
func RequireRole(roles ...string) fiber.Handler {
	allowed := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		if nr, ok := auth.NormalizeRole(r); ok {
			allowed[nr] = struct{}{}
		}
	}
	return func(c *fiber.Ctx) error {
		user, ok := GetUser(c)
		if !ok || user == nil {
			if strings.HasPrefix(c.Path(), "/api/") {
				return c.Status(401).JSON(fiber.Map{"error": "No autorizado"})
			}
			return c.Redirect("/login")
		}
		role, valid := auth.NormalizeRole(user.Role)
		if !valid {
			return c.Status(403).JSON(fiber.Map{"error": "Rol de usuario inválido"})
		}
		if _, ok := allowed[role]; !ok {
			userID := user.ID
			database.InsertLog("WARN", database.LogMsgWarn(
				"acceso denegado por rol (ruta "+c.Method()+" "+c.Path()+", rol: "+user.Role+")",
				user.Username,
			), "auth", &userID)
			if strings.HasPrefix(c.Path(), "/api/") {
				return c.Status(403).JSON(fiber.Map{"error": "Permisos insuficientes"})
			}
			return c.Redirect("/dashboard")
		}
		return c.Next()
	}
}
