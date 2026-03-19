package models

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// LoginError lleva una clave i18n para que el handler traduzca el mensaje según el idioma.
type LoginError struct {
	Key     string
	Default string
	Args    []interface{}
}

func (e *LoginError) Error() string { return e.Default }

// Claims son los claims del JWT.
type Claims struct {
	Username     string `json:"username"`
	UserID       int    `json:"user_id"`
	TokenVersion int    `json:"token_version"`
	jwt.RegisteredClaims
}

// User es el modelo de usuario (GORM y auth).
type User struct {
	ID        int    `gorm:"primaryKey"`
	Username  string `gorm:"unique;not null"`
	Password  string `gorm:"not null"`
	Email     string
	FirstName string
	LastName  string
	Role      string `gorm:"default:admin"`
	Timezone  string `gorm:"default:UTC"`

	LastLogin          *time.Time
	LoginCount         int       `gorm:"default:0"`
	TokenVersion       int       `gorm:"default:1"`
	FailedAttempts     int       `gorm:"default:0"`
	LockedUntil        *time.Time
	EmailNotifications bool `gorm:"default:false"`
	SystemAlerts       bool `gorm:"default:false"`
	SecurityAlerts     bool `gorm:"default:false"`
	ShowActivity       bool `gorm:"default:true"`
	DataCollection     bool `gorm:"default:false"`
	Analytics          bool `gorm:"default:false"`

	IsActive  bool `gorm:"default:true"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// SystemLog entrada de log del sistema.
type SystemLog struct {
	ID        uint      `gorm:"primaryKey"`
	Level     string    `gorm:"not null;index"`
	Message   string    `gorm:"type:text"`
	Source    string
	UserID    *int
	CreatedAt time.Time `gorm:"index"`
}

// SystemConfig clave-valor de configuración.
type SystemConfig struct {
	Key   string `gorm:"primaryKey"`
	Value string `gorm:"type:text"`
}

// SystemStatistic estadística (cpu, memoria, etc.).
type SystemStatistic struct {
	ID        uint      `gorm:"primaryKey"`
	Type      string    `gorm:"not null;index"`
	Value     float64   `gorm:"not null"`
	Timestamp time.Time `gorm:"index"`
}

// NetworkConfig configuración de red.
type NetworkConfig struct {
	ID             uint      `gorm:"primaryKey"`
	Interface      string    `gorm:"not null"`
	DHCPEnabled    bool      `gorm:"default:false"`
	DHCPRangeStart string
	DHCPRangeEnd   string
	Gateway        string
	DNSPrimary     string
	DNSSecondary   string
	UpdatedAt      time.Time
}

// VPNConfig configuración VPN (OpenVPN/WireGuard).
type VPNConfig struct {
	ID        uint      `gorm:"primaryKey"`
	Type      string    `gorm:"not null"`
	Config    string    `gorm:"type:text"`
	IsActive  bool      `gorm:"default:false"`
	UpdatedAt time.Time
}

// WireGuardConfig configuración WireGuard.
type WireGuardConfig struct {
	ID         uint      `gorm:"primaryKey"`
	Interface  string    `gorm:"not null"`
	PrivateKey string
	PublicKey  string
	Address    string
	DNS        string
	IsActive   bool `gorm:"default:false"`
	UpdatedAt  time.Time
}

// AdBlockConfig configuración AdBlock.
type AdBlockConfig struct {
	ID        uint      `gorm:"primaryKey"`
	Enabled   bool      `gorm:"default:false"`
	Lists     string    `gorm:"type:text"`
	UpdatedAt time.Time
}
