package wifi

import "testing"

func TestShellQuote(t *testing.T) {
	cases := map[string]string{
		"":           "''",
		"plain":      "'plain'",
		"with space": "'with space'",
		"abc$def":    "'abc$def'",
		"a;b":        "'a;b'",
		"it's":       "'it'\"'\"'s'",
	}
	for in, want := range cases {
		if got := shellQuote(in); got != want {
			t.Fatalf("shellQuote(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestQuoteWpaCliSetNetworkValue(t *testing.T) {
	args := quoteWpaCliSetNetworkValue("set_network", "0", "ssid", "My$WiFi")
	if len(args) != 4 || args[3] != "'My$WiFi'" {
		t.Fatalf("unexpected quoted args: %#v", args)
	}
	unchanged := quoteWpaCliSetNetworkValue("status")
	if len(unchanged) != 1 || unchanged[0] != "status" {
		t.Fatalf("unexpected status args: %#v", unchanged)
	}
}
