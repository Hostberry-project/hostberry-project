package network

import "testing"

func TestParseLinkState(t *testing.T) {
	out := "2: wlan0: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 state UP mode DORMANT"
	if got := parseLinkState(out); got != "up" {
		t.Fatalf("parseLinkState() = %q, want up", got)
	}
	if parseLinkState("2: eth0: <BROADCAST,MULTICAST> mtu 1500") != "" {
		t.Fatal("expected empty when state field is absent")
	}
}

func TestParseFirstIPv4FromIPAddr(t *testing.T) {
	out := "    inet 192.168.1.10/24 brd 192.168.1.255 scope global wlan0\n"
	ip, mask := parseFirstIPv4FromIPAddr(out)
	if ip != "192.168.1.10" || mask != "24" {
		t.Fatalf("got ip=%q mask=%q", ip, mask)
	}
}

func TestParseWPAState(t *testing.T) {
	out := "bssid=aa:bb:cc:dd:ee:ff\nwpa_state=COMPLETED\n"
	if got := parseWPAState(out); got != "COMPLETED" {
		t.Fatalf("parseWPAState() = %q", got)
	}
}

func TestParseDefaultGateway(t *testing.T) {
	routes := "default via 192.168.1.1 dev wlan0 proto dhcp\n"
	if got := parseDefaultGateway(routes, "wlan0"); got != "192.168.1.1" {
		t.Fatalf("parseDefaultGateway() = %q", got)
	}
	if got := parseDefaultGateway(routes, "eth0"); got != "" {
		t.Fatalf("expected empty for wrong iface, got %q", got)
	}
}
