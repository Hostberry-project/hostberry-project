#!/bin/bash
# Módulo: user_files.sh
create_user() {
    if id "$USER_NAME" &>/dev/null; then
        print_info "Usuario $USER_NAME ya existe"
        # Asegurar que el usuario esté en el grupo netdev (necesario para wpa_supplicant)
        if getent group netdev > /dev/null 2>&1; then
            if groups "$USER_NAME" | grep -q "\bnetdev\b"; then
                print_info "Usuario $USER_NAME ya está en el grupo netdev"
            else
                print_info "Agregando usuario $USER_NAME al grupo netdev..."
                usermod -a -G netdev "$USER_NAME"
                print_success "Usuario $USER_NAME agregado al grupo netdev"
            fi
        else
            print_warning "Grupo netdev no existe, creándolo..."
            groupadd -r netdev 2>/dev/null || true
            usermod -a -G netdev "$USER_NAME"
            print_success "Grupo netdev creado y usuario agregado"
        fi
    else
        print_info "Creando usuario $USER_NAME..."
        # Crear grupo netdev si no existe
        if ! getent group netdev > /dev/null 2>&1; then
            groupadd -r netdev 2>/dev/null || true
            print_info "Grupo netdev creado"
        fi
        # Crear usuario y agregarlo al grupo netdev
        useradd -r -s /bin/false -d "$INSTALL_DIR" -G netdev "$USER_NAME"
        print_success "Usuario $USER_NAME creado y agregado al grupo netdev"
    fi
}

# Copiar archivos del proyecto

