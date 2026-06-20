package hostapd

import (
	"strings"
	"testing"
)

func TestUpsertConfigKeyAddsAndUpdates(t *testing.T) {
	base := "interface=ap0\nssid=hostberry\n"
	got := upsertConfigKey(base, hostapdMaxNumSTAKey, "1")
	if !strings.Contains(got, "max_num_sta=1") {
		t.Fatalf("expected max_num_sta=1 in %q", got)
	}

	got = upsertConfigKey(got, hostapdMaxNumSTAKey, "1")
	if strings.Count(got, "max_num_sta=") != 1 {
		t.Fatalf("expected single max_num_sta line, got %q", got)
	}
}

func TestRemoveConfigKey(t *testing.T) {
	base := "interface=ap0\nmax_num_sta=1\nssid=hostberry\n"
	got := removeConfigKey(base, hostapdMaxNumSTAKey)
	if strings.Contains(got, "max_num_sta=") {
		t.Fatalf("expected max_num_sta removed, got %q", got)
	}
}

func TestSetDHCPRangeLineSingleClient(t *testing.T) {
	base := "# test\ndhcp-range=192.168.4.2,192.168.4.254,255.255.255.0,12h\n"
	got := mutateDnsmasqDHCPRange([]byte(base), true)
	// Setup: pool pequeño y lease corto para liberar la IP al desconectarse.
	want := "dhcp-range=192.168.4.2,192.168.4.20,255.255.255.0,2m"
	if !strings.Contains(string(got), want) {
		t.Fatalf("expected %q in %q", want, string(got))
	}

	got = mutateDnsmasqDHCPRange(got, false)
	want = "dhcp-range=192.168.4.2,192.168.4.254,255.255.255.0,12h"
	if !strings.Contains(string(got), want) {
		t.Fatalf("expected %q in %q", want, string(got))
	}
}
