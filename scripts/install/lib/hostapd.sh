#!/bin/bash
# Módulo: hostapd.sh
create_hostapd_default_config() {
    print_info "Creando configuración por defecto de HostAPD…"
    # Install/update: configurar ap0/hostapd/systemd. Si SSH entra por WiFi, no levantar el AP en caliente.
    if [ "${HOSTBERRY_DEFER_AP_START:-0}" = "1" ]; then
        print_info "Configuración AP aplicada; arranque en caliente omitido (HOSTBERRY_DEFER_AP_START)."
    elif [ -n "${SSH_CONNECTION:-}" ] || [ -n "${SSH_TTY:-}" ]; then
        print_info "SSH detectado: se aplicará ap0/hostapd (sin cortar la sesión si no es por WiFi)."
    fi
    if [ "${HOSTBERRY_DEFER_AP_START:-0}" != "1" ] && [ "$(is_default_route_over_wifi)" = "1" ]; then
        print_warning "Ruta por defecto por WiFi: al levantar el AP la conexión SSH por WiFi puede interrumpirse brevemente."
    fi
    
    # Valores por defecto (red "hostberry" abierta + portal cautivo hacia la web de Hostberry)
    HOSTAPD_INTERFACE="wlan0"
    HOSTAPD_SSID="hostberry"
    HOSTAPD_CHANNEL="6"
    HOSTAPD_GATEWAY="192.168.4.1"
    HOSTAPD_DHCP_START="192.168.4.2"
    HOSTAPD_DHCP_END="192.168.4.254"
    HOSTAPD_DHCP_RANGE_END="${HOSTAPD_DHCP_END}"
    HOSTAPD_LEASE_TIME="12h"

    # Muchas Pi sólo exponen wlan1 o nombres tipo wlx…; si wlan0 no existe, create-ap0/hostapd no levantan el SSID.
    hostberry_detect_sta_wifi_interface() {
        local _n _cand=""
        for _n in wlan0 wlan1 wlan2 wlan3; do
            if [ -d "/sys/class/net/${_n}/wireless" ] || [ -L "/sys/class/net/${_n}/phy80211" ]; then
                _cand="$_n"
                break
            fi
        done
        if [ -z "$_cand" ]; then
            for _p in /sys/class/net/wl*; do
                [ -e "$_p" ] || continue
                _n=$(basename "$_p")
                [ "$_n" = "ap0" ] && continue
                if [ -d "${_p}/wireless" ] || [ -L "${_p}/phy80211" ]; then
                    _cand="$_n"
                    break
                fi
            done
        fi
        if [ -z "$_cand" ] && command -v iw &>/dev/null; then
            _cand=$(iw dev 2>/dev/null | awk '/Interface/ { if ($2 != "ap0") { print $2; exit } }')
        fi
        [ -n "$_cand" ] && echo "$_cand"
    }
    _wifi_sta=$(hostberry_detect_sta_wifi_interface)
    if [ -n "$_wifi_sta" ] && [ "$_wifi_sta" != "$HOSTAPD_INTERFACE" ]; then
        print_info "Interfaz WiFi de estación: ${_wifi_sta} (create-ap0 y hostapd la usarán; no hay wlan0 fiable)"
        HOSTAPD_INTERFACE="$_wifi_sta"
    elif [ -z "$_wifi_sta" ]; then
        print_warning "No se detectó interfaz WiFi (¿sin driver?); se asume ${HOSTAPD_INTERFACE}"
    fi

    # Nombre de phy para iw/udev: debe ser "phy0", no el índice "0" (iw phy 0 falla en muchos sistemas).
    PHY_NAME=""
    MAC_ADDRESS=""
    if [ -r "/sys/class/net/${HOSTAPD_INTERFACE}/phy80211/name" ]; then
        PHY_NAME=$(tr -d '\n' < "/sys/class/net/${HOSTAPD_INTERFACE}/phy80211/name" 2>/dev/null || true)
    fi
    if [ -z "$PHY_NAME" ] && command -v iw &> /dev/null; then
        _wiphy=$(iw dev "$HOSTAPD_INTERFACE" info 2>/dev/null | awk '/wiphy/ {print $2; exit}')
        if [ -n "$_wiphy" ]; then
            case "$_wiphy" in
                phy*) PHY_NAME="$_wiphy" ;;
                *)    PHY_NAME="phy${_wiphy}" ;;
            esac
        fi
    fi
    if [ -z "$PHY_NAME" ]; then
        PHY_NAME="phy0"
    fi
    MAC_ADDRESS=$(cat "/sys/class/net/${HOSTAPD_INTERFACE}/address" 2>/dev/null | tr -d '\n' || true)

    # Código ISO del país para hostapd (sin esto, muchas Pi no emiten beacons visibles). Alineado con wpa_supplicant.
    WPA_CFG_EARLY="/etc/wpa_supplicant/wpa_supplicant-${HOSTAPD_INTERFACE}.conf"
    if [ ! -f "$WPA_CFG_EARLY" ]; then
        WPA_CFG_EARLY="/etc/wpa_supplicant/wpa_supplicant-wlan0.conf"
    fi
    HOSTAPD_COUNTRY="US"
    if [ -f "$WPA_CFG_EARLY" ] && grep -q '^country=' "$WPA_CFG_EARLY" 2>/dev/null; then
        _cc=$(grep '^country=' "$WPA_CFG_EARLY" | head -1 | cut -d= -f2 | tr -d '[:space:]' | tr '[:lower:]' '[:upper:]')
        _cc=$(echo "$_cc" | cut -c1-2)
        [ "${#_cc}" -eq 2 ] && HOSTAPD_COUNTRY="$_cc"
    fi
    
    # En instalación: siempre valores de fábrica (hostapd incluido). En actualización: preservar si ya existe.
    # Modo AP+STA según método del blog de TheWalrus (Raspberry Pi 3 B+)
    HOSTAPD_CONFIG="/etc/hostapd/hostapd.conf"
    if [ "$MODE" = "install" ] || [ ! -f "$HOSTAPD_CONFIG" ]; then
        print_info "Creando/configurando HostAPD de fábrica (modo AP+STA): $HOSTAPD_CONFIG"
        
        # Validar interfaz WiFi
        if [ ! -d "/sys/class/net/${HOSTAPD_INTERFACE}" ]; then
            print_warning "Interfaz WiFi no encontrada: ${HOSTAPD_INTERFACE}. Se usará esa interfaz si existe luego."
        fi

        # Verificar si iw está disponible para gestionar interfaces virtuales
        if ! command -v iw &> /dev/null; then
            print_warning "iw no está disponible; no se puede crear ap0. Se usará la interfaz física."
            AP_INTERFACE="$HOSTAPD_INTERFACE"
        fi

        # Crear regla udev para crear ap0 automáticamente al arrancar (método TheWalrus - Raspberry Pi 3 B+)
        if [ -n "$MAC_ADDRESS" ] && [ -n "$PHY_NAME" ]; then
            print_info "Creando regla udev para ap0 (método TheWalrus - Raspberry Pi 3 B+)..."
            UDEV_RULE="/etc/udev/rules.d/70-persistent-net.rules"
            if [ ! -f "$UDEV_RULE" ] || ! grep -q "ap0" "$UDEV_RULE" 2>/dev/null; then
                cat >> "$UDEV_RULE" <<EOF

