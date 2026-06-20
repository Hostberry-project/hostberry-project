package adblock

import "testing"

func TestBlockyBinaryExistsWithoutInstall(t *testing.T) {
	if blockyBinaryExists() {
		t.Skip("blocky present on system; skipping negative test")
	}
}

func TestBlockyConstants(t *testing.T) {
	if blockyHTTPPort != "4000" {
		t.Fatalf("unexpected blockyHTTPPort: %s", blockyHTTPPort)
	}
	if blockyConfigPath != "/etc/blocky/config.yml" {
		t.Fatalf("unexpected blockyConfigPath: %s", blockyConfigPath)
	}
}
