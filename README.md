# HostBerry

Sistema de gestión de red para Raspberry Pi y equipos Linux: WiFi, punto de acceso (HostAPD), VPN, WireGuard, Tor, AdBlock y monitorización.

## Requisitos

- **Sistema operativo**:  
  - Distribuciones basadas en Debian/Ubuntu (incluida Raspberry Pi OS).  
  - Kernel con soporte para `hostapd`, `dnsmasq` y `wpa_supplicant`.
- **Hardware recomendado**:  
  - Raspberry Pi 3 o superior, o equipo x86_64 con al menos 1 GB de RAM.  
  - Interfaz WiFi compatible con modo AP (punto de acceso) si quieres usar HostBerry como hotspot.
- **Dependencias principales** (el `install.sh` intenta instalarlas automáticamente):  
  - `golang-go` (Go), `git`, `hostapd`, `dnsmasq`, `iptables`, `iproute2`, `iw`, `wpa_supplicant`, `wpa_cli`, `curl`.  
  - Opcional: `golangci-lint`, `air` (hot reload) para desarrollo.

Para desarrollo local (sin `install.sh`), asegúrate de tener Go instalado (verifica con `go version`).

## Instalación

```bash
sudo ./install.sh
```

- **Actualizar**: `sudo ./install.sh --update` (preserva datos y configuración).
- **Desinstalar**: `sudo ./install.sh --uninstall`.

Si prefieres compilar manualmente:

```bash
make deps
make build        # binario para tu máquina
make build-arm    # binario para Raspberry Pi 3
```

### Seguridad en la instalación

En la **primera instalación** el script:

- **JWT**: Genera un `jwt_secret` aleatorio (64 caracteres) y lo escribe en `config.yaml` si detecta el valor por defecto. Así las sesiones no usan un secreto predecible.
- **Usuario admin**: Genera una contraseña aleatoria para el usuario `admin` y la inyecta en el servicio systemd (`HOSTBERRY_DEFAULT_ADMIN_PASSWORD`). No se usa `admin/admin` salvo que no se haya podido generar la variable.
- **Credenciales guardadas**: Crea el archivo `/opt/hostberry/INSTALL_CREDENTIALS.txt` (permisos `600`, dueño `hostberry:hostberry`) con:
  - Usuario admin
  - Contraseña inicial
  - JWT secret actual  
  **Guarda este archivo en un lugar seguro** y cambia la contraseña desde el panel tras el primer acceso.

Al finalizar, el script muestra la URL del panel y el mensaje de login inicial (usuario y contraseña generada, o aviso de guardar `INSTALL_CREDENTIALS.txt`).

## HTTPS

Puedes servir el panel por HTTPS de dos formas:

### 1. TLS integrado (HostBerry con certificados)

En `config.yaml` (sección `server`), configura:

```yaml
tls_cert_file: "/opt/hostberry/certs/fullchain.pem"
tls_key_file: "/opt/hostberry/certs/privkey.pem"
```

Si ambos archivos existen, el servidor arranca en HTTPS en el mismo puerto (p. ej. `https://IP:8000`). Puedes usar certificados auto-firmados o Let's Encrypt (copiando los archivos al directorio indicado).

### 2. Proxy inverso (Nginx, Traefik, etc.)

Deja HostBerry en HTTP (sin `tls_cert_file`/`tls_key_file`) y colócalo detrás de Nginx o Traefik con TLS. Asegúrate de que el proxy envíe `X-Forwarded-Proto: https` para que las cookies de sesión se marquen como `Secure` correctamente.

En el panel, la página **System** incluye una tarjeta "Estado HTTPS" que indica si la conexión actual es HTTP o HTTPS y da una guía breve para activar TLS.

## Wizard WiFi y red

- El **asistente de configuración** (setup wizard) permite conectar el equipo a tu WiFi (WPA2/WPA3 o red abierta), configurar el punto de acceso HostBerry y elegir VPN/WireGuard/Tor.
- Si accedes al panel por WiFi, al cambiar de red la sesión puede cortarse; se recomienda usar cable Ethernet para el paso de conexión WiFi en el wizard.
- El backend soporta **WPA3** (SAE) además de WPA2; los mensajes de error diferencian contraseña incorrecta de otros fallos (cobertura, autenticación).

## Roles y permisos