# Regla para crear interfaz virtual ap0 automáticamente (método TheWalrus - Raspberry Pi 3 B+)
SUBSYSTEM=="ieee80211", ACTION=="add|change", ATTR{macaddress}=="$MAC_ADDRESS", KERNEL=="$PHY_NAME", \
RUN+="/sbin/iw phy $PHY_NAME interface add ap0 type __ap", \
RUN+="/bin/ip link set ap0 address $MAC_ADDRESS"
EOF
                chmod 644 "$UDEV_RULE"
                print_success "Regla udev creada para ap0"
                udevadm control --reload-rules 2>/dev/null || true
                udevadm trigger 2>/dev/null || true
            else
                print_info "Regla udev para ap0 ya existe"
            fi
        fi
        
        # Intentar crear interfaz virtual ap0 si no existe (solo si iw está disponible)
        if [ "${HOSTBERRY_DEFER_AP_START:-0}" = "1" ]; then
            print_info "Creación de ap0 en caliente omitida; se hará al reiniciar (create-ap0.service)."
        elif command -v iw &> /dev/null; then
            if ! ip link show ap0 > /dev/null 2>&1; then
                print_info "Creando interfaz virtual ap0 para modo AP+STA..."
                
                # Asegurar que la interfaz física esté activa
                ip link set "$HOSTAPD_INTERFACE" up 2>/dev/null || true
                sleep 1
                
                # Intentar crear ap0 con múltiples métodos
                AP_CREATED=false
                
                # Método 1: Usando phy directamente
                if [ -n "$PHY_NAME" ] && iw phy "$PHY_NAME" interface add ap0 type __ap 2>/dev/null; then
                    AP_CREATED=true
                    print_success "Interfaz virtual ap0 creada usando phy $PHY_NAME"
                # Método 2: Usando la interfaz directamente
                elif iw dev "$HOSTAPD_INTERFACE" interface add ap0 type __ap 2>/dev/null; then
                    AP_CREATED=true
                    print_success "Interfaz virtual ap0 creada usando interfaz $HOSTAPD_INTERFACE"
                fi
                
                if [ "$AP_CREATED" = true ]; then
                    # Configurar MAC address de ap0 igual a wlan0
                    if [ -n "$MAC_ADDRESS" ]; then
                        ip link set ap0 address "$MAC_ADDRESS" 2>/dev/null || true
                    fi
                    # Activar la interfaz
                    ip link set ap0 up 2>/dev/null || true
                    sleep 1
                    
                    # Verificar que se creó correctamente
                    if ip link show ap0 > /dev/null 2>&1; then
                        print_success "Interfaz virtual ap0 verificada y activa"
                    else
                        print_warning "ap0 se creó pero no está disponible"
                    fi
                else
                    print_warning "No se pudo crear interfaz virtual ap0, usando interfaz física directamente"
                    print_info "Sugerencia: tu driver puede no soportar AP+STA. Verifica con: iw list | grep -A5 -i 'valid interface combinations'"
                    AP_INTERFACE="$HOSTAPD_INTERFACE"
                fi
            else
                print_success "Interfaz virtual ap0 ya existe"
                # Asegurar que esté activa
                ip link set ap0 up 2>/dev/null || true
            fi
        else
            print_warning "iw no está disponible, no se puede crear ap0"
            AP_INTERFACE="$HOSTAPD_INTERFACE"
        fi

        # Usar ap0 para la configuración si hay soporte (create-ap0.service + intento en caliente arriba).
        # No exigir MAC aquí: en instalación headless la interfaz puede no tener sysfs aún y MAC queda vacía;
        # el script hostberry-create-ap0.sh lee la MAC en tiempo de arranque.
        AP_INTERFACE="$HOSTAPD_INTERFACE"
        if command -v iw &> /dev/null && [ -n "$PHY_NAME" ]; then
            AP_INTERFACE="ap0"
            print_info "Configurando hostapd para usar ap0 (ap0 se creará en el arranque)."
        else
            # Fallback: si ap0 existe ya, úsalo; si no, usa la interfaz física
            if ip link show ap0 > /dev/null 2>&1; then
                AP_INTERFACE="ap0"
                print_info "Usando interfaz virtual ap0 (ya existente)"
            else
                print_info "Usando interfaz física $AP_INTERFACE (modo no concurrente)"
            fi
        fi
        
        cat > "$HOSTAPD_CONFIG" <<EOF
