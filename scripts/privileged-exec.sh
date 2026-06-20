#!/bin/bash
# HostBerry — ejecutor privilegiado con allowlist (única entrada sudoers recomendada).
set -euo pipefail

if [ "$#" -lt 1 ]; then
	echo "usage: privileged-exec <command>" >&2
	exit 1
fi

CMD="$1"

# Caracteres peligrosos (alineado con internal/utils validateShellCommandAllowList).
if [[ "$CMD" == *";"* || "$CMD" == *$'\n'* || "$CMD" == *$'\r'* || "$CMD" == *'`'* || "$CMD" == *'$'* ]]; then
	echo "command rejected" >&2
	exit 1
fi
if [[ "$CMD" == *"<<"* || "$CMD" == *">>"* || "$CMD" == *"<"* ]]; then
	echo "command rejected" >&2
	exit 1
fi

allowed_tokens=(
	hostname hostnamectl uname cat grep awk sed cut head tail journalctl
	top free df nproc iwlist nmcli iw ip wg wg-quick systemctl pgrep
	reboot shutdown poweroff rfkill ifconfig iwconfig
	hostapd hostapd_cli dnsmasq iptables iptables-save netfilter-persistent sysctl tee cp mkdir echo chmod chown rm
	dhclient udhcpc wpa_supplicant wpa_cli pkill killall true mount
	apt-get apt
)

first="${CMD%% *}"
first="${first#sudo }"
base="${first##*/}"

ok=0
for t in "${allowed_tokens[@]}"; do
	if [ "$base" = "$t" ]; then
		ok=1
		break
	fi
done
if [ "$ok" -ne 1 ]; then
	echo "command not allowed: $base" >&2
	exit 1
fi

exec /bin/sh -c "$CMD"
