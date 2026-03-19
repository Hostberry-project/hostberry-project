# Estructura modular del proyecto

El proyecto está organizado con paquetes internos bajo `internal/` para separar configuración, modelos, constantes y validadores del paquete principal.

## Paquetes `internal/`

| Paquete | Descripción |
|---------|-------------|
| **internal/config** | Tipos `Config`, `ServerConfig`, `DatabaseConfig`, `SecurityConfig`. `Load()` lee `config.yaml` y asigna `AppConfig`. `Normalize()` endurece JWT y bcrypt. `GenerateRandomSecret()` para secretos. |
| **internal/constants** | Constantes globales: `DefaultWiFiInterface`, `DefaultCountryCode`, `DefaultServerHost`, `DefaultServerPort`, `DefaultUnknownValue`. |
| **internal/models** | Modelos de dominio y BD: `User`, `Claims`, `LoginError`, `SystemLog`, `SystemConfig`, `SystemStatistic`, `NetworkConfig`, `VPNConfig`, `WireGuardConfig`, `AdBlockConfig`. |
| **internal/validators** | Funciones de validación: `ValidateUsername`, `ValidatePassword`, `ValidateEmail`, `ValidateIP`, `ValidateSSID`, `ValidateWireGuardConfig`, `ValidateVPNConfig`. |

## Uso desde `package main`

- **Configuración:** `config.Load()`, `config.Normalize(LogTf)`, `config.AppConfig`, `config.GenerateRandomSecret()`.
- **Constantes:** `constants.DefaultWiFiInterface`, `constants.DefaultCountryCode`, etc.
- **Modelos:** `models.User`, `models.Claims`, `models.SystemLog`, etc.
- **Validadores:** `validators.ValidateUsername()`, `validators.ValidatePassword()`, etc.

## Raíz del módulo

El resto del código sigue en el paquete `main` (handlers, middleware, database, auth, wifi, templates, health, utils, etc.) y consume los paquetes internos mediante imports `hostberry/internal/...`.

Para compilar y ejecutar tests:

```bash
go build -o hostberry .
go test ./...
```
