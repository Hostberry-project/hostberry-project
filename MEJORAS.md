## MEJORAS PROPUESTAS Y PROGRESO

Este documento recoge un resumen de mejoras arquitectónicas, de seguridad, operativas, UX y observabilidad para HostBerry, junto con su estado.

### 1. Arquitectura y mantenibilidad

- **1.1 Dividir archivos muy grandes (COMPLETADO)**
  - `api_compat.go` dividido en: `api_system.go`, `api_network.go`, `api_wifi.go`, `api_vpn.go`, `api_hostapd.go`, `api_misc.go`.
  - `api_compat.go` ahora solo contiene un comentario y `package main` para evitar duplicados.

- **1.2 Modularizar WiFi (COMPLETADO)**
  - `wifi_helpers.go`: helpers de bajo nivel (`WpaSupplicantConfigDir`, `WpaSocketDirs`, `getRunDir`, `ensureWpaSupplicantDirs`, `startWpaSupplicant`, etc.).
  - `wifi_handlers.go`: lógica de “negocio” WiFi (`scanWiFiNetworks`, `toggleWiFi`, `connectWiFi`, `autoConnectToLastNetwork`), con soporte WPA3 y gestión robusta de `wpa_supplicant`.

### 2. Seguridad

- **2.1 JWT y contraseñas seguras (COMPLETADO)**
  - `config.yaml.example`: añade `security.jwt_secret` con texto de advertencia.
  - `install.sh`:
    - Genera `GENERATED_JWT_SECRET` aleatorio (64 caracteres) en la primera instalación y lo escribe en `config.yaml` si detecta el placeholder por defecto.
    - Genera `GENERATED_ADMIN_PASSWORD` aleatorio (16 caracteres) y lo pasa al servicio systemd vía `Environment=HOSTBERRY_DEFAULT_ADMIN_PASSWORD=...`.
    - Crea `INSTALL_CREDENTIALS.txt` en `/opt/hostberry` (600, dueño `hostberry:hostberry`) con:
      - Usuario admin.
      - Contraseña inicial.
      - JWT secret actual.
  - `utils.go:createDefaultAdmin`:
    - Usa `HOSTBERRY_DEFAULT_ADMIN_PASSWORD` si está definido.
    - Si no, crea admin con `admin/admin`, pero el log ya **no imprime la contraseña** (solo indica que es la contraseña por defecto y que debe cambiarse).

- **2.2 Endurecer configuración de seguridad en runtime (COMPLETADO)**
  - `main.go:loadConfig` + bloque posterior:
    - Si `security.jwt_secret` está vacío, genera un nuevo secreto aleatorio en memoria y lo registra en logs (sin exponerlo).
    - Normaliza `security.bcrypt_cost` a un rango seguro (4–15). Fuera de rango ⇒ se fija en `10`.

- **2.3 Cookies de sesión más seguras (COMPLETADO)**
  - `handlers.go` (`loginAPIHandler` y `firstLoginChangeAPIHandler`):
    - La cookie `access_token` ahora se marca con `Secure=true` cuando:
      - La conexión es HTTPS (`c.Secure()`) o
      - El reverse proxy envía `X-Forwarded-Proto: https`.
    - Mantiene `HTTPOnly` y `SameSite=Lax`.

### 3. WiFi y red

- **3.1 Mejoras en el wizard WiFi (COMPLETADO)**
  - `setup_wizard.js`:
    - Muestra tipo de seguridad por red: `WPA3`, `WPA2` o `Abierta`, usando `net.security`.
    - Redes abiertas: ocultan el campo de contraseña automáticamente.
    - Botón “Continuar (mantener conexión)”:
      - Comprueba `/api/v1/wifi/status` y solo avanza al paso 2 si hay conexión activa (Ethernet o WiFi).
      - Si no hay red, muestra aviso en ES/EN.
    - Refresco periódico:
      - Mientras estás en el paso 1, cada 10s re-llama a `fetchWifiStatus()` para mantener actualizado el banner y el estado.

