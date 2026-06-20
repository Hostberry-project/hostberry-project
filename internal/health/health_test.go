package health

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestHealthEndpointsAreMinimal(t *testing.T) {
	app := fiber.New()
	app.Get("/health", HealthCheckHandler)
	app.Get("/health/ready", ReadinessCheckHandler)
	app.Get("/health/live", LivenessCheckHandler)

	tests := []struct {
		path         string
		statusCode   int
		statusValue  string
		forbiddenKey string
	}{
		{path: "/health", statusCode: 503, statusValue: "degraded", forbiddenKey: "services"},
		{path: "/health/ready", statusCode: 503, statusValue: "not_ready", forbiddenKey: "message"},
		{path: "/health/live", statusCode: 200, statusValue: "alive", forbiddenKey: "message"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("app.Test failed: %v", err)
			}
			if resp.StatusCode != tt.statusCode {
				t.Fatalf("unexpected status: got %d want %d", resp.StatusCode, tt.statusCode)
			}
			if got := resp.Header.Get("Cache-Control"); got != "no-store" {
				t.Fatalf("unexpected Cache-Control header: %q", got)
			}

			var body map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
				t.Fatalf("decode failed: %v", err)
			}
			if body["status"] != tt.statusValue {
				t.Fatalf("unexpected status value: got %v want %s", body["status"], tt.statusValue)
			}
			if _, ok := body[tt.forbiddenKey]; ok {
				t.Fatalf("response must not expose %q", tt.forbiddenKey)
			}
			if _, ok := body["version"]; ok {
				t.Fatal("response must not expose version")
			}
		})
	}
}
