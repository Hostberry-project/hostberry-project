#!/bin/bash
# Módulo: main.sh
main() {
    local mode_label
    if [ "$HB_INSTALL_LANG" = "en" ]; then
        mode_label="INSTALLATION"
        [ "$MODE" = "update" ] && mode_label="UPDATE"
        [ "$MODE" = "remove" ] && mode_label="REMOVE"
    else
        mode_label="INSTALACIÓN"
        [ "$MODE" = "update" ] && mode_label="ACTUALIZACIÓN"
        [ "$MODE" = "remove" ] && mode_label="DESINSTALACIÓN"
    fi

    if [ "$MODE" = "install" ] || [ "$MODE" = "update" ]; then
        NEED_REBOOT_FOR_AP0=1
        if declare -F hostberry_defer_ap_during_install >/dev/null 2>&1 && hostberry_defer_ap_during_install; then
            HOSTBERRY_DEFER_AP_START=1
            print_warning "SSH por WiFi detectado: el AP no se iniciará hasta el reinicio (para no cortar esta sesión)."
            print_info "Los servicios quedarán habilitados; tras reiniciar se activará el SSID hostberry."
            print_info "Para forzar el AP ahora: HOSTBERRY_START_AP_NOW=1 sudo ./install.sh"
        fi
        if [ "${HOSTBERRY_SKIP_REBOOT:-0}" = "1" ]; then
            NEED_REBOOT_FOR_AP0=0
            print_warning "HOSTBERRY_SKIP_REBOOT=1: no se reiniciará el sistema al final."
        elif [ "$(is_default_route_over_wifi)" = "1" ]; then
            print_warning "Ruta por defecto por WiFi: al final se reiniciará el equipo (la sesión SSH por WiFi puede cortarse hasta que vuelva a arrancar)."
            if [ "${HOSTBERRY_DEFER_AP_START:-0}" = "1" ]; then
                print_info "La instalación debería completarse antes del reinicio. Para omitirlo: HOSTBERRY_SKIP_REBOOT=1"
            fi
        fi
    fi

    print_banner "$mode_label"
    
    check_root
    
    # Desinstalación completa
    if [ "$MODE" = "remove" ]; then
        do_uninstall
        show_final_info
        return 0
    fi

    fix_hostname
    detect_os

    install_apt_packages
    download_project
    clean_previous_installation
    create_user
    install_files
    migrate_hostberry_tls_standard_ports
    migrate_tls_certs_if_present
    setup_mkcert_tls
    configure_avahi_mdns
    build_project
    create_database
    configure_permissions
    create_hostapd_default_config
    configure_firewall
    create_systemd_service
    if hostberry_fast_install_enabled; then
        print_info "Instalación rápida: omitiendo Blocky y LibreSpeed CLI (HOSTBERRY_FAST_INSTALL=1)."
    else
        install_blocky
        install_librespeed_cli
    fi
    start_service
    enable_and_start_hostberry_wifi_ap
    finalize_systemd_hostberry_network
    cleanup_temp
    show_final_info

    if [ "$NEED_REBOOT_FOR_AP0" -eq 1 ]; then
        print_warning "Reiniciando el sistema para aplicar scripts, unidades systemd y la red HostBerry."
        sync 2>/dev/null || true
        if command -v systemctl &> /dev/null && systemctl reboot 2>/dev/null; then
            exit 0
        fi
        if command -v shutdown &> /dev/null; then
            shutdown -r now "HostBerry install/update" 2>/dev/null || true
        fi
        reboot 2>/dev/null || /sbin/reboot
    fi
}

