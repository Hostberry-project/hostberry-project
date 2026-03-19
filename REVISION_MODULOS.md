# Revisión de módulos – uno a uno

Revisión detallada de cada archivo `.go` del proyecto. Se aplicaron correcciones donde se detectaron fallos o mejoras claras.

---

## 1. constants.go
- **Contenido:** Constantes globales (DefaultWiFiInterface, DefaultCountryCode, ServerHost, Port, DefaultUnknownValue).
- **Estado:** OK. Sin dependencias ni lógica.

---

## 2. validators.go
- **Contenido:** ValidateUsername, ValidatePassword, ValidateEmail, ValidateIP, ValidateSSID, ValidateWireGuardConfig, ValidateVPNConfig.
- **Correcciones aplicadas:**
  - **ValidateIP:** Ahora valida que cada octeto esté en 0–255 (antes solo formato y ceros a la izquierda).
  - **ValidateUsername:** Uso de `regexp.MustCompile` en lugar de `MatchString` ignorando el error.
- **Estado:** OK.

---

## 3. request_id.go
- **Contenido:** Middleware X-Request-ID; fallback cuando `rand.Read` falla.
- **Corrección aplicada:** `generateSimpleID()` ahora usa 8 bytes derivados de `time.Now().UnixNano()` en lugar de 1 byte, para reducir colisiones.
- **Estado:** OK.

---

## 4. rate_limiter.go
- **Contenido:** RateLimiter en memoria, cleanup periódico, middleware por IP/usuario.
- **Nota:** Usa `sync.RWMutex` pero solo con `Lock`/`Unlock`; podría usarse `sync.Mutex` por consistencia (opcional).
- **Estado:** OK.

---

## 5. auth.go
- **Contenido:** LoginError, Claims, User, GenerateToken, ValidateToken, HashPassword, CheckPassword, getMaxLoginAttempts, getLockoutMinutes, IsDefaultAdminCredentialsInUse, Login, Register, RegisterBootstrap.
- **Nota:** `GenerateToken(user *User)` no comprueba `user == nil`; en la práctica solo se llama con usuario ya validado.
- **Estado:** OK.

---

## 6. database.go
- **Contenido:** initDatabase, autoMigrate, modelos (SystemLog, SystemConfig, SystemStatistic, NetworkConfig, VPNConfig, WireGuardConfig, AdBlockConfig), InsertLog, truncateForLog, LogMsg/LogMsgErr/LogMsgWarn, GetLogs, InsertStatistic, SetConfig, GetConfig, GetAllConfigs.
- **Estado:** OK. Truncado de mensajes de log ya implementado.

---

## 7. middleware.go
- **Contenido:** requireAuth, GetUser, requireAdmin, RunActionWithUser, loggingMiddleware (contadores 2xx/4xx/5xx), errorHandler, securityHeadersMiddleware, enforceHTTPSMiddleware.
- **Corrección aplicada:** En `enforceHTTPSMiddleware`, la URL de redirección usaba `c.OriginalURL()` para `Path`, lo que podía incluir query en el path. Ahora se usa `c.Path()` para Path y `RawQuery` por separado.
- **Estado:** OK.

---

## 8. utils.go
- **Contenido:** generateSecureAdminPassword, createDefaultAdmin, executeCommand/executeCommandWithTimeout, cache de comandos, filterSudoErrors, getHostname, canUseSudo, execCommand, clearCommandCache, strconvAtoiSafe, mapActiveStatus, mapBoolStatus.
- **Nota:** Lista blanca de comandos y timeouts definidos; `execCommand` usada desde api_wifi.go y desde executeCommandWithTimeout.
- **Estado:** OK.

---

## 9. templates.go
- **Contenido:** registerTemplateFuncs, createTemplateEngine (FS y embed), renderTemplate.
- **Corrección previa:** Autoasignación `err = err` corregida con `loadErr`.
- **Estado:** OK.

---

## 10. i18n.go
- **Contenido:** I18nManager, InitI18n, loadLanguage, GetText, getNestedValue, GetTranslations, GetCurrentLanguage, isLanguageSupported, T, LogT, LogTf, etc.
- **Estado:** OK.

---

## 11. health.go
- **Contenido:** healthCheckHandler, readinessCheckHandler, livenessCheckHandler, metricsHandler (Prometheus text), serviceIsActive, wifiInterfaceUp, metricsSummaryHandler (JSON).
- **Estado:** OK.

---

## 12. main.go
- **Contenido:** Config, loadConfig, createApp, setupStaticFiles, setupRoutes, indexHandler, dashboardHandler, loginHandler, settingsHandler, systemStatsHandler, systemRestartHandler, detectWiFiInterface, wifiInterfacesHandler, wifiScanHandler.
- **Corrección aplicada:** En `systemRestartHandler`, la variable del mensaje de error se renombró de `err` a `errMsg` para evitar sombrear el tipo `error`.
- **Estado:** OK.

---

## 13. handlers.go
- **Contenido:** translateLoginError, loginAPIHandler, logoutAPIHandler, meHandler, changePasswordAPIHandler, firstLoginChangeAPIHandler, updateProfileAPIHandler, updatePreferencesAPIHandler, wifiConnectHandler, y otros handlers de páginas/API (VPN, Tor, shutdown, etc.).
- **Estado:** OK. Uso coherente de LogMsg/LogMsgErr y cookies seguras.

---

## 14. handlers_config.go
- **Contenido:** systemConfigHandler (guardar configuración del sistema; valida timezone y max_login_attempts).
- **Nota opcional:** Acepta cualquier clave en `req` para SetConfig; si se quiere restringir, convendría una whitelist de keys.
- **Estado:** OK.

