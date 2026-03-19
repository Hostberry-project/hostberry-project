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
			err := ValidateIfaceName(tt.iface)
			if tt.valid && err != nil {
				t.Fatalf("expected valid, got %v", err)
			}
			if !tt.valid && err == nil {
				t.Fatal("expected invalid")
			}
		})
	}
}

func TestValidatePhyName(t *testing.T) {
	for _, phy := range []string{"phy0", "phy1"} {
		if err := ValidatePhyName(phy); err != nil {
			t.Fatalf("%s: %v", phy, err)
		}
	}
	for _, bad := range []string{"", "PHY0", "phy", "phy01x"} {
		if ValidatePhyName(bad) == nil {
			t.Fatalf("expected invalid: %q", bad)
		}
	}
}

func TestValidateDhcpLeaseTime(t *testing.T) {
	for _, s := range []string{"12h", "30m", "3600s", "1d"} {
		if err := ValidateDhcpLeaseTime(s); err != nil {
			t.Fatalf("%q: %v", s, err)
		}
	}
	if ValidateDhcpLeaseTime("never") == nil {
		t.Fatal("expected invalid")
	}
}

func TestValidateCountryCode(t *testing.T) {
	if err := ValidateCountryCode("ES"); err != nil {
		t.Fatal(err)
	}
	if ValidateCountryCode("ESP") == nil {
		t.Fatal("expected invalid")
	}
}

func TestValidateWPAPSK(t *testing.T) {
	if err := ValidateWPAPSK("abcdefgh"); err != nil {
		t.Fatal(err)
	}
	if ValidateWPAPSK("short") == nil {
		t.Fatal("expected invalid")
	}
	if ValidateWPAPSK("abcd\x00efghij") == nil {
		t.Fatal("expected invalid control char")
	}
}
