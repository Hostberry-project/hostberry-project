#!/bin/bash
# Módulo: args.sh — modos de operación y parsing de argumentos.

is_default_route_over_wifi() {
    local dev
    dev="$(ip route 2>/dev/null | awk '/^default/ {print $5; exit}')"
    if [ -z "$dev" ]; then
        echo 0
        return
    fi
    case "$dev" in
        wl*|wlan*) echo 1 ;;
        *)         echo 0 ;;
    esac
}

# Interfaz local por la que entra la sesión SSH (ej. wlan0, eth0).
hostberry_ssh_server_interface() {
    [ -n "${SSH_CONNECTION:-}" ] || return 1
    local host_ip ssh_dev
    host_ip="$(echo "$SSH_CONNECTION" | awk '{print $3}')"
    [ -n "$host_ip" ] || return 1
    ssh_dev="$(ip -o addr show 2>/dev/null | awk -v ip="$host_ip" '$4 ~ "^" ip "/" { print $2; exit }')"
    [ -n "$ssh_dev" ] || return 1
    printf '%s' "$ssh_dev"
}

hostberry_ssh_session_over_wifi() {
    local dev
    dev="$(hostberry_ssh_server_interface 2>/dev/null)" || return 1
    case "$dev" in
        wl*|wlan*) return 0 ;;
        *)         return 1 ;;
    esac
}

# Durante install/update: no levantar hostapd/ap0 en caliente si cortaría SSH (p. ej. instalación remota por WiFi).
hostberry_defer_ap_during_install() {
    case "${HOSTBERRY_START_AP_NOW:-auto}" in
        1|true|yes) return 1 ;;
        0|false|no) return 0 ;;
    esac
    if [ "${HOSTBERRY_SKIP_AP_START:-0}" = "1" ]; then
        return 0
    fi
    [ -n "${SSH_CONNECTION:-}" ] || return 1
    if hostberry_ssh_session_over_wifi; then
        return 0
    fi
    [ "$(is_default_route_over_wifi)" = "1" ]
}

# Modo: install | update | remove
MODE="install"

show_usage() {
    if [ "$HB_INSTALL_LANG" = "en" ]; then
        echo "Usage: $0 [OPTION]"
        echo ""
        echo "Options:"
        echo "  --install      Install HostBerry (default)"
        echo "  --update       Update (preserves data); daemon-reload and reboot at the end"
        echo "                 (skip reboot: HOSTBERRY_SKIP_REBOOT=1 sudo $0 --update)"
        echo "  --remove       Remove HostBerry (service, files, user, logs)"
        echo "  -h, --help     Show this help"
        echo ""
        echo "Legacy aliases: (no args) = --install; --uninstall = --remove"
        echo ""
        echo "Language: LANG/LC_MESSAGES (es_* → Spanish; otherwise English)."
        echo "Override: HOSTBERRY_INSTALL_LANG=es|en|auto"
        echo "Fast install: HOSTBERRY_FAST_INSTALL=1 (skip VPN extras, Blocky, LibreSpeed)"
        echo "SSH over WiFi: AP starts after reboot (HOSTBERRY_START_AP_NOW=1 to force now)"
        echo ""
        echo "Examples:"
        echo "  sudo $0 --install"
        echo "  sudo $0 --update"
        echo "  sudo $0 --remove"
    else
        echo "Uso: $0 [OPCIÓN]"
        echo ""
        echo "Opciones:"
        echo "  --install      Instalar HostBerry (por defecto)"
        echo "  --update       Actualizar (preserva datos); al terminar reinicia el sistema"
        echo "                 (omitir reinicio: HOSTBERRY_SKIP_REBOOT=1 sudo $0 --update)"
        echo "  --remove       Desinstalar HostBerry (servicio, archivos, usuario, logs)"
        echo "  -h, --help     Mostrar esta ayuda"
        echo ""
        echo "Alias heredados: (sin args) = --install; --uninstall = --remove"
        echo ""
        echo "Idioma: LANG/LC_MESSAGES (es_* → español; si no, inglés)."
        echo "Forzar: HOSTBERRY_INSTALL_LANG=es|en|auto"
        echo "Instalación rápida: HOSTBERRY_FAST_INSTALL=1 (omite VPN, Blocky, LibreSpeed)"
        echo "SSH por WiFi: el AP arranca tras reinicio (HOSTBERRY_START_AP_NOW=1 para forzar ahora)"
        echo ""
        echo "Ejemplos:"
        echo "  sudo $0 --install"
        echo "  sudo $0 --update"
        echo "  sudo $0 --remove"
    fi
    exit 0
}

# Procesar argumentos (solo un modo explícito; sin args = install)
_explicit_mode=0
while [[ "$#" -gt 0 ]]; do
    case $1 in
        --install)   MODE="install"; _explicit_mode=1 ;;
        --update)    MODE="update"; _explicit_mode=1 ;;
        --remove|--uninstall) MODE="remove"; _explicit_mode=1 ;;
        -h|--help)   show_usage ;;
        *)
            print_error "Opción desconocida: $1. Usa --help para ver opciones."
            exit 1
            ;;
    esac
    shift
done
unset _explicit_mode
