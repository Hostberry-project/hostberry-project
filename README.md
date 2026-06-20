# HostBerry

Panel de administración para Raspberry Pi y Linux: WiFi, red, VPN, hostapd, AdBlock, Tor y sistema.

## Requisitos

- **SO:** Debian 12+, Ubuntu 22+, Raspberry Pi OS (64-bit recomendado)
- **Go:** ≥ 1.23 (el instalador lo instala vía `apt`)
- **Hardware:** Raspberry Pi 3/4/5 o PC Linux con WiFi (opcional)
- **Privilegios:** `sudo` para instalación

## Instalación rápida

```bash
git clone https://github.com/Hostberry-project/hostberry-project.git
cd hostberry-project
sudo ./install.sh
```

Para acortar la instalación (omite VPN, Blocky y LibreSpeed; usa binario precompilado si hay release):

```bash
HOSTBERRY_FAST_INSTALL=1 sudo ./install.sh
```

Tras instalar:

1. Accede por HTTPS: `https://hostberry.local` o `https://<IP>` (mkcert en el instalador)
2. Credenciales iniciales en `/opt/hostberry/INSTALL_CREDENTIALS.txt` (se borran tras el primer cambio de contraseña)
3. Cambia la contraseña de `admin` en el primer acceso

## Modos del instalador

| Comando | Descripción |
|---------|-------------|
| `sudo ./install.sh --install` | Instalación completa (también sin argumentos) |
| `sudo ./install.sh --update` | Actualiza binario y archivos (conserva datos) |
| `sudo ./install.sh --remove` | Desinstala el servicio (`--uninstall` sigue funcionando) |

El script principal (`install.sh`, ~10 líneas) carga módulos desde `scripts/install/lib/`. En `--install` y `--update` instala todos los paquetes apt en **un solo paso** (git, golang-go, WiFi, mkcert, avahi, etc.) e intenta un binario precompilado de GitHub Releases (`HOSTBERRY_USE_RELEASE_BINARY=0` para desactivar).

## Variables de entorno

| Variable | Descripción |
|----------|-------------|
| `HOSTBERRY_DEFAULT_ADMIN_PASSWORD` | Contraseña inicial del usuario `admin` (generada por el instalador) |
| `HOSTBERRY_SKIP_MKCERT=1` | No generar certificados TLS con mkcert |
| `HOSTBERRY_REGENERATE_MKCERT=1` | Regenerar certificados aunque existan |
| `HOSTBERRY_SKIP_REBOOT=1` | No reiniciar al final de install/update |
| `HOSTBERRY_SKIP_AP_START=1` | No iniciar el AP en caliente (solo tras reinicio) |
| `HOSTBERRY_START_AP_NOW=1` | Forzar inicio del AP durante la instalación (puede cortar SSH por WiFi) |
| `HOSTBERRY_BUILD_TIMEOUT=1200` | Timeout de compilación en segundos |
| `HOSTBERRY_USE_RELEASE_BINARY=1` | Descargar binario de GitHub Releases en lugar de compilar |
| `HOSTBERRY_FAST_INSTALL=1` | Instalación mínima: omite Tor/OpenVPN/WireGuard, Blocky y LibreSpeed CLI |
| `HOSTBERRY_RELEASE_TAG=v2.1.0` | Tag concreto del release (por defecto: versión del proyecto) |
| `HOSTBERRY_PRIVILEGED_EXEC` | Ruta al wrapper sudo (`/usr/local/sbin/hostberry-safe/privileged-exec`) |
| `HOSTBERRY_INSTALL_LANG=es\|en` | Idioma de mensajes del instalador |

## Configuración

Copia `config.yaml.example` a `config.yaml` (el instalador lo hace en `/opt/hostberry/config.yaml`).

```yaml
server:
  port: 443
  tls_cert_file: "/opt/hostberry/certs/hostberry.pem"
  tls_key_file: "/opt/hostberry/certs/hostberry-key.pem"
security:
  enforce_https: true
  token_expiry: 30
logging:
  level: info
  file: logs/hostberry.log
  max_size: 10
  max_backups: 5
```

## Desarrollo local

```bash
cp config.yaml.example config.yaml
# Sin TLS en desarrollo:
#   enforce_https: false
export HOSTBERRY_DEFAULT_ADMIN_PASSWORD='Hb!DevPass9a'
go run .
```

```bash
go test ./...
go vet ./...
```

## Roles de usuario

| Rol | Permisos |
|-----|----------|
| `admin` | Acceso completo (reinicio, WiFi, firewall, configuración) |
| `operator` | Lectura y monitorización; sin acciones destructivas |

Los administradores pueden asignar roles vía `POST /api/v1/auth/users/role`.

## API

- Base: `/api/v1`
- Especificación OpenAPI: `/api/v1/openapi.yaml`
- Salud: `/health`, `/health/ready`, `/health/live`
- Métricas Prometheus (admin): `/metrics`

## Backup y restauración

- Crear backup: `POST /api/v1/system/backup` (admin)
- Listar: `GET /api/v1/system/backups` (admin)
- Restaurar: `POST /api/v1/system/restore` con `{"file":"nombre.tar.gz"}` (admin)

Los backups se guardan en `/opt/hostberry/backups/`.

## Estructura del proyecto

```
├── main.go
├── install.sh              # Instalador principal
├── scripts/
│   ├── install/lib/        # Módulos del instalador
│   ├── privileged-exec.sh  # Wrapper sudo con allowlist
│   └── validate-i18n.sh    # Valida claves es/en
├── internal/               # Código Go
├── website/                # Templates y estáticos
├── locales/                # Traducciones
└── docs/openapi.yaml       # Especificación API
```

## Solución de problemas

**Compilación en Pi:** SQLite pure Go (sin CGO). El instalador descarga binarios precompilados si hay release publicado. Usa `HOSTBERRY_BUILD_TIMEOUT=1800` si compila desde fuente.

**CSP estricta:** scripts solo desde `/static/js/` (sin `unsafe-inline`); el asistente de configuración usa `data-wizard-step4` en lugar de scripts inline.

**HTTPS no funciona:** Ejecuta `sudo ./install.sh --update` o revisa certificados en `/opt/hostberry/certs/`.

**WiFi no responde:** Comprueba `sudo systemctl status hostberry` y permisos en `/etc/sudoers.d/hostberry`.

**Rollback tras update fallido:** El instalador conserva `hostberry.prev` y restaura si el binario nuevo no arranca.

## Licencia

Ver repositorio del proyecto HostBerry.
