package wifi

import (
	"encoding/json"
	"os"
	"testing"
)

func TestEnsureDualBandHostapdLive(t *testing.T) {
	if os.Getenv("HB_LIVE_WIFI") == "" {
		t.Skip("set HB_LIVE_WIFI=1 to run")
	}
	result := EnsureDualBandHostapd("wlan0", true)
	b, _ := json.MarshalIndent(result, "", "  ")
	t.Logf("EnsureDualBandHostapd: %s", string(b))
	if success, ok := result["success"].(bool); ok && !success {
		t.Fatalf("EnsureDualBandHostapd failed: %v", result["error"])
	}
}
