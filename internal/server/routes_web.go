package server

import (
	"github.com/gofiber/fiber/v2"
	adblockHandlers "hostberry/internal/adblock"
	authHandlers "hostberry/internal/auth"
	hostapdHandlers "hostberry/internal/hostapd"
	middleware "hostberry/internal/middleware"
	networkHandlers "hostberry/internal/network"
	sys "hostberry/internal/system"
	torHandlers "hostberry/internal/tor"
	vpnHandlers "hostberry/internal/vpn"
	wifiHandlers "hostberry/internal/wifi"
)

func setupWebRoutes(app *fiber.App) {
	web := app.Group("/")
	{
		web.Get("/login", authHandlers.LoginPageHandler)
		web.Get("/first-login", sys.FirstLoginPageHandler)
		web.Get("/", authHandlers.IndexPageHandler)

		protected := web.Group("/", middleware.RequireAuth)
		protected.Get("/dashboard", authHandlers.DashboardPageHandler)
		protected.Get("/settings", authHandlers.SettingsPageHandler)
		protected.Get("/network", networkHandlers.NetworkPageHandler)
		protected.Get("/wifi", wifiHandlers.WifiPageHandler)
		protected.Get("/wifi-scan", wifiHandlers.WifiScanPageHandler)
		protected.Get("/vpn", vpnHandlers.VpnPageHandler)
		protected.Get("/wireguard", vpnHandlers.WireguardPageHandler)
		protected.Get("/adblock", adblockHandlers.AdblockPageHandler)
		protected.Get("/tor", torHandlers.TorPageHandler)
		protected.Get("/hostapd", hostapdHandlers.HostapdPageHandler)
		protected.Get("/setup-wizard", sys.SetupWizardPageHandler)
		protected.Get("/setup-wizard/vpn", sys.SetupWizardVpnPageHandler)
		protected.Get("/setup-wizard/wireguard", sys.SetupWizardWireguardPageHandler)
		protected.Get("/setup-wizard/tor", sys.SetupWizardTorPageHandler)
		protected.Get("/profile", sys.ProfilePageHandler)
		protected.Get("/system", sys.SystemPageHandler)
		protected.Get("/monitoring", sys.MonitoringPageHandler)
		protected.Get("/update", sys.UpdatePageHandler)
	}
}

