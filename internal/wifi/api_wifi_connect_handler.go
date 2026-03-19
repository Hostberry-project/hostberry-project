package wifi

import (
	"fmt"
	"regexp"

	"github.com/gofiber/fiber/v2"
	"hostberry/internal/constants"
	"hostberry/internal/database"
	middleware "hostberry/internal/middleware"
	"hostberry/internal/validators"
)

// WifiConnectHandler conecta a una red WiFi (útil para el setup wizard).
func WifiConnectHandler(c *fiber.Ctx) error {
	var req struct {
		SSID     string `json:"ssid"`
		Password string `json:"password"`
		Country  string `json:"country"`
		Interface string `json:"interface"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Datos inválidos",
		})
	}

	if err := validators.ValidateSSID(req.SSID); err != nil {
		return err
	}

	if len(req.Password) > 128 {
		return c.Status(400).JSON(fiber.Map{
			"error": "La contraseña no puede tener más de 128 caracteres",
		})
	}

	// Para el setup wizard puede que no haya sesión/token.
	// En ese caso permitimos conectar igualmente y usamos un usuario "setup_wizard" solo para logs.
	username := "setup_wizard"
	var userID *int
	if u, ok := middleware.GetUser(c); ok && u != nil {
		username = u.Username
		id := u.ID
		userID = &id
	}

	country := req.Country
	if country == "" {
		country = c.Query("country", constants.DefaultCountryCode)
	}
	if country == "" {
		country = constants.DefaultCountryCode
	}

	interfaceName := req.Interface
	if interfaceName == "" {
		interfaceName = constants.DefaultWiFiInterface
	}

	if len(interfaceName) > 16 || !regexp.MustCompile(`^[a-zA-Z0-9_-]+$`).MatchString(interfaceName) {
		return c.Status(400).JSON(fiber.Map{
			"error": "Nombre de interfaz inválido",
		})
	}

	result := ConnectWiFi(req.SSID, req.Password, interfaceName, country, username)

	if _, hasSuccess := result["success"]; !hasSuccess {
		result["success"] = false
	}
	if _, hasError := result["error"]; !hasError {
		if result["success"] == false {
			result["error"] = "Error desconocido al conectar a la red WiFi"
		} else {
			result["error"] = ""
		}
	}

	if success, ok := result["success"].(bool); ok && success {
		if userID != nil {
			database.InsertLog("INFO", database.LogMsg("Conexión WiFi a "+req.SSID+" correcta", username), "wifi", userID)
		}
		return c.JSON(result)
	}

	errorMsg := "Error desconocido"
	if errorMsgVal, ok := result["error"].(string); ok && errorMsgVal != "" {
		errorMsg = errorMsgVal
	}
	if userID != nil {
		database.InsertLog("ERROR", database.LogMsgErr("conectar WiFi a "+req.SSID, errorMsg, username), "wifi", userID)
	}

	return c.Status(500).JSON(fiber.Map{
		"success": false,
		"error":   errorMsg,
		"message": fmt.Sprintf("Error conectando a %s", req.SSID),
	})
}