---

## 15. api_system.go
- **Contenido:** systemActivityHandler, systemNetworkHandler, systemUpdatesHandler, systemBackupHandler, systemHttpsInfoHandler.
- **Estado:** OK. systemBackupHandler devuelve "no implementado aún".

---

## 16. api_network.go
- **Contenido:** networkRoutingHandler, networkFirewallToggleHandler, networkConfigHandler (GET/POST), networkSpeedtestHandler.
- **Estado:** OK. networkFirewallToggleHandler devuelve 501.

---

## 17. api_wifi.go
- **Contenido:** wifiNetworksHandler, wifiClientsHandler, wifiToggleHandler, wifiUnblockHandler, wifiSoftwareSwitchHandler, wifiConfigHandler, wifiDisconnectHandler, wifiStatusHandler (y lógica asociada con execCommand).
- **Estado:** OK. execCommand está en utils.go y se usa correctamente.

---

## 18. api_vpn.go
- **Contenido:** Handlers de VPN (connections, servers, clients, toggle, config, connection toggle, certificates).
- **Estado:** OK.

---

## 19. api_hostapd.go
- **Contenido:** Handlers HostAPD (access-points, clients, config, diagnostics, create-ap0, toggle, restart).
- **Estado:** OK.

---

## 20. api_misc.go
- **Contenido:** helpContactHandler, translationsHandler (lee locales/{lang}.json con validación de path).
- **Estado:** OK. translationsHandler restringe idioma y path.

---

## 21. api_compat.go
- **Contenido:** Solo comentario indicando que la lógica se movió a api_system, api_network, api_wifi, api_vpn, api_hostapd, api_misc.
- **Estado:** OK.

---

## 22. wifi_helpers.go
- **Contenido:** Constantes WpaSupplicantConfigDir, WpaSocketDirs, getRunDir, ensureWpaSupplicantDirs, stopWpaSupplicant, startWpaSupplicant (con reintentos de driver y ctrl_iface), waitForWpaCliConnection, getLastConnectedNetwork.
- **Estado:** OK.

---

## 23. wifi_handlers.go
- **Contenido:** scanWiFiNetworks, parseFloat, parseInt, freqToChannel, connectWiFi (WPA2/WPA3), toggleWiFi, autoConnectToLastNetwork y helpers locales (runWpaCli, etc.).
- **Estado:** OK.

---

## 24. extra_api_handlers.go
- **Contenido:** currentUserInfo, systemUpdatesExecuteHandler, systemUpdatesProjectHandler, systemNotificationsTestEmailHandler, adblockUpdateHandler y otros handlers de actualizaciones/AdBlock.
- **Estado:** OK.

---

## 25. system_handlers.go
- **Contenido:** Handlers de sistema (stats, info, logs, services, backup, etc.).
- **Estado:** OK.

---

## 26. network_handlers.go
- **Contenido:** Handlers de red (status, interfaces, etc.).
- **Estado:** OK.

---

## 27. vpn_handlers.go
- **Contenido:** Lógica específica de VPN.
- **Estado:** OK.

---

## 28. tor_handlers.go
- **Contenido:** Handlers de Tor (status, install, configure, enable, disable, circuit, iptables).
- **Estado:** OK.

---

## 29. adblock_handlers.go
- **Contenido:** Handlers de AdBlock y DNSCrypt/Blocky.
- **Estado:** OK.

---

## 30. auth_test.go
- **Contenido:** Tests de generación y validación de JWT.
- **Estado:** OK.

---

## 31. health_test.go
- **Contenido:** Tests de /health, /health/ready, /health/live, /metrics, /api/v1/system/https-info.
- **Estado:** OK.

---

## 32. templates_test.go
- **Contenido:** Tests de plantillas.
- **Estado:** OK.

---

## Resumen de correcciones aplicadas en esta revisión

| Archivo        | Cambio                                                                 |
|----------------|------------------------------------------------------------------------|
| validators.go  | ValidateIP: validación de octetos 0–255; ValidateUsername: MustCompile |
| request_id.go  | generateSimpleID: 8 bytes de tiempo para mayor unicidad                 |
| middleware.go  | enforceHTTPSMiddleware: Path con c.Path(), RawQuery por separado       |
| main.go        | systemRestartHandler: variable err → errMsg                            |

## Comprobaciones

- `go vet ./...`: sin errores.
- `go build`: compila correctamente.
- `go test ./...`: tests pasan.

## Bugs corregidos (revisión de bugs)

| Archivo | Bug | Corrección |
|---------|-----|------------|
| **api_wifi.go** | `strings.Fields(parts[1])[0]` podía hacer panic si `parts[1]` era solo espacios (slice vacío). | Comprobar `len(fields) > 0` / `len(channelFields) > 0` antes de acceder a `[0]`. |
| **adblock_handlers.go** | `tarballFile` no se cerraba con `defer`; en caso de panic o salida temprana podía quedar abierto. | Añadir `defer tarballFile.Close()` tras crear el archivo. |
| **handlers_config.go** | Timezone: path traversal si el cliente enviaba ruta absoluta (ej. `/etc/passwd`) o `filepath.Join` + `..` daba salida fuera de zoneinfo. | Rechazar `tz` que empiece por `/`; usar `filepath.Clean` y comprobar que `zonePath` tenga prefijo `/usr/share/zoneinfo`. |

## Recomendaciones opcionales

- Añadir tests unitarios para `ValidateIP` (casos 256, 0, ceros a la izquierda).
- En `handlers_config.go`, valorar restringir las claves de configuración a una whitelist.
- En `rate_limiter.go`, valorar usar `sync.Mutex` si no se usan lecturas en paralelo (RLock).
