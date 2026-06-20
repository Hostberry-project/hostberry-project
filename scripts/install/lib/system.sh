#!/bin/bash
# Módulo: system.sh

hostberry_apt_updated=0

hostberry_apt_update_once() {
    if [ "$hostberry_apt_updated" -eq 1 ]; then
        return 0
    fi
    apt-get update -qq
    hostberry_apt_updated=1
}

hostberry_fast_install_enabled() {
    [ "${HOSTBERRY_FAST_INSTALL:-0}" = "1" ]
}

hostberry_pkg_installed() {
    local package="$1"
    if dpkg -l 2>/dev/null | grep -q "^ii.*${package} "; then
        return 0
    fi
    case "$package" in
        wpa_supplicant)
            command -v wpa_supplicant &>/dev/null || [ -f /usr/sbin/wpa_supplicant ] || [ -f /sbin/wpa_supplicant ]
            ;;
        hostapd)
            command -v hostapd &>/dev/null || [ -f /usr/sbin/hostapd ] || [ -f /sbin/hostapd ]
            ;;
        dnsmasq)
            # dnsmasq-base sólo trae el binario; hace falta el paquete dnsmasq (unidad systemd + DHCP).
            hostberry_dnsmasq_pkg_installed
            ;;
        wireguard-tools)
            command -v wg &>/dev/null || dpkg -l 2>/dev/null | grep -q "^ii.*wireguard "
            ;;
        *)
            command -v "$package" &>/dev/null
            ;;
    esac
}

hostberry_dnsmasq_pkg_installed() {
    dpkg -l 2>/dev/null | grep -q '^ii  dnsmasq '
}

hostberry_dnsmasq_unit_name() {
    if [ -f /lib/systemd/system/dnsmasq.service ] || [ -f /etc/systemd/system/dnsmasq.service ]; then
        printf '%s' 'dnsmasq.service'
    else
        printf '%s' 'hostberry-dnsmasq.service'
    fi
}

hostberry_systemctl_dnsmasq() {
    local action="$1"
    local unit
    unit="$(hostberry_dnsmasq_unit_name)"
    systemctl "$action" "$unit" 2>/dev/null || true
}

hostberry_install_dnsmasq_restart_helper() {
    local script="/usr/local/sbin/hostberry-restart-dnsmasq.sh"
    mkdir -p "$(dirname "$script")"
    cat > "$script" <<'EOF'
#!/bin/bash
# Asegura dnsmasq activo sin reiniciarlo si ya reparte DHCP (evita cortes al arrancar hostapd).
if systemctl cat dnsmasq.service &>/dev/null; then
    if systemctl is-active --quiet dnsmasq.service; then
        exit 0
    fi
    systemctl start dnsmasq.service
elif systemctl cat hostberry-dnsmasq.service &>/dev/null; then
    if systemctl is-active --quiet hostberry-dnsmasq.service; then
        exit 0
    fi
    systemctl start hostberry-dnsmasq.service
fi
EOF
    chmod 755 "$script"
    chown root:root "$script"
}

hostberry_install_fallback_dnsmasq_unit() {
    local unit="/etc/systemd/system/hostberry-dnsmasq.service"
    local prep="/usr/local/sbin/hostberry-dnsmasq-prep-ap0.sh"
    [ -x /usr/sbin/dnsmasq ] || [ -x /sbin/dnsmasq ] || return 1
    hostberry_install_dnsmasq_restart_helper
    cat > "$unit" <<EOF
[Unit]
Description=HostBerry DHCP/DNS on ap0 (dnsmasq-base)
After=network.target create-ap0.service hostapd.service
Wants=create-ap0.service hostapd.service

[Service]
Type=simple
Environment=PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/sbin:/usr/bin:/bin
ExecStartPre=-${prep}
ExecStart=/usr/sbin/dnsmasq -k --conf-file=/etc/dnsmasq.conf
Restart=on-failure
RestartSec=3

[Install]
WantedBy=multi-user.target
EOF
    chmod 644 "$unit"
    systemctl daemon-reload 2>/dev/null || true
    print_success "Unidad hostberry-dnsmasq.service creada (dnsmasq-base sin paquete dnsmasq)"
}

hostberry_ensure_dnsmasq_ready() {
    if ! hostberry_dnsmasq_pkg_installed; then
        print_info "Instalando paquete dnsmasq (servicio DHCP; dnsmasq-base no basta)..."
        hostberry_apt_update_once
        if ! apt-get install -y --no-install-recommends dnsmasq > /dev/null 2>&1; then
            print_warning "No se pudo instalar el paquete dnsmasq por apt."
        fi
    fi
    if [ -f /lib/systemd/system/dnsmasq.service ] || [ -f /etc/systemd/system/dnsmasq.service ]; then
        hostberry_install_dnsmasq_restart_helper
        return 0
    fi
    hostberry_install_fallback_dnsmasq_unit
}

