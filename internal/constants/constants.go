package constants

const (
	Version              = "2.1.0"
	DefaultWiFiInterface = "wlan0"
	DefaultCountryCode   = "US"
	DefaultServerHost    = "0.0.0.0"
	DefaultServerPort    = 8000
	DefaultUnknownValue  = "N/A"
	// Red WiFi del AP HostBerry (portal cautivo); debe coincidir con hostapd GATEWAY/DHCP.
	DefaultAPNetworkCIDR = "192.168.4.0/24"
	DefaultAPGatewayIP   = "192.168.4.1"
	DefaultAPPortalURL      = "http://192.168.4.1/setup-wizard"
	DefaultAPSetupURL       = "http://192.168.4.1/setup-wizard"
	DefaultAPCaptiveAPIURL  = "http://192.168.4.1/api/captive-portal"
)
