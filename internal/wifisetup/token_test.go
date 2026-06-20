package wifisetup

import (
	"testing"

	"hostberry/internal/config"
)

func TestBypassAllowed_explicitToken(t *testing.T) {
	config.AppConfig = &config.Config{
		Security: config.SecurityConfig{WifiSetupToken: "my-fixed-setup-token-value"},
	}
	Init()
	RefreshSetupMode()
	if !BypassAllowed() {
		t.Fatal("explicit token should allow bypass")
	}
	if !Valid("my-fixed-setup-token-value") {
		t.Fatal("expected valid token")
	}
}

func TestDisableSetupBypass_afterSetup(t *testing.T) {
	config.AppConfig = &config.Config{Security: config.SecurityConfig{}}
	Init()
	RefreshSetupMode()
	token := TokenForDisplay()
	if token == "" {
		t.Fatal("expected generated token")
	}
	if !Valid(token) {
		t.Fatal("expected valid during setup mode")
	}
	DisableSetupBypass()
	if Valid(token) {
		t.Fatal("expected invalid after disable without explicit config token")
	}
}
