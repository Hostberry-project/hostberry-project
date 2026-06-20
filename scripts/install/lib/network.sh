#!/bin/bash
# Módulo: network.sh
hostberry_migrate_blocky_dns_loopback() {
    local BLOCKY_CONFIG_FILE="/etc/blocky/config.yml"
    [ -f "$BLOCKY_CONFIG_FILE" ] || return 0
    if ! grep -qE '^[[:space:]]*dns:[[:space:]]*("53"|53)[[:space:]]*$' "$BLOCKY_CONFIG_FILE" 2>/dev/null; then
        return 0
    fi
    sed -i 's/^\([[:space:]]*dns:\)[[:space:]]*"53"[[:space:]]*$/\1 127.0.0.1:53/' "$BLOCKY_CONFIG_FILE" 2>/dev/null || true
    sed -i 's/^\([[:space:]]*dns:\)[[:space:]]*53[[:space:]]*$/\1 127.0.0.1:53/' "$BLOCKY_CONFIG_FILE" 2>/dev/null || true
    print_info "Blocky: DNS restringido a 127.0.0.1:53 (compatible con dnsmasq en ap0)"
    if command -v systemctl &>/dev/null; then
        systemctl try-restart blocky.service 2>/dev/null || true
    fi
}

# Instalar Blocky (proxy DNS y ad-blocker para la página Adblock)

install_blocky() {
    local BLOCKY_VERSION="v0.28.2"
    local BLOCKY_CONFIG_DIR="/etc/blocky"
    local BLOCKY_CONFIG_FILE="${BLOCKY_CONFIG_DIR}/config.yml"
    local BLOCKY_SERVICE="/etc/systemd/system/blocky.service"

    hostberry_migrate_blocky_dns_loopback

    if [ -x "/usr/local/bin/blocky" ] || systemctl cat blocky &>/dev/null; then
        print_success "Blocky ya está instalado"
        return 0
    fi

    print_info "Instalando Blocky (DNS proxy y ad-blocker)..."

    local ARCH
    ARCH=$(uname -m)
    case "$ARCH" in
        x86_64|amd64) BLOCKY_ARCH="x86_64" ;;
        aarch64)      BLOCKY_ARCH="arm64" ;;
        armv7l|armhf) BLOCKY_ARCH="armv7" ;;
        armv6l)       BLOCKY_ARCH="armv6" ;;
        *)            BLOCKY_ARCH="x86_64" ;;
    esac

    local BLOCKY_URL="https://github.com/0xERR0R/blocky/releases/download/${BLOCKY_VERSION}/blocky_${BLOCKY_VERSION}_Linux_${BLOCKY_ARCH}.tar.gz"
    local TMP_BLOCKY="/tmp/blocky_install"
    rm -rf "$TMP_BLOCKY"
    mkdir -p "$TMP_BLOCKY"

    if command -v wget &>/dev/null; then
        wget -q -O "${TMP_BLOCKY}/blocky.tar.gz" "$BLOCKY_URL" || true
    else
        curl -sL -o "${TMP_BLOCKY}/blocky.tar.gz" "$BLOCKY_URL" || true
    fi

    if [ ! -s "${TMP_BLOCKY}/blocky.tar.gz" ]; then
        print_warning "No se pudo descargar Blocky (${BLOCKY_ARCH}). Puedes instalarlo desde la web Adblock más tarde."
        rm -rf "$TMP_BLOCKY"
        return 0
    fi

    export PATH="/usr/bin:/bin:$PATH"
    tar -xzf "${TMP_BLOCKY}/blocky.tar.gz" -C "$TMP_BLOCKY"
    cp "${TMP_BLOCKY}/blocky" /usr/local/bin/blocky
    chmod +x /usr/local/bin/blocky
    rm -rf "$TMP_BLOCKY"

    mkdir -p "$BLOCKY_CONFIG_DIR"
    if [ ! -f "$BLOCKY_CONFIG_FILE" ]; then
        cat > "$BLOCKY_CONFIG_FILE" <<'BLOCKYCONF'
# Blocky - Configuración por defecto HostBerry
upstreams:
  groups:
    default:
    - 1.1.1.1
    - 8.8.8.8
    - https://dns.cloudflare.com/dns-query

blocking:
  denylists:
    default:
    - https://raw.githubusercontent.com/StevenBlack/hosts/master/hosts
    - https://adguardteam.github.io/AdGuardSDNSFilter/Filters/filter.txt
  blockType: zeroIp
  refreshPeriod: 4h

ports:
  dns: 127.0.0.1:53
  http: 127.0.0.1:4000

log:
  level: warn
  format: text
BLOCKYCONF
        print_success "Configuración por defecto de Blocky creada: $BLOCKY_CONFIG_FILE"
    fi

    cat > "$BLOCKY_SERVICE" <<EOF
[Unit]
Description=Blocky DNS proxy and ad-blocker
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/blocky --config $BLOCKY_CONFIG_FILE
Restart=on-failure
RestartSec=3

[Install]
WantedBy=multi-user.target
EOF
    systemctl daemon-reload
    systemctl enable blocky
    print_success "Blocky instalado (arrancará con el sistema; configúralo desde la web Adblock)"
}

# Instalar LibreSpeed CLI (test de velocidad en la página Red) vía apt

install_librespeed_cli() {
    if command -v librespeed-cli &>/dev/null; then
        print_success "LibreSpeed CLI ya está instalado"
        return 0
    fi
    if ! command -v librespeed-cli &>/dev/null; then
        print_warning "LibreSpeed CLI no instalado. Instálalo con: sudo apt install librespeed-cli"
    fi
}

