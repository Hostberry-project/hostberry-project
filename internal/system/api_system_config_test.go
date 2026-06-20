package system

import "testing"

func TestNormalizeSystemConfigValue(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		value    interface{}
		want     string
		wantErr  bool
		wantSkip bool
	}{
		{name: "language ok", key: "language", value: "ES", want: "es"},
		{name: "language bad", key: "language", value: "fr", wantErr: true},
		{name: "bool ok", key: "cache_enabled", value: true, want: "true"},
		{name: "bool string ok", key: "smtp_tls", value: "off", want: "false"},
		{name: "int ok", key: "session_timeout", value: float64(60), want: "60"},
		{name: "int out of range", key: "session_timeout", value: float64(1), wantErr: true},
		{name: "dns ok", key: "dns_server", value: "1.1.1.1,8.8.8.8", want: "1.1.1.1,8.8.8.8"},
		{name: "dns bad", key: "dns_server", value: "1.1.1.1,999.1.1.1", wantErr: true},
		{name: "smtp password blank skipped", key: "smtp_password", value: "   ", wantSkip: true},
		{name: "iface bad", key: "dhcp_interface", value: "eth0;rm", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, skip, err := normalizeSystemConfigValue(tt.key, tt.value)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if skip != tt.wantSkip {
				t.Fatalf("unexpected skip: got %v want %v", skip, tt.wantSkip)
			}
			if got != tt.want {
				t.Fatalf("unexpected value: got %q want %q", got, tt.want)
			}
		})
	}
}

func TestValidateDHCPConfig(t *testing.T) {
	valid := map[string]string{
		"dhcp_enabled":     "true",
		"dhcp_interface":   "eth0",
		"dhcp_range_start": "192.168.1.100",
		"dhcp_range_end":   "192.168.1.200",
		"dhcp_gateway":     "192.168.1.1",
		"dhcp_lease_time":  "12h",
	}
	if err := validateDHCPConfig(valid); err != nil {
		t.Fatalf("expected valid DHCP config: %v", err)
	}

	invalidRange := map[string]string{
		"dhcp_enabled":     "true",
		"dhcp_interface":   "eth0",
		"dhcp_range_start": "192.168.1.220",
		"dhcp_range_end":   "192.168.1.200",
		"dhcp_gateway":     "192.168.1.1",
		"dhcp_lease_time":  "12h",
	}
	if err := validateDHCPConfig(invalidRange); err == nil {
		t.Fatal("expected invalid DHCP range ordering")
	}

	gatewayInsideRange := map[string]string{
		"dhcp_enabled":     "true",
		"dhcp_interface":   "eth0",
		"dhcp_range_start": "192.168.1.100",
		"dhcp_range_end":   "192.168.1.200",
		"dhcp_gateway":     "192.168.1.150",
		"dhcp_lease_time":  "12h",
	}
	if err := validateDHCPConfig(gatewayInsideRange); err == nil {
		t.Fatal("expected gateway-inside-range config to be rejected")
	}
}
