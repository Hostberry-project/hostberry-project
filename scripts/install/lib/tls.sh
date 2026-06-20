#!/bin/bash
# Módulo: tls.sh
install_mkcert_binary() {
    if command -v mkcert &>/dev/null; then
        print_success "mkcert disponible"
        return 0
    fi
    print_info "mkcert no encontrado; descargando binario..."
    local ver="v1.4.4"
    local arch march url tmp
    arch="$(uname -m)"
    case "$arch" in
        x86_64|amd64) march="amd64" ;;
        aarch64) march="arm64" ;;
        armv7l|armhf|armv6l) march="arm" ;;
        *)
            print_error "Arquitectura no soportada para mkcert: $arch"
            return 1
            ;;
    esac
    url="https://github.com/FiloSottile/mkcert/releases/download/${ver}/mkcert-${ver}-linux-${march}"
    tmp="$(mktemp)"
    print_info "Descargando mkcert (${march})..."
    if wget -q -O "$tmp" "$url" 2>/dev/null || curl -sL -f -o "$tmp" "$url"; then
        install -m 0755 "$tmp" /usr/local/bin/mkcert
        rm -f "$tmp"
        print_success "mkcert instalado en /usr/local/bin/mkcert"
        return 0
    fi
    rm -f "$tmp"
    print_error "No se pudo instalar mkcert"
    return 1
}

# Migra config antigua 8443/8000 → 443/80 (URLs sin puerto explícito).

migrate_hostberry_tls_standard_ports() {
    [ -f "$CONFIG_FILE" ] || return 0
    if ! grep -qE '^[[:space:]]*tls_cert_file:.*hostberry\.pem' "$CONFIG_FILE" 2>/dev/null; then
        return 0
    fi
    sed -i 's/^  port: 8443$/  port: 443/' "$CONFIG_FILE" 2>/dev/null || true
    sed -i 's/^  http_redirect_port: 8000$/  http_redirect_port: 80/' "$CONFIG_FILE" 2>/dev/null || true
}

# Activa TLS en config.yaml si mkcert generó certificados pero la config quedó sin tls_* activos.
migrate_tls_certs_if_present() {
    local cert="${INSTALL_DIR}/certs/hostberry.pem"
    local key="${INSTALL_DIR}/certs/hostberry-key.pem"
    [ -f "$CONFIG_FILE" ] || return 0
    [ -f "$cert" ] && [ -f "$key" ] || return 0
    if grep -qE '^[[:space:]]*tls_cert_file:.*hostberry\.pem' "$CONFIG_FILE" 2>/dev/null; then
        return 0
    fi
    print_info "Certificados TLS encontrados; activando HTTPS en config.yaml…"
    sed -i '/^[[:space:]]*#tls_cert_file:/d;/^[[:space:]]*#tls_key_file:/d' "$CONFIG_FILE" 2>/dev/null || true
    sed -i '/^[[:space:]]*http_redirect_port:/d;/^[[:space:]]*tls_cert_file:/d;/^[[:space:]]*tls_key_file:/d' "$CONFIG_FILE" 2>/dev/null || true
    sed -i 's/^  port: 8000$/  port: 443/' "$CONFIG_FILE" 2>/dev/null || true
    sed -i 's/^  port: 8443$/  port: 443/' "$CONFIG_FILE" 2>/dev/null || true
    if grep -q '^  write_timeout:' "$CONFIG_FILE"; then
        sed -i '/^  write_timeout:/a\
  http_redirect_port: 80\
  tls_cert_file: "'"${INSTALL_DIR}/certs/hostberry.pem"'"\
  tls_key_file: "'"${INSTALL_DIR}/certs/hostberry-key.pem"'"' "$CONFIG_FILE"
    elif grep -q '^  read_timeout:' "$CONFIG_FILE"; then
        sed -i '/^  read_timeout:/a\
  http_redirect_port: 80\
  tls_cert_file: "'"${INSTALL_DIR}/certs/hostberry.pem"'"\
  tls_key_file: "'"${INSTALL_DIR}/certs/hostberry-key.pem"'"' "$CONFIG_FILE"
    else
        sed -i '/^  port:/a\
  http_redirect_port: 80\
  tls_cert_file: "'"${INSTALL_DIR}/certs/hostberry.pem"'"\
  tls_key_file: "'"${INSTALL_DIR}/certs/hostberry-key.pem"'"' "$CONFIG_FILE"
    fi
    if ! grep -q '^  enforce_https:' "$CONFIG_FILE" 2>/dev/null; then
        sed -i '/^security:/a\
  enforce_https: true' "$CONFIG_FILE"
    else
        sed -i 's/^  enforce_https: false$/  enforce_https: true/' "$CONFIG_FILE" 2>/dev/null || true
    fi
    print_success "TLS activado en config (HTTPS :443, HTTP :80 → HTTPS)"
}

