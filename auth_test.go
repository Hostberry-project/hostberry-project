package main

import (
	"testing"
	"time"
)

// TestGenerateAndValidateToken comprueba que un token generado con GenerateToken
// se puede validar con ValidateToken y que contiene el UserID correcto.
func TestGenerateAndValidateToken(t *testing.T) {
	appConfig = Config{
		Security: SecurityConfig{
			JWTSecret:    "test-secret",
			TokenExpiry:  5,
			BcryptCost:   4,
			RateLimitRPS: 10,
		},
	}

	u := &User{
		ID:        42,
		Username:  "testuser",
		CreatedAt: time.Now(),
	}

	token, err := GenerateToken(u)
	if err != nil {
		t.Fatalf("GenerateToken devolvió error: %v", err)
	}
	if token == "" {
		t.Fatalf("GenerateToken devolvió token vacío")
	}

	claims, err := ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken devolvió error: %v", err)
	}
	if claims.UserID != u.ID {
		t.Fatalf("UserID inesperado en claims: got=%d want=%d", claims.UserID, u.ID)
	}
	if claims.Username != u.Username {
		t.Fatalf("Username inesperado en claims: got=%s want=%s", claims.Username, u.Username)
	}
}

