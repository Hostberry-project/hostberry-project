package server

import (
	"net/http"
	"net/url"
	"testing"
)

func TestHTTPSRedirectURL(t *testing.T) {
	cases := []struct {
		name     string
		reqURL   string
		httpsPort int
		wantPrefix string
	}{
		{"ipv4 with port", "http://192.168.1.5:8000/foo?q=1", 8443, "https://192.168.1.5:8443/foo?q=1"},
		{"host without port", "http://192.168.1.5/", 8443, "https://192.168.1.5:8443/"},
		{"port 443 omits port in URL", "http://example.com/path", 443, "https://example.com/path"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u, err := url.Parse(tc.reqURL)
			if err != nil {
				t.Fatal(err)
			}
			r := &http.Request{URL: u, Host: u.Host}
			got := httpsRedirectURL(r, tc.httpsPort)
			if got != tc.wantPrefix {
				t.Fatalf("got %q want %q", got, tc.wantPrefix)
			}
		})
	}
}
