package wifi

import (
	"strconv"
	"strings"
	"testing"
)

func TestChannelToCenterFreq(t *testing.T) {
	cases := map[int]int{
		1: 2412, 6: 2437, 13: 2472, 36: 5180, 64: 5320, 0: 0, 200: 0,
	}
	for ch, want := range cases {
		if got := channelToCenterFreq(ch); got != want {
			t.Fatalf("channelToCenterFreq(%d) = %d, want %d", ch, got, want)
		}
	}
}

func TestParseFrequencyFromIwLink(t *testing.T) {
	out := "Connected to aa:bb (on wlan0)\n\tfreq: 5320.0\n\tsignal: -70 dBm"
	if got := parseFrequencyFromIwLink(out); got != 5320 {
		t.Fatalf("parseFrequencyFromIwLink = %d, want 5320", got)
	}
}

func TestFreqToChannelMatchesAPSync(t *testing.T) {
	if ch := freqToChannel(5320); ch != 64 {
		t.Fatalf("freqToChannel(5320) = %d, want 64", ch)
	}
}

func TestReadHostapdChannelModeLastWins(t *testing.T) {
	content := "hw_mode=g\nchannel=6\nchannel=64\nssid=hostberry\n"
	ch, mode := parseHostapdConfigContent(content)
	if ch != 64 || mode != "g" {
		t.Fatalf("got ch=%d mode=%q, want 64 g", ch, mode)
	}
}

func parseHostapdConfigContent(content string) (channel int, hwMode string) {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "channel=") {
			if v, e := strconv.Atoi(strings.TrimPrefix(line, "channel=")); e == nil && v > 0 {
				channel = v
			}
		}
		if strings.HasPrefix(line, "hw_mode=") {
			if v := strings.TrimPrefix(line, "hw_mode="); v != "" {
				hwMode = v
			}
		}
	}
	return channel, hwMode
}

func atoiTest(s string) int {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			break
		}
		n = n*10 + int(r-'0')
	}
	return n
}
