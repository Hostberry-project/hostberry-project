#!/bin/bash
# Carga ordenada de módulos del instalador HostBerry.
# Requiere: SCRIPT_DIR (raíz del proyecto).

_LIB_DIR="${SCRIPT_DIR}/scripts/install/lib"
_MODULES=(
    common
    config
    args
    system
    project
    user_files
    update_safe
    release_binary
    build
    tls
    firewall_db
    hostapd
    network
    service
    finish
    uninstall
    main
)

for _mod in "${_MODULES[@]}"; do
    _path="${_LIB_DIR}/${_mod}.sh"
    if [ ! -f "$_path" ]; then
        echo "Error: módulo del instalador no encontrado: $_path" >&2
        exit 1
    fi
    # shellcheck source=/dev/null
    source "$_path"
done
unset _mod _path _MODULES _LIB_DIR