# Configuración de HostAPD para modo AP+STA según método TheWalrus (Raspberry Pi 3 B+)
# Red abierta (sin contraseña) - portal cautivo hacia la web de Hostberry
interface=${AP_INTERFACE}
driver=nl80211
# Socket para hostapd_cli (p. ej. hostapd_cli -i ap0 status); sin esto: wpa_ctrl_open No such file
ctrl_interface=/run/hostapd
ctrl_interface_group=0
ssid=${HOSTAPD_SSID}
hw_mode=g
channel=${HOSTAPD_CHANNEL}
auth_algs=1
wpa=0
country_code=${HOSTAPD_COUNTRY}
ieee80211d=1
ignore_broadcast_ssid=0
wmm_enabled=1
ieee80211n=1
# Solo un dispositivo durante la configuración inicial (HostBerry amplía al terminar el wizard)
max_num_sta=1
# Asegurar que wlan0 esté en modo managed (no AP)
# Esto se hace automáticamente cuando wpa_supplicant se ejecuta en wlan0
EOF
        chmod 644 "$HOSTAPD_CONFIG"
        HOSTAPD_DHCP_RANGE_END="${HOSTAPD_DHCP_START}"
        print_success "Archivo de configuración de HostAPD creado con valores por defecto"
        print_info "  - Interfaz AP: $AP_INTERFACE"
        print_info "  - Interfaz STA: $HOSTAPD_INTERFACE (para wpa_supplicant)"
        print_info "  - SSID: $HOSTAPD_SSID (red abierta, sin contraseña)"
        print_info "  - Gateway: $HOSTAPD_GATEWAY"
    else
        # Actualización: archivo ya existe, solo ajustar SSID y red abierta
        print_info "Archivo de configuración de HostAPD ya existe (actualización)"
        if grep -q "ssid=hostberry-ap" "$HOSTAPD_CONFIG" 2>/dev/null || grep -q "^ssid=.*" "$HOSTAPD_CONFIG" 2>/dev/null; then
            sed -i "s/^ssid=.*/ssid=${HOSTAPD_SSID}/" "$HOSTAPD_CONFIG" 2>/dev/null || true
            print_info "  SSID actualizado a: ${HOSTAPD_SSID}"
        fi
        # Quitar WPA2/3; no borrar wpa=0 (red abierta)
        sed -i '/^wpa=[1-9]/d' "$HOSTAPD_CONFIG" 2>/dev/null || true
        sed -i '/^wpa_passphrase=/d' "$HOSTAPD_CONFIG" 2>/dev/null || true
        sed -i '/^wpa_key_mgmt=/d' "$HOSTAPD_CONFIG" 2>/dev/null || true
        sed -i '/^wpa_pairwise=/d' "$HOSTAPD_CONFIG" 2>/dev/null || true
        sed -i '/^rsn_pairwise=/d' "$HOSTAPD_CONFIG" 2>/dev/null || true
        if ! grep -q "^auth_algs=" "$HOSTAPD_CONFIG" 2>/dev/null; then
            sed -i "/^channel=/a auth_algs=1" "$HOSTAPD_CONFIG" 2>/dev/null || true
        else
            sed -i "s/^auth_algs=.*/auth_algs=1/" "$HOSTAPD_CONFIG" 2>/dev/null || true
        fi
        grep -q '^wpa=0$' "$HOSTAPD_CONFIG" 2>/dev/null || echo 'wpa=0' >> "$HOSTAPD_CONFIG"
        print_info "  Red configurada como abierta (sin contraseña)"
    fi

    if ! grep -q '^ctrl_interface=' "$HOSTAPD_CONFIG" 2>/dev/null; then
        print_info "Añadiendo ctrl_interface a hostapd.conf (hostapd_cli -i ap0)…"
        {
            echo ""
            echo "ctrl_interface=/run/hostapd"
            echo "ctrl_interface_group=0"
        } >> "$HOSTAPD_CONFIG"
    fi

    # Configs antiguas: país / beacons / 11n (sin country_code muchas Pi no “ven” el SSID al escanear)
    if ! grep -q '^country_code=' "$HOSTAPD_CONFIG" 2>/dev/null; then
        print_info "Añadiendo country_code=${HOSTAPD_COUNTRY} a hostapd.conf…"
        {
            echo "country_code=${HOSTAPD_COUNTRY}"
            echo "ieee80211d=1"
            echo "ignore_broadcast_ssid=0"
            echo "wmm_enabled=1"
            echo "ieee80211n=1"
        } >> "$HOSTAPD_CONFIG"
    else
        grep -q '^ieee80211d=' "$HOSTAPD_CONFIG" 2>/dev/null || echo "ieee80211d=1" >> "$HOSTAPD_CONFIG"
        grep -q '^ignore_broadcast_ssid=' "$HOSTAPD_CONFIG" 2>/dev/null || echo "ignore_broadcast_ssid=0" >> "$HOSTAPD_CONFIG"
        grep -q '^wmm_enabled=' "$HOSTAPD_CONFIG" 2>/dev/null || echo "wmm_enabled=1" >> "$HOSTAPD_CONFIG"
        grep -q '^ieee80211n=' "$HOSTAPD_CONFIG" 2>/dev/null || echo "ieee80211n=1" >> "$HOSTAPD_CONFIG"
    fi
    # Red abierta HostBerry: wpa=0 explícito (varios clientes no asocian si falta)
    if grep -q '^auth_algs=1' "$HOSTAPD_CONFIG" 2>/dev/null && ! grep -q '^wpa=[1-9]' "$HOSTAPD_CONFIG" 2>/dev/null; then
        grep -q '^wpa=0$' "$HOSTAPD_CONFIG" 2>/dev/null || echo 'wpa=0' >> "$HOSTAPD_CONFIG"
    fi

    # Modo ap0+STA: hostapd debe anunciar el SSID en ap0, no en wlan* (si no, no se ve "hostberry" al escanear).
    if command -v iw &>/dev/null && [ -n "$PHY_NAME" ]; then
        if grep -q '^interface=' "$HOSTAPD_CONFIG" 2>/dev/null; then
            sed -i 's/^interface=.*/interface=ap0/' "$HOSTAPD_CONFIG" 2>/dev/null || true
            print_info "hostapd.conf: interface=ap0 (SSID en la interfaz virtual del AP)"
        fi
    fi

    # Misma radio: el AP debe usar el mismo canal (y banda) que wlan0 en STA o no se emiten beacons útiles
    SYNC_HOSTAPD_CH="/usr/local/sbin/hostberry-sync-hostapd-channel.sh"
    print_info "Instalando ${SYNC_HOSTAPD_CH} (alinear canal AP con ${HOSTAPD_INTERFACE})…"
    cat > "$SYNC_HOSTAPD_CH" <<EOF
#!/bin/bash
# Alinear canal del AP con la STA en la misma radio; validar contra frecuencias soportadas.
CONF="${HOSTAPD_CONFIG}"
WLAN="${HOSTAPD_INTERFACE}"
DEFAULT_CH=6
DEFAULT_MODE=g

[ -f "\$CONF" ] || exit 0

ACTIVE="/opt/hostberry/data/hostapd-active.conf"
if [ -f "\$ACTIVE" ]; then
    cp "\$ACTIVE" "\$CONF"
    chmod 644 "\$CONF"
fi

phy_name() {
    local p=""
    if [ -r "/sys/class/net/\${WLAN}/phy80211/name" ]; then
        p=\$(tr -d '\n' < "/sys/class/net/\${WLAN}/phy80211/name" 2>/dev/null || true)
    fi
    [ -n "\$p" ] || p=\$(basename "\$(ls -d /sys/class/ieee80211/phy* 2>/dev/null | head -1)" 2>/dev/null || true)
    printf '%s' "\$p"
}

channel_supported() {
    local ch="\$1" phy="\$2"
    [ -n "\$ch" ] && [ -n "\$phy" ] || return 1
    iw phy "\$phy" info 2>/dev/null | awk -v ch="\$ch" '
        index(\$0, "[" ch "]") && \$0 !~ /disabled/ { found=1 }
        END { exit !found }
    '
}

parse_channel_line() {
    echo "\$1" | sed -n 's/.*channel[[:space:]]\\+\\([0-9]\\+\\).*/\\1/p' | head -1
}

parse_freq_mhz() {
    echo "\$1" | sed -n 's/.*[(]\\([0-9]\\{4,\\}\\)[[:space:]]*MHz[)].*/\\1/p' | head -1
}

is_dfs_channel() {
    # Canales DFS (radar): 52-64 y 100-144. El AP solo puede usarlos si la STA ya está enlazada
    # en ese canal (la radio ya hizo el CAC como cliente); nunca en arranque en frío sin STA.
    local ch="\$1"
    [ "\$ch" -ge 52 ] 2>/dev/null && [ "\$ch" -le 144 ] 2>/dev/null
}

apply_ap_channel() {
    local mode="\$1" ch="\$2"
    sed -i "s/^hw_mode=.*/hw_mode=\${mode}/" "\$CONF"
    sed -i "s/^channel=.*/channel=\${ch}/" "\$CONF"
    sed -i '/^vht_/d' "\$CONF"
    if [ "\$mode" = "a" ]; then
        grep -q '^ieee80211n=' "\$CONF" || echo 'ieee80211n=1' >> "\$CONF"
        grep -q '^ieee80211ac=' "\$CONF" || echo 'ieee80211ac=1' >> "\$CONF"
    else
        sed -i '/^ieee80211ac=/d' "\$CONF"
    fi
}

