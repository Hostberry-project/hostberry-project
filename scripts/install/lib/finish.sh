#!/bin/bash
# Módulo: finish.sh
show_final_info() {
    echo ""
    case "$MODE" in
        update)    print_success "Actualización completa" ;;
        remove) print_success "Desinstalación completa" ;;
        *)         print_success "Instalación completa" ;;
    esac

    # Para desinstalación, no hay endpoints/paths que mostrar
    if [ "$MODE" = "remove" ]; then
        echo ""
        return 0
    fi

    local ip port web_url scheme http_redir mdns_name
    mdns_name="hostberry.local"
    ip="$(hostname -I 2>/dev/null | awk '{print $1}')"
    port="$(awk '/^[[:space:]]*port:/{gsub(/"/,"",$2); print $2; exit}' "$CONFIG_FILE" 2>/dev/null)"
    port="${port:-8000}"
    http_redir="$(awk '/^[[:space:]]*http_redirect_port:/{gsub(/"/,"",$2); print $2; exit}' "$CONFIG_FILE" 2>/dev/null)"
    scheme="http"
    if grep -qE '^[[:space:]]*tls_cert_file:' "$CONFIG_FILE" 2>/dev/null && grep -qE '^[[:space:]]*tls_key_file:' "$CONFIG_FILE" 2>/dev/null; then
        scheme="https"
    fi

    if [ -n "$ip" ] && [ "$ip" != "127.0.0.1" ]; then
        web_url="$(public_url "$scheme" "$ip" "$port")"
    else
        web_url="$(public_url "$scheme" "localhost" "$port")"
    fi

    print_info "Web:    ${web_url}"
    print_info "        $(public_url "$scheme" "$mdns_name" "$port")  (mDNS: hostberry.local)"
    print_info "WiFi AP: red «hostberry». Al conectar se abre http://192.168.4.1/portal (configuración automática)."
    print_info "        Tras conectar recibirás IP 192.168.4.x. En el móvil desactiva «WiFi inteligente»."
    print_info "        Portal cautivo: sudo systemctl restart hostberry-captive-portal  |  sudo iptables -t nat -L PREROUTING -n"
    if [ "$scheme" = "https" ] && [ -n "$http_redir" ] && [ "$http_redir" != "0" ] && [ "$http_redir" != "$port" ]; then
        print_info "HTTP:   $(public_url "http" "${ip:-localhost}" "$http_redir") (redirige a HTTPS)"
        print_info "        $(public_url "http" "$mdns_name" "$http_redir") (redirige a HTTPS)"
    fi
    print_info "Config: ${CONFIG_FILE}"
    print_info "Logs:   journalctl -u ${SERVICE_NAME} -f"

    # Contraseña admin: en install viene en memoria; en update u otras ejecuciones, leer INSTALL_CREDENTIALS.txt
    local saved_admin_password=""
    if [ -n "$GENERATED_ADMIN_PASSWORD" ]; then
        saved_admin_password="$GENERATED_ADMIN_PASSWORD"
    elif [ -r "${INSTALL_DIR}/INSTALL_CREDENTIALS.txt" ]; then
        saved_admin_password="$(grep -m1 '^Contraseña inicial admin: ' "${INSTALL_DIR}/INSTALL_CREDENTIALS.txt" 2>/dev/null | sed 's/^Contraseña inicial admin: //')"
    fi

    if [ -n "$saved_admin_password" ]; then
        print_warning "Login inicial: admin / ${saved_admin_password} (se ha guardado también en ${INSTALL_DIR}/INSTALL_CREDENTIALS.txt)"
        if [ "$(is_default_route_over_wifi)" = "1" ] && [ "${NEED_REBOOT_FOR_AP0:-0}" -eq 1 ]; then
            print_warning "El sistema reiniciará en segundos; si usas SSH por WiFi, vuelve a conectar cuando arranque la Pi."
        elif [ "$(is_default_route_over_wifi)" = "1" ] && [ "${NEED_REBOOT_FOR_AP0:-0}" -eq 0 ]; then
            print_warning "Reinicio automático omitido (p. ej. HOSTBERRY_SKIP_REBOOT=1). Reinicia manualmente cuando puedas."
        fi
    else
        print_warning "Login:  admin / admin (cámbiala)"
    fi
    echo ""
}

# Limpiar directorio temporal al finalizar

cleanup_temp() {
    if [ -d "$TEMP_CLONE_DIR" ] && [ "$TEMP_CLONE_DIR" != "$SCRIPT_DIR" ]; then
        print_info "Limpiando directorio temporal..."
        rm -rf "$TEMP_CLONE_DIR"
    fi
}

# Desinstalación completa: servicio, archivos, usuario y logs

