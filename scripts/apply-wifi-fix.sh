#!/bin/bash
# Aplica el binario con correcciones WiFi/wpa_supplicant y reinicia HostBerry.
set -euo pipefail

SRC="/tmp/hostberry-new"
DEST="/opt/hostberry/hostberry"
DNSMASQ_AP="/etc/dnsmasq.d/hostberry-ap.conf"

if [ ! -f "$SRC" ]; then
	echo "Compilando HostBerry..." >&2
	(cd /home/hostberry/hostberry-project-main && go build -o "$SRC" .)
fi

if [ -f "$DNSMASQ_AP" ] && ! grep -q 'address=/hostberry.local/' "$DNSMASQ_AP"; then
	echo "Añadiendo hostberry.local → 192.168.4.1 en dnsmasq..."
	sed -i '/^address=\/#\//i address=/hostberry.local/192.168.4.1' "$DNSMASQ_AP"
	systemctl restart dnsmasq
fi

echo "Deteniendo servicio..."
systemctl stop hostberry
cp "$SRC" "$DEST"
chown hostberry:hostberry "$DEST"
chmod 755 "$DEST"
echo "Iniciando servicio..."
systemctl start hostberry
systemctl restart hostberry-captive-portal 2>/dev/null || true
systemctl is-active --quiet hostberry && echo "✅ hostberry activo"
