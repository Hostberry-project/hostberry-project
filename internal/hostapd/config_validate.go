package hostapd

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"hostberry/internal/validators"
)

// HostapdConfigBody es el cuerpo JSON de configuración HostAPD (POST).
type HostapdConfigBody struct {
	Interface      string `json:"interface"`
	SSID           string `json:"ssid"`
	Password       string `json:"password"`
	Channel        int    `json:"channel"`
	Security       string `json:"security"`
	Gateway        string `json:"gateway"`
	DHCPRangeStart string `json:"dhcp_range_start"`
	DHCPRangeEnd   string `json:"dhcp_range_end"`
	LeaseTime      string `json:"lease_time"`
	Country        string `json:"country"`
}

func respondValidatorError(c *fiber.Ctx, err error) error {
	var fe *fiber.Error
	if errors.As(err, &fe) {
		return c.Status(fe.Code).JSON(fiber.Map{"error": fe.Message, "success": false})
	}
	return c.Status(400).JSON(fiber.Map{"error": err.Error(), "success": false})
}

// validateHostapdPOST valida campos tras aplicar valores por defecto.
func validateHostapdPOST(req *HostapdConfigBody) error {
	if err := validators.ValidateIfaceName(req.Interface); err != nil {
		return err
	}
	if err := validators.ValidateSSID(req.SSID); err != nil {
		return err
	}
	if req.Channel < 1 || req.Channel > 13 {
		return fiber.NewError(400, "Canal WiFi debe estar entre 1 y 13")
	}
	if err := validators.ValidateIP(req.Gateway); err != nil {
		return err
	}
	if err := validators.ValidateIP(req.DHCPRangeStart); err != nil {
		return err
	}
	if err := validators.ValidateIP(req.DHCPRangeEnd); err != nil {
		return err
	}
	if err := validators.ValidateDhcpLeaseTime(req.LeaseTime); err != nil {
		return err
	}
	if err := validators.ValidateCountryCode(req.Country); err != nil {
		return err
	}
	if req.Security == "wpa2" || req.Security == "wpa3" {
		if err := validators.ValidateWPAPSK(req.Password); err != nil {
			return err
		}
	}
	return nil
}

// validateHostapdPhyName opcional: tras autodetectar phy desde el sistema.
func validateHostapdPhyName(phyName string) error {
	if phyName == "" {
		return fiber.NewError(400, "No se pudo determinar el dispositivo phy WiFi")
	}
	return validators.ValidatePhyName(phyName)
}
