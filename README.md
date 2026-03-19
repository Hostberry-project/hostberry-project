# HostBerry

Sistema de gestión de red para Raspberry Pi y equipos Linux: WiFi, punto de acceso (HostAPD), VPN, WireGuard, Tor, AdBlock y monitorización.

## Instalación

```bash
sudo ./install.sh
```

- **Actualizar**: `sudo ./install.sh --update` (preserva datos y configuración).
- **Desinstalar**: `sudo ./install.sh --uninstall`.

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
