#!/bin/bash
# Módulo: build.sh
try_go_mod_download() {
    local env_kv="$1"
    local attempt="$2"
    local max="$3"
    local tmp_log
    local timeout_secs="${HOSTBERRY_GO_MOD_DOWNLOAD_TIMEOUT:-180}"
    local ret=1

    tmp_log="$(mktemp)"
    export GOTOOLCHAIN=local

    if command -v timeout >/dev/null 2>&1; then
        if [ -n "$env_kv" ]; then
            timeout "$timeout_secs" env GOTOOLCHAIN=local $env_kv go mod download >"$tmp_log" 2>&1
        else
            timeout "$timeout_secs" env GOTOOLCHAIN=local go mod download >"$tmp_log" 2>&1
        fi
    else
        if [ -n "$env_kv" ]; then
            env GOTOOLCHAIN=local $env_kv go mod download >"$tmp_log" 2>&1
        else
            env GOTOOLCHAIN=local go mod download >"$tmp_log" 2>&1
        fi
    fi
    ret=$?

    if [ "$ret" -eq 0 ]; then
        rm -f "$tmp_log"
        return 0
    fi
    cp -f "$tmp_log" "$HOSTBERRY_GO_DEPS_ERROR_LOG" 2>/dev/null || true
    rm -f "$tmp_log"
    return 1
}


download_go_deps() {
    local retries="${HOSTBERRY_GO_MOD_RETRIES:-5}"
    local sleep_secs="${HOSTBERRY_GO_MOD_RETRY_SLEEP:-4}"

    # 1) Intentar con el entorno actual
    for ((i=1; i<=retries; i++)); do
        if try_go_mod_download "" "$i" "$retries"; then
            export HOSTBERRY_GO_MOD_ENV=""
            return 0
        fi
        sleep "$sleep_secs"
    done

    # 2) Fallback a modo directo (sin proxy)
    for ((i=1; i<=retries; i++)); do
        if try_go_mod_download "GOPROXY=direct" "$i" "$retries"; then
            export HOSTBERRY_GO_MOD_ENV="GOPROXY=direct"
            return 0
        fi
        sleep "$sleep_secs"
    done

    # 3) (Opcional) último recurso: desactivar sumdb (menos seguro)
    if [ "${HOSTBERRY_ALLOW_GOSUMDB_OFF:-0}" = "1" ]; then
        for ((i=1; i<=retries; i++)); do
            if try_go_mod_download "GOPROXY=direct GOSUMDB=off" "$i" "$retries"; then
                export HOSTBERRY_GO_MOD_ENV="GOPROXY=direct GOSUMDB=off"
                return 0
            fi
            sleep "$sleep_secs"
        done
    fi

    print_error "Error al descargar dependencias de Go"
    if [ -s "$HOSTBERRY_GO_DEPS_ERROR_LOG" ]; then
        print_info "Detalle del último intento:"
        cat "$HOSTBERRY_GO_DEPS_ERROR_LOG" | head -50
        rm -f "$HOSTBERRY_GO_DEPS_ERROR_LOG"
    fi
    print_info "Sugerencia: compruebe conexión a Internet, proxy/firewall (proxy.golang.org) o ejecute con HOSTBERRY_GO_MOD_RETRIES=10"
    return 1
}

# Lee la salida de `go build -v` y muestra porcentaje aproximado (una línea ≈ un paquete).
# Tras la última línea impresa, Go puede tardar mucho sin volver a escribir: compilación de
# `main` con go:embed (website/static + templates) y enlazado — no es un cuelgue.
# pkg_total: resultado de `go list -deps` (debe calcularse antes del pipeline para evitar carreras).

