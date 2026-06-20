#!/bin/bash
# Funciones de backup/verificación/rollback del binario HostBerry.

hostberry_backup_binary() {
    local bin="${INSTALL_DIR}/hostberry"
    [ -f "$bin" ] || return 0
    cp -f "$bin" "${INSTALL_DIR}/hostberry.prev"
    if command -v sha256sum &>/dev/null; then
        sha256sum "$bin" | awk '{print $1}' > "${INSTALL_DIR}/hostberry.prev.sha256"
    fi
}

hostberry_verify_binary() {
    local bin="${INSTALL_DIR}/hostberry"
    [ -x "$bin" ] || return 1
    if ! "$bin" -version >/dev/null 2>&1; then
        return 1
    fi
    if command -v sha256sum &>/dev/null && [ -f "${bin}.sha256" ]; then
        local expected actual
        expected="$(cat "${bin}.sha256")"
        actual="$(sha256sum "$bin" | awk '{print $1}')"
        [ "$expected" = "$actual" ] || return 1
    fi
    return 0
}

hostberry_store_binary_checksum() {
    local bin="${INSTALL_DIR}/hostberry"
    command -v sha256sum &>/dev/null || return 0
    [ -f "$bin" ] || return 0
    sha256sum "$bin" | awk '{print $1}' > "${bin}.sha256"
}

hostberry_rollback_binary() {
    local prev="${INSTALL_DIR}/hostberry.prev"
    [ -f "$prev" ] || return 1
    cp -f "$prev" "${INSTALL_DIR}/hostberry"
    chmod +x "${INSTALL_DIR}/hostberry"
    print_warning "Rollback: restaurado binario anterior (hostberry.prev)"
    return 0
}
