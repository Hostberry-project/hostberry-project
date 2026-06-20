#!/bin/bash
# Reparación rápida: DHCP en la red hostberry (dnsmasq + ap0).
set -euo pipefail

if [ "$EUID" -ne 0 ]; then
    echo "Ejecuta con: sudo $0"
    exit 1
fi

OVERRIDE="/etc/systemd/system/dnsmasq.service.d/hostberry.conf"
mkdir -p "$(dirname "$OVERRIDE")"
cat > "$OVERRIDE" <<'EOF'
[Unit]
After=network-online.target create-ap0.service hostapd.service

[Service]
Restart=always
RestartSec=3
ExecStartPre=-/usr/local/sbin/hostberry-dnsmasq-prep-ap0.sh
EOF

cat > /usr/local/sbin/hostberry-restart-dnsmasq.sh <<'EOF'
#!/bin/bash
if systemctl cat dnsmasq.service &>/dev/null; then
    systemctl is-active --quiet dnsmasq.service && exit 0
    systemctl start dnsmasq.service
fi
EOF
chmod 755 /usr/local/sbin/hostberry-restart-dnsmasq.sh

# Quitar reinicio de dnsmasq en hostapd (provoca paradas en cadena)
OVERRIDE_H="/etc/systemd/system/hostapd.service.d/override.conf"
if [ -f "$OVERRIDE_H" ]; then
    sed -i '/hostberry-restart-dnsmasq/d' "$OVERRIDE_H"
fi

systemctl daemon-reload
systemctl start create-ap0.service 2>/dev/null || true
ip addr replace 192.168.4.1/24 dev ap0 2>/dev/null || ip addr add 192.168.4.1/24 dev ap0 2>/dev/null || true
systemctl restart hostapd.service 2>/dev/null || true
sleep 2
systemctl enable dnsmasq.service
systemctl restart dnsmasq.service

if ss -ulnp | grep -q ':67 '; then
    echo "[+] DHCP activo en UDP/67 — los clientes deberían recibir 192.168.4.x"
else
    echo "[!] DHCP no escucha en :67. Revisa: journalctl -u dnsmasq -n 30"
    exit 1
fi