PHY=\$(phy_name)
link_out=\$(iw dev "\$WLAN" link 2>/dev/null || true)
ch=""
freq=""
if ! echo "\$link_out" | grep -qi "Not connected"; then
    ch=\$(parse_channel_line "\$(echo "\$link_out" | grep -E 'channel[[:space:]]+[0-9]+' | head -1)")
    freq=\$(parse_freq_mhz "\$link_out")
    if [ -z "\$freq" ]; then
        freq=\$(echo "\$link_out" | sed -n 's/.*freq:[[:space:]]*\\([0-9.]*\\).*/\\1/p' | head -1 | cut -d. -f1)
    fi
    if [ -z "\$ch" ] && [ -n "\$freq" ]; then
        if [ "\$freq" -ge 2412 ] 2>/dev/null && [ "\$freq" -le 2484 ] 2>/dev/null; then
            ch=\$(( (freq - 2412) / 5 + 1 ))
        elif [ "\$freq" -ge 5000 ] 2>/dev/null && [ "\$freq" -le 5825 ] 2>/dev/null; then
            ch=\$(( (freq - 5000) / 5 ))
        fi
    fi
fi

if [ -z "\$ch" ]; then
    # Sin STA: respetar el canal ya configurado (p. ej. wizard moviendo AP antes de conectar upstream).
    # Pero NUNCA arrancar el AP en frío en un canal DFS (brcmfmac no hace CAC): degradar a no-DFS.
    existing_ch=\$(grep -E '^channel=' "\$CONF" | head -1 | cut -d= -f2)
    existing_mode=\$(grep -E '^hw_mode=' "\$CONF" | head -1 | cut -d= -f2)
    if [ -n "\$existing_ch" ] && is_dfs_channel "\$existing_ch"; then
        apply_ap_channel "\$DEFAULT_MODE" "\$DEFAULT_CH"
    elif [ -n "\$existing_ch" ] && channel_supported "\$existing_ch" "\$PHY"; then
        apply_ap_channel "\${existing_mode:-\$DEFAULT_MODE}" "\$existing_ch"
    else
        apply_ap_channel "\$DEFAULT_MODE" "\$DEFAULT_CH"
    fi
    exit 0
fi

if [ -z "\$freq" ]; then
    if [ "\$ch" -le 14 ] 2>/dev/null; then freq=2437; else freq=5180; fi
fi

if [ "\$freq" -lt 3000 ] 2>/dev/null; then
    if channel_supported "\$ch" "\$PHY"; then
        apply_ap_channel g "\$ch"
    else
        apply_ap_channel g "\$DEFAULT_CH"
    fi
else
    if channel_supported "\$ch" "\$PHY"; then
        apply_ap_channel a "\$ch"
    else
        apply_ap_channel g "\$DEFAULT_CH"
    fi
fi
exit 0
EOF
    chmod 755 "$SYNC_HOSTAPD_CH"
    chown root:root "$SYNC_HOSTAPD_CH"
    
    # Configuración dnsmasq para DHCP y DNS en la red hostberry (ap0)
    # Usar archivo dedicado en /etc/dnsmasq.d para no pisar la config del sistema
    DNSMASQ_D_DIR="/etc/dnsmasq.d"
    DNSMASQ_AP_CONFIG="${DNSMASQ_D_DIR}/hostberry-ap.conf"
    mkdir -p "$DNSMASQ_D_DIR"
    print_info "Escribiendo configuración DHCP/DNS para la red hostberry en ${DNSMASQ_AP_CONFIG}..."
    # no-dhcp sólo en la interfaz STA si existe (si no, dnsmasq puede fallar al resolver el nombre)
    DNSMASQ_NO_DHCP_LINE=""
    if [ -d "/sys/class/net/${HOSTAPD_INTERFACE}" ]; then
        DNSMASQ_NO_DHCP_LINE="no-dhcp-interface=${HOSTAPD_INTERFACE}"
    fi
    {
        echo "# HostBerry: DHCP y DNS para la red WiFi hostberry (ap0)"
        echo "# bind-interfaces: la IPv4 en ap0 la aplica hostberry-dnsmasq-prep-ap0.sh antes de arrancar dnsmasq"
        echo "# (evita carrera con hostapd y \"unknown interface ap0\" sin inet)."
        echo "# No usar loopback: listen-address=127.0.0.1 en dnsmasq.conf choca con blocky en :53."
        echo "except-interface=lo"
        echo "bind-interfaces"
        echo "interface=ap0"
        echo "listen-address=${HOSTAPD_GATEWAY}"
        echo "dhcp-authoritative"
        [ -n "$DNSMASQ_NO_DHCP_LINE" ] && echo "$DNSMASQ_NO_DHCP_LINE"
        echo "dhcp-range=${HOSTAPD_DHCP_START},${HOSTAPD_DHCP_RANGE_END},255.255.255.0,${HOSTAPD_LEASE_TIME}"
        echo "dhcp-option=3,${HOSTAPD_GATEWAY}"
        echo "dhcp-option=6,${HOSTAPD_GATEWAY}"
        echo "dhcp-option=114,http://${HOSTAPD_GATEWAY}/api/captive-portal"
        echo "address=/hostberry.local/${HOSTAPD_GATEWAY}"
        echo "address=/#/${HOSTAPD_GATEWAY}"
        echo "domain-needed"
        echo "bogus-priv"
    } > "$DNSMASQ_AP_CONFIG"
    chmod 644 "$DNSMASQ_AP_CONFIG"
    print_success "Configuración dnsmasq para hostberry escrita"
    
    # Asegurar que dnsmasq cargue /etc/dnsmasq.d (común en Debian/Ubuntu)
    DNSMASQ_CONFIG="/etc/dnsmasq.conf"
    if [ -f "$DNSMASQ_CONFIG" ]; then
        if ! grep -q '^conf-dir=' "$DNSMASQ_CONFIG" 2>/dev/null && ! grep -q '^conf-dir=' "$DNSMASQ_CONFIG" 2>/dev/null; then
            if ! grep -q 'conf-dir' "$DNSMASQ_CONFIG" 2>/dev/null; then
                echo "" >> "$DNSMASQ_CONFIG"
                echo "# Cargar configs adicionales (HostBerry AP)" >> "$DNSMASQ_CONFIG"
                echo "conf-dir=/etc/dnsmasq.d" >> "$DNSMASQ_CONFIG"
                print_info "Añadido conf-dir=/etc/dnsmasq.d a $DNSMASQ_CONFIG"
            fi
        fi
    else
        # Crear /etc/dnsmasq.conf mínimo para que dnsmasq arranque y cargue nuestro archivo
        print_info "Creando $DNSMASQ_CONFIG mínimo..."
        cat > "$DNSMASQ_CONFIG" <<EOF
