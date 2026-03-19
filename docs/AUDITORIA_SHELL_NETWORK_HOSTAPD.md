# Auditoría: `fmt.Sprintf` + `sh -c` / shell en `network` y `hostapd`

Ámbito: `internal/network/*.go`, `internal/hostapd/hostapd.go`, y el helper `internal/utils.ExecuteCommand` usado por ambos.

## Remediación aplicada (parcial)

- **`validators.ValidateIfaceName`:** nombres de interfaz Linux (≤15 caracteres, charset seguro) antes de interpolar en `sh -c` en `network.go` y `api_network_interfaces_handler.go`.
- **`hostname -I` + `grep`:** solo se construye el comando si la IP pasa `validators.ValidateIP` (IPv4).
- **`nmcli` en `api_network.go`:** DNS y gateway vía `runSudoNmcli` (`internal/network/nmcli_exec.go`) con argumentos separados, **sin** shell ni comillas alrededor del nombre de conexión.
- **Rutas `ip`/`route` con `dev`:** el interfaz devuelto por el sistema se valida con `ValidateIfaceName` antes de `executeCommand`.

Pendiente (volumen alto): refactor similar en **`internal/hostapd/hostapd.go`**.

## Resumen ejecutivo

| Área | Patrón | Riesgo principal |
|------|--------|------------------|
| **`utils.ExecuteCommand`** | Siempre `exec.Command("sh", "-c", cmd)` tras `validateShellCommandAllowList` | Mitiga mucha inyección (`;`, `` ` ``, `$`, saltos de línea). **No** filtra comilla simple `'`; la validación por tokens (`strings.Fields`) rechita comandos cuyo “primer token” tras operadores no esté en allowlist (p. ej. `nc`, `sh`). |
| **`exec.Command("sh", "-c", fmt.Sprintf(...))` directo** | Sin allowlist | Cualquier valor interpolado interpretado por el shell es vector de inyección si es controlable. |
| **Volúmen** | Decenas de `sh -c` + muchos `fmt.Sprintf` → `executeCommand` en **hostapd** | Superficie grande; conviene migrar a `exec.Command(bin, arg1, arg2, …)` donde haya datos dinámicos. |

---

## 1. `internal/utils` (`ExecuteCommand` / `execCommand`)

- **Archivo:** `internal/utils/utils.go` (~L315–327)  
- **Comportamiento:** Construye `sh -c "<cadena>"` (con prefijo `sudo` si aplica).  
- **Validación previa:** `validateShellCommandAllowList` — bloquea `;`, `\n`, `` ` ``, `$`; recorre tokens separados por espacios y operadores `|`, `||`, `&&`; exige que cada comando “base” esté en lista blanca (`nmcli`, `ip`, `cp`, etc.).  
- **Limitaciones:**  
  - Argumentos con **espacios** mal manejados por `strings.Fields` en la validación (p. ej. comillas en cadena única).  
  - Comilla simple **no** está prohibida; el riesgo depende de que el shell final reciba una cadena mal cerrada.  
- **Recomendación:** Para nuevas rutas, **evitar** pasar por `sh -c` cuando los datos sean dinámicos; usar `exec.Command` con argumentos por slice.

---

## 2. `internal/network/api_network.go`

### 2.1 `exec.Command("sh", "-c", …)` **sin** pasar por `ExecuteCommand`

| Líneas (aprox.) | Comando | Datos interpolados | Origen / validación | Notas |
|-----------------|---------|--------------------|---------------------|--------|
| 24 | `ip route` | Ninguno | — | Solo lectura. |
| 89–145 | `hostnamectl`, `ip`, `nmcli`, `resolvectl`, `grep` | Ninguno | — | Lectura configuración. |
| 226 | `sudo cp -f %s %s` | `tmpHostname`, `hostnameFile` | Temp bajo `/tmp/...` + ruta fija `/etc/hostname` | Bajo riesgo. |
| 386 | `sudo cat %s \| sudo tee %s` | `tmpFile`, `hostsFile` | Temp + `/etc/hosts` | Bajo riesgo. |
| 407 | `cat %s > %s` (vía `sudo sh -c`) | Igual | Igual | Bajo riesgo. |
| 497–504 | `nmcli … head -1` + `executeCommand` con `'%s'` | `connName`, `dnsStr` | **connName:** salida `nmcli` en el equipo. **dnsStr:** solo IPs validadas con regex IPv4 | Riesgo bajo desde API; **si** existiera conexión NM con nombre raro (p. ej. apóstrofe), el shell podría romperse; mejor `exec.Command("nmcli", …, connName)`. |
| 542, 575 | `cat > %s` | `resolvedConf`, `resolvConf` | Rutas fijas | Contenido vía `Stdin`; ruta fija. OK. |
| 600–607 | `nmcli` vía `executeCommand` | `connName`, `req.Gateway` | Gateway: regex IPv4 | Mismo comentario que DNS sobre `connName`. |
| 622–661 | `ip route`, `route` vía `executeCommand` | `req.Gateway`, `iface` | Gateway: regex. **iface:** salida `ip`/`route` en el sistema | Nombres de interfaz suelen ser alfanuméricos cortos; ideal validar con mismo criterio que `validators` (p. ej. patrón interfaz Linux). |

### 2.2 `executeCommand(fmt.Sprintf(...))` con entrada HTTP

| Dato | Validación | Riesgo |
|------|------------|--------|
| `req.Hostname` | Regex `^[a-zA-Z0-9]([a-zA-Z0-9\-\.]*[a-zA-Z0-9])?$`, longitud | Muy bajo para inyección shell. |
| `req.DNS1` / `req.DNS2` | Regex IPv4 | Muy bajo. |
| `req.Gateway` | Regex IPv4 | Muy bajo. |

### 2.3 `internal/network/api_network_status_speed_handlers.go`

- `exec.CommandContext(ctx, bin, "--json", …)` con `bin` resuelto de forma controlada — **no** usa `sh -c` para el speedtest (revisar que `bin` no venga de usuario).

### 2.4 `internal/network/network.go` y `api_network_interfaces_handler.go`

- Múltiples `sh -c` + `fmt.Sprintf` con **`ifaceName`** en bucles (`ip`, `cat /sys/...`, `wpa_cli`, `grep` sobre `ps`, etc.).  
- **Origen típico:** interfaces listadas por el SO (`ip -o link`, etc.), no el cuerpo JSON directo.  
- **Riesgo:** bajo en uso normal; si el kernel/NM expusiera un nombre anómalo, habría superficie teórica.  
- **Recomendación:** Validar `ifaceName` con regexp estricta (`^[a-zA-Z0-9._@-]{1,15}$` o similar) antes de interpolar.

---

## 3. `internal/hostapd/hostapd.go`

- **Muchísimas** cadenas `fmt.Sprintf` pasadas a `executeCommand` (que usa `sh -c`) **o** a `exec.Command("sh", "-c", fmt.Sprintf(...))`.  
- Variables interpoladas típicas: `interfaceName`, `phyInterface`, `phyName`, `apInterface`, `gatewayIP`, `req.Gateway`, rutas bajo `/etc`, `/tmp`, reglas `iptables`, contenidos de config.  
- **Lectura de interfaz desde `/etc/hostapd/hostapd.conf`:** si un atacante con acceso al fichero (root o despliegue malicioso) pone `interface=…` arbitrario, esa cadena acaba en `iw`, `hostapd_cli`, `ip`, etc.  
- **Handlers HTTP:** comprobar que todos los campos que llegan al body y terminan en shell estén validados (SSID, interfaz, IP, etc.); en una búsqueda rápida **no** hay uso evidente de `validators` en este paquete — **revisión manual de cada handler recomendada**.  
- **Prioridad refactor:** sustituir progresivamente  
  `executeCommand(fmt.Sprintf("sudo ip … %s …", x))`  
  por  
  `exec.Command("sudo", "ip", …, x)`  
  con `x` validado.

---

## 4. Prioridades de remediación

1. **Alta (diseño):** Dejar de interpolar en `sh -c` para valores que puedan tener espacios o comillas (**`connName` de nmcli**, nombres de interfaz, SSID si aplica). Usar `exec.Command` con argumentos.  
2. **Media:** Validar **siempre** nombres de interfaz antes de `fmt.Sprintf` en `network.go` / `api_network_interfaces_handler.go`.  
3. **Media:** En **hostapd**, auditar cada handler que parsea JSON y enlazar con `internal/validators` (SSID, interfaz, IP, canal).  
4. **Baja:** Sustituir `sh -c` de solo lectura (`ip route`, `systemctl is-active`, …) por llamadas con argumentos cuando sea posible (legibilidad y menos shell).

---

## 5. Inventario rápido por archivo (grep `sh` + `Sprintf`)

- **`network.go`:** ~15+ `sh -c` con `fmt.Sprintf` (mayoría `ifaceName`).  
- **`api_network_interfaces_handler.go`:** Patrón duplicado respecto a `network.go` (considerar DRY + validación centralizada).  
- **`api_network.go`:** Mezcla de `sh -c` fijos, interpolación segura de rutas temp, y `nmcli`/`executeCommand` con `connName`.  
- **`hostapd.go`:** Decenas de `fmt.Sprintf` + `executeCommand` o `sh -c` (el mayor volumen del proyecto en esta categoría).

---

*Documento generado como auditoría estática; no sustituye pruebas de penetración ni revisión de cada endpoint HTTP.*
