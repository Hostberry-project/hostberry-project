package database

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"hostberry/internal/config"
	"hostberry/internal/i18n"
	"hostberry/internal/models"

	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DB es la conexión global a la base de datos (asignada tras Init).
var DB *gorm.DB

const maxLogMessageLen = 1024

// Init inicializa la base de datos según la configuración de la aplicación.
func Init() error {
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
			cfg.Database.Host, cfg.Database.User, cfg.Database.Password,
			cfg.Database.Database, cfg.Database.Port)
		dialector = postgres.Open(dsn)
	case "mysql":
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			cfg.Database.User, cfg.Database.Password, cfg.Database.Host,
			cfg.Database.Port, cfg.Database.Database)
		dialector = mysql.Open(dsn)
	default:
		return fmt.Errorf("tipo de base de datos no soportado: %s", cfg.Database.Type)
	}

	gormLogger := logger.Default
	if !cfg.Server.Debug {
		gormLogger = logger.Default.LogMode(logger.Silent)
	}

	DB, err = gorm.Open(dialector, &gorm.Config{Logger: gormLogger})
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
	if err := DB.AutoMigrate(
		&models.User{},
		&models.SystemLog{},
		&models.SystemStatistic{},
		&models.NetworkConfig{},
		&models.VPNConfig{},
		&models.WireGuardConfig{},
		&models.AdBlockConfig{},
		&models.SystemConfig{},
	); err != nil {
		return err
	}
	// Cuentas con varios logins exitosos ya completaron el flujo antiguo (antes de first_login_completed).
	// login_count == 2 puede ser usuario atascado por bug o recién cambiado: se deja en false para permitir POST /first-login/change.
	if err := DB.Model(&models.User{}).Where("login_count >= ?", 3).Update("first_login_completed", true).Error; err != nil {
		return err
	}
	return nil
}

// InsertLog registra una entrada en el log del sistema.
func InsertLog(level, message, source string, userID *int) error {
	log := models.SystemLog{
		Level: level, Message: message, Source: source, UserID: userID,
	}
	return DB.Create(&log).Error
}

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

// LogMsg formatea un mensaje de log con acción y usuario.
func LogMsg(action, user string) string {
	action = truncateForLog(strings.TrimRight(action, "."), 800)
	user = truncateForLog(user, 120)
	if user == "" {
		return action + "."
	}
	return action + ". Usuario: " + user + "."
}

// LogMsgErr formatea un mensaje de error para logs.
func LogMsgErr(action, reason, user string) string {
	action = truncateForLog(strings.TrimRight(action, "."), 200)
	reason = truncateForLog(reason, 600)
	user = truncateForLog(user, 120)
	if user == "" {
		return "Error al " + action + ": " + reason + "."
	}
	return "Error al " + action + ": " + reason + ". Usuario: " + user + "."
}

// LogMsgWarn formatea un mensaje de advertencia para logs.
func LogMsgWarn(desc, user string) string {
	desc = truncateForLog(desc, 800)
	user = truncateForLog(user, 120)
	if user == "" {
		return "Advertencia: " + desc + "."
	}
	return "Advertencia: " + desc + ". Usuario: " + user + "."
}

// GetLogs devuelve entradas del log con paginación.
func GetLogs(level string, limit, offset int) ([]models.SystemLog, int64, error) {
	var logs []models.SystemLog
	var total int64
	query := DB.Model(&models.SystemLog{})
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

// InsertStatistic registra una estadística.
func InsertStatistic(statType string, value float64) error {
	stat := models.SystemStatistic{Type: statType, Value: value, Timestamp: time.Now()}
	return DB.Create(&stat).Error
}

// SetConfig guarda un par clave-valor de configuración.
func SetConfig(key, value string) error {
	c := models.SystemConfig{Key: key, Value: value}
	return DB.Save(&c).Error
}

// GetConfig obtiene el valor de una clave de configuración.
func GetConfig(key string) (string, error) {
	var c models.SystemConfig
	if err := DB.First(&c, "key = ?", key).Error; err != nil {
		return "", err
	}
	return c.Value, nil
}

// GetAllConfigs devuelve todas las configuraciones como mapa.
func GetAllConfigs() (map[string]string, error) {
	if DB == nil {
		return make(map[string]string), nil
	}
	var configs []models.SystemConfig
	if err := DB.Find(&configs).Error; err != nil {
		return nil, err
	}
	result := make(map[string]string)
	for _, c := range configs {
		result[c.Key] = c.Value
	}
	return result, nil
}
