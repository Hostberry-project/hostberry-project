#!/bin/bash
# Módulo: firewall_db.sh
configure_firewall() {
    local HTTPS_PORT HTTP_EXTRA
    HTTPS_PORT=$(awk '/^[[:space:]]*port:/{gsub(/"/,"",$2); print $2; exit}' "$CONFIG_FILE" 2>/dev/null)
    HTTPS_PORT="${HTTPS_PORT:-8000}"
    HTTP_EXTRA=$(awk '/^[[:space:]]*http_redirect_port:/{gsub(/"/,"",$2); print $2; exit}' "$CONFIG_FILE" 2>/dev/null)

    # Verificar si ufw está instalado y activo
    if command -v ufw &> /dev/null; then
        if ufw status | grep -q "Status: active"; then
            ufw allow "${HTTPS_PORT}/tcp" 2>/dev/null || true
            if [ -n "$HTTP_EXTRA" ] && [ "$HTTP_EXTRA" != "0" ] && [ "$HTTP_EXTRA" != "$HTTPS_PORT" ]; then
                ufw allow "${HTTP_EXTRA}/tcp" 2>/dev/null || true
            fi
        fi
    elif command -v firewall-cmd &> /dev/null; then
        firewall-cmd --permanent --add-port="${HTTPS_PORT}/tcp" 2>/dev/null || true
        if [ -n "$HTTP_EXTRA" ] && [ "$HTTP_EXTRA" != "0" ] && [ "$HTTP_EXTRA" != "$HTTPS_PORT" ]; then
            firewall-cmd --permanent --add-port="${HTTP_EXTRA}/tcp" 2>/dev/null || true
        fi
        firewall-cmd --reload 2>/dev/null || true
    fi
}

# Crear base de datos inicial

create_database() {
    print_info "Preparando base de datos..."
    
    # Asegurar que el directorio de datos existe
    mkdir -p "$DATA_DIR"
    chown -R "$USER_NAME:$GROUP_NAME" "$DATA_DIR"
    chmod 755 "$DATA_DIR"
    
    # El archivo de BD se creará automáticamente al iniciar el servicio
    # pero creamos el directorio y verificamos permisos
    DB_FILE="${DATA_DIR}/hostberry.db"
    if [ -f "$DB_FILE" ]; then
        print_info "Base de datos existente encontrada: $DB_FILE"
        chown "$USER_NAME:$GROUP_NAME" "$DB_FILE"
        chmod 600 "$DB_FILE"
        print_warning "Si la BD tiene datos antiguos, el usuario admin puede no crearse automáticamente"
    else
        print_info "Base de datos se creará automáticamente al iniciar el servicio"
        print_info "El usuario admin se creará automáticamente si la BD está vacía"
    fi
    
    print_success "Directorio de base de datos preparado: $DATA_DIR"
}

# Configurar permisos y sudoers

