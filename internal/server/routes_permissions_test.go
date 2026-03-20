package server

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gofiber/fiber/v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"hostberry/internal/auth"
	"hostberry/internal/config"
	"hostberry/internal/database"
	"hostberry/internal/models"
)

func setupPermissionTestApp(t *testing.T) *fiber.App {
	t.Helper()

	prevConfig := config.AppConfig
	prevDB := database.DB
	t.Cleanup(func() {
		config.AppConfig = prevConfig
		database.DB = prevDB
	})

	config.AppConfig = &config.Config{
		Server: config.ServerConfig{
			ReadTimeout:  30,
			WriteTimeout: 30,
		},
		Security: config.SecurityConfig{
			JWTSecret:    "test-secret",
			TokenExpiry:  60,
			BcryptCost:   4,
			RateLimitRPS: 10,
		},
	}

	dbPath := filepath.Join(t.TempDir(), "permissions.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open failed: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.SystemLog{}); err != nil {
		t.Fatalf("AutoMigrate failed: %v", err)
	}
	database.DB = db

	app := fiber.New()
	setupApiRoutes(app)
	setupHealthRoutes(app)
	return app
}

func createTestUserToken(t *testing.T, role string) string {
	t.Helper()

	user := models.User{
		Username:     "tester_" + role,
		Password:     "$2a$10$012345678901234567890uYv6tM7V3uVd5uJxM7iY5b6lG7h8i9jK", // placeholder bcrypt
		Role:         role,
		IsActive:     true,
		TokenVersion: 1,
	}
	if err := database.DB.Create(&user).Error; err != nil {
		t.Fatalf("creating test user failed: %v", err)
	}
	token, err := auth.GenerateToken(&user)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}
	return token
}

func TestProtectedCriticalRoutesRequireAuthentication(t *testing.T) {
	app := setupPermissionTestApp(t)

	tests := []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/api/v1/vpn/config"},
		{method: http.MethodPost, path: "/api/v1/system/config"},
		{method: http.MethodPost, path: "/api/v1/tor/iptables-enable"},
		{method: http.MethodPost, path: "/api/v1/wifi/connect"},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("app.Test failed: %v", err)
			}
			if resp.StatusCode != http.StatusUnauthorized {
				t.Fatalf("unexpected status: got %d want %d", resp.StatusCode, http.StatusUnauthorized)
			}
		})
	}
}

func TestCriticalAdminRoutesRejectNonAdminUser(t *testing.T) {
	app := setupPermissionTestApp(t)
	token := createTestUserToken(t, "user")

	tests := []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/api/v1/vpn/config"},
		{method: http.MethodPost, path: "/api/v1/system/config"},
		{method: http.MethodPost, path: "/api/v1/tor/iptables-enable"},
		{method: http.MethodPost, path: "/api/v1/wifi/connect"},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			req.Header.Set("Authorization", "Bearer "+token)
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("app.Test failed: %v", err)
			}
			if resp.StatusCode != http.StatusForbidden {
				t.Fatalf("unexpected status: got %d want %d", resp.StatusCode, http.StatusForbidden)
			}
		})
	}
}

func TestHealthRoutesRemainPublic(t *testing.T) {
	app := setupPermissionTestApp(t)

	tests := []string{"/health", "/health/ready", "/health/live"}
	for _, path := range tests {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("app.Test failed: %v", err)
			}
			if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
				t.Fatalf("health route should be public, got %d", resp.StatusCode)
			}
		})
	}
}
