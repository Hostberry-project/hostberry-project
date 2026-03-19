package validators

import "testing"

func TestValidateIfaceName(t *testing.T) {
	tests := []struct {
		name  string
		iface string
		valid bool
	}{
		{"eth0", "eth0", true},
		{"wlan0", "wlan0", true},
		{"ap0", "ap0", true},
		{"br-lan", "br-lan", true},
		{"too long name 123456", "br012345678901234", false},
		{"empty", "", false},
		{"space", "eth 0", false},
		{"semicolon", "eth0;evil", false},
		{"dot start", ".eth0", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateIfaceName(tt iface)
			if tt.valid && err != nil {
				t.Fatalf("expected valid, got %v", err)
			}
			if !tt.valid && err == nil {
				t.Fatal("expected invalid")
			}
		})
	}
}
