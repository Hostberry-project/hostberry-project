package auth

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestCheckPasswordRejectsPlaintextStoredPassword(t *testing.T) {
	if CheckPassword("admin", "admin") {
		t.Fatal("expected plaintext stored password to be rejected")
	}
}

func TestCheckPasswordAcceptsBcryptHash(t *testing.T) {
	hashBytes, err := bcrypt.GenerateFromPassword([]byte("s3cret-pass"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("GenerateFromPassword failed: %v", err)
	}
	hash := string(hashBytes)
	if !CheckPassword("s3cret-pass", hash) {
		t.Fatal("expected bcrypt hash to validate")
	}
}
