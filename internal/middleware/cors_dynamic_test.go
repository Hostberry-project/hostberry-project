package middleware

import "testing"

func TestCorsOriginMatchesRequest(t *testing.T) {
	extra := []string{"https://panel.example.com"}
	tests := []struct {
		name       string
		hostHeader string
		srvPort    int
		origin     string
		want       bool
	}{
		{"match IP and port", "192.168.1.10:8000", 8000, "http://192.168.1.10:8000", true},
		{"host without port uses server port", "192.168.1.10", 8000, "http://192.168.1.10:8000", true},
		{"wrong port", "192.168.1.10:8000", 8000, "http://192.168.1.10:9000", false},
		{"localhost alias", "10.0.0.1:8000", 8000, "http://127.0.0.1:8000", true},
		{"extra list", "10.0.0.1:8000", 8000, "https://panel.example.com", true},
		{"empty origin", "127.0.0.1:8000", 8000, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CorsOriginMatchesRequest(tt.hostHeader, tt.srvPort, extra, tt.origin); got != tt.want {
				t.Fatalf("CorsOriginMatchesRequest(...) = %v, want %v", got, tt.want)
			}
		})
	}
}
