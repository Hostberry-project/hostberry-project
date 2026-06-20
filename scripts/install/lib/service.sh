#!/bin/bash
# Módulo: service.sh
create_systemd_service() {
    print_info "Creando servicio systemd..."
    
    cat > "$SERVICE_FILE" <<EOF
[Unit]
Description=HostBerry - Sistema de Gestión de Red
After=network.target

[Service]
Type=simple
User=${USER_NAME}
Group=${GROUP_NAME}
WorkingDirectory=${INSTALL_DIR}
Environment=HOSTBERRY_DEFAULT_ADMIN_PASSWORD=${GENERATED_ADMIN_PASSWORD}
ExecStart=${INSTALL_DIR}/hostberry
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal
SyslogIdentifier=${SERVICE_NAME}

# Seguridad
# NoNewPrivileges=true  # Deshabilitado para permitir sudo en comandos WiFi
# PrivateTmp desactivado: wpa_cli crea su socket de respuesta en /tmp y wpa_supplicant (en el
# namespace del host) debe poder entregar la respuesta; con PrivateTmp el escaneo agota el tiempo.
PrivateTmp=false
ProtectSystem=strict
ProtectHome=true
# Rutas de escritura para los comandos privilegiados (hostapd/dnsmasq/wpa_supplicant) vía privileged-exec.
# /tmp es necesario para el socket local de wpa_cli (escaneo WiFi) con ProtectSystem=strict.
ReadWritePaths=${INSTALL_DIR} ${LOG_DIR} ${DATA_DIR} /etc/hostapd /etc/dnsmasq.d /etc/wpa_supplicant -/etc/openvpn -/etc/wireguard -/etc/tor -/var/lib/tor -/var/log/tor -/etc/iptables /tmp

# Puertos privilegiados 80/443 con usuario no root.
AmbientCapabilities=CAP_NET_BIND_SERVICE
# CapabilityBoundingSet sin restringir: sudo/privileged-exec necesitan CAP_SETUID/CAP_SETGID para
# escalar a root en los comandos WiFi (wpa_cli, iw, hostapd). Limitarlo rompe TODO comando privilegiado.

# Recursos
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
EOF
    
    create_setup_supplicant_unit

    # Recargar systemd
    systemctl daemon-reload
    
    print_success "Servicio systemd creado: $SERVICE_FILE"
}

# Crea el unit del wpa_supplicant dedicado para wlan0 durante el asistente. En radio única (AP+STA),
# NetworkManager escanea wlan0 periódicamente y tira al cliente del portal cautivo. Durante el setup,
# la app saca wlan0 de NM (nmcli) y arranca este supplicant SIN redes y SIN autoscan, de modo que la
# radio solo deja el canal del AP cuando el usuario pide escanear. No se habilita (la app lo
# arranca/para según el estado del asistente).
create_setup_supplicant_unit() {
    local wpa_bin
    wpa_bin="$(command -v wpa_supplicant || echo /usr/sbin/wpa_supplicant)"

    cat > "$SETUP_SUPPLICANT_CONF_FILE" <<EOF
ctrl_interface=/run/wpa_supplicant
ctrl_interface_group=netdev
update_config=1
EOF
    chmod 600 "$SETUP_SUPPLICANT_CONF_FILE" 2>/dev/null || true

    cat > "$SETUP_SUPPLICANT_UNIT_FILE" <<EOF
[Unit]
Description=HostBerry setup-mode WiFi supplicant (wlan0, sin escaneo en segundo plano)
After=network-pre.target
Wants=network-pre.target

[Service]
Type=simple
ExecStart=${wpa_bin} -i wlan0 -D nl80211 -c ${SETUP_SUPPLICANT_CONF_FILE}
Restart=on-failure
RestartSec=2
EOF

    print_success "Unit del supplicant de asistente creado: $SETUP_SUPPLICANT_UNIT_FILE"
}

# Iniciar servicio

start_service() {
    print_info "Iniciando servicio ${SERVICE_NAME}..."
    
    systemctl enable "${SERVICE_NAME}"
    systemctl start "${SERVICE_NAME}"
    systemctl restart "${SERVICE_NAME}"
    
    # Esperar un momento y verificar
    sleep 2
    
    if systemctl is-active --quiet "${SERVICE_NAME}"; then
        print_success "Servicio iniciado correctamente"
        print_info "Estado: $(systemctl is-active ${SERVICE_NAME})"
        
        # Esperar un poco más para que se cree el usuario admin
        sleep 2
        
        # Verificar si se creó el usuario admin
        print_info "Verificando creación de usuario admin..."
        if journalctl -u "${SERVICE_NAME}" -n 20 --no-pager | grep -q "Usuario admin creado exitosamente"; then
            print_success "Usuario admin creado correctamente"
        elif journalctl -u "${SERVICE_NAME}" -n 20 --no-pager | grep -q "Error creando usuario admin"; then
            print_warning "Hubo un error al crear el usuario admin"
            print_info "Revisa los logs: sudo journalctl -u ${SERVICE_NAME} -n 50"
        else
            print_info "Revisa los logs para ver el estado del usuario admin:"
            print_info "  sudo journalctl -u ${SERVICE_NAME} -n 50 | grep -i admin"
        fi

        if systemctl list-unit-files hostberry-captive-portal.service &>/dev/null 2>&1; then
            systemctl restart hostberry-captive-portal.service 2>/dev/null || true
        fi
    else
        print_warning "El servicio no se inició correctamente"
        print_info "Revisa los logs con: journalctl -u ${SERVICE_NAME} -f"
    fi
}

# URL sin :puerto si es HTTPS/443 o HTTP/80.

public_url() {
    local scheme="$1" host="$2" port="$3"
    case "${scheme}:${port}" in
        https:443) printf 'https://%s' "$host" ;;
        http:80)   printf 'http://%s' "$host" ;;
        *)         printf '%s://%s:%s' "$scheme" "$host" "$port" ;;
    esac
}

# Habilita e inicia AP WiFi + DHCP + portal cautivo (install y --update, también por SSH).

