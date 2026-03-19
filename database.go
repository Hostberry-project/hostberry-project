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

		dialector = sqlite.Open(cfg.Database.Path)
	case "postgres":
		dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=disable",
			cfg.Database.Host,
			cfg.Database.User,
			cfg.Database.Password,
			cfg.Database.Database,
			cfg.Database.Port,
		)
		dialector = postgres.Open(dsn)
	case "mysql":
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			cfg.Database.User,
			cfg.Database.Password,
			cfg.Database.Host,
			cfg.Database.Port,
			cfg.Database.Database,
		)
		dialector = mysql.Open(dsn)
	default:
		return fmt.Errorf("tipo de base de datos no soportado: %s", cfg.Database.Type)
	}

	gormLogger := logger.Default
	if !cfg.Server.Debug {
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

	i18n.LogTln("logs.db_initialized")
	i18n.LogTf("logs.db_location", cfg.Database.Path)
	return nil
}

func autoMigrate() error {
	return db.AutoMigrate(
		&models.User{},
		&models.SystemLog{},
		&models.SystemStatistic{},
		&models.NetworkConfig{},
		&models.VPNConfig{},
		&models.WireGuardConfig{},
		&models.AdBlockConfig{},
		&models.SystemConfig{},
	)
}

func InsertLog(level, message, source string, userID *int) error {
	log := models.SystemLog{
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

func GetLogs(level string, limit, offset int) ([]models.SystemLog, int64, error) {
	var logs []models.SystemLog
	var total int64

	query := db.Model(&models.SystemLog{})
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
	stat := models.SystemStatistic{
		Type:      statType,
		Value:     value,
		Timestamp: time.Now(),
	}
	return db.Create(&stat).Error
}

func SetConfig(key, value string) error {
	c := models.SystemConfig{Key: key, Value: value}
	return db.Save(&c).Error
}

func GetConfig(key string) (string, error) {
	var c models.SystemConfig
	if err := db.First(&c, "key = ?", key).Error; err != nil {
		return "", err
	}
	return c.Value, nil
}

func GetAllConfigs() (map[string]string, error) {
	if db == nil {
		return make(map[string]string), nil
	}
	var configs []models.SystemConfig
	if err := db.Find(&configs).Error; err != nil {
		return nil, err
	}
	
	result := make(map[string]string)
	for _, c := range configs {
		result[c.Key] = c.Value
	}
	return result, nil
}
