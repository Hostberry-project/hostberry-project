# Estructura modular del proyecto

El proyecto está organizado con paquetes bajo `internal/` para separar configuración, modelos, lógica de negocio y utilidades del paquete principal.

## Paquetes `internal/`

| Paquete | Descripción |
|---------|-------------|
| **internal/config** | Tipos `Config`, `ServerConfig`, `DatabaseConfig`, `SecurityConfig`. `Load()` lee `config.yaml` y asigna `AppConfig`. `Normalize()` endurece JWT y bcrypt. `GenerateRandomSecret()` para secretos. |
| **internal/constants** | Constantes globales: `DefaultWiFiInterface`, `DefaultCountryCode`, `DefaultServerHost`, `DefaultServerPort`, `DefaultUnknownValue`. |
| **internal/models** | Modelos de dominio y BD: `User`, `Claims`, `LoginError`, `SystemLog`, `SystemConfig`, `SystemStatistic`, `NetworkConfig`, `VPNConfig`, `WireGuardConfig`, `AdBlockConfig`. |
| **internal/validators** | Funciones de validación: `ValidateUsername`, `ValidatePassword`, `ValidateEmail`, `ValidateIP`, `ValidateSSID`, `ValidateWireGuardConfig`, `ValidateVPNConfig`. |
| **internal/metrics** | Contadores HTTP por clase de estado (2xx, 4xx, 5xx) para métricas y health. `Add2xx()`, `Add4xx()`, `Add5xx()`, `Load2xx()`, etc. |
| **internal/i18n** | Internacionalización: `Init()`, `T()`, `GetCurrentLanguage()`, `TemplateFuncs()`, `LanguageMiddleware`, `LogT`, `LogTf`, `LogTln`, `LogTfatal`, `SetLogLanguage`, `GetLogLanguage`, `Ready()`. |
| **internal/database** | Conexión y operaciones de BD: `Init()`, `DB`, `InsertLog`, `GetLogs`, `SetConfig`, `GetConfig`, `GetAllConfigs`, `InsertStatistic`, `LogMsg`, `LogMsgErr`, `LogMsgWarn`. |
| **internal/auth** | Autenticación y JWT: `GenerateToken`, `ValidateToken`, `HashPassword`, `CheckPassword`, `Login`, `Register`, `RegisterBootstrap`, `IsDefaultAdminCredentialsInUse`. |
| **internal/middleware** | Middlewares/auth: `RequireAuth` (JWT/cookie; API WiFi de setup solo con `X-HostBerry-WiFi-Setup-Token` o `wifi_setup_token`; CSRF: rechazo si `Sec-Fetch-Site: cross-site` con cookie en métodos mutantes), `RequireAdmin`, `GetUser`, `RunActionWithUser`, `DynamicCORSWithCredentials` (CORS sin `*` con credenciales; reflejo de Origin según Host y `cors_allow_origins`), `LoggingMiddleware`, `ErrorHandler`, `SecurityHeadersMiddleware`, `EnforceHTTPSMiddleware`, `RequestIDMiddleware`, `RateLimitMiddleware`. |
| **internal/wifisetup** | Token de arranque (o fijo en `config.yaml`) para `/api/v1/wifi/{status,scan,connect,disconnect}` sin sesión; `Init`, `Valid`, `ExtractFromRequest`. |
| **internal/health** | Endpoints de salud y métricas: `HealthCheckHandler`, `ReadinessCheckHandler`, `LivenessCheckHandler`, `MetricsHandler`, `MetricsSummaryHandler`. |
| **internal/templates** | Motor de templates y render: `CreateTemplateEngine`, `RenderTemplate`. |
| **internal/utils** | Utilidades compartidas: ejecución de comandos permitidos (caché/filtrado), parseos seguros y helpers de estado. |
| **internal/adblock** | Lógica DNS (dnsmasq/pihole/dnscrypt) y AdBlock (blocky): estados/instalación/configuración/proxy. |
| **internal/network** | Lógica de red: interfaces y estado (gateways/DNS) para endpoints. |
| **internal/wifi** | Lógica WiFi: `ScanWiFiNetworks`, `ToggleWiFi`, `ConnectWiFi`, `AutoConnectToLastNetwork` y helpers wpa_supplicant. |
| **internal/vpn** | Lógica VPN: OpenVPN y WireGuard (`GetVPNStatus`, `GetOpenVPNConfig`, `ConnectVPN`, `ConfigureWireGuard`, etc.). |
| **internal/system** | Lógica del sistema: info/estadísticas y acciones como `SystemRestart`/`SystemShutdown`. |
| **internal/tor** | Lógica Tor: instalación/configuración/iptables (`GetTorStatus`, `InstallTor`, `ConfigureTor`, `EnableTorIptables`, etc.). |
| **internal/hostapd** | Lógica Hostapd: APs/clients/configuración/diagnósticos y operaciones sobre hostapd. |

## Uso desde `package main`

- **Configuración:** `config.Load()`, `config.Normalize()`, `config.AppConfig`.
- **Constantes:** `constants.DefaultWiFiInterface`, `constants.DefaultCountryCode`, etc.
- **Modelos:** `models.User`, `models.Claims`, `models.SystemLog`, etc.
- **Validadores:** `validators.ValidateUsername()`, `validators.ValidatePassword()`, etc.
- **Métricas:** `metrics.Add2xx()`, `metrics.Load2xx()`, etc. (usado por middleware y health).
- **i18n:** `i18n.Init()`, `i18n.T()`, `i18n.LanguageMiddleware`, `i18n.LogTf()`, etc.
- **Base de datos:** `database.Init()`, `database.DB`, `database.InsertLog()`, `database.GetLogs()`, etc.
- **Auth:** `auth.Login()`, `auth.ValidateToken()`, `auth.GenerateToken()`, `auth.RegisterBootstrap()`, etc.

## Raíz del módulo

En el paquete `main` permanecen: `main.go`, handlers (`handlers.go`, `api_*.go`, …), `utils.go` (wrappers) y el resto de archivos que orquestan la app y usan los paquetes internos. (Los middlewares viven en `internal/middleware`; la lógica WiFi en `internal/wifi`, VPN en `internal/vpn`, Tor en `internal/tor`, AdBlock en `internal/adblock`, red en `internal/network`, sistema en `internal/system` y hostapd en `internal/hostapd`.)

Se eliminó `api_compat.go` (estaba vacío; la compatibilidad se cubre en otros módulos).

## Compilar y tests

```bash
go build -o hostberry .
go test ./...
```
