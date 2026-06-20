package wifi

import "testing"

func TestValidateInterfaceName(t *testing.T) {
	if err := validateInterfaceName("wlan0"); err != nil {
		t.Fatalf("wlan0 should be valid: %v", err)
	}
	if err := validateInterfaceName(""); err == nil {
		t.Fatal("empty should fail")
	}
	if err := validateInterfaceName("wlan0;rm"); err == nil {
		t.Fatal("injection should fail")
	}
}