show_build_progress() {
    local pkg_total="${1:-1}"
    local n=0 pct line hb_pid=""

    if ! [[ "$pkg_total" =~ ^[0-9]+$ ]] || [ "$pkg_total" -lt 1 ]; then
        pkg_total=1
    fi

    while IFS= read -r line || [ -n "$line" ]; do
        [ -z "$line" ] && continue
        n=$((n + 1))
        pct=$((n * 100 / pkg_total))
        if [ "$pct" -gt 100 ]; then
            pct=100
        fi
        # %b interpreta \033 en DIM/NC (definidos con comillas simples, son literales hasta %b/echo -e)
        printf '\r\033[K%b[%3d%%] %s%b' "$DIM" "$pct" "$line" "${NC}" >&2

        # Latido si no llega otra línea en ~12 s (fase larga sin salida en -v)
        if [ -n "$hb_pid" ] && kill -0 "$hb_pid" 2>/dev/null; then
            kill "$hb_pid" 2>/dev/null
            wait "$hb_pid" 2>/dev/null || true
        fi
        (
            sleep 12
            while true; do
                printf '\n%b   ... sigue compilando (no colgado): main/embed y enlazado; en Raspberry Pi puede tardar minutos. Último: %s%b\n' "$DIM" "$line" "$NC" >&2
                sleep 12
            done
        ) &
        hb_pid=$!
    done
    if [ -n "$hb_pid" ] && kill -0 "$hb_pid" 2>/dev/null; then
        kill "$hb_pid" 2>/dev/null
        wait "$hb_pid" 2>/dev/null || true
    fi
    echo "" >&2
}

# Instala el binario mkcert (apt o release en GitHub).

