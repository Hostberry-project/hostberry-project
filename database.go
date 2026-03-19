package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"hostberry/internal/config"
	"hostberry/internal/models"

	"gorm.io/driver/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var db *gorm.DB

func initDatabase() error {
	var err error
	var dialector gorm.Dialector
	cfg := config.AppConfig

	switch cfg.Database.Type {
	case "sqlite":
		dbDir := filepath.Dir(cfg.Database.Path)
		if err := os.MkdirAll(dbDir, 0755); err != nil {
			return fmt.Errorf("error creando directorio de BD: %v", err)
		}

		dialector = sqlite.Open(appConfig.Database.Path)
	case "postgres":
		dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=disable",
			appConfig.Database.Host,
			appConfig.Database.User,
			appConfig.Database.Password,
			appConfig.Database.Database,
			appConfig.Database.Port,
		)
		dialector = postgres.Open(dsn)
	case "mysql":
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			appConfig.Database.User,
			appConfig.Database.Password,
			appConfig.Database.Host,
			appConfig.Database.Port,
			appConfig.Database.Database,
		)
		dialector = mysql.Open(dsn)
	default:
		return fmt.Errorf("tipo de base de datos no soportado: %s", appConfig.Database.Type)
	}

	gormLogger := logger.Default
	if !appConfig.Server.Debug {
		gormLogger = logger.Default.LogMode(logger.Silent)
	}

	db, err = gorm.Open(dialector, &gorm.Config{
		Logger: gormLogger,
	})
	if err != nil {
		return fmt.Errorf("error conectando a la base de datos: %v", err)
	}

	if err := autoMigrate(); err != nil {
		return fmt.Errorf("error en auto-migración: %v", err)
	}

	LogTln("logs.db_initialized")
	LogTf("logs.db_location", appConfig.Database.Path)
	return nil
}

func autoMigrate() error {
	return db.AutoMigrate(
		&User{},
		&SystemLog{},
		&SystemStatistic{},
		&NetworkConfig{},
		&VPNConfig{},
		&WireGuardConfig{},
		&AdBlockConfig{},
		&SystemConfig{},
	)
}

type SystemLog struct {
	ID        uint      `gorm:"primaryKey"`
	Level     string    `gorm:"not null;index"`
	Message   string    `gorm:"type:text"`
	Source    string
	UserID    *int
	CreatedAt time.Time `gorm:"index"`
}

type SystemConfig struct {
	Key   string `gorm:"primaryKey"`
	Value string `gorm:"type:text"`
}

type SystemStatistic struct {
	ID        uint      `gorm:"primaryKey"`
	Type      string    `gorm:"not null;index"` // cpu_usage, memory_usage, disk_usage
	Value     float64   `gorm:"not null"`
	Timestamp time.Time `gorm:"index"`
}

type NetworkConfig struct {
	ID            uint   `gorm:"primaryKey"`
	Interface     string `gorm:"not null"`
	DHCPEnabled   bool   `gorm:"default:false"`
	DHCPRangeStart string
	DHCPRangeEnd   string
	Gateway        string
	DNSPrimary     string
	DNSSecondary   string
	UpdatedAt      time.Time
}

type VPNConfig struct {
	ID        uint   `gorm:"primaryKey"`
	Type      string `gorm:"not null"` // openvpn, wireguard
	Config    string `gorm:"type:text"`
	IsActive  bool   `gorm:"default:false"`
	UpdatedAt time.Time
}

type WireGuardConfig struct {
	ID          uint   `gorm:"primaryKey"`
	Interface   string `gorm:"not null"`
	PrivateKey  string
	PublicKey   string
	Address     string
	DNS         string
	IsActive    bool   `gorm:"default:false"`
	UpdatedAt   time.Time
}

type AdBlockConfig struct {
	ID        uint   `gorm:"primaryKey"`
	Enabled   bool   `gorm:"default:false"`
	Lists     string `gorm:"type:text"` // JSON array de URLs
	UpdatedAt time.Time
}

func InsertLog(level, message, source string, userID *int) error {
	log := SystemLog{
		Level:   level,
		Message: message,
		Source:  source,
		UserID:  userID,
	}
	return db.Create(&log).Error
}

const maxLogMessageLen = 1024 // evita mensajes enormes por datos de usuario o dumps de error

func truncateForLog(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 {
		max = maxLogMessageLen
	}
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

// LogMsg unifica el formato de mensajes: "Descripción clara. Usuario: nombre."
// Si user está vacío (ej. wizard), devuelve solo "Descripción clara."
// Recorta cadenas largas para que los logs sigan siendo legibles.
func LogMsg(action, user string) string {
	action = truncateForLog(strings.TrimRight(action, "."), 800)
	user = truncateForLog(user, 120)
	if user == "" {
		return action + "."
	}
	return action + ". Usuario: " + user + "."
}

// LogMsgErr formato para errores: "Error al [acción]: motivo. Usuario: nombre."
func LogMsgErr(action, reason, user string) string {
	action = truncateForLog(strings.TrimRight(action, "."), 200)
	reason = truncateForLog(reason, 600)
	user = truncateForLog(user, 120)
	if user == "" {
		return "Error al " + action + ": " + reason + "."
	}
	return "Error al " + action + ": " + reason + ". Usuario: " + user + "."
}

// LogMsgWarn formato para advertencias: "Advertencia: descripción. Usuario: nombre."
func LogMsgWarn(desc, user string) string {
	desc = truncateForLog(desc, 800)
	user = truncateForLog(user, 120)
	if user == "" {
		return "Advertencia: " + desc + "."
	}
	return "Advertencia: " + desc + ". Usuario: " + user + "."
}

func GetLogs(level string, limit, offset int) ([]SystemLog, int64, error) {
	var logs []SystemLog
	var total int64

	query := db.Model(&SystemLog{})
	if level != "" && level != "all" {
		query = query.Where("level = ?", level)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}

func InsertStatistic(statType string, value float64) error {
	stat := SystemStatistic{
		Type:      statType,
		Value:     value,
		Timestamp: time.Now(),
	}
	return db.Create(&stat).Error
}

func SetConfig(key, value string) error {
	config := SystemConfig{Key: key, Value: value}
	return db.Save(&config).Error
}

func GetConfig(key string) (string, error) {
	var config SystemConfig
	if err := db.First(&config, "key = ?", key).Error; err != nil {
		return "", err
	}
	return config.Value, nil
}

func GetAllConfigs() (map[string]string, error) {
	if db == nil {
		return make(map[string]string), nil
	}
	
	var configs []SystemConfig
	if err := db.Find(&configs).Error; err != nil {
		return nil, err
	}
	
	result := make(map[string]string)
	for _, config := range configs {
		result[config.Key] = config.Value
	}
	return result, nil
}
