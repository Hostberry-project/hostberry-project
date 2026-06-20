package wifi

import (
	"strings"
	"testing"
)

func TestBandFromFrequency(t *testing.T) {
	cases := map[int]string{
		2437: band24GHz,
		2412: band24GHz,
		5280: band5GHz,
		5180: band5GHz,
		0:    "",
		3000: "",
	}
	for freq, want := range cases {
		if got := bandFromFrequency(freq); got != want {
			t.Fatalf("bandFromFrequency(%d) = %q, want %q", freq, got, want)
		}
	}
}

func TestHwModeForFrequency(t *testing.T) {
	if got := hwModeForFrequency(2437); got != "g" {
		t.Fatalf("hwModeForFrequency(2437) = %q, want g", got)
	}
	if got := hwModeForFrequency(5280); got != "a" {
		t.Fatalf("hwModeForFrequency(5280) = %q, want a", got)
	}
}

func TestDefaultAPChannelForBand(t *testing.T) {
	if got := defaultAPChannelForBand(band24GHz); got != 6 {
		t.Fatalf("defaultAPChannelForBand(2.4) = %d, want 6", got)
	}
	if got := defaultAPChannelForBand(band5GHz); got != 36 {
		t.Fatalf("defaultAPChannelForBand(5) = %d, want 36", got)
	}
}

func TestFilterScanNetworksByBand(t *testing.T) {
	nets := []map[string]interface{}{
		{"ssid": "a", "frequency": 2437},
		{"ssid": "b", "frequency": 5280},
		{"ssid": "c", "channel": 56},
	}
	got24 := filterScanNetworksByBand(nets, band24GHz)
	if len(got24) != 1 || got24[0]["ssid"] != "a" {
		t.Fatalf("filter 2.4: got %#v", got24)
	}
	got5 := filterScanNetworksByBand(nets, band5GHz)
	if len(got5) != 2 {
		t.Fatalf("filter 5: got len=%d, want 2", len(got5))
	}
}

func TestScan5GHzFreqArg(t *testing.T) {
	arg := scan5GHzFreqArg()
	if arg == "" || arg[:5] != "freq=" {
		t.Fatalf("scan5GHzFreqArg = %q", arg)
	}
	if !strings.Contains(arg, "5180") || !strings.Contains(arg, "5280") {
		t.Fatalf("scan5GHzFreqArg missing expected freqs: %q", arg)
	}
}