build_project() {
    if declare -F hostberry_try_release_binary >/dev/null 2>&1 && hostberry_try_release_binary; then
        print_success "Binario precompilado instalado (sin compilar)."
        return 0
    fi

    if declare -F verify_golang >/dev/null 2>&1; then
        verify_golang || exit 1
    fi

    print_info "Compilando HostBerry en ${INSTALL_DIR}..."
    
    # Verificar que estamos en el directorio correcto
    if [ ! -d "$INSTALL_DIR" ]; then
        print_error "Error: Directorio de instalación no existe: $INSTALL_DIR"
        exit 1
    fi
    
    # Cambiar al directorio de instalación
    cd "$INSTALL_DIR" || {
        print_error "Error: No se pudo cambiar al directorio $INSTALL_DIR"
        exit 1
    }
    
    print_info "Directorio de trabajo: $(pwd)"
    
    # Verificar que los templates están presentes antes de compilar
    if [ ! -d "${INSTALL_DIR}/website/templates" ]; then
        print_error "Error: Directorio de templates no encontrado: ${INSTALL_DIR}/website/templates"
        print_info "Verificando estructura del directorio..."
        ls -la "${INSTALL_DIR}/" 2>/dev/null || true
        exit 1
    fi
    
    TEMPLATE_COUNT=$(find "${INSTALL_DIR}/website/templates" -name "*.html" 2>/dev/null | wc -l)
    if [ "$TEMPLATE_COUNT" -eq 0 ]; then
        print_error "Error: No se encontraron archivos .html en ${INSTALL_DIR}/website/templates"
        print_info "Contenido del directorio:"
        ls -la "${INSTALL_DIR}/website/templates/" 2>/dev/null || true
        exit 1
    fi
    print_success "Verificado: $TEMPLATE_COUNT templates encontrados en ${INSTALL_DIR}/website/templates"
    
    # Verificar que main.go existe
    if [ ! -f "${INSTALL_DIR}/main.go" ]; then
        print_error "Error: main.go no encontrado en ${INSTALL_DIR}"
        print_info "Archivos .go encontrados:"
        ls -la "${INSTALL_DIR}"/*.go 2>/dev/null || true
        exit 1
    fi
    
    # Verificar que go.mod existe
    if [ ! -f "${INSTALL_DIR}/go.mod" ]; then
        print_error "Error: go.mod no encontrado en ${INSTALL_DIR}"
        exit 1
    fi
    
    # Asegurar que Go está en el PATH
    export PATH=$PATH:/usr/local/go/bin
    
    # Verificar que Go está disponible
    if ! command -v go &> /dev/null; then
        print_error "Error: Go no está instalado o no está en el PATH"
        exit 1
    fi
    
    # Verificar estructura antes de compilar
    if [ ! -f "${INSTALL_DIR}/main.go" ]; then
        print_error "Error: main.go no encontrado"
        exit 1
    fi
    
    if [ ! -d "${INSTALL_DIR}/website/templates" ]; then
        print_error "Error: Directorio de templates no encontrado"
        exit 1
    fi
    
    # Descargar dependencias (puede tardar; con timeout para no colgarse)
    print_info "Descargando dependencias Go (puede tardar 1-2 min)..."
    if ! download_go_deps; then
        exit 1
    fi
    
    # Fuerza modo módulos para evitar que un GO111MODULE=off externo rompa imports.
    export GO111MODULE=on
    export GOWORK=off

    export GOTOOLCHAIN=local
    env $HOSTBERRY_GO_MOD_ENV go mod tidy > /dev/null 2>&1 || true
    
    # Compilación pure Go (modernc SQLite, sin CGO)
    BUILD_JOBS=$(nproc 2>/dev/null || echo 4)
    export GOMAXPROCS="${BUILD_JOBS}"
    export CGO_ENABLED=0
    
    BUILD_TIMEOUT="${HOSTBERRY_BUILD_TIMEOUT:-900}"

    # Total de paquetes (dependencias + main) para porcentaje aproximado con `go build -v`
    BUILD_PKG_TOTAL=$(go list -deps -f '{{.ImportPath}}' . 2>/dev/null | wc -l | tr -d ' \n')
    BUILD_PKG_TOTAL=${BUILD_PKG_TOTAL:-1}
    if [ "$BUILD_PKG_TOTAL" -lt 1 ] 2>/dev/null; then
        BUILD_PKG_TOTAL=1
    fi

    print_info "Compilando (usando ${BUILD_JOBS} núcleos, ~${BUILD_PKG_TOTAL} paquetes; el % es orientativo)."
    print_info "Si la última línea se queda fija, es normal: Go sigue con main (embed) y enlazado — en Raspberry Pi puede tardar unos minutos."
    if declare -F hostberry_backup_binary >/dev/null 2>&1; then
        hostberry_backup_binary
    fi
    build_ret=0
    set +e
    set -o pipefail 2>/dev/null || true

    if command -v timeout >/dev/null 2>&1; then
        if command -v stdbuf >/dev/null 2>&1; then
            timeout "$BUILD_TIMEOUT" stdbuf -oL -eL env $HOSTBERRY_GO_MOD_ENV go build -p "$BUILD_JOBS" -trimpath -ldflags="-s -w" -v -o "${INSTALL_DIR}/hostberry" . 2>&1 | show_build_progress "$BUILD_PKG_TOTAL"
        else
            timeout "$BUILD_TIMEOUT" env $HOSTBERRY_GO_MOD_ENV go build -p "$BUILD_JOBS" -trimpath -ldflags="-s -w" -v -o "${INSTALL_DIR}/hostberry" . 2>&1 | show_build_progress "$BUILD_PKG_TOTAL"
        fi
        build_ret=${PIPESTATUS[0]:-1}
    else
        if command -v stdbuf >/dev/null 2>&1; then
            stdbuf -oL -eL env $HOSTBERRY_GO_MOD_ENV go build -p "$BUILD_JOBS" -trimpath -ldflags="-s -w" -v -o "${INSTALL_DIR}/hostberry" . 2>&1 | show_build_progress "$BUILD_PKG_TOTAL"
        else
            env $HOSTBERRY_GO_MOD_ENV go build -p "$BUILD_JOBS" -trimpath -ldflags="-s -w" -v -o "${INSTALL_DIR}/hostberry" . 2>&1 | show_build_progress "$BUILD_PKG_TOTAL"
        fi
        build_ret=${PIPESTATUS[0]:-1}
    fi

    set +o pipefail 2>/dev/null || true
    set -e
    if [ "$build_ret" -eq 0 ] && [ -f "${INSTALL_DIR}/hostberry" ]; then
        chmod +x "${INSTALL_DIR}/hostberry"
        chown "$USER_NAME:$GROUP_NAME" "${INSTALL_DIR}/hostberry"
        if declare -F hostberry_verify_binary >/dev/null 2>&1 && ! hostberry_verify_binary; then
            print_error "El binario compilado no pasó la verificación (-version)."
            if declare -F hostberry_rollback_binary >/dev/null 2>&1; then
                hostberry_rollback_binary || true
            fi
            exit 1
        fi
        if declare -F hostberry_store_binary_checksum >/dev/null 2>&1; then
            hostberry_store_binary_checksum
        fi
        print_success "Compilación completada."
    elif [ "$build_ret" -eq 124 ]; then
        print_error "Compilación cancelada: tiempo de espera agotado (${BUILD_TIMEOUT}s). En Raspberry Pi puede tardar más; ejecute de nuevo con HOSTBERRY_BUILD_TIMEOUT=1200"
        exit 1
    else
        print_error "Error en la compilación (código $build_ret). Compruebe Go >= 1.23 y conexión a Internet (go mod download)."
        exit 1
    fi
}

# Configurar firewall

