package auth

import "testing"

func TestCheckPasswordRejectsPlaintextStoredPassword(t *testing.T) {
	if CheckPassword("admin", "admin") {
		t.Fatal("expected plaintext stored password to be rejected")
	}
}

func TestCheckPasswordAcceptsBcryptHash(t *testing.T) {
	hash, err := HashPassword("s3cret-pass")
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	if !CheckPassword("s3cret-pass", hash) {
		t.Fatal("expected bcrypt hash to validate")
	}
}
