package server

import (
	adblockHandlers "hostberry/internal/adblock"
	authHandlers "hostberry/internal/auth"
	health "hostberry/internal/health"
	hostapdHandlers "hostberry/internal/hostapd"
	i18n "hostberry/internal/i18n"
	networkHandlers "hostberry/internal/network"
	sys "hostberry/internal/system"
	torHandlers "hostberry/internal/tor"
	vpnHandlers "hostberry/internal/vpn"
	wifiHandlers "hostberry/internal/wifi"

	middleware "hostberry/internal/middleware"

	"github.com/gofiber/fiber/v2"

	"time"
)

func setupApiRoutes(app *fiber.App) {
	api := app.Group("/api/v1")
	{
		authRoutes := api.Group("/auth")
		{
			authRoutes.Post("/login", authHandlers.LoginRateLimitMiddleware, authHandlers.LoginAPIHandler)
			authRoutes.Post("/logout", middleware.RequireAuth, authHandlers.LogoutAPIHandler)
			authRoutes.Post("/logout-all", middleware.RequireAuth, authHandlers.LogoutAllSessionsAPIHandler)
			authRoutes.Get("/me", middleware.RequireAuth, authHandlers.MeHandler)
			authRoutes.Post("/change-password", middleware.RequireAuth, authHandlers.ChangePasswordAPIHandler)
			authRoutes.Post("/first-login/change", authHandlers.FirstLoginRateLimitMiddleware, authHandlers.FirstLoginChangeAPIHandler)
			authRoutes.Post("/profile", middleware.RequireAuth, authHandlers.UpdateProfileAPIHandler)
			authRoutes.Post("/preferences", middleware.RequireAuth, authHandlers.UpdatePreferencesAPIHandler)
		}

		system := api.Group("/system", middleware.RequireAuth)
		{
			system.Get("/stats", sys.SystemStatsHandler)
			system.Get("/info", sys.SystemInfoHandler)
			system.Get("/https-info", sys.SystemHttpsInfoHandler)
			system.Get("/logs", sys.SystemLogsHandler)
			system.Get("/activity", sys.SystemActivityHandler)
			system.Get("/network", sys.SystemNetworkHandler)
			system.Get("/updates", sys.SystemUpdatesHandler)
			system.Get("/services", sys.SystemServicesHandler)
			system.Get("/metrics", health.MetricsSummaryHandler)
			system.Post("/backup", middleware.RequireAdmin, sys.SystemBackupHandler)
			system.Post("/config", middleware.RequireAdmin, sys.SystemConfigHandler)
			system.Post("/updates/execute", middleware.RequireAdmin, sys.SystemUpdatesExecuteHandler)
			system.Post("/updates/project", middleware.RequireAdmin, sys.SystemUpdatesProjectHandler)
			system.Post("/notifications/test-email", middleware.RequireAdmin, sys.SystemNotificationsTestEmailHandler)
			system.Post("/restart", middleware.RequireAdmin, sys.SystemRestartHandler)
			system.Post("/shutdown", middleware.RequireAdmin, sys.SystemShutdownHandler)
		}

		network := api.Group("/network", middleware.RequireAuth)
		{
			network.Get("/status", networkHandlers.NetworkStatusHandler)
			network.Get("/interfaces", networkHandlers.NetworkInterfacesHandler)
			network.Get("/routing", networkHandlers.NetworkRoutingHandler)
			network.Post("/firewall/toggle", middleware.RequireAdmin, middleware.MutationLock("network_stack", 10*time.Second), networkHandlers.NetworkFirewallToggleHandler)
			network.Post("/speedtest", middleware.RequireAdmin, middleware.MutationLock("network_stack", 10*time.Second), networkHandlers.NetworkSpeedtestHandler)
			network.Get("/config", networkHandlers.NetworkConfigHandler)
			network.Post("/config", middleware.RequireAdmin, middleware.MutationLock("network_stack", 10*time.Second), networkHandlers.NetworkConfigHandler)
		}

		wifi := api.Group("/wifi", middleware.RequireAuth)
		{
			wifi.Get("/status", wifiHandlers.WifiStatusHandler)
			wifi.Get("/scan", wifiHandlers.WifiScanHandler)
			wifi.Post("/scan", wifiHandlers.WifiScanHandler)
			wifi.Get("/interfaces", wifiHandlers.WifiInterfacesHandler)
			wifi.Post("/connect", middleware.RequireAdmin, middleware.MutationLock("radio", 10*time.Second), wifiHandlers.WifiConnectHandler)
			wifi.Post("/disconnect", middleware.RequireAdmin, middleware.MutationLock("radio", 10*time.Second), wifiHandlers.WifiLegacyDisconnectHandler)
			wifi.Get("/networks", wifiHandlers.WifiNetworksHandler)
			wifi.Get("/clients", wifiHandlers.WifiClientsHandler)
			wifi.Post("/toggle", middleware.RequireAdmin, middleware.MutationLock("radio", 10*time.Second), wifiHandlers.WifiToggleHandler)
			wifi.Post("/unblock", middleware.RequireAdmin, middleware.MutationLock("radio", 10*time.Second), wifiHandlers.WifiUnblockHandler)
			wifi.Post("/software-switch", middleware.RequireAdmin, middleware.MutationLock("radio", 10*time.Second), wifiHandlers.WifiSoftwareSwitchHandler)
			wifi.Post("/config", middleware.RequireAdmin, middleware.MutationLock("radio", 10*time.Second), wifiHandlers.WifiConfigHandler)
		}

		vpn := api.Group("/vpn", middleware.RequireAuth)
		{
			vpn.Get("/status", vpnHandlers.VpnStatusHandler)
			vpn.Get("/config", middleware.RequireAdmin, vpnHandlers.VpnGetConfigHandler)
			vpn.Post("/connect", middleware.RequireAdmin, middleware.MutationLock("network_stack", 10*time.Second), vpnHandlers.VpnConnectHandler)
			vpn.Get("/connections", vpnHandlers.VpnConnectionsHandler)
			vpn.Get("/servers", vpnHandlers.VpnServersHandler)
			vpn.Get("/clients", vpnHandlers.VpnClientsHandler)
			vpn.Post("/toggle", middleware.RequireAdmin, middleware.MutationLock("network_stack", 10*time.Second), vpnHandlers.VpnToggleHandler)
			vpn.Post("/config", middleware.RequireAdmin, middleware.MutationLock("network_stack", 10*time.Second), vpnHandlers.VpnConfigHandler)
			vpn.Post("/connections/:name/toggle", middleware.RequireAdmin, middleware.MutationLock("network_stack", 10*time.Second), vpnHandlers.VpnConnectionToggleHandler)
			vpn.Post("/certificates/generate", middleware.RequireAdmin, middleware.MutationLock("network_stack", 10*time.Second), vpnHandlers.VpnCertificatesGenerateHandler)
		}

		hostapd := api.Group("/hostapd", middleware.RequireAuth)
		{
			hostapd.Get("/access-points", hostapdHandlers.HostapdAccessPointsHandler)
			hostapd.Get("/clients", hostapdHandlers.HostapdClientsHandler)
			hostapd.Get("/config", hostapdHandlers.HostapdGetConfigHandler)
			hostapd.Get("/diagnostics", hostapdHandlers.HostapdDiagnosticsHandler)
			hostapd.Post("/create-ap0", middleware.RequireAdmin, middleware.MutationLock("radio", 10*time.Second), hostapdHandlers.HostapdCreateAp0Handler)
			hostapd.Post("/toggle", middleware.RequireAdmin, middleware.MutationLock("radio", 10*time.Second), hostapdHandlers.HostapdToggleHandler)
			hostapd.Post("/restart", middleware.RequireAdmin, middleware.MutationLock("radio", 10*time.Second), hostapdHandlers.HostapdRestartHandler)
			hostapd.Post("/config", middleware.RequireAdmin, middleware.MutationLock("radio", 10*time.Second), hostapdHandlers.HostapdConfigHandler)
		}

		help := api.Group("/help", middleware.RequireAuth)
		{
			help.Post("/contact", sys.HelpContactHandler)
		}

		// Traducciones: endpoint sin auth (lo usa el frontend).
		api.Get("/translations/:lang", i18n.TranslationsHandler)

		wireguard := api.Group("/wireguard", middleware.RequireAuth)
		{
			wireguard.Get("/status", vpnHandlers.WireguardStatusHandler)
			wireguard.Get("/interfaces", vpnHandlers.WireguardInterfacesHandler)
			wireguard.Get("/peers", vpnHandlers.WireguardPeersHandler)
			wireguard.Get("/config", middleware.RequireAdmin, vpnHandlers.WireguardGetConfigHandler)
			wireguard.Post("/config", middleware.RequireAdmin, vpnHandlers.WireguardConfigHandler)
			wireguard.Post("/toggle", middleware.RequireAdmin, vpnHandlers.WireguardToggleHandler)
			wireguard.Post("/restart", middleware.RequireAdmin, vpnHandlers.WireguardRestartHandler)
		}

		adblock := api.Group("/adblock", middleware.RequireAuth)
		{
			adblock.Get("/status", adblockHandlers.AdblockStatusHandler)
			adblock.Get("/lists", sys.AdblockListsHandler)
			adblock.Get("/domains", sys.AdblockDomainsHandler)
			adblock.Post("/enable", middleware.RequireAdmin, adblockHandlers.AdblockEnableHandler)
			adblock.Post("/disable", middleware.RequireAdmin, adblockHandlers.AdblockDisableHandler)
			adblock.Post("/update", middleware.RequireAdmin, sys.AdblockUpdateHandler)
			adblock.Post("/lists/:name/toggle", middleware.RequireAdmin, sys.AdblockToggleListHandler)
			adblock.Post("/domains/:name/toggle", middleware.RequireAdmin, sys.AdblockToggleDomainHandler)
			adblock.Post("/config", middleware.RequireAdmin, sys.AdblockConfigHandler)

			dnscrypt := adblock.Group("/dnscrypt")
			{
				dnscrypt.Get("/status", adblockHandlers.DnscryptStatusHandler)
				dnscrypt.Post("/install", middleware.RequireAdmin, adblockHandlers.DnscryptInstallHandler)
				dnscrypt.Post("/configure", middleware.RequireAdmin, adblockHandlers.DnscryptConfigureHandler)
				dnscrypt.Post("/enable", middleware.RequireAdmin, adblockHandlers.DnscryptEnableHandler)
				dnscrypt.Post("/disable", middleware.RequireAdmin, adblockHandlers.DnscryptDisableHandler)
			}

			adblock.Get("/blocky/status", adblockHandlers.BlockyStatusHandler)
			adblock.Get("/blocky/config", adblockHandlers.BlockyConfigHandler)
			adblock.Post("/blocky/install", middleware.RequireAdmin, adblockHandlers.BlockyInstallHandler)
			adblock.Post("/blocky/configure", middleware.RequireAdmin, adblockHandlers.BlockyConfigureHandler)
			adblock.Post("/blocky/enable", middleware.RequireAdmin, adblockHandlers.BlockyEnableHandler)
			adblock.Post("/blocky/disable", middleware.RequireAdmin, adblockHandlers.BlockyDisableHandler)
			adblock.Get("/blocky/api/*", middleware.RequireAdmin, adblockHandlers.BlockyAPIProxyHandler)
			adblock.Post("/blocky/api/*", middleware.RequireAdmin, adblockHandlers.BlockyAPIProxyHandler)
		}

		tor := api.Group("/tor", middleware.RequireAuth)
		{
			tor.Get("/status", torHandlers.TorStatusHandler)
			tor.Post("/install", middleware.RequireAdmin, torHandlers.TorInstallHandler)
			tor.Post("/configure", middleware.RequireAdmin, torHandlers.TorConfigureHandler)
			tor.Post("/enable", middleware.RequireAdmin, torHandlers.TorEnableHandler)
			tor.Post("/disable", middleware.RequireAdmin, torHandlers.TorDisableHandler)
			tor.Get("/circuit", torHandlers.TorCircuitHandler)
			tor.Post("/iptables-enable", middleware.RequireAdmin, middleware.MutationLock("iptables", 10*time.Second), torHandlers.TorIptablesEnableHandler)
			tor.Post("/iptables-disable", middleware.RequireAdmin, middleware.MutationLock("iptables", 10*time.Second), torHandlers.TorIptablesDisableHandler)
		}
	}

	// Legacy WiFi endpoints bajo /api/wifi
	wifiLegacy := app.Group("/api/wifi", middleware.RequireAuth)
	wifiLegacy.Get("/status", wifiHandlers.WifiLegacyStatusHandler)
	wifiLegacy.Get("/stored_networks", wifiHandlers.WifiLegacyStoredNetworksHandler)
	wifiLegacy.Get("/autoconnect", wifiHandlers.WifiLegacyAutoconnectHandler)
	wifiLegacy.Get("/scan", wifiHandlers.WifiLegacyScanHandler)
	wifiLegacy.Post("/disconnect", middleware.MutationLock("radio", 10*time.Second), wifiHandlers.WifiLegacyDisconnectHandler)
}