configure_permissions() {
    print_info "Configurando permisos y sudoers..."
    
    # Crear directorio para scripts seguros
    SAFE_DIR="/usr/local/sbin/hostberry-safe"
    mkdir -p "$SAFE_DIR"
    
    # Crear script set-timezone
    cat > "$SAFE_DIR/set-timezone" <<EOF
#!/bin/bash
TZ="\$1"
if [ -z "\$TZ" ]; then echo "Timezone required"; exit 1; fi
if [ ! -f "/usr/share/zoneinfo/\$TZ" ]; then echo "Invalid timezone"; exit 1; fi
timedatectl set-timezone "\$TZ"
EOF
    chmod 750 "$SAFE_DIR/set-timezone"
    chown root:$GROUP_NAME "$SAFE_DIR/set-timezone"

    if [ -f "${SCRIPT_DIR}/scripts/privileged-exec.sh" ]; then
        install -m 0750 "${SCRIPT_DIR}/scripts/privileged-exec.sh" "$SAFE_DIR/privileged-exec"
        chown root:$GROUP_NAME "$SAFE_DIR/privileged-exec"
        print_info "Wrapper sudo restrictivo: $SAFE_DIR/privileged-exec"
    fi
    
    # Detectar rutas de comandos WiFi
    NMCLI_PATH=""
    RFKILL_PATH=""
    IFCONFIG_PATH=""
    IW_PATH=""
    IWCONFIG_PATH=""
    
    # Buscar nmcli
    if command -v nmcli &> /dev/null; then
        NMCLI_PATH=$(command -v nmcli)
    elif [ -f "/usr/bin/nmcli" ]; then
        NMCLI_PATH="/usr/bin/nmcli"
    fi
    
    # Buscar rfkill
    if command -v rfkill &> /dev/null; then
        RFKILL_PATH=$(command -v rfkill)
    elif [ -f "/usr/sbin/rfkill" ]; then
        RFKILL_PATH="/usr/sbin/rfkill"
    fi
    
    # Buscar ifconfig
    if command -v ifconfig &> /dev/null; then
        IFCONFIG_PATH=$(command -v ifconfig)
    elif [ -f "/sbin/ifconfig" ]; then
        IFCONFIG_PATH="/sbin/ifconfig"
    elif [ -f "/usr/sbin/ifconfig" ]; then
        IFCONFIG_PATH="/usr/sbin/ifconfig"
    fi
    
    # Buscar iw (para cambiar región WiFi)
    if command -v iw &> /dev/null; then
        IW_PATH=$(command -v iw)
    elif [ -f "/usr/sbin/iw" ]; then
        IW_PATH="/usr/sbin/iw"
    elif [ -f "/sbin/iw" ]; then
        IW_PATH="/sbin/iw"
    fi
    
    # Buscar iwconfig (para gestión WiFi)
    if command -v iwconfig &> /dev/null; then
        IWCONFIG_PATH=$(command -v iwconfig)
    elif [ -f "/usr/sbin/iwconfig" ]; then
        IWCONFIG_PATH="/usr/sbin/iwconfig"
    elif [ -f "/sbin/iwconfig" ]; then
        IWCONFIG_PATH="/sbin/iwconfig"
    fi
    
    # Detectar rutas de comandos de sistema
    REBOOT_PATH=""
    SHUTDOWN_PATH=""
    
    # Buscar reboot
    if command -v reboot &> /dev/null; then
        REBOOT_PATH=$(command -v reboot)
    elif [ -f "/usr/sbin/reboot" ]; then
        REBOOT_PATH="/usr/sbin/reboot"
    elif [ -f "/sbin/reboot" ]; then
        REBOOT_PATH="/sbin/reboot"
    fi
    
    # Buscar shutdown (ya detectado arriba, pero asegurarse)
    if command -v shutdown &> /dev/null; then
        SHUTDOWN_PATH=$(command -v shutdown)
    elif [ -f "/usr/sbin/shutdown" ]; then
        SHUTDOWN_PATH="/usr/sbin/shutdown"
    elif [ -f "/sbin/shutdown" ]; then
        SHUTDOWN_PATH="/sbin/shutdown"
    fi
    
    # Configurar sudoers con configuración para evitar logs en sistemas read-only
    cat > "/etc/sudoers.d/hostberry" <<EOF
# Permisos para HostBerry
# Deshabilitar logging de sudo para evitar errores en sistemas read-only
Defaults!ALL !logfile
Defaults!ALL !syslog
$USER_NAME ALL=(ALL) NOPASSWD: $SAFE_DIR/set-timezone
EOF
    
    # Sudo restrictivo: wrapper con allowlist + systemctl explícito + apagado/reinicio.
    if [ -x "$SAFE_DIR/privileged-exec" ]; then
        echo "$USER_NAME ALL=(ALL) NOPASSWD: $SAFE_DIR/privileged-exec" >> "/etc/sudoers.d/hostberry"
        print_info "sudoers: acceso privilegiado vía $SAFE_DIR/privileged-exec"
    else
        print_warning "privileged-exec no instalado; comandos de red pueden fallar sin sudo amplio"
    fi

    if [ -n "$SHUTDOWN_PATH" ]; then
        echo "$USER_NAME ALL=(ALL) NOPASSWD: $SHUTDOWN_PATH" >> "/etc/sudoers.d/hostberry"
    fi
    if [ -n "$REBOOT_PATH" ]; then
        echo "$USER_NAME ALL=(ALL) NOPASSWD: $REBOOT_PATH" >> "/etc/sudoers.d/hostberry"
    fi

    SYSTEMCTL_PATH=""
    if command -v systemctl &> /dev/null; then
        SYSTEMCTL_PATH=$(command -v systemctl)
        for unit_cmd in             "reboot" "poweroff" "shutdown" "daemon-reload"             "start wpa_supplicant" "stop wpa_supplicant" "restart wpa_supplicant" "status wpa_supplicant"             "stop NetworkManager"             "start hostapd" "stop hostapd" "restart hostapd" "status hostapd" "enable hostapd" "disable hostapd" "unmask hostapd"             "start dnsmasq" "stop dnsmasq" "restart dnsmasq" "enable dnsmasq" "disable dnsmasq" "unmask dnsmasq"             "start blocky" "stop blocky" "enable blocky" "disable blocky" "restart blocky"             "restart systemd-resolved"             "start tor" "stop tor" "enable tor" "disable tor"; do
            echo "$USER_NAME ALL=(ALL) NOPASSWD: $SYSTEMCTL_PATH $unit_cmd" >> "/etc/sudoers.d/hostberry"
        done
    fi

    if command -v hostnamectl &> /dev/null; then
        HOSTNAMECTL_PATH=$(command -v hostnamectl)
        echo "$USER_NAME ALL=(ALL) NOPASSWD: $HOSTNAMECTL_PATH set-hostname *" >> "/etc/sudoers.d/hostberry"
    fi

    # Crear directorio /etc/hostapd con permisos correctos
    print_info "Creando directorio /etc/hostapd..."
    if [ ! -d "/etc/hostapd" ]; then
        mkdir -p /etc/hostapd
        chmod 755 /etc/hostapd
        print_success "Directorio /etc/hostapd creado con permisos 755"
    else
        chmod 755 /etc/hostapd 2>/dev/null || true
        print_info "Directorio /etc/hostapd ya existe, permisos verificados"
    fi
    
    # Crear también el directorio para systemd override si no existe
    if [ ! -d "/etc/systemd/system/hostapd.service.d" ]; then
        mkdir -p /etc/systemd/system/hostapd.service.d
        print_info "Directorio systemd override para hostapd creado"
    fi
    
    # Validar configuración de sudoers
    if visudo -c -f "/etc/sudoers.d/hostberry" 2>/dev/null; then
        chmod 440 "/etc/sudoers.d/hostberry"
        print_success "Permisos y sudoers configurados correctamente"
    else
        print_warning "Advertencia: Error al validar configuración de sudoers"
        print_info "Revisa manualmente: visudo -c -f /etc/sudoers.d/hostberry"
        chmod 440 "/etc/sudoers.d/hostberry"
    fi

    # Crear directorio temporal persistente para la configuración de wpa_supplicant
    FALLBACK_WPA_DIR="/tmp/hostberry/wpa_supplicant"
    if [ ! -d "$FALLBACK_WPA_DIR" ]; then
        mkdir -p "$FALLBACK_WPA_DIR"
        chown root:netdev "$FALLBACK_WPA_DIR"
        chmod 775 "$FALLBACK_WPA_DIR"
        print_info "Directorio temporal persistente creado: $FALLBACK_WPA_DIR"
    else
        chown root:netdev "$FALLBACK_WPA_DIR"
        chmod 775 "$FALLBACK_WPA_DIR"
    fi
}

# Crear configuración por defecto de HostAPD