fix_hostname() {
    CURRENT_HOSTNAME=$(hostname)
    if [ -n "$CURRENT_HOSTNAME" ]; then
        # Verificar si el hostname ya está en /etc/hosts
        if ! grep -q "127.0.0.1.*$CURRENT_HOSTNAME" /etc/hosts 2>/dev/null; then
            print_info "Configurando hostname '$CURRENT_HOSTNAME' en /etc/hosts..."
            # Agregar hostname a la línea de 127.0.0.1
            if grep -q "^127.0.0.1" /etc/hosts; then
                # La línea existe, agregar el hostname si no está
                sed -i "s/^127.0.0.1.*/& $CURRENT_HOSTNAME/" /etc/hosts 2>/dev/null || true
            else
                # La línea no existe, crearla
                echo "127.0.0.1 localhost $CURRENT_HOSTNAME" >> /etc/hosts
            fi
            # También agregar a 127.0.1.1 si no existe
            if ! grep -q "^127.0.1.1" /etc/hosts; then
                echo "127.0.1.1 $CURRENT_HOSTNAME" >> /etc/hosts
            fi
            print_success "Hostname configurado en /etc/hosts"
        fi
    fi
}

# Detectar sistema operativo

detect_os() {
    if [ -f /etc/os-release ]; then
        . /etc/os-release
        OS=$ID
        OS_VERSION=$VERSION_ID
        print_info "Sistema: $OS $OS_VERSION"
    else
        print_error "No se pudo detectar el sistema operativo"
        exit 1
    fi
}

hostberry_go_ok() {
    command -v go &>/dev/null && go version 2>/dev/null | grep -qE 'go1\.(2[3-9]|[3-9][0-9])'
}

hostberry_need_apt_pkg() {
    local pkg="$1"
    case "$pkg" in
        golang-go)
            ! hostberry_go_ok
            ;;
        mkcert)
            ! command -v mkcert &>/dev/null && ! hostberry_pkg_installed "$pkg"
            ;;
        librespeed-cli)
            ! command -v librespeed-cli &>/dev/null && ! hostberry_pkg_installed "$pkg"
            ;;
        *)
            ! hostberry_pkg_installed "$pkg"
            ;;
    esac
}

hostberry_collect_apt_packages() {
    local -n _out=$1
    _out=()
    local pkg
    local core=(
        git golang-go wget curl iw isc-dhcp-client
        hostapd dnsmasq iptables wpa_supplicant
    )

    for pkg in "${core[@]}"; do
        if hostberry_need_apt_pkg "$pkg"; then
            _out+=("$pkg")
        fi
    done

    if hostberry_fast_install_enabled; then
        print_info "Instalación rápida: omitiendo Tor, OpenVPN, WireGuard y LibreSpeed (HOSTBERRY_FAST_INSTALL=1)."
    else
        for pkg in tor openvpn wireguard-tools librespeed-cli; do
            if hostberry_need_apt_pkg "$pkg"; then
                _out+=("$pkg")
            fi
        done
    fi

    if [ "${HOSTBERRY_SKIP_MKCERT:-0}" != "1" ]; then
        if hostberry_need_apt_pkg mkcert; then
            _out+=(mkcert)
        fi
    fi

    if [ "${HOSTBERRY_SKIP_AVAHI:-0}" != "1" ]; then
        if hostberry_need_apt_pkg avahi-daemon; then
            _out+=(avahi-daemon)
        fi
    fi
}

verify_golang() {
    if hostberry_go_ok; then
        print_success "Go listo: $(go version)"
        return 0
    fi
    if command -v go &>/dev/null; then
        print_error "Go >= 1.23 requerido; versión actual: $(go version 2>/dev/null || echo desconocida)"
    else
        print_error "Go no encontrado tras apt-get install golang-go"
    fi
    return 1
}

# Un solo apt-get update + apt-get install para todos los paquetes del sistema.

install_apt_packages() {
    local to_install=()
    hostberry_collect_apt_packages to_install

    if [ ${#to_install[@]} -eq 0 ]; then
        print_success "Paquetes apt ya instalados"
        if command -v git &>/dev/null; then
            print_success "Git: $(git --version)"
        fi
        verify_golang || true
        return 0
    fi

    print_info "Instalando paquetes del sistema (apt, un solo paso)..."
    hostberry_apt_update_once
    print_info "Paquetes: ${to_install[*]}"

    if apt-get install -y --no-install-recommends "${to_install[@]}" > /dev/null 2>&1; then
        print_success "Paquetes apt instalados"
    else
        print_warning "Instalación por lotes falló; reintentando paquetes individualmente..."
        local failed=()
        for pkg in "${to_install[@]}"; do
            if ! hostberry_need_apt_pkg "$pkg"; then
                continue
            fi
            if apt-get install -y --no-install-recommends "$pkg" > /dev/null 2>&1; then
                continue
            fi
            if [ "$pkg" = "wireguard-tools" ] && apt-get install -y --no-install-recommends wireguard > /dev/null 2>&1; then
                continue
            fi
            failed+=("$pkg")
        done
        if [ ${#failed[@]} -gt 0 ]; then
            print_warning "Paquetes no instalados: ${failed[*]} (algunas funciones pueden no estar disponibles)"
        else
            print_success "Paquetes apt instalados"
        fi
    fi

    if command -v git &>/dev/null; then
        print_success "Git: $(git --version)"
    fi
    verify_golang || true
    if declare -F hostberry_ensure_dnsmasq_ready >/dev/null 2>&1; then
        hostberry_ensure_dnsmasq_ready
    fi
}

# Descargar proyecto de GitHub (siempre desde GitHub, nunca local)

