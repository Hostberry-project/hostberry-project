package captiveportal

import "testing"

func TestIsProbePath(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"/generate_204", true},
		{"/hotspotdetect.html", true},
		{"/mobile/status.php", true},
		{"/api/captive-portal", false},
		{"/dashboard", false},
	}
	for _, tc := range cases {
		if got := IsProbePath(tc.path); got != tc.want {
			t.Fatalf("IsProbePath(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestIsAllowedWebPath(t *testing.T) {
	if !IsAllowedWebPath("/setup-wizard/vpn") {
		t.Fatal("setup wizard subpath should be allowed")
	}
	if !IsAllowedWebPath(APIPath) {
		t.Fatal("captive portal API should be allowed")
	}
	if IsAllowedWebPath("/random") {
		t.Fatal("random path should not be allowed")
	}
}