install_files() {
    print_info "Instalando archivos en $INSTALL_DIR..."
    
    # Verificar que estamos en el directorio correcto con todos los archivos
    local missing_files=0
    for item in "website" "locales" "main.go" "go.mod"; do
        if [ ! -e "${SCRIPT_DIR}/${item}" ]; then
            print_warning "No se encontró '${item}' en ${SCRIPT_DIR}"
            missing_files=$((missing_files + 1))
        fi
    done

    if [ $missing_files -gt 0 ]; then
        print_error "Error: Faltan archivos del proyecto en ${SCRIPT_DIR}"
        print_info "Asegúrate de ejecutar el script desde la raíz del repositorio clonado."
        print_info "Si has descargado solo el script, necesitas descargar el proyecto completo."
        exit 1
    fi

    # Crear directorios
    mkdir -p "$INSTALL_DIR"

    mkdir -p "$LOG_DIR"
    mkdir -p "$DATA_DIR"
    # Lua ya no se usa - todo está en Go ahora
    mkdir -p "${INSTALL_DIR}/locales"
    mkdir -p "${INSTALL_DIR}/website/static"
    mkdir -p "${INSTALL_DIR}/website/templates"
    
    # Copiar archivos necesarios
    print_info "Copiando archivos del proyecto..."
    
    # Archivos Go
    cp -f "${SCRIPT_DIR}"/*.go "${INSTALL_DIR}/" 2>/dev/null || true
    cp -f "${SCRIPT_DIR}/go.mod" "${INSTALL_DIR}/" 2>/dev/null || true
    cp -f "${SCRIPT_DIR}/go.sum" "${INSTALL_DIR}/" 2>/dev/null || true

    # Copiar el árbol interno del módulo (incluye internal/handlers, etc.).
    # Sin esto, `go build` falla al resolver imports `hostberry/internal/...`.
    if [ -d "${SCRIPT_DIR}/internal" ]; then
        rm -rf "${INSTALL_DIR}/internal" 2>/dev/null || true
        cp -r "${SCRIPT_DIR}/internal" "${INSTALL_DIR}/internal" 2>/dev/null || true
    fi
    
    # Directorios (lua ya no se usa - todo está en Go)
    if [ -d "${SCRIPT_DIR}/locales" ]; then
        cp -r "${SCRIPT_DIR}/locales/"* "${INSTALL_DIR}/locales/" 2>/dev/null || true
    fi

    if [ -d "${SCRIPT_DIR}/docs" ]; then
        mkdir -p "${INSTALL_DIR}/docs"
        cp -r "${SCRIPT_DIR}/docs/"* "${INSTALL_DIR}/docs/" 2>/dev/null || true
    fi

    mkdir -p "${INSTALL_DIR}/backups"
    chown "$USER_NAME:$GROUP_NAME" "${INSTALL_DIR}/backups" 2>/dev/null || true
    
    if [ -d "${SCRIPT_DIR}/website" ]; then
        # Asegurar que los directorios destino existen
        mkdir -p "${INSTALL_DIR}/website/templates"
        mkdir -p "${INSTALL_DIR}/website/static"
        
        # Copiar templates con verificación
        if [ -d "${SCRIPT_DIR}/website/templates" ]; then
            if ! cp -r "${SCRIPT_DIR}/website/templates/"* "${INSTALL_DIR}/website/templates/" 2>/dev/null; then
                print_error "Error al copiar templates"
                exit 1
            fi
            # Verificar que templates críticos existen
            if [ ! -f "${INSTALL_DIR}/website/templates/base.html" ] || \
               [ ! -f "${INSTALL_DIR}/website/templates/dashboard.html" ] || \
               [ ! -f "${INSTALL_DIR}/website/templates/login.html" ]; then
                print_error "Templates críticos no encontrados"
                exit 1
            fi
        else
            print_error "Error: Directorio ${SCRIPT_DIR}/website/templates no existe"
            exit 1
        fi
        
        # Copiar archivos estáticos
        if [ -d "${SCRIPT_DIR}/website/static" ]; then
            cp -r "${SCRIPT_DIR}/website/static/"* "${INSTALL_DIR}/website/static/" 2>/dev/null || true
        fi
    else
        print_error "Error: Directorio website no encontrado en ${SCRIPT_DIR}"
        exit 1
    fi
    
    # Configuración
    if [ ! -f "$CONFIG_FILE" ]; then
        if [ -f "${SCRIPT_DIR}/config.yaml.example" ]; then
            cp "${SCRIPT_DIR}/config.yaml.example" "$CONFIG_FILE"
        else
            cat > "$CONFIG_FILE" <<EOF
server:
  port: 8000
  debug: false
  read_timeout: 30
  write_timeout: 30

database:
  type: "sqlite"
  path: "data/hostberry.db"

security:
  jwt_secret: ""
  token_expiry: 30
  bcrypt_cost: 10
  rate_limit_rps: 10
  lockout_minutes: 15
  enforce_https: true
EOF
        fi
    fi

    # Secretos (sólo en instalación inicial)
    if [ "$MODE" = "install" ] && [ -f "$CONFIG_FILE" ]; then
        # JWT secret aleatorio si está vacío o usa el placeholder antiguo
        if grep -q 'jwt_secret:' "$CONFIG_FILE"; then
            if grep -qE 'jwt_secret: ""|cambiar-este-secreto-en-produccion-usar-secretos-seguros' "$CONFIG_FILE"; then
                GENERATED_JWT_SECRET="$(tr -dc 'A-Za-z0-9' </dev/urandom | head -c 64 || echo "hostberry$(date +%s)")"
                sed -i "s|jwt_secret: \".*\"|jwt_secret: \"${GENERATED_JWT_SECRET}\"|" "$CONFIG_FILE"
            fi
        else
            GENERATED_JWT_SECRET="$(tr -dc 'A-Za-z0-9' </dev/urandom | head -c 64 || echo "hostberry$(date +%s)")"
            {
                echo ""
                echo "security:"
                echo "  jwt_secret: \"${GENERATED_JWT_SECRET}\""
                echo "  token_expiry: 60"
                echo "  bcrypt_cost: 10"
                echo "  rate_limit_rps: 10"
            } >> "$CONFIG_FILE"
        fi

        # Password aleatoria para usuario admin por defecto (se inyecta vía systemd)
        # Debe cumplir validators.ValidatePassword: mayúscula, minúscula, número y carácter especial.
        GENERATED_ADMIN_PASSWORD="Hb!$(tr -dc 'A-Za-z0-9' </dev/urandom | head -c 14 || echo 'xX1yY2zZ3')9a"

        # Guardar credenciales iniciales en un fichero sólo-lectura root/hostberry
        local cred_file="${INSTALL_DIR}/INSTALL_CREDENTIALS.txt"
        {
            echo "HostBerry - credenciales iniciales generadas automáticamente"
            echo ""
            echo "Usuario admin: admin"
            echo "Contraseña inicial admin: ${GENERATED_ADMIN_PASSWORD}"
            if [ -n "$GENERATED_JWT_SECRET" ]; then
                echo "JWT secret (security.jwt_secret en config.yaml): ${GENERATED_JWT_SECRET}"
            fi
            echo ""
            echo "Guarda este archivo en lugar seguro y cámbiala tras el primer acceso."
        } > "$cred_file"
        chmod 600 "$cred_file"
        chown "$USER_NAME:$GROUP_NAME" "$cred_file"
    fi
    
    # Permisos
    chown -R "$USER_NAME:$GROUP_NAME" "$INSTALL_DIR"
    chown -R "$USER_NAME:$GROUP_NAME" "$LOG_DIR"
    chown -R "$USER_NAME:$GROUP_NAME" "$DATA_DIR"
    chmod 755 "$INSTALL_DIR"
    chmod 644 "$CONFIG_FILE"
}

# Compilar el proyecto
#
# Descarga de dependencias Go: reintentos y fallbacks para redes lentas o bloqueo de proxy.golang.org
# Guardamos el último log de error para mostrarlo si fallan todos los intentos
HOSTBERRY_GO_DEPS_ERROR_LOG="${HOSTBERRY_GO_DEPS_ERROR_LOG:-/tmp/hostberry_go_deps_error.log}"


