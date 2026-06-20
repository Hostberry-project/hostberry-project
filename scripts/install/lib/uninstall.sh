#!/bin/bash
# Módulo: uninstall.sh
do_uninstall() {
    print_info "Iniciando desinstalación de HostBerry..."

    # 1. Detener y deshabilitar el servicio
    if systemctl is-active --quiet "${SERVICE_NAME}" 2>/dev/null; then
        print_info "Deteniendo servicio ${SERVICE_NAME}..."
        systemctl stop "${SERVICE_NAME}" 2>/dev/null || true
        sleep 2
    fi
    if systemctl is-enabled --quiet "${SERVICE_NAME}" 2>/dev/null; then
        print_info "Deshabilitando servicio ${SERVICE_NAME}..."
        systemctl disable "${SERVICE_NAME}" 2>/dev/null || true
    fi

    # 2. Eliminar archivo de unidad systemd (principal + supplicant de asistente)
    systemctl stop hostberry-wifi-setup.service 2>/dev/null || true
    nmcli device set wlan0 managed yes 2>/dev/null || true
    rm -f "${SETUP_SUPPLICANT_UNIT_FILE:-/etc/systemd/system/hostberry-wifi-setup.service}" 2>/dev/null || true
    rm -f "${SETUP_SUPPLICANT_CONF_FILE:-/etc/wpa_supplicant/hostberry-wlan0-setup.conf}" 2>/dev/null || true
    if [ -f "$SERVICE_FILE" ]; then
        print_info "Eliminando $SERVICE_FILE..."
        rm -f "$SERVICE_FILE"
        print_success "Archivo de servicio eliminado"
        systemctl daemon-reload 2>/dev/null || true
    fi

    # 3. Eliminar directorio de instalación
    if [ -d "$INSTALL_DIR" ]; then
        print_info "Eliminando $INSTALL_DIR..."
        rm -rf "$INSTALL_DIR"
        print_success "Directorio de instalación eliminado"
    fi

    # 4. Eliminar directorio de logs
    if [ -d "$LOG_DIR" ]; then
        print_info "Eliminando $LOG_DIR..."
        rm -rf "$LOG_DIR"
        print_success "Logs eliminados"
    fi

    # 5. Eliminar usuario y grupo del sistema (opcional, solo si existen)
    if id "$USER_NAME" &>/dev/null; then
        print_info "Eliminando usuario $USER_NAME..."
        userdel -r "$USER_NAME" 2>/dev/null || userdel "$USER_NAME" 2>/dev/null || true
        print_success "Usuario eliminado"
    fi
    if getent group "$GROUP_NAME" &>/dev/null; then
        print_info "Eliminando grupo $GROUP_NAME..."
        groupdel "$GROUP_NAME" 2>/dev/null || true
        print_success "Grupo eliminado"
    fi

    print_success "Desinstalación completada"
}

# Función principal

