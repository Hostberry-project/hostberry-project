package wifi

import (
	"os"
	"testing"
)

func TestOperatingBandForScanLive(t *testing.T) {
	if os.Getenv("HB_LIVE_WIFI") == "" {
		t.Skip("set HB_LIVE_WIFI=1 to run")
	}
	if !concurrentAPInterfacePresent() {
		t.Skip("no concurrent AP")
	}
	band := operatingBandForScan("wlan0")
	if band != band5GHz && band != band24GHz {
		t.Fatalf("unexpected band %q", band)
	}
	t.Logf("operating band for scan: %s", band)
}

func TestScanWiFiNetworksLive(t *testing.T) {
	if os.Getenv("HB_LIVE_WIFI") == "" {
		t.Skip("set HB_LIVE_WIFI=1 to run")
	}
	result := ScanWiFiNetworks("wlan0", true)
	if success, _ := result["success"].(bool); !success {
		t.Fatalf("scan failed: %v", result["error"])
	}
	nets, _ := result["networks"].([]map[string]interface{})
	if len(nets) == 0 {
		t.Fatal("expected networks, got 0")
	}
	t.Logf("found %d networks", len(nets))
}