- **3.2 Backend WiFi más robusto (COMPLETADO)**
  - `wifi_handlers.go:connectWiFi`:
    - Asegura que la interfaz WiFi está desbloqueada y levantada (`rfkill unblock wifi`, `ip link set <iface> up`) antes de iniciar `wpa_supplicant`.
    - Soporte WPA3:
      - Usa `scanWiFiNetworks` para detectar `security=WPA3` y genera bloque `wpa_supplicant` con `key_mgmt=SAE` y `sae_password`.
    - Tiempo de espera con diagnóstico:
      - Tras esperar `wpa_state=COMPLETED`, si no conecta:
        - Si `wpa_cli status` contiene `AUTH_FAILED`/`WRONG_KEY`, devuelve mensaje claro de contraseña incorrecta.
        - Si contiene `4WAY_HANDSHAKE`, devuelve error indicando problema de autenticación WPA/WPA3.
        - En otros casos, timeout con mensaje mencionando contraseña y cobertura.

- **3.3 Instalación segura cuando hay WiFi / SSH (COMPLETADO Y MEJORADO)**
  - `install.sh`:
    - Variable `RUNNING_OVER_SSH` ya controla:
      - No crear/activar `ap0` en caliente.
      - No lanzar `udevadm trigger` ni `systemctl unmask/start hostapd`/`hostberry-captive-portal` cuando puede cortar SSH.
    - Nueva función `is_default_route_over_wifi()`:
      - Comprueba si la ruta por defecto usa una interfaz `wl*`/`wlan*`.
    - Lógica de reinicio (`NEED_REBOOT_FOR_AP0`):
      - En modo `install`, solo activa reinicio automático si la ruta por defecto **no** es WiFi.
      - Si detecta ruta por defecto sobre WiFi:
        - No reinicia automáticamente.
        - Muestra aviso claro en la salida final explicando que no se ha reiniciado para no cortar la conexión y que puede reiniciarse manualmente (idealmente por cable).

### 4. Observabilidad y métricas

- **4.1 Health checks (YA EXISTENTE, REVISADO)**
  - `GET /health`: estado general (incluye DB e i18n).
  - `GET /health/ready`: readiness (centrado en la base de datos).
  - `GET /health/live`: liveness simple.

- **4.2 Endpoint de métricas (NUEVO, COMPLETADO)**
  - `GET /metrics`:
    - Implementado en `health.go:metricsHandler` y registrado en `main.go:setupRoutes`.
    - Devuelve texto plano en formato tipo Prometheus, sin información sensible:
      - **Estado general y build**:
        - `hostberry_up 1`
        - `hostberry_build_info{version="2.0.0",go_version="go1.x"} 1`
        - `hostberry_unix_time_seconds <timestamp>`
      - **Uso de recursos**:
        - `hostberry_mem_bytes <bytes>`
        - `hostberry_goroutines <n>`
      - **Tráfico HTTP por clase de estado**:
        - `hostberry_http_requests_total{code_class="2xx"} <n>`
        - `hostberry_http_requests_total{code_class="4xx"} <n>`
        - `hostberry_http_requests_total{code_class="5xx"} <n>`
      - **Estado de servicios de red**:
        - `hostberry_service_up{service="hostapd"} 0|1` (via `systemctl is-active hostapd`)
        - `hostberry_service_up{service="dnsmasq"} 0|1`
        - `hostberry_wifi_interface_up{interface="wlan0"} 0|1` (estado UP/DOWN de la interfaz WiFi principal)

### 5. Próximas mejoras sugeridas (pendientes)

Estas son ideas alineadas con el espíritu de `MEJORAS.md` que aún pueden implementarse:

- **5.1 Seguridad / roles**
  - Añadir control de permisos por rol (`admin`, `user`, etc.) en endpoints sensibles: configuración de hostapd, VPN, Tor, actualización del sistema.
  - Registrar de forma más estructurada los intentos fallidos de login y accesos a endpoints críticos.

- **5.2 UX / setup**
  - Añadir una sección en el panel que muestre claramente:
    - Si el servidor está en HTTP o HTTPS.
    - Un asistente para configurar certificados (auto-firmado vs. Let’s Encrypt detrás de proxy).

- **5.3 Testing**
  - Añadir tests básicos (Go) para:
    - `connectWiFi` (mockeando comandos).
    - Rutas auth (`/api/v1/auth/login`, `/api/v1/auth/me`).
    - Health/metrics.

- **5.4 Documentación**
  - Documentar en `README.md`:
    - Nuevo comportamiento de `install.sh` (JWT, password admin aleatoria, reinicio condicional).
    - Uso de `/metrics` y ejemplos de configuración en Prometheus.