# Genera certificados con mkcert y ajusta config (HTTPS 443 + HTTP 80 → HTTPS, sin puerto en la URL).
# HOSTBERRY_SKIP_MKCERT=1 lo omite. En --update: no regenera si ya existen (salvo HOSTBERRY_REGENERATE_MKCERT=1).

setup_mkcert_tls() {
    if [ "${HOSTBERRY_SKIP_MKCERT:-0}" = "1" ]; then
        print_info "Omitiendo mkcert (HOSTBERRY_SKIP_MKCERT=1)."
        return 0
    fi

    if [ "$MODE" = "update" ] && [ -f "${INSTALL_DIR}/certs/hostberry.pem" ] && [ "${HOSTBERRY_REGENERATE_MKCERT:-0}" != "1" ]; then
        print_info "Certificados TLS ya presentes; omitiendo mkcert (HOSTBERRY_REGENERATE_MKCERT=1 para regenerar)."
        return 0
    fi

    if ! install_mkcert_binary; then
        print_warning "mkcert no disponible: se mantiene HTTP. Instala mkcert o define tls_cert_file/tls_key_file a mano."
        return 0
    fi

    local CERT_DIR="${INSTALL_DIR}/certs"
    mkdir -p "$CERT_DIR"
    export CAROOT="${CERT_DIR}/mkcert-rootca"
    mkdir -p "$CAROOT"

    print_info "Instalando CA local de mkcert (confianza en navegadores en este equipo)..."
    if ! mkcert -install 2>/dev/null; then
        print_warning "mkcert -install no completó; los navegadores pueden avisar hasta importar rootCA.pem."
    fi

    local san=()
    san+=(localhost 127.0.0.1 ::1 hostberry.local)
    [ -n "$(hostname -s 2>/dev/null)" ] && san+=("$(hostname -s)")
    [ -n "$(hostname -f 2>/dev/null)" ] && san+=("$(hostname -f)")
    local ip
    for ip in $(hostname -I 2>/dev/null); do
        san+=("$ip")
    done

    print_info "Generando certificado TLS (mkcert) para: ${san[*]}"
    if ! mkcert -cert-file "${CERT_DIR}/hostberry.pem" -key-file "${CERT_DIR}/hostberry-key.pem" "${san[@]}"; then
        print_error "mkcert no pudo generar certificados."
        return 1
    fi

    chmod 750 "$CERT_DIR"
    chmod 644 "${CERT_DIR}/hostberry.pem"
    chmod 640 "${CERT_DIR}/hostberry-key.pem"
    chown root:"$GROUP_NAME" "${CERT_DIR}/hostberry.pem" "${CERT_DIR}/hostberry-key.pem" 2>/dev/null || true
    chown root:"$GROUP_NAME" "$CERT_DIR" 2>/dev/null || true

    if ! grep -q 'tls_cert_file:.*hostberry\.pem' "$CONFIG_FILE" 2>/dev/null; then
        print_info "Configurando HTTPS (puerto 443), redirección HTTP→HTTPS (puerto 80) y security.enforce_https…"
        sed -i '/^[[:space:]]*http_redirect_port:/d;/^[[:space:]]*tls_cert_file:/d;/^[[:space:]]*tls_key_file:/d' "$CONFIG_FILE"
        sed -i 's/^  port: 8000$/  port: 443/' "$CONFIG_FILE"
        sed -i 's/^  port: 8443$/  port: 443/' "$CONFIG_FILE"
        sed -i 's/^  enforce_https: false$/  enforce_https: true/' "$CONFIG_FILE"
        if ! grep -q '^  enforce_https:' "$CONFIG_FILE"; then
            sed -i '/^security:/a\
  enforce_https: true' "$CONFIG_FILE"
        fi
        if grep -q '^  write_timeout:' "$CONFIG_FILE"; then
            sed -i '/^  write_timeout:/a\
  http_redirect_port: 80\
  tls_cert_file: "'"${INSTALL_DIR}/certs/hostberry.pem"'"\
  tls_key_file: "'"${INSTALL_DIR}/certs/hostberry-key.pem"'"' "$CONFIG_FILE"
        elif grep -q '^  read_timeout:' "$CONFIG_FILE"; then
            sed -i '/^  read_timeout:/a\
  http_redirect_port: 80\
  tls_cert_file: "'"${INSTALL_DIR}/certs/hostberry.pem"'"\
  tls_key_file: "'"${INSTALL_DIR}/certs/hostberry-key.pem"'"' "$CONFIG_FILE"
        else
            sed -i '/^  port:/a\
  http_redirect_port: 80\
  tls_cert_file: "'"${INSTALL_DIR}/certs/hostberry.pem"'"\
  tls_key_file: "'"${INSTALL_DIR}/certs/hostberry-key.pem"'"' "$CONFIG_FILE"
        fi
    fi

    chown -R "$USER_NAME:$GROUP_NAME" "$INSTALL_DIR"
    chmod 644 "$CONFIG_FILE"
    chmod 750 "$CERT_DIR"
    chmod 640 "${CERT_DIR}/hostberry-key.pem"
    chmod 644 "${CERT_DIR}/hostberry.pem"
    # La CA privada de mkcert no debe ser legible por el usuario del servicio
    if [ -d "${CERT_DIR}/mkcert-rootca" ]; then
        chown -R root:root "${CERT_DIR}/mkcert-rootca"
        chmod 700 "${CERT_DIR}/mkcert-rootca"
        find "${CERT_DIR}/mkcert-rootca" -type f -exec chmod 600 {} \; 2>/dev/null || true
    fi

    # Migrar configs antiguas (8443/8000) a puertos estándar
    sed -i 's/^  port: 8443$/  port: 443/' "$CONFIG_FILE" 2>/dev/null || true
    sed -i 's/^  http_redirect_port: 8000$/  http_redirect_port: 80/' "$CONFIG_FILE" 2>/dev/null || true
    print_success "TLS listo: https://hostberry.local o https://<IP> (HTTP en :80 redirige a HTTPS)."
    print_warning "Otros dispositivos (móviles, PCs) deben confiar en la CA: copia ${CAROOT}/rootCA.pem e impórtala, o usa un certificado público (Let's Encrypt)."
    return 0
}

