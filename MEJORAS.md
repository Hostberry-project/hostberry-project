# Cómo mejorar HostBerry

Recomendaciones priorizadas para arquitectura, seguridad, mantenibilidad y experiencia de usuario.

---

## 1. Arquitectura y mantenibilidad

### 1.1 Dividir archivos muy grandes

- **`api_compat.go` (~3400 líneas)**  
  Contiene lógica de WiFi, VPN, sistema, Tor, etc. Conviene:
  - Extraer por dominio en archivos como `api_wifi.go`, `api_system.go`, `api_vpn.go`, manteniendo el mismo `package main`.
  - O agrupar handlers en tipos (structs) por módulo para que el código sea más navegable.

- **`wifi_handlers.go` (~1600 líneas)**  
  - Sacar helpers a un `wifi_helpers.go` (detección de interfaz, generación de config, parseo de `wpa_cli`).
  - Dejar en `wifi_handlers.go` solo los handlers que llaman a la API (Fiber).

### 1.2 Reducir duplicación

- Hay lógica duplicada entre **handlers en `handlers.go`** y **`api_compat.go`** (por ejemplo estado de OpenVPN, servicios).
- Opciones:
  - Unificar en un solo conjunto de rutas y deprecar la API “compat”.
  - O extraer funciones compartidas (p. ej. `getOpenVPNStatus()`) y llamarlas desde ambos sitios.

### 1.3 Configuración y 12-factor

- **Variables de entorno** para producción:
  - `HOSTBERRY_CONFIG` (ruta del YAML).
  - `HOSTBERRY_JWT_SECRET` (prioritario sobre YAML en producción).
  - `HOSTBERRY_DB_PATH` para SQLite.
- Así se evita tener secretos en el YAML y se facilita despliegue en contenedores/cloud.
- El `config.yaml.example` incluye una sección `logging` que no existe en la struct `Config` de `main.go`: o se implementa o se quita del ejemplo.

---

## 2. Seguridad

### 2.1 Endpoints WiFi públicos (wizard)

- `/api/v1/wifi/status`, `scan`, `connect`, `disconnect` son públicos para el wizard.
- Mejoras:
  - **Rate limit por IP** solo para estos endpoints (más estricto que el global), para evitar abuso.
  - Opcional: permitir estos endpoints solo si “first-login” no está completado (flag en BD o en config), y después exigir auth.

### 2.2 JWT y CORS

- Asegurar que **JWT secret** no sea el valor por defecto en producción (validar al arrancar si `debug=false`).
- En producción, **CORS** ya limita orígenes; mantener solo los necesarios y no usar `*` cuando la app tenga front en otro dominio.

### 2.3 Ejecución de comandos

- En `utils.go` ya existe una lista blanca de comandos; seguir evitando construir comandos con entrada de usuario sin sanitizar (SSID, nombres de interfaz, etc. ya se validan en `validators.go`; mantener este patrón en todos los sitios).

---

## 3. Testing

- Hoy solo existe **`templates_test.go`** (carga de plantillas).
- Añadir tests para:
  - **Auth**: login, cambio de contraseña, first-login, expiración de token.
  - **Validators**: `ValidateUsername`, `ValidatePassword`, `ValidateSSID`, `ValidateWireGuardConfig`.
  - **WiFi**: mock de `exec` o tests de integración con interfaz fake para `connectWiFi` (config generada, no ejecución real).
  - **Health**: que `/health` y `/health/ready` devuelvan los códigos esperados cuando la BD está bien o mal.
- Tests de handlers con Fiber: usar `fiber.New()` y `app.Test(req)` para GET/POST y comprobar status y cuerpo JSON.

---

## 4. Operaciones y resiliencia

### 4.1 Health check

- `/health` ya comprueba BD e i18n; está bien.
- Opcional: en `/health/ready`, comprobar que exista al menos un usuario admin activo (para indicar “sistema listo para uso”).
- Documentar que un balanceador o Kubernetes usen `/health/ready` para readiness y `/health/live` para liveness.

### 4.2 Logs

- Unificar uso de **LogTf / LogT** frente a **log.Printf** para que todo pase por el mismo sistema (y por i18n si aplica).
- Para producción: valorar **logs estructurados** (JSON con nivel, mensaje, request_id, usuario) para integrar con agregadores (Loki, Elastic, etc.).

### 4.3 Timeouts

- Los `exec` ya usan `executeCommandWithTimeout` en muchos sitios; revisar que **todas** las llamadas a comandos externos (wpa_supplicant, wpa_cli, nmcli, etc.) tengan timeout para evitar bloqueos.

---

## 5. Experiencia de usuario (UX)

### 5.1 Wizard y WiFi

- Ya se muestran mensajes de error más claros al fallar `wpa_supplicant`; mantener mensajes traducidos (i18n) para todos los errores de conexión WiFi.
- En el wizard, si la conexión tarda: **indicador de progreso** (“Conectando… no cierres la página”) y deshabilitar el botón hasta terminar o timeout.

### 5.2 Consistencia de la API

- Estandarizar respuestas JSON:
  - Éxito: `{"success": true, "data": ...}` o `{"success": true}`.
  - Error: `{"success": false, "error": "mensaje"}` o `{"error": "mensaje"}` y usar el mismo formato en todos los endpoints para que el frontend trate errores de forma uniforme.

### 5.3 Primera ejecución

- Si no existe `config.yaml`, crear uno por defecto o guiar con un mensaje claro (“Copia config.yaml.example a config.yaml y ajusta la configuración”).

---

## 6. Documentación

- **README.md** con:
  - Descripción del proyecto y características.
  - Requisitos (Go, wpa_supplicant, hostapd, etc.).
  - Instalación (script y/o manual).
  - Configuración (YAML y variables de entorno).
  - Cómo ejecutar (desarrollo y producción).
  - Estructura básica del repositorio (backend, frontend, install).
- **Comentarios en el código**: en handlers públicos (Fiber) y en funciones que ejecutan comandos del sistema, una línea describiendo qué hace y qué parámetros esperan.
- Opcional: **OpenAPI/Swagger** para la API REST (documentación y pruebas desde el navegador).

---

## 7. Priorización sugerida

| Prioridad | Mejora | Esfuerzo | Impacto |
|-----------|--------|----------|---------|
| Alta | Devolver y mostrar el error real de wpa_supplicant (ya hecho) | Bajo | Alto |
| Alta | Configuración por variables de entorno (JWT, DB) | Bajo | Seguridad y despliegue |
| Alta | README con instalación y configuración | Medio | Onboarding |
| Media | Rate limit en endpoints WiFi públicos | Bajo | Seguridad |
| Media | Tests para auth y validators | Medio | Estabilidad |
| Media | Dividir api_compat.go por dominio | Alto | Mantenibilidad |
| Baja | Logs estructurados (JSON) | Medio | Operaciones |
| Baja | OpenAPI/Swagger | Medio | Documentación y DX |

---

## 8. Resumen

- **Código**: modularizar archivos muy grandes, reducir duplicación entre handlers y api_compat, tiempo de espera en todas las llamadas a comandos.
- **Seguridad**: config por env, proteger/limitarlos endpoints WiFi públicos, no usar JWT por defecto en producción.
- **Calidad**: más tests (auth, validators, health, WiFi con mocks).
- **Operaciones**: health listo para Kubernetes/balanceadores, logs consistentes y opcionalmente estructurados.
- **Documentación**: README claro y, si se puede, OpenAPI para la API.

Si indicas por qué parte quieres empezar (config, tests, dividir api_compat, etc.), se puede bajar al nivel de cambios concretos en archivos y funciones.
