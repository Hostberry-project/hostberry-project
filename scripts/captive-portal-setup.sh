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