# Anuncia hostberry.local en la red local vía mDNS (Avahi), para no depender sólo de la IP.

configure_avahi_mdns() {
    if [ "${HOSTBERRY_SKIP_AVAHI:-0}" = "1" ]; then
        print_info "Omitiendo Avahi (HOSTBERRY_SKIP_AVAHI=1)."
        return 0
    fi

    print_info "Configurando mDNS (hostberry.local) con Avahi..."
    if ! command -v avahi-daemon &>/dev/null; then
        print_warning "avahi-daemon no instalado; hostberry.local puede no resolverse. Reinstala con: sudo ./install.sh"
        return 0
    fi

    local conf="/etc/avahi/avahi-daemon.conf"
    if [ ! -f "$conf" ]; then
        print_warning "No existe $conf; omitiendo mDNS."
        return 0
    fi

    if grep -q '^host-name=hostberry' "$conf" 2>/dev/null; then
        print_success "Avahi ya anuncia host-name=hostberry (hostberry.local)"
    else
        sed -i '/^host-name=/d' "$conf"
        if grep -q '^\[server\]' "$conf"; then
            sed -i '/^\[server\]/a host-name=hostberry' "$conf"
        else
            printf '\n[server]\nhost-name=hostberry\n' >> "$conf"
        fi
        print_success "Avahi anunciará este equipo como hostberry.local en la LAN"
    fi

    systemctl enable avahi-daemon 2>/dev/null || true
    systemctl restart avahi-daemon 2>/dev/null || true
}