# Crear servicio systemd

hostberry_ensure_dnsmasq_running() {
    local unit="dnsmasq.service"
    if declare -F hostberry_ensure_dnsmasq_ready >/dev/null 2>&1; then
        hostberry_ensure_dnsmasq_ready
    fi
    if declare -F hostberry_dnsmasq_unit_name >/dev/null 2>&1; then
        unit="$(hostberry_dnsmasq_unit_name)"
    fi
    if ! systemctl is-active --quiet "$unit" 2>/dev/null; then
        systemctl start "$unit" 2>/dev/null || true
    fi
}

hostberry_verify_ap_dhcp() {
    local unit
    unit="$(hostberry_dnsmasq_unit_name)"
    sleep 2
    if ! systemctl is-active --quiet "$unit" 2>/dev/null; then
        print_warning "DHCP (${unit}) no está activo: la red hostberry no repartirá IP."
        if declare -F hostberry_ensure_dnsmasq_ready >/dev/null 2>&1; then
            hostberry_ensure_dnsmasq_ready
        fi
        hostberry_systemctl_dnsmasq start
        sleep 1
    fi
    if ss -ulnp 2>/dev/null | grep -q ':67 '; then
        print_success "DHCP activo (puerto 67); los clientes deberían recibir 192.168.4.x"
    elif systemctl is-active --quiet "$unit" 2>/dev/null; then
        print_warning "dnsmasq activo pero no escucha en UDP/67; revisa: journalctl -u ${unit} -n 30"
    else
        print_warning "Sin DHCP en la red hostberry. Ejecuta: sudo ./install.sh --update"
    fi
}

enable_and_start_hostberry_wifi_ap() {
    if ! command -v systemctl &>/dev/null; then
        return 0
    fi
    print_info "Habilitando en el arranque: ap0, hostapd (SSID hostberry), dnsmasq y portal cautivo…"
    systemctl daemon-reload 2>/dev/null || true
    rfkill unblock wifi 2>/dev/null || true
    systemctl unmask hostapd.service 2>/dev/null || true
    if declare -F hostberry_ensure_dnsmasq_ready >/dev/null 2>&1; then
        hostberry_ensure_dnsmasq_ready
    fi
    hostberry_systemctl_dnsmasq unmask 2>/dev/null || true
    systemctl unmask dnsmasq.service 2>/dev/null || true
    systemctl enable create-ap0.service 2>/dev/null || true
    systemctl enable hostapd.service 2>/dev/null || true
    hostberry_systemctl_dnsmasq enable
    systemctl enable dnsmasq.service 2>/dev/null || true
    systemctl enable hostberry-captive-portal.service 2>/dev/null || true

    if [ "${HOSTBERRY_DEFER_AP_START:-0}" = "1" ]; then
        print_success "AP diferido: servicios habilitados; se activarán tras reinicio del sistema."
        return 0
    fi

    if systemctl is-active --quiet NetworkManager 2>/dev/null; then
        systemctl reload NetworkManager 2>/dev/null || systemctl restart NetworkManager 2>/dev/null || true
    fi

    if [ "$(is_default_route_over_wifi)" = "1" ]; then
        print_warning "Ruta por defecto por WiFi: se inicia el AP igualmente (SSH por esa WiFi puede cortarse); preferible cable o consola."
    fi

    print_info "Iniciando ap0, hostapd, dnsmasq y reglas iptables del portal cautivo…"
    systemctl start create-ap0.service 2>/dev/null || true
    sleep 2
    systemctl start hostapd.service 2>/dev/null || true
    sleep 2
    hostberry_ensure_dnsmasq_running
    hostberry_systemctl_dnsmasq start
    systemctl start dnsmasq.service 2>/dev/null || true
    sleep 1
    systemctl start hostberry-captive-portal.service 2>/dev/null || true
    hostberry_verify_ap_dhcp
    if systemctl is-active --quiet hostapd.service 2>/dev/null; then
        print_success "Punto de acceso WiFi (hostapd) activo"
    else
        print_warning "hostapd no está activo; tras reinicio comprueba: journalctl -u hostapd -u create-ap0 -b"
    fi
}

# Siempre tras install/--update: daemon-reload. Si no hay reinicio del sistema, reiniciar AP/DHCP/portal en caliente.

finalize_systemd_hostberry_network() {
    command -v systemctl &>/dev/null || return 0
    print_info "Aplicando systemd: daemon-reload…"
    systemctl daemon-reload 2>/dev/null || true
    if [ "${HOSTBERRY_DEFER_AP_START:-0}" -eq 1 ] || [ "${NEED_REBOOT_FOR_AP0:-0}" -eq 1 ]; then
        print_info "AP/reinicio pendiente: se omiten reinicios en caliente de hostapd/dnsmasq."
        return 0
    fi
    print_info "Reiniciando hostapd, dnsmasq, ${SERVICE_NAME} y hostberry-captive-portal…"
    systemctl restart hostapd.service 2>/dev/null || true
    sleep 2
    hostberry_systemctl_dnsmasq restart
    systemctl restart dnsmasq.service 2>/dev/null || true
    systemctl try-restart "${SERVICE_NAME}.service" 2>/dev/null || true
    sleep 1
    systemctl restart hostberry-captive-portal.service 2>/dev/null || true
    print_success "Servicios HostBerry recargados"
}

# Mostrar información final