- Los usuarios se crean con rol `admin` por defecto. Las rutas sensibles (HostAPD, VPN, Tor, firewall, actualizaciones, reinicio/apagado, configuración WiFi avanzada, etc.) exigen **rol admin**.
- Si un usuario no es admin:
  - Las peticiones a esas rutas devuelven **403** ("Permisos insuficientes").
  - En la interfaz se ocultan las acciones de reinicio y apagado en el menú de usuario.
- Los intentos de acceso denegado a rutas solo-admin se registran en los logs del sistema.

## Métricas y monitorización

- **`GET /metrics`** (público, texto tipo Prometheus):  
  `hostberry_up`, memoria, goroutines, contadores HTTP (2xx/4xx/5xx), estado de servicios `hostapd`/`dnsmasq` y estado de la interfaz WiFi principal.
- **`GET /api/v1/system/metrics`** (requiere login):  
  Mismo contenido en JSON para el panel.
- En la página **Monitoring** del panel se muestran las peticiones HTTP por clase (2xx/4xx/5xx) y el estado de hostapd, dnsmasq y WiFi.

## Configuración de ejemplo

Copia `config.yaml.example` a `config.yaml` (o deja que la instalación lo haga) y ajusta `server`, `database` y `security` según tu entorno.

### Campos principales de `config.yaml`

```yaml
server:
  host: "0.0.0.0"  # Escuchar en todas las interfaces (permite acceso por IP de red)
  port: 8000
  debug: false
  read_timeout: 30
  write_timeout: 30
  #tls_cert_file: "/opt/hostberry/certs/fullchain.pem"
  #tls_key_file: "/opt/hostberry/certs/privkey.pem"

database:
  type: "sqlite"
  path: "data/hostberry.db"

security:
  jwt_secret: "cambiar-este-secreto-en-produccion-usar-secretos-seguros"
  token_expiry: 60
  bcrypt_cost: 10
  rate_limit_rps: 10
```

- **server.host**: normalmente `"0.0.0.0"` para acceder por IP desde tu red local.  
- **server.port**: puerto HTTP/HTTPS del panel (por defecto `8000`).  
- **database.type**: `sqlite` (por defecto) o `postgres`/`mysql` si quieres base de datos externa.  
- **security.jwt_secret**: el instalador lo reemplaza por un valor aleatorio seguro en la primera instalación. No uses el valor de ejemplo en producción.  
- **security.bcrypt_cost**: coste de hashing de contraseñas (entre 4 y 15, por defecto 10).

### Ejemplo mínimo para producción con HTTPS integrado

```yaml
server:
  host: "0.0.0.0"
  port: 8000
  debug: false
  tls_cert_file: "/opt/hostberry/certs/fullchain.pem"
  tls_key_file: "/opt/hostberry/certs/privkey.pem"

database:
  type: "sqlite"
  path: "data/hostberry.db"

security:
  # Este valor será sobrescrito automáticamente en la primera instalación.
  jwt_secret: "no-importa-valor-inicial"
  token_expiry: 60
  bcrypt_cost: 10
```

## Estructura del proyecto (resumen)

Algunos directorios y archivos relevantes:

- `main.go`, `handlers.go`, `middleware.go`, `health.go`  
  Lógica principal del servidor, rutas HTTP, autenticación, salud y métricas.
- `api_system.go`, `api_network.go`, `api_wifi.go`, `api_vpn.go`, `api_hostapd.go`, `api_misc.go`  
  Handlers de API agrupados por dominio funcional (sistema, red, WiFi, VPN, HostAPD, varios).
- `wifi_handlers.go`, `wifi_helpers.go`  
  Gestión de WiFi: escaneo de redes, conexión/desconexión, soporte WPA2/WPA3, integración con `wpa_supplicant`.
- `website/templates/`  
  Plantillas HTML del panel (dashboard, wizard, páginas de sistema, VPN, Tor, etc.).
- `website/static/js/`  
  JavaScript del frontend, incluido el **setup wizard**, página de WiFi, monitoring, system, etc.
- `website/static/css/`  
  Estilos principales (`hostberry.css`, `dashboard.css`, `custom.css`) y fuentes de iconos locales.
- `install.sh`  
  Script de instalación/actualización/desinstalación, creación de usuario, servicios systemd, configuración inicial de WiFi/AP, generación de secretos.
- `MEJORAS.md`  
  Documento con roadmap y mejoras propuestas/implementadas.
- `auth_test.go`, `health_test.go`  
  Tests básicos de autenticación y endpoints de salud/métricas.

Para una exploración más profunda del código y su arquitectura, consulta también `MEJORAS.md`, donde se documentan decisiones y tareas realizadas.