# Configuración mínima para HostBerry (DHCP/DNS en ap0 via /etc/dnsmasq.d)
conf-dir=/etc/dnsmasq.d
EOF
        chmod 644 "$DNSMASQ_CONFIG"
    fi

    # El paquete dnsmasq en Debian/Raspberry Pi OS suele dejar listen-address=127.0.0.1 activo.
    # Si blocky (HostBerry), systemd-resolved u otro ya usa 127.0.0.1:53, dnsmasq falla al arrancar.
    if [ -f "$DNSMASQ_CONFIG" ]; then
        if grep -qE '^[[:space:]]*listen-address=127\.0\.0\.1(\s|$)' "$DNSMASQ_CONFIG" 2>/dev/null; then
            sed -i 's/^\([[:space:]]*\)listen-address=127\.0\.0\.1\>.*$/\1# listen-address=127.0.0.1  (desactivado HostBerry: conflicto en loopback :53)/' "$DNSMASQ_CONFIG" 2>/dev/null || true
            print_info "Ajustado $DNSMASQ_CONFIG: listen-address loopback comentado (evita conflicto con DNS en 127.0.0.1)"
        fi
        if grep -qE '^[[:space:]]*listen-address=::1(\s|$)' "$DNSMASQ_CONFIG" 2>/dev/null; then
            sed -i 's/^\([[:space:]]*\)listen-address=::1\>.*$/\1# listen-address=::1  (desactivado HostBerry)/' "$DNSMASQ_CONFIG" 2>/dev/null || true
        fi
    fi

    # Otros paquetes pueden añadir listen-address=127.0.0.1 en /etc/dnsmasq.d/*.conf
    if [ -d "$DNSMASQ_D_DIR" ]; then
        shopt -s nullglob
        for _dmq in "$DNSMASQ_D_DIR"/*.conf; do
            [ "$_dmq" = "$DNSMASQ_AP_CONFIG" ] && continue
            if grep -qE '^[[:space:]]*listen-address=127\.0\.0\.1(\s|$)' "$_dmq" 2>/dev/null; then
                sed -i 's/^\([[:space:]]*\)listen-address=127\.0\.0\.1\>.*$/\1# listen-address=127.0.0.1  (desactivado HostBerry: conflicto loopback :53)/' "$_dmq" 2>/dev/null || true
                print_info "Ajustado $_dmq: listen-address loopback comentado"
            fi
        done
        shopt -u nullglob
    fi
    
# Configurar wpa_supplicant para modo STA
print_info "Configurando wpa_supplicant para modo estación (STA)..."

# Crear directorio de configuración de wpa_supplicant
print_info "Creando directorio de configuración de wpa_supplicant..."
mkdir -p /etc/wpa_supplicant
chown root:netdev /etc/wpa_supplicant 2>/dev/null || chown root:root /etc/wpa_supplicant
chmod 755 /etc/wpa_supplicant
print_success "Directorio /etc/wpa_supplicant configurado"

# Crear directorio de socket de control de wpa_supplicant
print_info "Creando directorio de socket de control de wpa_supplicant..."
mkdir -p /var/run/wpa_supplicant
chown root:netdev /var/run/wpa_supplicant 2>/dev/null || chown root:root /var/run/wpa_supplicant
chmod 775 /var/run/wpa_supplicant
print_success "Directorio /var/run/wpa_supplicant configurado con permisos 775"

# También crear /run/wpa_supplicant (algunos sistemas usan este)
mkdir -p /run/wpa_supplicant
chown root:netdev /run/wpa_supplicant 2>/dev/null || chown root:root /run/wpa_supplicant
chmod 775 /run/wpa_supplicant
print_success "Directorio /run/wpa_supplicant configurado con permisos 775"

# Crear archivo de configuración base de wpa_supplicant (misma interfaz que la STA detectada)
WPA_CONFIG="/etc/wpa_supplicant/wpa_supplicant-${HOSTAPD_INTERFACE}.conf"
if [ ! -f "$WPA_CONFIG" ]; then
    print_info "Creando archivo de configuración de wpa_supplicant: $WPA_CONFIG"
    cat > "$WPA_CONFIG" <<EOF
ctrl_interface=DIR=/var/run/wpa_supplicant GROUP=netdev
ctrl_interface_group=netdev
update_config=1
country=US

# Redes guardadas se agregarán aquí automáticamente
EOF
    chmod 600 "$WPA_CONFIG"
    chown root:root "$WPA_CONFIG"
    print_success "Archivo de configuración de wpa_supplicant creado"
else
    print_info "Archivo de configuración de wpa_supplicant ya existe"
    # Verificar que tenga el grupo netdev en ctrl_interface
    if ! grep -q "GROUP=netdev" "$WPA_CONFIG" 2>/dev/null; then
        print_info "Actualizando archivo de configuración para incluir GROUP=netdev..."
        sed -i 's|ctrl_interface=DIR=/var/run/wpa_supplicant|ctrl_interface=DIR=/var/run/wpa_supplicant GROUP=netdev|g' "$WPA_CONFIG" 2>/dev/null || true
    fi
fi
    
    # Crear script + servicio systemd para ap0 al arrancar (iw + phy; MAC opcional en instalación)
    if command -v iw &> /dev/null && [ -n "$PHY_NAME" ]; then
        CREATE_AP0_SCRIPT="/usr/local/sbin/hostberry-create-ap0.sh"
        AP0_SERVICE="/etc/systemd/system/create-ap0.service"
        IW_BIN="$(command -v iw 2>/dev/null || true)"
        if [ -z "$IW_BIN" ]; then
            for _iwtry in /usr/sbin/iw /sbin/iw; do
                if [ -x "$_iwtry" ]; then
                    IW_BIN="$_iwtry"
                    break
                fi
            done
        fi
        print_info "Instalando script y unidad systemd para crear ap0 al arrancar..."
        cat > "$CREATE_AP0_SCRIPT" <<EOF
#!/bin/bash
# Generado por HostBerry: crea ap0 cuando nl80211 y la interfaz STA están listos.
# "command failed: No such file or directory (-2)" de iw = ENOENT nl80211 (phy no listo / nombre incorrecto).
if command -v rfkill >/dev/null 2>&1; then
    rfkill unblock wifi 2>/dev/null || true
fi
set -u
WLAN_IF="$HOSTAPD_INTERFACE"
IW_BIN="$IW_BIN"
HOSTAPD_GATEWAY="$HOSTAPD_GATEWAY"
MAC_FALLBACK="$MAC_ADDRESS"


hostberry_resolve_phy() {
    local p=""
    if [ -r "/sys/class/net/\${WLAN_IF}/phy80211/name" ]; then
        p=\$(tr -d '\n' < "/sys/class/net/\${WLAN_IF}/phy80211/name" 2>/dev/null || true)
    fi
    if [ -z "\$p" ] && [ -d /sys/class/ieee80211 ]; then
        p=\$(basename "\$(ls -d /sys/class/ieee80211/phy* 2>/dev/null | head -1)" 2>/dev/null || true)
    fi
    case "\$p" in
        phy*) echo "\$p" ;;
        [0-9]*) echo "phy\${p}" ;;
        *)      echo "phy0" ;;
    esac
}

if [ ! -x "\$IW_BIN" ]; then
    echo "hostberry-create-ap0: no se ejecuta \$IW_BIN (instala el paquete iw)" >&2
    exit 1
fi

# Esperar a que exista wlan y nl80211 acepte comandos (puede tardar tras udev).
for _ in \$(seq 1 120); do
    if [ -d "/sys/class/net/\${WLAN_IF}" ]; then
        /bin/ip link set "\${WLAN_IF}" up 2>/dev/null || true
        PHY="\$(hostberry_resolve_phy)"
        if [ -n "\$PHY" ] && [ -d "/sys/class/ieee80211/\${PHY}" ] && "\$IW_BIN" phy "\$PHY" info >/dev/null 2>&1; then
            break
        fi
    fi
    sleep 1
done

PHY="\$(hostberry_resolve_phy)"
MAC="\${MAC_FALLBACK}"
if [ -z "\$MAC" ] || [ "\$MAC" = "00:00:00:00:00:00" ]; then
    MAC=\$(tr -d '\n' < "/sys/class/net/\${WLAN_IF}/address" 2>/dev/null || true)
fi

/bin/ip link set "\${WLAN_IF}" up 2>/dev/null || true
"\$IW_BIN" dev "\${WLAN_IF}" set power_save off 2>/dev/null || true

# ap0 puede existir en "ip link" pero no en nl80211 (iw) → ENODEV; hay que borrarla y recrear.
if ip link show ap0 >/dev/null 2>&1; then
    if ! "\$IW_BIN" dev ap0 info >/dev/null 2>&1; then
        echo "hostberry-create-ap0: ap0 rota (ip sí, iw no); recreando…" >&2
        /bin/ip link set ap0 down 2>/dev/null || true
        "\$IW_BIN" dev ap0 del 2>/dev/null || true
        sleep 1
    else
        /bin/ip link set ap0 up 2>/dev/null || true
    fi
fi

if ! ip link show ap0 >/dev/null 2>&1; then
    ok=0
    for _ in \$(seq 1 15); do
        if "\$IW_BIN" phy "\$PHY" interface add ap0 type __ap 2>/dev/null; then
            ok=1
            break
        fi
        if "\$IW_BIN" dev "\${WLAN_IF}" interface add ap0 type __ap 2>/dev/null; then
            ok=1
            break
        fi
        sleep 2
    done
    if [ "\$ok" -ne 1 ]; then
        echo "hostberry-create-ap0: no se pudo crear ap0 (phy=\$PHY wlan=\${WLAN_IF})" >&2
        "\$IW_BIN" phy "\$PHY" info >&2 || true
        exit 1
    fi
    if [ -n "\$MAC" ]; then
        /bin/ip link set ap0 address "\$MAC" 2>/dev/null || true
    fi
    /bin/ip link set ap0 up
fi

"\$IW_BIN" dev ap0 set power_save off 2>/dev/null || true
/bin/ip link set ap0 up 2>/dev/null || true

/bin/ip addr add "\${HOSTAPD_GATEWAY}/24" dev ap0 2>/dev/null || true
if ! ip link show ap0 >/dev/null 2>&1; then
    echo "hostberry-create-ap0: ap0 no existe tras el script" >&2
    exit 1
fi
if ! "\$IW_BIN" dev ap0 info >/dev/null 2>&1; then
    echo "hostberry-create-ap0: ap0 no es visible para nl80211 (iw dev ap0 info falla)" >&2
    exit 1
fi
exit 0
EOF
        chmod 755 "$CREATE_AP0_SCRIPT"
        chown root:root "$CREATE_AP0_SCRIPT"
        cat > "$AP0_SERVICE" <<EOF
[Unit]
Description=Create virtual WiFi interface ap0 for AP+STA mode
After=network-pre.target systemd-udevd.service
Before=hostapd.service
Wants=network-pre.target

[Service]
Type=oneshot
RemainAfterExit=yes
Environment=PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/sbin:/usr/bin:/bin
ExecStart=${CREATE_AP0_SCRIPT}
# Sin ExecStop destructivo: si se hace "systemctl stop create-ap0", borrar ap0 rompe hostapd y "iw dev ap0" da ENODEV (-19).

[Install]
WantedBy=multi-user.target
EOF
        chmod 644 "$AP0_SERVICE"
        systemctl daemon-reload 2>/dev/null || true
        systemctl enable create-ap0.service 2>/dev/null || true
        if [ "${HOSTBERRY_DEFER_AP_START:-0}" = "1" ]; then
            print_success "Servicio systemd para ap0 actualizado y habilitado (inicio diferido hasta reinicio)"
        else
            systemctl start create-ap0.service 2>/dev/null || true
            print_success "Servicio systemd para ap0 actualizado, habilitado e iniciado"
            sleep 2
            if ip link show ap0 > /dev/null 2>&1; then
                print_success "Interfaz ap0 creada y verificada por el servicio systemd"
            else
                print_warning "El servicio se inició pero ap0 no está disponible aún (puede necesitar reinicio)"
            fi
        fi
    else
        # Sin iw, hostapd.conf usa interfaz física; ExecStartPre debe existir o hostapd falla al arrancar.
        print_warning "iw no está disponible: no se puede crear ap0; instalando script mínimo y unidad create-ap0 (no-op)."
        CREATE_AP0_SCRIPT="/usr/local/sbin/hostberry-create-ap0.sh"
        cat > "$CREATE_AP0_SCRIPT" <<'HOSTBERRY_AP0_STUB'
#!/bin/bash
# HostBerry: sin iw no hay interfaz virtual ap0; hostapd usa la interfaz física configurada en hostapd.conf.
exit 0
HOSTBERRY_AP0_STUB
        chmod 755 "$CREATE_AP0_SCRIPT"
        chown root:root "$CREATE_AP0_SCRIPT"
        AP0_SERVICE="/etc/systemd/system/create-ap0.service"
        cat > "$AP0_SERVICE" <<EOF
[Unit]
Description=HostBerry create-ap0 stub (iw not installed)
After=network-pre.target
Before=hostapd.service

[Service]
Type=oneshot
RemainAfterExit=yes
ExecStart=${CREATE_AP0_SCRIPT}

[Install]
WantedBy=multi-user.target
EOF
        chmod 644 "$AP0_SERVICE"
        systemctl daemon-reload 2>/dev/null || true
        systemctl enable create-ap0.service 2>/dev/null || true
        if [ "${HOSTBERRY_DEFER_AP_START:-0}" != "1" ]; then
            systemctl start create-ap0.service 2>/dev/null || true
        fi
    fi
    
    # Override hostapd: la unidad base suele usar Type=forking + PIDFile=/run/hostapd.pid.
    # Sin -P, hostapd en segundo plano no escribe ese PID y systemd avisa (aunque el AP funcione).
    OVERRIDE_DIR="/etc/systemd/system/hostapd.service.d"
    OVERRIDE_FILE="${OVERRIDE_DIR}/override.conf"
    print_info "Escribiendo override de systemd para hostapd (PID coherente con Type=forking)..."
    mkdir -p "$OVERRIDE_DIR"
    cat > "$OVERRIDE_FILE" <<EOF
[Unit]
After=create-ap0.service
Wants=create-ap0.service

[Service]
# create-ap0 es oneshot RemainAfterExit: si ap0 desaparece después, no se vuelve a ejecutar.
# Antes de hostapd, asegurar ap0 (idempotente) + canal alineado con wlan0.
ExecStartPre=/usr/local/sbin/hostberry-create-ap0.sh
ExecStartPre=${SYNC_HOSTAPD_CH}
ExecStart=
ExecStart=/usr/sbin/hostapd -B -P /run/hostapd.pid ${HOSTAPD_CONFIG}
PIDFile=/run/hostapd.pid
Type=forking
TimeoutStartSec=120
EOF
    chmod 644 "$OVERRIDE_FILE"
    print_success "Override de hostapd actualizado"

    # Debian/Ubuntu: /etc/init.d/hostapd y herramientas leen DAEMON_CONF; sin esto a veces el servicio no usa nuestra conf.
    HOSTAPD_DEFAULT="/etc/default/hostapd"
    if [ -f "$HOSTAPD_DEFAULT" ] || [ -d "/etc/hostapd" ]; then
        print_info "Asegurando ${HOSTAPD_DEFAULT} (DAEMON_CONF → ${HOSTAPD_CONFIG})…"
        if [ -f "$HOSTAPD_DEFAULT" ]; then
            sed -i '/^DAEMON_CONF=/d' "$HOSTAPD_DEFAULT" 2>/dev/null || true
            echo "DAEMON_CONF=\"$HOSTAPD_CONFIG\"" >> "$HOSTAPD_DEFAULT"
            grep -q '^RUN_DAEMON=' "$HOSTAPD_DEFAULT" 2>/dev/null || echo 'RUN_DAEMON=yes' >> "$HOSTAPD_DEFAULT"
        else
            cat > "$HOSTAPD_DEFAULT" <<HDEOF
# HostBerry: hostapd usa esta configuración (coherente con systemd override)
RUN_DAEMON=yes
DAEMON_CONF="$HOSTAPD_CONFIG"
HDEOF
            chmod 644 "$HOSTAPD_DEFAULT"
        fi
        print_success "Actualizado $HOSTAPD_DEFAULT"
    fi

    # NetworkManager suele “poseer” wlan* y hostapd no puede crear ap0 / emitir SSID hostberry.
    NM_DROPIN="/etc/NetworkManager/conf.d/99-hostberry-unmanaged.conf"
    if [ -d /etc/NetworkManager/conf.d ]; then
        # Sólo ap0: si marcamos wlan* como unmanaged, NetworkManager suelta el cliente WiFi y se corta SSH por WiFi.
        print_info "Excluyendo solo ap0 de NetworkManager (wlan* sigue gestionada para no perder la conexión)…"
        cat > "$NM_DROPIN" <<'NMEOF'
[keyfile]
# HostBerry: sólo la interfaz virtual del AP; wlan* la sigue gestionando NM/wpa_supplicant (cliente WiFi).
unmanaged-devices=interface-name:ap0
NMEOF
        chmod 644 "$NM_DROPIN"
        print_success "Creado $NM_DROPIN (reinicia NetworkManager o el sistema para aplicar)"
    fi
    
    print_info "Verificando estado del servicio hostapd…"
    if systemctl is-enabled hostapd 2>&1 | grep -q "masked"; then
        print_info "Desbloqueando servicio hostapd…"
        systemctl unmask hostapd 2>/dev/null || true
        print_success "Servicio hostapd desbloqueado"
    fi
    
    systemctl daemon-reload 2>/dev/null || true
    
    # Script: asigna IPv4 en ap0 antes de dnsmasq (evita carrera con hostapd activo pero Post aún no ejecutado).
    DNSMASQ_PREP_SCRIPT="/usr/local/sbin/hostberry-dnsmasq-prep-ap0.sh"
    print_info "Instalando ${DNSMASQ_PREP_SCRIPT}…"
    cat > "/tmp/hostberry-dnsmasq-prep-ap0.sh" <<EOSCRIPT
#!/bin/bash
GW="${HOSTAPD_GATEWAY}"
for i in \$(seq 1 160); do
  if ip link show ap0 >/dev/null 2>&1; then
    ip addr replace "\${GW}/24" dev ap0 2>/dev/null || ip addr add "\${GW}/24" dev ap0 2>/dev/null || true
    sleep 0.3
    if ip -4 addr show dev ap0 2>/dev/null | grep -qE ' inet '; then
      exit 0
    fi
  fi
  sleep 0.25
done
echo "HostBerry: ap0 no disponible o sin IPv4 (\${GW}/24)." >&2
exit 1
EOSCRIPT
    mkdir -p "$(dirname "$DNSMASQ_PREP_SCRIPT")"
    cp "/tmp/hostberry-dnsmasq-prep-ap0.sh" "$DNSMASQ_PREP_SCRIPT"
    chmod 755 "$DNSMASQ_PREP_SCRIPT"
    chown root:root "$DNSMASQ_PREP_SCRIPT"
    rm -f "/tmp/hostberry-dnsmasq-prep-ap0.sh"

    # Paquete dnsmasq + unidad systemd (dnsmasq-base no reparte DHCP por sí solo).
    hostberry_ensure_dnsmasq_ready

    # dnsmasq: tras hostapd; override del paquete o hostberry-dnsmasq.service de respaldo.
    DNSMASQ_UNIT="$(hostberry_dnsmasq_unit_name)"
    DNSMASQ_OVERRIDE_DIR="/etc/systemd/system/${DNSMASQ_UNIT}.d"
    DNSMASQ_OVERRIDE_FILE="${DNSMASQ_OVERRIDE_DIR}/hostberry.conf"
    print_info "Configurando override systemd para ${DNSMASQ_UNIT} (prep ap0 + tras hostapd)…"
    mkdir -p "$DNSMASQ_OVERRIDE_DIR"
    cat > "$DNSMASQ_OVERRIDE_FILE" <<EOF
[Unit]
After=network-online.target create-ap0.service hostapd.service
# Sin Wants=hostapd: dnsmasq no debe pararse cuando hostapd se reinicia.

[Service]
Restart=always
RestartSec=3
# El guión '-' evita que un fallo temporal de ap0 impida arrancar dnsmasq (se reintenta al reiniciar).
ExecStartPre=-${DNSMASQ_PREP_SCRIPT}
EOF
    chmod 644 "$DNSMASQ_OVERRIDE_FILE"
    print_success "Override de ${DNSMASQ_UNIT} actualizado"
    
    print_info "HostAPD y dnsmasq configurados; se habilitan para el arranque y enable_and_start_hostberry_wifi_ap inicia el AP al final."
    systemctl daemon-reload 2>/dev/null || true
    
    # Asegurar permisos correctos del archivo de configuración
    chmod 644 "$HOSTAPD_CONFIG" 2>/dev/null || true
    
    # ----- Portal cautivo: redirigir HTTP (80) desde ap0 al puerto de la web de Hostberry -----
    CAPTIVE_SCRIPT="${INSTALL_DIR}/scripts/captive-portal-setup.sh"
    CAPTIVE_SYSTEM_SCRIPT="/usr/local/sbin/hostberry-captive-port-setup.sh"
    CAPTIVE_SERVICE="/etc/systemd/system/hostberry-captive-portal.service"
    mkdir -p "${INSTALL_DIR}/scripts"
    print_info "Creando/actualizando script de portal cautivo (IP en ap0, DHCP, HTTP -> web Hostberry)..."
    cat > "/tmp/hostberry-captive-port-setup.sh" <<'CAPTIVE_EOF'
#!/bin/bash
# HostBerry: portal cautivo en ap0 — DNS engaña dominios externos, iptables captura HTTP/HTTPS.
set -euo pipefail

CONFIG_FILE="/opt/hostberry/config.yaml"
GW="192.168.4.1"
IFACE="ap0"

hostberry_captive_target_port() {
    local config="${1:-$CONFIG_FILE}"
    local main_port=8000 http_redir=0 tls_cert="" tls_key=""
    if [ ! -f "$config" ]; then
        echo 8000
        return
    fi
    main_port=$(awk '/^[[:space:]]*port:/{gsub(/"/,"",$2); print $2; exit}' "$config")
    http_redir=$(awk '/^[[:space:]]*http_redirect_port:/{gsub(/"/,"",$2); print $2; exit}' "$config")
    tls_cert=$(awk '/^[[:space:]]*tls_cert_file:/{gsub(/"/,"",$2); print $2; exit}' "$config")
    tls_key=$(awk '/^[[:space:]]*tls_key_file:/{gsub(/"/,"",$2); print $2; exit}' "$config")
    main_port=${main_port:-8000}
    if [ -n "$http_redir" ] && [ "$http_redir" != "0" ]; then
        echo "$http_redir"
        return
    fi
    if [ -n "$tls_cert" ] && [ -f "$tls_cert" ] && [ -n "$tls_key" ] && [ -f "$tls_key" ]; then
        echo 80
        return
    fi
    echo "$main_port"
}

hostberry_captive_clear_redirect() {
    local iface="$1" dport="$2"
    local p
    for p in 8000 443 8443 8080 80 4433; do
        while iptables -t nat -C PREROUTING -i "$iface" -p tcp --dport "$dport" -j REDIRECT --to-ports "$p" 2>/dev/null; do
            iptables -t nat -D PREROUTING -i "$iface" -p tcp --dport "$dport" -j REDIRECT --to-ports "$p" 2>/dev/null || break
        done
    done
}

hostberry_captive_add_redirect() {
    local iface="$1" dport="$2" target="$3"
    hostberry_captive_clear_redirect "$iface" "$dport"
    if ! iptables -t nat -C PREROUTING -i "$iface" -p tcp --dport "$dport" -j REDIRECT --to-ports "$target" 2>/dev/null; then
        iptables -t nat -A PREROUTING -i "$iface" -p tcp --dport "$dport" -j REDIRECT --to-ports "$target"
    fi
}

# Rechaza un puerto TCP entrante con RST inmediato (idempotente).
hostberry_captive_reject_tcp() {
    local iface="$1" dport="$2"
    hostberry_captive_clear_redirect "$iface" "$dport"
    while iptables -C INPUT -i "$iface" -p tcp --dport "$dport" -j REJECT --reject-with tcp-reset 2>/dev/null; do
        iptables -D INPUT -i "$iface" -p tcp --dport "$dport" -j REJECT --reject-with tcp-reset 2>/dev/null || break
    done
    iptables -I INPUT 1 -i "$iface" -p tcp --dport "$dport" -j REJECT --reject-with tcp-reset
}

# Rechaza un puerto UDP entrante con ICMP port-unreachable (idempotente).
hostberry_captive_reject_udp() {
    local iface="$1" dport="$2"
    while iptables -C INPUT -i "$iface" -p udp --dport "$dport" -j REJECT --reject-with icmp-port-unreachable 2>/dev/null; do
        iptables -D INPUT -i "$iface" -p udp --dport "$dport" -j REJECT --reject-with icmp-port-unreachable 2>/dev/null || break
    done
    iptables -I INPUT 1 -i "$iface" -p udp --dport "$dport" -j REJECT --reject-with icmp-port-unreachable
}

if ! command -v iptables >/dev/null 2>&1; then
    echo "HostBerry: iptables no disponible" >&2
    exit 1
fi

if ! ip link show "$IFACE" >/dev/null 2>&1; then
    echo "HostBerry: interfaz ${IFACE} no disponible (portal cautivo omitido)" >&2
    exit 0
fi

ip addr replace "${GW}/24" dev "$IFACE" 2>/dev/null || ip addr add "${GW}/24" dev "$IFACE" 2>/dev/null || true

if [ -x /usr/local/sbin/hostberry-restart-dnsmasq.sh ]; then
    /usr/local/sbin/hostberry-restart-dnsmasq.sh 2>/dev/null || true
else
    systemctl start dnsmasq.service 2>/dev/null || true
    systemctl start hostberry-dnsmasq.service 2>/dev/null || true
fi

PORT="$(hostberry_captive_target_port)"
hostberry_captive_add_redirect "$IFACE" 80 "$PORT"

# El sondeo HTTPS de detección de portal debe FALLAR LIMPIO (RST), no recibir un certificado
# inválido. Así Android/iOS recientes muestran el portal de forma fiable; el portal se sirve
# por HTTP (sondeo HTTP redirigido al puerto del servidor).
hostberry_captive_reject_tcp "$IFACE" 443

# DNS privado de Android (DNS-over-TLS, TCP/853; DNS-over-QUIC, UDP/853): rechazar para que el
# móvil caiga al DNS de texto plano del portal (dnsmasq) y resuelva los dominios de detección
# (connectivitycheck.gstatic.com, etc.) hacia el gateway. Sin esto, con "DNS privado: Automático"
# el teléfono no consulta dnsmasq y el portal cautivo nunca se abre.
hostberry_captive_reject_tcp "$IFACE" 853
hostberry_captive_reject_udp "$IFACE" 853

echo "HostBerry: portal cautivo activo (DNS→${GW}, ${IFACE}:80→:${PORT}, 443/853→RST)"
exit 0
CAPTIVE_EOF
    cp "/tmp/hostberry-captive-port-setup.sh" "$CAPTIVE_SYSTEM_SCRIPT"
    cp "/tmp/hostberry-captive-port-setup.sh" "$CAPTIVE_SCRIPT"
    chmod 755 "$CAPTIVE_SYSTEM_SCRIPT" "$CAPTIVE_SCRIPT"
    chown root:root "$CAPTIVE_SYSTEM_SCRIPT" "$CAPTIVE_SCRIPT"
    rm -f "/tmp/hostberry-captive-port-setup.sh"
    print_success "Script de portal cautivo actualizado: $CAPTIVE_SYSTEM_SCRIPT"
    print_info "Actualizando unidad systemd del portal cautivo (tras HostBerry y dnsmasq)…"
    cat > "$CAPTIVE_SERVICE" <<EOF
[Unit]
Description=HostBerry captive portal - DNS hijack + HTTP redirect + HTTPS reset on AP
After=network.target create-ap0.service hostapd.service ${DNSMASQ_UNIT} ${SERVICE_NAME}.service
Wants=hostapd.service ${SERVICE_NAME}.service

[Service]
Type=oneshot
RemainAfterExit=yes
ExecStart=${CAPTIVE_SYSTEM_SCRIPT}

[Install]
WantedBy=multi-user.target
EOF
    chmod 644 "$CAPTIVE_SERVICE"
    systemctl daemon-reload 2>/dev/null || true
    systemctl enable hostberry-captive-portal.service 2>/dev/null || true
    print_info "El portal cautivo se inicia tras arrancar HostBerry (ver enable_and_start_hostberry_wifi_ap o reinicio)."
    print_success "Servicio de portal cautivo registrado y habilitado"
    
    print_success "Configuración por defecto de HostAPD creada"
}

# Blocky en "dns: 53" escucha en *:53 y deja sin puerto a dnsmasq en ap0 (portal cautivo / DHCP).
# Sólo loopback: el LAN usa dnsmasq en la IP del AP; la app HostBerry habla con Blocky vía 127.0.0.1.

