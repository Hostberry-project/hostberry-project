#!/bin/bash

# HostBerry - Script de Instalación para Linux
# Compatible con Debian, Ubuntu, Raspberry Pi OS

set -e  # Salir si hay algún error

# Colores para output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Estilos
BOLD='\033[1m'
DIM='\033[2m'

# Variables de configuración
INSTALL_DIR="/opt/hostberry"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Idioma de mensajes: es | en. Detección por locale; override: HOSTBERRY_INSTALL_LANG=auto|es|en
declare -gA HB_INSTALL_EN=()
HB_INSTALL_LANG="es"

hostberry_install_detect_lang() {
    local o="${HOSTBERRY_INSTALL_LANG:-auto}"
    case "${o,,}" in
        es|spa|spanish) HB_INSTALL_LANG="es"; return ;;
        en|eng|english) HB_INSTALL_LANG="en"; return ;;
        auto|"") ;;
        *) HB_INSTALL_LANG="en" ;;
    esac
    local loc="${LC_ALL:-${LC_MESSAGES:-${LANG:-}}}"
    loc="${loc%%.*}"
    case "${loc,,}" in
        es|es_*) HB_INSTALL_LANG="es" ;;
        *)       HB_INSTALL_LANG="en" ;;
    esac
}

hostberry_install_load_translations() {
    HB_INSTALL_EN=()
    [ "$HB_INSTALL_LANG" = "en" ] || return 0
    local f="${SCRIPT_DIR}/locale/install.lang.en.tsv"
    [ -f "$f" ] || return 0
    local line es en
    while IFS= read -r line || [ -n "$line" ]; do
        [[ -z "$line" || "$line" == \#* ]] && continue
        es="${line%%	*}"
        en="${line#*	}"
        [[ "$es" == "$line" ]] && continue
        HB_INSTALL_EN["$es"]="$en"
    done <"$f"
}

hostberry_install_msg() {
    local s="$1"
    if [ "$HB_INSTALL_LANG" = "es" ]; then
        printf '%s' "$s"
        return 0
    fi
    if [[ -n "${HB_INSTALL_EN[$s]+x}" ]]; then
        printf '%s' "${HB_INSTALL_EN[$s]}"
    else
        printf '%s' "$s"
    fi
}

hostberry_install_detect_lang
hostberry_install_load_translations

SERVICE_NAME="hostberry"
USER_NAME="hostberry"
GROUP_NAME="hostberry"
SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service"
CONFIG_FILE="${INSTALL_DIR}/config.yaml"
LOG_DIR="/var/log/hostberry"
DATA_DIR="${INSTALL_DIR}/data"
GITHUB_REPO="https://github.com/Hostberry-project/hostberry-project.git"
TEMP_CLONE_DIR="/tmp/hostberry-install"

# Secretos generados en instalación (JWT y password admin inicial)
GENERATED_JWT_SECRET=""
GENERATED_ADMIN_PASSWORD=""

# Reboot al final en install y --update (salvo HOSTBERRY_SKIP_REBOOT=1).
NEED_REBOOT_FOR_AP0=0

# Detectar si la ruta por defecto sale por una interfaz WiFi (wlan* / wl*)
is_default_route_over_wifi() {
    local dev
    dev="$(ip route 2>/dev/null | awk '/^default/ {print $5; exit}')"
    if [ -z "$dev" ]; then
        echo 0
        return
    fi
    case "$dev" in
        wl*|wlan*)
            echo 1
            ;;
        *)
            echo 0
            ;;
    esac
}

# Modo de operación
MODE="install"  # install, update o uninstall

# Mensajes (hora + icono)
_ts() { date +%H:%M:%S 2>/dev/null || echo "00:00:00"; }
print_info()    { echo -e "$(_ts) ${BLUE}[i]${NC} $(hostberry_install_msg "$1")"; }
print_success() { echo -e "$(_ts) ${GREEN}[+]${NC} $(hostberry_install_msg "$1")"; }
print_warning() { echo -e "$(_ts) ${YELLOW}[!]${NC} $(hostberry_install_msg "$1")"; }
print_error()   { echo -e "$(_ts) ${RED}[x]${NC} $(hostberry_install_msg "$1")"; }

# Logo ASCII (basado en website/static/hostberry.png)
print_logo() {
    printf "%b" "$RED"
    cat <<'EOF'
                       $x                    
                      $$$                    
            $$$$$$   $$$  .$$$$$$            
            $$$$$$$$ $$ .$$$$$$$             
             $$$$$$$$$$$$$$$$$$              
               $$$$$$$$$$$$$$.               
          XXXXXXXXXXXXXXXXXXXXXXXX           
        :XXXXXXXXXXXXXXXXXXXXXXXXXXX         
        XXXXXXXXXXX;:::::XXXXXXXXXXX.        
     .XXXXXXXXX:::::XXXX:::::XXXXXXXXX+      
    XXXXXXXXXXX:XXXX;:::XXXX:+XXXXXXXXXX     
    XXXXXXX::XXXX::::XX+:::XXXX::XXXXXXX     
    :XXXXX::XXXXXXXXXXXXXXXXXXXX::XXXXXX     
      XXXX:XX;:XXXXX:XX::XXXX::XX::XXX:      
      XXX+:XXX:::XXXXXXXXXX::::XX::XXX       
      XXXXXXXXXXXXXXX$XXXXXXXXXXXXXXXXx      
      XXXXXXXXX::::::::::::::XXXXXXXXX       
       XXXXXXX::X$:$X:::::::::XXXXXXX        
          XXXX::::::::::::::::XXXX           
          XXXXXX$XXXXXXXXXXXXXXXXX           
           XXXXXXXXXXXXXXXXXXXXXX            
              xXXXXXXXXXXXXXXX               
                 .XXXXXXXXX                  
                    XXXX.                    
                            

EOF
    printf "%b\n" "$NC"
}

print_banner() {
    local label="$1"
    local accent="$BLUE"
    case "$MODE" in
        install)   accent="$GREEN" ;;
        update)    accent="$BLUE" ;;
        uninstall) accent="$RED" ;;
    esac

    echo ""
    print_logo
    printf "%b\n" "${accent}${BOLD}HostBerry${NC} ${DIM}${label}${NC}"
    echo ""
}

# Ayuda de uso
show_usage() {
    if [ "$HB_INSTALL_LANG" = "en" ]; then
        echo "Usage: $0 [OPTION]"
        echo ""
        echo "Options:"
        echo "  (none)         Install HostBerry"
        echo "  --update       Update (preserves data); at the end, daemon-reload and system reboot"
        echo "                 (skip reboot: HOSTBERRY_SKIP_REBOOT=1 sudo $0 --update)"
        echo "  --uninstall    Uninstall HostBerry (removes service, files, user, and logs)"
        echo "  -h, --help     Show this help"
        echo ""
        echo "Language: detected from LANG/LC_MESSAGES (es_* → Spanish; otherwise English)."
        echo "Override: HOSTBERRY_INSTALL_LANG=es|en|auto"
        echo ""
        echo "Examples:"
        echo "  sudo $0              # Install"
        echo "  sudo $0 --update     # Update"
        echo "  sudo $0 --uninstall  # Uninstall"
    else
        echo "Uso: $0 [OPCIÓN]"
        echo ""
        echo "Opciones:"
        echo "  (sin opción)   Instalar HostBerry"
        echo "  --update       Actualizar (preserva datos); al terminar, daemon-reload y reinicio del sistema"
        echo "                 (omitir reinicio: HOSTBERRY_SKIP_REBOOT=1 sudo $0 --update)"
        echo "  --uninstall    Desinstalar HostBerry (elimina servicio, archivos, usuario y logs)"
        echo "  -h, --help     Mostrar esta ayuda"
        echo ""
        echo "Idioma: se detecta con LANG/LC_MESSAGES (es_* → español; si no, inglés)."
        echo "Forzar: HOSTBERRY_INSTALL_LANG=es|en|auto"
        echo ""
        echo "Ejemplos:"
        echo "  sudo $0              # Instalar"
        echo "  sudo $0 --update     # Actualizar"
        echo "  sudo $0 --uninstall  # Desinstalar"
    fi
    exit 0
}

# Procesar argumentos
while [[ "$#" -gt 0 ]]; do
    case $1 in
        --update)    MODE="update" ;;
        --uninstall) MODE="uninstall" ;;
        -h|--help)   show_usage ;;
        *) print_error "Opción desconocida: $1. Usa --help para ver opciones."; exit 1 ;;
    esac
    shift
done

# Verificar si se ejecuta como root
check_root() {
    if [ "$EUID" -ne 0 ]; then 
        print_error "Ejecuta con sudo/root"
        exit 1
    fi
}

# Configurar hostname en /etc/hosts para evitar warnings de sudo
fix_hostname() {
    CURRENT_HOSTNAME=$(hostname)
    if [ -n "$CURRENT_HOSTNAME" ]; then
        # Verificar si el hostname ya está en /etc/hosts
        if ! grep -q "127.0.0.1.*$CURRENT_HOSTNAME" /etc/hosts 2>/dev/null; then
            print_info "Configurando hostname '$CURRENT_HOSTNAME' en /etc/hosts..."
            # Agregar hostname a la línea de 127.0.0.1
            if grep -q "^127.0.0.1" /etc/hosts; then
                # La línea existe, agregar el hostname si no está
                sed -i "s/^127.0.0.1.*/& $CURRENT_HOSTNAME/" /etc/hosts 2>/dev/null || true
            else
                # La línea no existe, crearla
                echo "127.0.0.1 localhost $CURRENT_HOSTNAME" >> /etc/hosts
            fi
            # También agregar a 127.0.1.1 si no existe
            if ! grep -q "^127.0.1.1" /etc/hosts; then
                echo "127.0.1.1 $CURRENT_HOSTNAME" >> /etc/hosts
            fi
            print_success "Hostname configurado en /etc/hosts"
        fi
    fi
}

# Detectar sistema operativo
detect_os() {
    if [ -f /etc/os-release ]; then
        . /etc/os-release
        OS=$ID
        OS_VERSION=$VERSION_ID
        print_info "Sistema: $OS $OS_VERSION"
    else
        print_error "No se pudo detectar el sistema operativo"
        exit 1
    fi
}

# Instalar git (necesario para descargar el proyecto)
install_git() {
    if ! command -v git &> /dev/null; then
        print_info "Instalando git..."
        apt-get update -qq
        apt-get install -y git
        print_success "Git listo"
    else
        print_success "Git: $(git --version)"
    fi
}

# Instalar Go (Golang) desde apt - necesario para compilar HostBerry
install_golang() {
    if command -v go &> /dev/null; then
        print_success "Go ya instalado: $(go version)"
        return 0
    fi

    print_info "Instalando Go (Golang) desde apt..."
    apt-get update -qq
    apt-get install -y golang-go

    if command -v go &> /dev/null; then
        print_success "Go instalado: $(go version)"
    else
        print_error "Go no disponible tras apt-get install golang-go"
        return 1
    fi
}

# Instalar dependencias del sistema
install_dependencies() {
    print_info "Instalando dependencias..."
    
    # Actualizar lista de paquetes
    apt-get update -qq
    
    # Instalar dependencias básicas (ccache acelera recompilaciones con CGO)
    DEPS="wget curl build-essential iw isc-dhcp-client ccache"
    
    # Instalar hostapd y herramientas relacionadas
    print_info "Instalando hostapd, wpa_supplicant y herramientas WiFi..."
    
    # Instalar paquetes individualmente para identificar fallos específicos
    local failed_packages=()
    local installed_packages=()
    
    # Lista de paquetes WiFi
    local wifi_packages=("hostapd" "dnsmasq" "iptables" "wpa_supplicant")
    
    for package in "${wifi_packages[@]}"; do
        # Verificar si ya está instalado (múltiples métodos)
        local is_installed=false
        if dpkg -l | grep -q "^ii.*${package} "; then
            is_installed=true
        elif command -v "${package}" &> /dev/null; then
            is_installed=true
        elif [ "${package}" = "wpa_supplicant" ] && (command -v wpa_supplicant &> /dev/null || [ -f "/usr/sbin/wpa_supplicant" ] || [ -f "/sbin/wpa_supplicant" ]); then
            is_installed=true
        elif [ "${package}" = "hostapd" ] && (command -v hostapd &> /dev/null || [ -f "/usr/sbin/hostapd" ] || [ -f "/sbin/hostapd" ]); then
            is_installed=true
        elif [ "${package}" = "dnsmasq" ] && (command -v dnsmasq &> /dev/null || [ -f "/usr/sbin/dnsmasq" ] || [ -f "/sbin/dnsmasq" ]); then
            is_installed=true
        fi
        
        if [ "$is_installed" = true ]; then
            installed_packages+=("${package}")
        else
            print_info "Instalando ${package}..."
            
            # Intentar instalar con salida visible para diagnóstico
            local install_output
            local install_exit_code
            install_output=$(apt-get install -y "${package}" 2>&1)
            install_exit_code=$?
            
            if [ $install_exit_code -eq 0 ]; then
                # Verificar que realmente se instaló
                local verify_installed=false
                if dpkg -l | grep -q "^ii.*${package} "; then
                    verify_installed=true
                elif command -v "${package}" &> /dev/null; then
                    verify_installed=true
                elif [ "${package}" = "wpa_supplicant" ] && (command -v wpa_supplicant &> /dev/null || [ -f "/usr/sbin/wpa_supplicant" ] || [ -f "/sbin/wpa_supplicant" ]); then
                    verify_installed=true
                elif [ "${package}" = "hostapd" ] && (command -v hostapd &> /dev/null || [ -f "/usr/sbin/hostapd" ] || [ -f "/sbin/hostapd" ]); then
                    verify_installed=true
                elif [ "${package}" = "dnsmasq" ] && (command -v dnsmasq &> /dev/null || [ -f "/usr/sbin/dnsmasq" ] || [ -f "/sbin/dnsmasq" ]); then
                    verify_installed=true
                fi
                
                if [ "$verify_installed" = true ]; then
                    installed_packages+=("${package}")
                else
                    failed_packages+=("${package}")
                fi
            else
                # Verificar si el paquete está disponible en los repositorios
                if ! apt-cache search "${package}" 2>/dev/null | grep -q "^${package} "; then
                    # Intentar actualizar repositorios y reinstalar
                    if apt-get update -qq && apt-get install -y "${package}" > /dev/null 2>&1; then
                        if dpkg -l | grep -q "^ii.*${package} " || command -v "${package}" &> /dev/null; then
                            installed_packages+=("${package}")
                            continue
                        fi
                    fi
                else
                    # Intentar con --fix-broken si hay problemas de dependencias
                    if echo "$install_output" | grep -q "broken\|dependenc"; then
                        if apt-get install -f -y > /dev/null 2>&1; then
                            if apt-get install -y "${package}" > /dev/null 2>&1; then
                                if dpkg -l | grep -q "^ii.*${package} " || command -v "${package}" &> /dev/null; then
                                    installed_packages+=("${package}")
                                    continue
                                fi
                            fi
                        fi
                    fi
                fi
                
                failed_packages+=("${package}")
            fi
        fi
    done
    
    # Verificar instalación final
    local missing_critical=()
    for package in "${wifi_packages[@]}"; do
        if ! command -v "${package}" &> /dev/null && ! dpkg -l | grep -q "^ii.*${package} "; then
            missing_critical+=("${package}")
        fi
    done
    
    if [ ${#failed_packages[@]} -gt 0 ]; then
        if [ ${#missing_critical[@]} -gt 0 ]; then
            print_warning "Paquetes faltantes: ${missing_critical[*]} (algunas funciones WiFi pueden no estar disponibles)"
        fi
    fi
    
    # Instalar Tor, OpenVPN y WireGuard (VPN/Seguridad)
    print_info "Instalando Tor, OpenVPN y WireGuard..."
    apt-get update -qq 2>/dev/null || true
    for pkg in tor openvpn wireguard-tools; do
        if dpkg -l 2>/dev/null | grep -q "^ii.*${pkg} "; then
            print_success "${pkg}: ya instalado"
        elif apt-get install -y "${pkg}" > /dev/null 2>&1; then
            print_success "${pkg}: instalado"
        else
            if [ "$pkg" = "wireguard-tools" ]; then
                if apt-get install -y wireguard > /dev/null 2>&1; then
                    print_success "wireguard: instalado (incluye herramientas)"
                else
                    print_warning "No se pudo instalar WireGuard. Manual: sudo apt-get install wireguard-tools"
                fi
            else
                print_warning "No se pudo instalar ${pkg}. Manual: sudo apt-get install ${pkg}"
            fi
        fi
    done
    
    # Verificar e instalar iw si no está disponible
    if ! command -v iw &> /dev/null; then
        apt-get install -y iw > /dev/null 2>&1 || true
    fi
    
    # Instalar otras dependencias
    apt-get install -y $DEPS > /dev/null 2>&1
}

# Descargar proyecto de GitHub (siempre desde GitHub, nunca local)
download_project() {
    # Si el proyecto ya está presente localmente (ejecutas install.sh desde el repo),
    # usamos el código local y evitamos clonar desde GitHub (para que los cambios en
    # HTML/JS/etc. se vean en el instalador).
    if [ -f "${SCRIPT_DIR}/main.go" ] && [ -f "${SCRIPT_DIR}/go.mod" ] && [ -d "${SCRIPT_DIR}/website/static" ]; then
        print_info "Proyecto local detectado: usando código local (sin clonar GitHub)..."
        return 0
    fi

    if [ "$MODE" = "update" ]; then
        print_info "Modo actualización: descargando desde GitHub..."
    else
        print_info "Descargando proyecto desde GitHub..."
    fi
    
    # Limpiar directorio temporal si existe
    if [ -d "$TEMP_CLONE_DIR" ]; then
        rm -rf "$TEMP_CLONE_DIR"
    fi
    
    # Clonar repositorio desde GitHub
    if git clone "$GITHUB_REPO" "$TEMP_CLONE_DIR" 2>/dev/null; then
        print_success "Proyecto descargado desde GitHub"
        SCRIPT_DIR="$TEMP_CLONE_DIR"
        return 0
    else
        print_error "Error al descargar el proyecto desde GitHub"
        print_info "Verifica tu conexión a internet y que el repositorio sea accesible"
        exit 1
    fi
}

# Limpiar instalación anterior
clean_previous_installation() {
    if [ -d "$INSTALL_DIR" ]; then
        if [ "$MODE" = "update" ]; then
            # En modo actualización, preservar datos y configuración
            print_info "Modo actualización: preservando datos y configuración..."
            
            # Detener servicio si está corriendo
            if systemctl is-active --quiet "${SERVICE_NAME}" 2>/dev/null; then
                print_info "Deteniendo servicio ${SERVICE_NAME}..."
                systemctl stop "${SERVICE_NAME}" 2>/dev/null || true
                # Esperar un momento para que el servicio se detenga completamente
                sleep 2
            fi
            
            # Crear directorio temporal para guardar datos importantes
            TEMP_BACKUP_DIR="/tmp/hostberry-update-backup-$$"
            mkdir -p "$TEMP_BACKUP_DIR"
            
            # Hacer backup de la base de datos ANTES de eliminar nada
            if [ -d "$DATA_DIR" ]; then
                print_info "Guardando backup de base de datos..."
                # Copiar todo el contenido del directorio data
                if cp -r "$DATA_DIR" "$TEMP_BACKUP_DIR/data" 2>/dev/null; then
                    print_success "Backup de base de datos guardado en $TEMP_BACKUP_DIR/data"
                    # Verificar que el archivo de BD existe en el backup
                    if [ -f "$TEMP_BACKUP_DIR/data/hostberry.db" ]; then
                        DB_SIZE=$(du -h "$TEMP_BACKUP_DIR/data/hostberry.db" | cut -f1)
                        print_info "Base de datos respaldada: $DB_SIZE"
                    fi
                else
                    print_error "ERROR: No se pudo hacer backup de la base de datos"
                    print_error "Abortando actualización para proteger los datos"
                    rm -rf "$TEMP_BACKUP_DIR"
                    exit 1
                fi
            else
                print_warning "Directorio de datos no encontrado: $DATA_DIR"
            fi
            
            # Hacer backup de la configuración
            if [ -f "$CONFIG_FILE" ]; then
                print_info "Guardando backup de configuración..."
                if cp "$CONFIG_FILE" "$TEMP_BACKUP_DIR/config.yaml" 2>/dev/null; then
                    print_success "Configuración respaldada"
                else
                    print_warning "No se pudo hacer backup de la configuración"
                fi
            fi
            
            # Mover el directorio data fuera temporalmente para preservarlo
            TEMP_DATA_DIR="/tmp/hostberry-data-temp-$$"
            if [ -d "$DATA_DIR" ]; then
                print_info "Moviendo directorio de datos temporalmente para preservarlo..."
                # Verificar que el directorio data contiene la base de datos
                if [ -f "$DATA_DIR/hostberry.db" ]; then
                    DB_SIZE=$(du -h "$DATA_DIR/hostberry.db" | cut -f1)
                    print_info "Base de datos encontrada: $DB_SIZE"
                fi
                
                if mv "$DATA_DIR" "$TEMP_DATA_DIR" 2>/dev/null; then
                    print_success "Directorio de datos movido temporalmente a $TEMP_DATA_DIR"
                    # Verificar que el archivo de BD está en el directorio temporal
                    if [ -f "$TEMP_DATA_DIR/hostberry.db" ]; then
                        print_success "Base de datos preservada en directorio temporal"
                    else
                        print_warning "Advertencia: No se encontró hostberry.db en el directorio temporal"
                    fi
                else
                    # Fallback: si mv falla (permiso / rename entre FS / etc.), intentar copy-preserve.
                    print_warning "mv falló al mover $DATA_DIR; intentando cp -a como fallback..."
                    if cp -a "$DATA_DIR" "$TEMP_DATA_DIR" 2>/dev/null && rm -rf "$DATA_DIR" 2>/dev/null; then
                        print_success "Directorio de datos copiado/respaldado en $TEMP_DATA_DIR"
                        if [ -f "$TEMP_DATA_DIR/hostberry.db" ]; then
                            print_success "Base de datos preservada en directorio temporal"
                        else
                            print_warning "Advertencia: No se encontró hostberry.db en el directorio temporal"
                        fi
                    else
                        print_error "ERROR: No se pudo mover/copy el directorio de datos"
                        print_error "Abortando actualización para proteger los datos"
                        rm -rf "$TEMP_BACKUP_DIR"
                        exit 1
                    fi
                fi
            else
                print_warning "Directorio de datos no existe: $DATA_DIR (primera instalación?)"
            fi
            
            # Eliminar directorio de instalación (data ya está fuera)
            print_info "Eliminando archivos antiguos (preservando datos)..."
            # Asegurarse de que no eliminamos el directorio data si aún existe
            if [ -d "$DATA_DIR" ]; then
                print_warning "Advertencia: El directorio data aún existe, moviéndolo antes de eliminar..."
                if mv "$DATA_DIR" "$TEMP_DATA_DIR" 2>/dev/null; then
                    true
                else
                    print_warning "mv falló al mover $DATA_DIR (segunda fase); intentando cp -a..."
                    cp -a "$DATA_DIR" "$TEMP_DATA_DIR" 2>/dev/null && rm -rf "$DATA_DIR" 2>/dev/null || {
                        print_error "ERROR: No se pudo mover/copy el directorio de datos antes de eliminar"
                        exit 1
                    }
                fi
            fi
            rm -rf "$INSTALL_DIR"
            print_success "Archivos antiguos eliminados"
            
            # Restaurar directorio de datos
            if [ -d "$TEMP_DATA_DIR" ]; then
                print_info "Restaurando directorio de datos..."
                mkdir -p "$(dirname "$DATA_DIR")"
                if mv "$TEMP_DATA_DIR" "$DATA_DIR" 2>/dev/null; then
                    print_success "Directorio de datos restaurado"
                    # Verificar que la BD existe
                    if [ -f "$DATA_DIR/hostberry.db" ]; then
                        DB_SIZE=$(du -h "$DATA_DIR/hostberry.db" | cut -f1)
                        print_success "✅ Base de datos preservada exitosamente: $DB_SIZE"
                    else
                        print_warning "Advertencia: No se encontró hostberry.db después de restaurar"
                        # Intentar restaurar desde backup
                        if [ -d "$TEMP_BACKUP_DIR/data" ] && [ -f "$TEMP_BACKUP_DIR/data/hostberry.db" ]; then
                            print_info "Intentando restaurar desde backup..."
                            cp -r "$TEMP_BACKUP_DIR/data/"* "$DATA_DIR/" 2>/dev/null && {
                                print_success "Base de datos restaurada desde backup"
                            } || {
                                print_error "ERROR: No se pudo restaurar desde backup"
                            }
                        fi
                    fi
                else
                    print_error "ERROR: No se pudo restaurar el directorio de datos"
                    print_error "Intentando restaurar desde backup..."
                    # Intentar restaurar desde backup como fallback
                    if [ -d "$TEMP_BACKUP_DIR/data" ]; then
                        mkdir -p "$DATA_DIR"
                        if cp -r "$TEMP_BACKUP_DIR/data/"* "$DATA_DIR/" 2>/dev/null; then
                            print_success "Base de datos restaurada desde backup"
                            if [ -f "$DATA_DIR/hostberry.db" ]; then
                                DB_SIZE=$(du -h "$DATA_DIR/hostberry.db" | cut -f1)
                                print_success "Base de datos verificada: $DB_SIZE"
                            fi
                        else
                            print_error "ERROR CRÍTICO: No se pudo restaurar la base de datos"
                            print_error "El backup está en: $TEMP_BACKUP_DIR"
                            print_error "El directorio temporal está en: $TEMP_DATA_DIR"
                            exit 1
                        fi
                    else
                        print_error "ERROR CRÍTICO: No hay backup disponible"
                        print_error "El directorio temporal está en: $TEMP_DATA_DIR"
                        exit 1
                    fi
                fi
            elif [ -d "$TEMP_BACKUP_DIR/data" ]; then
                # Si no se pudo mover, restaurar desde backup
                print_info "Restaurando base de datos desde backup..."
                mkdir -p "$DATA_DIR"
                if cp -r "$TEMP_BACKUP_DIR/data/"* "$DATA_DIR/" 2>/dev/null; then
                    print_success "Base de datos restaurada desde backup"
                    if [ -f "$DATA_DIR/hostberry.db" ]; then
                        DB_SIZE=$(du -h "$DATA_DIR/hostberry.db" | cut -f1)
                        print_success "Base de datos verificada: $DB_SIZE"
                    fi
                else
                    print_error "ERROR CRÍTICO: No se pudo restaurar la base de datos"
                    print_error "El backup está en: $TEMP_BACKUP_DIR"
                    exit 1
                fi
            else
                print_warning "No se encontró directorio de datos ni backup para restaurar"
                print_info "Se creará una nueva base de datos al iniciar el servicio"
            fi
            
            # Restaurar configuración
            if [ -f "$TEMP_BACKUP_DIR/config.yaml" ]; then
                print_info "Restaurando configuración..."
                mkdir -p "$(dirname "$CONFIG_FILE")"
                cp "$TEMP_BACKUP_DIR/config.yaml" "$CONFIG_FILE" 2>/dev/null || true
            fi
            
            # Limpiar backup temporal
            rm -rf "$TEMP_BACKUP_DIR"
            
            print_success "Archivos actualizados, datos preservados"
        else
            # En modo instalación, eliminar todo
            print_info "Eliminando instalación anterior en $INSTALL_DIR..."
            
            # Detener servicio si está corriendo
            if systemctl is-active --quiet "${SERVICE_NAME}" 2>/dev/null; then
                print_info "Deteniendo servicio ${SERVICE_NAME}..."
                systemctl stop "${SERVICE_NAME}" 2>/dev/null || true
            fi
            
            # Deshabilitar servicio
            if systemctl is-enabled --quiet "${SERVICE_NAME}" 2>/dev/null; then
                print_info "Deshabilitando servicio ${SERVICE_NAME}..."
                systemctl disable "${SERVICE_NAME}" 2>/dev/null || true
            fi
            
            # Eliminar directorio de instalación
            rm -rf "$INSTALL_DIR"
            print_success "Instalación anterior eliminada"
        fi
    else
        print_info "No hay instalación anterior que eliminar"
    fi
}

# Crear usuario del sistema
create_user() {
    if id "$USER_NAME" &>/dev/null; then
        print_info "Usuario $USER_NAME ya existe"
        # Asegurar que el usuario esté en el grupo netdev (necesario para wpa_supplicant)
        if getent group netdev > /dev/null 2>&1; then
            if groups "$USER_NAME" | grep -q "\bnetdev\b"; then
                print_info "Usuario $USER_NAME ya está en el grupo netdev"
            else
                print_info "Agregando usuario $USER_NAME al grupo netdev..."
                usermod -a -G netdev "$USER_NAME"
                print_success "Usuario $USER_NAME agregado al grupo netdev"
            fi
        else
            print_warning "Grupo netdev no existe, creándolo..."
            groupadd -r netdev 2>/dev/null || true
            usermod -a -G netdev "$USER_NAME"
            print_success "Grupo netdev creado y usuario agregado"
        fi
    else
        print_info "Creando usuario $USER_NAME..."
        # Crear grupo netdev si no existe
        if ! getent group netdev > /dev/null 2>&1; then
            groupadd -r netdev 2>/dev/null || true
            print_info "Grupo netdev creado"
        fi
        # Crear usuario y agregarlo al grupo netdev
        useradd -r -s /bin/false -d "$INSTALL_DIR" -G netdev "$USER_NAME"
        print_success "Usuario $USER_NAME creado y agregado al grupo netdev"
    fi
}

# Copiar archivos del proyecto
install_files() {
    print_info "Instalando archivos en $INSTALL_DIR..."
    
    # Verificar que estamos en el directorio correcto con todos los archivos
    local missing_files=0
    for item in "website" "locales" "main.go" "go.mod"; do
        if [ ! -e "${SCRIPT_DIR}/${item}" ]; then
            print_warning "No se encontró '${item}' en ${SCRIPT_DIR}"
            missing_files=$((missing_files + 1))
        fi
    done

    if [ $missing_files -gt 0 ]; then
        print_error "Error: Faltan archivos del proyecto en ${SCRIPT_DIR}"
        print_info "Asegúrate de ejecutar el script desde la raíz del repositorio clonado."
        print_info "Si has descargado solo el script, necesitas descargar el proyecto completo."
        exit 1
    fi

    # Crear directorios
    mkdir -p "$INSTALL_DIR"

    mkdir -p "$LOG_DIR"
    mkdir -p "$DATA_DIR"
    # Lua ya no se usa - todo está en Go ahora
    mkdir -p "${INSTALL_DIR}/locales"
    mkdir -p "${INSTALL_DIR}/website/static"
    mkdir -p "${INSTALL_DIR}/website/templates"
    
    # Copiar archivos necesarios
    print_info "Copiando archivos del proyecto..."
    
    # Archivos Go
    cp -f "${SCRIPT_DIR}"/*.go "${INSTALL_DIR}/" 2>/dev/null || true
    cp -f "${SCRIPT_DIR}/go.mod" "${INSTALL_DIR}/" 2>/dev/null || true
    cp -f "${SCRIPT_DIR}/go.sum" "${INSTALL_DIR}/" 2>/dev/null || true

    # Copiar el árbol interno del módulo (incluye internal/handlers, etc.).
    # Sin esto, `go build` falla al resolver imports `hostberry/internal/...`.
    if [ -d "${SCRIPT_DIR}/internal" ]; then
        rm -rf "${INSTALL_DIR}/internal" 2>/dev/null || true
        cp -r "${SCRIPT_DIR}/internal" "${INSTALL_DIR}/internal" 2>/dev/null || true
    fi
    
    # Directorios (lua ya no se usa - todo está en Go)
    if [ -d "${SCRIPT_DIR}/locales" ]; then
        cp -r "${SCRIPT_DIR}/locales/"* "${INSTALL_DIR}/locales/" 2>/dev/null || true
    fi
    
        if [ -d "${SCRIPT_DIR}/website" ]; then
        # Asegurar que los directorios destino existen
        mkdir -p "${INSTALL_DIR}/website/templates"
        mkdir -p "${INSTALL_DIR}/website/static"
        
        # Copiar templates con verificación
        if [ -d "${SCRIPT_DIR}/website/templates" ]; then
            if ! cp -r "${SCRIPT_DIR}/website/templates/"* "${INSTALL_DIR}/website/templates/" 2>/dev/null; then
                print_error "Error al copiar templates"
                exit 1
            fi
            # Verificar que templates críticos existen
            if [ ! -f "${INSTALL_DIR}/website/templates/base.html" ] || \
               [ ! -f "${INSTALL_DIR}/website/templates/dashboard.html" ] || \
               [ ! -f "${INSTALL_DIR}/website/templates/login.html" ]; then
                print_error "Templates críticos no encontrados"
                exit 1
            fi
        else
            print_error "Error: Directorio ${SCRIPT_DIR}/website/templates no existe"
            exit 1
        fi
        
        # Copiar archivos estáticos
        if [ -d "${SCRIPT_DIR}/website/static" ]; then
            cp -r "${SCRIPT_DIR}/website/static/"* "${INSTALL_DIR}/website/static/" 2>/dev/null || true
        fi
    else
        print_error "Error: Directorio website no encontrado en ${SCRIPT_DIR}"
        exit 1
    fi
    
    # Configuración
    if [ ! -f "$CONFIG_FILE" ]; then
        if [ -f "${SCRIPT_DIR}/config.yaml.example" ]; then
            cp "${SCRIPT_DIR}/config.yaml.example" "$CONFIG_FILE"
        else
            cat > "$CONFIG_FILE" <<EOF
server:
  port: 8000
  debug: false
  read_timeout: 30
  write_timeout: 30

database:

security:
  token_expiry: 60
  bcrypt_cost: 10
  rate_limit_rps: 10
EOF
        fi
    fi

    # Secretos (sólo en instalación inicial)
    if [ "$MODE" = "install" ] && [ -f "$CONFIG_FILE" ]; then
        # JWT secret aleatorio si está el placeholder por defecto
        if grep -q 'jwt_secret:' "$CONFIG_FILE"; then
            if grep -q 'cambiar-este-secreto-en-produccion-usar-secretos-seguros' "$CONFIG_FILE"; then
                GENERATED_JWT_SECRET="$(tr -dc 'A-Za-z0-9' </dev/urandom | head -c 64 || echo "hostberry$(date +%s)")"
                sed -i "s|jwt_secret: \".*\"|jwt_secret: \"${GENERATED_JWT_SECRET}\"|" "$CONFIG_FILE"
            fi
        else
            GENERATED_JWT_SECRET="$(tr -dc 'A-Za-z0-9' </dev/urandom | head -c 64 || echo "hostberry$(date +%s)")"
            {
                echo ""
                echo "security:"
                echo "  jwt_secret: \"${GENERATED_JWT_SECRET}\""
                echo "  token_expiry: 60"
                echo "  bcrypt_cost: 10"
                echo "  rate_limit_rps: 10"
            } >> "$CONFIG_FILE"
        fi

        # Password aleatoria para usuario admin por defecto (se inyecta vía systemd)
        # Debe cumplir validators.ValidatePassword: mayúscula, minúscula, número y carácter especial.
        GENERATED_ADMIN_PASSWORD="Hb!$(tr -dc 'A-Za-z0-9' </dev/urandom | head -c 14 || echo 'xX1yY2zZ3')9a"

        # Guardar credenciales iniciales en un fichero sólo-lectura root/hostberry
        local cred_file="${INSTALL_DIR}/INSTALL_CREDENTIALS.txt"
        {
            echo "HostBerry - credenciales iniciales generadas automáticamente"
            echo ""
            echo "Usuario admin: admin"
            echo "Contraseña inicial admin: ${GENERATED_ADMIN_PASSWORD}"
            if [ -n "$GENERATED_JWT_SECRET" ]; then
                echo "JWT secret (security.jwt_secret en config.yaml): ${GENERATED_JWT_SECRET}"
            fi
            echo ""
            echo "Guarda este archivo en lugar seguro y cámbiala tras el primer acceso."
        } > "$cred_file"
        chmod 600 "$cred_file"
        chown "$USER_NAME:$GROUP_NAME" "$cred_file"
    fi
    
    # Permisos
    chown -R "$USER_NAME:$GROUP_NAME" "$INSTALL_DIR"
    chown -R "$USER_NAME:$GROUP_NAME" "$LOG_DIR"
    chown -R "$USER_NAME:$GROUP_NAME" "$DATA_DIR"
    chmod 755 "$INSTALL_DIR"
    chmod 644 "$CONFIG_FILE"
}

# Compilar el proyecto
#
# Descarga de dependencias Go: reintentos y fallbacks para redes lentas o bloqueo de proxy.golang.org
# Guardamos el último log de error para mostrarlo si fallan todos los intentos
HOSTBERRY_GO_DEPS_ERROR_LOG="${HOSTBERRY_GO_DEPS_ERROR_LOG:-/tmp/hostberry_go_deps_error.log}"

try_go_mod_download() {
    local env_kv="$1"
    local attempt="$2"
    local max="$3"
    local tmp_log
    local timeout_secs="${HOSTBERRY_GO_MOD_DOWNLOAD_TIMEOUT:-180}"
    local ret=1

    tmp_log="$(mktemp)"
    export GOTOOLCHAIN=local

    if command -v timeout >/dev/null 2>&1; then
        if [ -n "$env_kv" ]; then
            timeout "$timeout_secs" env GOTOOLCHAIN=local $env_kv go mod download >"$tmp_log" 2>&1
        else
            timeout "$timeout_secs" env GOTOOLCHAIN=local go mod download >"$tmp_log" 2>&1
        fi
    else
        if [ -n "$env_kv" ]; then
            env GOTOOLCHAIN=local $env_kv go mod download >"$tmp_log" 2>&1
        else
            env GOTOOLCHAIN=local go mod download >"$tmp_log" 2>&1
        fi
    fi
    ret=$?

    if [ "$ret" -eq 0 ]; then
        rm -f "$tmp_log"
        return 0
    fi
    cp -f "$tmp_log" "$HOSTBERRY_GO_DEPS_ERROR_LOG" 2>/dev/null || true
    rm -f "$tmp_log"
    return 1
}

download_go_deps() {
    local retries="${HOSTBERRY_GO_MOD_RETRIES:-5}"
    local sleep_secs="${HOSTBERRY_GO_MOD_RETRY_SLEEP:-4}"

    # 1) Intentar con el entorno actual
    for ((i=1; i<=retries; i++)); do
        if try_go_mod_download "" "$i" "$retries"; then
            export HOSTBERRY_GO_MOD_ENV=""
            return 0
        fi
        sleep "$sleep_secs"
    done

    # 2) Fallback a modo directo (sin proxy)
    for ((i=1; i<=retries; i++)); do
        if try_go_mod_download "GOPROXY=direct" "$i" "$retries"; then
            export HOSTBERRY_GO_MOD_ENV="GOPROXY=direct"
            return 0
        fi
        sleep "$sleep_secs"
    done

    # 3) (Opcional) último recurso: desactivar sumdb (menos seguro)
    if [ "${HOSTBERRY_ALLOW_GOSUMDB_OFF:-0}" = "1" ]; then
        for ((i=1; i<=retries; i++)); do
            if try_go_mod_download "GOPROXY=direct GOSUMDB=off" "$i" "$retries"; then
                export HOSTBERRY_GO_MOD_ENV="GOPROXY=direct GOSUMDB=off"
                return 0
            fi
            sleep "$sleep_secs"
        done
    fi

    print_error "Error al descargar dependencias de Go"
    if [ -s "$HOSTBERRY_GO_DEPS_ERROR_LOG" ]; then
        print_info "Detalle del último intento:"
        cat "$HOSTBERRY_GO_DEPS_ERROR_LOG" | head -50
        rm -f "$HOSTBERRY_GO_DEPS_ERROR_LOG"
    fi
    print_info "Sugerencia: compruebe conexión a Internet, proxy/firewall (proxy.golang.org) o ejecute con HOSTBERRY_GO_MOD_RETRIES=10"
    return 1
}

# Lee la salida de `go build -v` y muestra porcentaje aproximado (una línea ≈ un paquete).
# Tras la última línea impresa, Go puede tardar mucho sin volver a escribir: compilación de
# `main` con go:embed (website/static + templates), enlazado y CGO — no es un cuelgue.
# pkg_total: resultado de `go list -deps` (debe calcularse antes del pipeline para evitar carreras).
show_build_progress() {
    local pkg_total="${1:-1}"
    local n=0 pct line hb_pid=""

    if ! [[ "$pkg_total" =~ ^[0-9]+$ ]] || [ "$pkg_total" -lt 1 ]; then
        pkg_total=1
    fi

    while IFS= read -r line || [ -n "$line" ]; do
        [ -z "$line" ] && continue
        n=$((n + 1))
        pct=$((n * 100 / pkg_total))
        if [ "$pct" -gt 100 ]; then
            pct=100
        fi
        # %b interpreta \033 en DIM/NC (definidos con comillas simples, son literales hasta %b/echo -e)
        printf '\r\033[K%b[%3d%%] %s%b' "$DIM" "$pct" "$line" "${NC}" >&2

        # Latido si no llega otra línea en ~12 s (fase larga sin salida en -v)
        if [ -n "$hb_pid" ] && kill -0 "$hb_pid" 2>/dev/null; then
            kill "$hb_pid" 2>/dev/null
            wait "$hb_pid" 2>/dev/null || true
        fi
        (
            sleep 12
            while true; do
                printf '\n%b   ... sigue compilando (no colgado): main/embed, enlazado o CGO; en Raspberry Pi puede tardar minutos. Último: %s%b\n' "$DIM" "$line" "$NC" >&2
                sleep 12
            done
        ) &
        hb_pid=$!
    done
    if [ -n "$hb_pid" ] && kill -0 "$hb_pid" 2>/dev/null; then
        kill "$hb_pid" 2>/dev/null
        wait "$hb_pid" 2>/dev/null || true
    fi
    echo "" >&2
}

# Instala el binario mkcert (apt o release en GitHub).
install_mkcert_binary() {
    if command -v mkcert &>/dev/null; then
        print_success "mkcert disponible"
        return 0
    fi
    print_info "Instalando mkcert..."
    if apt-get install -y mkcert 2>/dev/null && command -v mkcert &>/dev/null; then
        print_success "mkcert instalado (apt)"
        return 0
    fi
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
        if ! apt-get install -y avahi-daemon 2>/dev/null; then
            print_warning "No se pudo instalar avahi-daemon; hostberry.local puede no resolverse en la red."
            return 0
        fi
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

build_project() {
    print_info "Compilando HostBerry en ${INSTALL_DIR}..."
    
    # Verificar que estamos en el directorio correcto
    if [ ! -d "$INSTALL_DIR" ]; then
        print_error "Error: Directorio de instalación no existe: $INSTALL_DIR"
        exit 1
    fi
    
    # Cambiar al directorio de instalación
    cd "$INSTALL_DIR" || {
        print_error "Error: No se pudo cambiar al directorio $INSTALL_DIR"
        exit 1
    }
    
    print_info "Directorio de trabajo: $(pwd)"
    
    # Verificar que los templates están presentes antes de compilar
    if [ ! -d "${INSTALL_DIR}/website/templates" ]; then
        print_error "Error: Directorio de templates no encontrado: ${INSTALL_DIR}/website/templates"
        print_info "Verificando estructura del directorio..."
        ls -la "${INSTALL_DIR}/" 2>/dev/null || true
        exit 1
    fi
    
    TEMPLATE_COUNT=$(find "${INSTALL_DIR}/website/templates" -name "*.html" 2>/dev/null | wc -l)
    if [ "$TEMPLATE_COUNT" -eq 0 ]; then
        print_error "Error: No se encontraron archivos .html en ${INSTALL_DIR}/website/templates"
        print_info "Contenido del directorio:"
        ls -la "${INSTALL_DIR}/website/templates/" 2>/dev/null || true
        exit 1
    fi
    print_success "Verificado: $TEMPLATE_COUNT templates encontrados en ${INSTALL_DIR}/website/templates"
    
    # Verificar que main.go existe
    if [ ! -f "${INSTALL_DIR}/main.go" ]; then
        print_error "Error: main.go no encontrado en ${INSTALL_DIR}"
        print_info "Archivos .go encontrados:"
        ls -la "${INSTALL_DIR}"/*.go 2>/dev/null || true
        exit 1
    fi
    
    # Verificar que go.mod existe
    if [ ! -f "${INSTALL_DIR}/go.mod" ]; then
        print_error "Error: go.mod no encontrado en ${INSTALL_DIR}"
        exit 1
    fi
    
    # Asegurar que Go está en el PATH
    export PATH=$PATH:/usr/local/go/bin
    
    # Verificar que Go está disponible
    if ! command -v go &> /dev/null; then
        print_error "Error: Go no está instalado o no está en el PATH"
        exit 1
    fi
    
    # Verificar estructura antes de compilar
    if [ ! -f "${INSTALL_DIR}/main.go" ]; then
        print_error "Error: main.go no encontrado"
        exit 1
    fi
    
    if [ ! -d "${INSTALL_DIR}/website/templates" ]; then
        print_error "Error: Directorio de templates no encontrado"
        exit 1
    fi
    
    # Descargar dependencias (puede tardar; con timeout para no colgarse)
    print_info "Descargando dependencias Go (puede tardar 1-2 min)..."
    if ! download_go_deps; then
        exit 1
    fi
    
    # Fuerza modo módulos para evitar que un GO111MODULE=off externo rompa imports.
    export GO111MODULE=on
    export GOWORK=off

    export GOTOOLCHAIN=local
    env $HOSTBERRY_GO_MOD_ENV go mod tidy > /dev/null 2>&1 || true
    
    # Acelerar compilación: todos los núcleos, ccache para CGO si está instalado
    BUILD_JOBS=$(nproc 2>/dev/null || echo 4)
    export GOMAXPROCS="${BUILD_JOBS}"
    export CGO_ENABLED=1
    if command -v ccache &>/dev/null; then
        export CC="ccache gcc"
    fi
    
    BUILD_TIMEOUT="${HOSTBERRY_BUILD_TIMEOUT:-900}"

    # Total de paquetes (dependencias + main) para porcentaje aproximado con `go build -v`
    BUILD_PKG_TOTAL=$(go list -deps -f '{{.ImportPath}}' . 2>/dev/null | wc -l | tr -d ' \n')
    BUILD_PKG_TOTAL=${BUILD_PKG_TOTAL:-1}
    if [ "$BUILD_PKG_TOTAL" -lt 1 ] 2>/dev/null; then
        BUILD_PKG_TOTAL=1
    fi

    print_info "Compilando (usando ${BUILD_JOBS} núcleos, ~${BUILD_PKG_TOTAL} paquetes; el % es orientativo)."
    print_info "Si la última línea se queda fija, es normal: Go sigue con main (embed), enlazado y CGO — puede tardar minutos en Raspberry Pi."
    build_ret=0
    set +e
    set -o pipefail 2>/dev/null || true

    if command -v timeout >/dev/null 2>&1; then
        if command -v stdbuf >/dev/null 2>&1; then
            timeout "$BUILD_TIMEOUT" stdbuf -oL -eL env $HOSTBERRY_GO_MOD_ENV go build -p "$BUILD_JOBS" -trimpath -ldflags="-s -w" -v -o "${INSTALL_DIR}/hostberry" . 2>&1 | show_build_progress "$BUILD_PKG_TOTAL"
        else
            timeout "$BUILD_TIMEOUT" env $HOSTBERRY_GO_MOD_ENV go build -p "$BUILD_JOBS" -trimpath -ldflags="-s -w" -v -o "${INSTALL_DIR}/hostberry" . 2>&1 | show_build_progress "$BUILD_PKG_TOTAL"
        fi
        build_ret=${PIPESTATUS[0]:-1}
    else
        if command -v stdbuf >/dev/null 2>&1; then
            stdbuf -oL -eL env $HOSTBERRY_GO_MOD_ENV go build -p "$BUILD_JOBS" -trimpath -ldflags="-s -w" -v -o "${INSTALL_DIR}/hostberry" . 2>&1 | show_build_progress "$BUILD_PKG_TOTAL"
        else
            env $HOSTBERRY_GO_MOD_ENV go build -p "$BUILD_JOBS" -trimpath -ldflags="-s -w" -v -o "${INSTALL_DIR}/hostberry" . 2>&1 | show_build_progress "$BUILD_PKG_TOTAL"
        fi
        build_ret=${PIPESTATUS[0]:-1}
    fi

    set +o pipefail 2>/dev/null || true
    set -e
    if [ "$build_ret" -eq 0 ] && [ -f "${INSTALL_DIR}/hostberry" ]; then
        chmod +x "${INSTALL_DIR}/hostberry"
        chown "$USER_NAME:$GROUP_NAME" "${INSTALL_DIR}/hostberry"
        print_success "Compilación completada."
    elif [ "$build_ret" -eq 124 ]; then
        print_error "Compilación cancelada: tiempo de espera agotado (${BUILD_TIMEOUT}s). En Raspberry Pi puede tardar más; ejecute de nuevo con HOSTBERRY_BUILD_TIMEOUT=1200"
        exit 1
    else
        print_error "Error en la compilación (código $build_ret). Compruebe que build-essential y gcc están instalados."
        exit 1
    fi
}

# Configurar firewall
configure_firewall() {
    local HTTPS_PORT HTTP_EXTRA
    HTTPS_PORT=$(awk '/^[[:space:]]*port:/{gsub(/"/,"",$2); print $2; exit}' "$CONFIG_FILE" 2>/dev/null)
    HTTPS_PORT="${HTTPS_PORT:-8000}"
    HTTP_EXTRA=$(awk '/^[[:space:]]*http_redirect_port:/{gsub(/"/,"",$2); print $2; exit}' "$CONFIG_FILE" 2>/dev/null)

    # Verificar si ufw está instalado y activo
    if command -v ufw &> /dev/null; then
        if ufw status | grep -q "Status: active"; then
            ufw allow "${HTTPS_PORT}/tcp" 2>/dev/null || true
            if [ -n "$HTTP_EXTRA" ] && [ "$HTTP_EXTRA" != "0" ] && [ "$HTTP_EXTRA" != "$HTTPS_PORT" ]; then
                ufw allow "${HTTP_EXTRA}/tcp" 2>/dev/null || true
            fi
        fi
    elif command -v firewall-cmd &> /dev/null; then
        firewall-cmd --permanent --add-port="${HTTPS_PORT}/tcp" 2>/dev/null || true
        if [ -n "$HTTP_EXTRA" ] && [ "$HTTP_EXTRA" != "0" ] && [ "$HTTP_EXTRA" != "$HTTPS_PORT" ]; then
            firewall-cmd --permanent --add-port="${HTTP_EXTRA}/tcp" 2>/dev/null || true
        fi
        firewall-cmd --reload 2>/dev/null || true
    fi
}

# Crear base de datos inicial
create_database() {
    print_info "Preparando base de datos..."
    
    # Asegurar que el directorio de datos existe
    mkdir -p "$DATA_DIR"
    chown -R "$USER_NAME:$GROUP_NAME" "$DATA_DIR"
    chmod 755 "$DATA_DIR"
    
    # El archivo de BD se creará automáticamente al iniciar el servicio
    # pero creamos el directorio y verificamos permisos
    DB_FILE="${DATA_DIR}/hostberry.db"
    if [ -f "$DB_FILE" ]; then
        print_info "Base de datos existente encontrada: $DB_FILE"
        chown "$USER_NAME:$GROUP_NAME" "$DB_FILE"
        chmod 644 "$DB_FILE"
        print_warning "Si la BD tiene datos antiguos, el usuario admin puede no crearse automáticamente"
    else
        print_info "Base de datos se creará automáticamente al iniciar el servicio"
        print_info "El usuario admin se creará automáticamente si la BD está vacía"
    fi
    
    print_success "Directorio de base de datos preparado: $DATA_DIR"
}

# Configurar permisos y sudoers
configure_permissions() {
    print_info "Configurando permisos y sudoers..."
    
    # Crear directorio para scripts seguros
    SAFE_DIR="/usr/local/sbin/hostberry-safe"
    mkdir -p "$SAFE_DIR"
    
    # Crear script set-timezone
    cat > "$SAFE_DIR/set-timezone" <<EOF
#!/bin/bash
TZ="\$1"
if [ -z "\$TZ" ]; then echo "Timezone required"; exit 1; fi
if [ ! -f "/usr/share/zoneinfo/\$TZ" ]; then echo "Invalid timezone"; exit 1; fi
timedatectl set-timezone "\$TZ"
EOF
    chmod 750 "$SAFE_DIR/set-timezone"
    chown root:$GROUP_NAME "$SAFE_DIR/set-timezone"
    
    # Detectar rutas de comandos WiFi
    NMCLI_PATH=""
    RFKILL_PATH=""
    IFCONFIG_PATH=""
    IW_PATH=""
    IWCONFIG_PATH=""
    
    # Buscar nmcli
    if command -v nmcli &> /dev/null; then
        NMCLI_PATH=$(command -v nmcli)
    elif [ -f "/usr/bin/nmcli" ]; then
        NMCLI_PATH="/usr/bin/nmcli"
    fi
    
    # Buscar rfkill
    if command -v rfkill &> /dev/null; then
        RFKILL_PATH=$(command -v rfkill)
    elif [ -f "/usr/sbin/rfkill" ]; then
        RFKILL_PATH="/usr/sbin/rfkill"
    fi
    
    # Buscar ifconfig
    if command -v ifconfig &> /dev/null; then
        IFCONFIG_PATH=$(command -v ifconfig)
    elif [ -f "/sbin/ifconfig" ]; then
        IFCONFIG_PATH="/sbin/ifconfig"
    elif [ -f "/usr/sbin/ifconfig" ]; then
        IFCONFIG_PATH="/usr/sbin/ifconfig"
    fi
    
    # Buscar iw (para cambiar región WiFi)
    if command -v iw &> /dev/null; then
        IW_PATH=$(command -v iw)
    elif [ -f "/usr/sbin/iw" ]; then
        IW_PATH="/usr/sbin/iw"
    elif [ -f "/sbin/iw" ]; then
        IW_PATH="/sbin/iw"
    fi
    
    # Buscar iwconfig (para gestión WiFi)
    if command -v iwconfig &> /dev/null; then
        IWCONFIG_PATH=$(command -v iwconfig)
    elif [ -f "/usr/sbin/iwconfig" ]; then
        IWCONFIG_PATH="/usr/sbin/iwconfig"
    elif [ -f "/sbin/iwconfig" ]; then
        IWCONFIG_PATH="/sbin/iwconfig"
    fi
    
    # Detectar rutas de comandos de sistema
    REBOOT_PATH=""
    SHUTDOWN_PATH=""
    
    # Buscar reboot
    if command -v reboot &> /dev/null; then
        REBOOT_PATH=$(command -v reboot)
    elif [ -f "/usr/sbin/reboot" ]; then
        REBOOT_PATH="/usr/sbin/reboot"
    elif [ -f "/sbin/reboot" ]; then
        REBOOT_PATH="/sbin/reboot"
    fi
    
    # Buscar shutdown (ya detectado arriba, pero asegurarse)
    if command -v shutdown &> /dev/null; then
        SHUTDOWN_PATH=$(command -v shutdown)
    elif [ -f "/usr/sbin/shutdown" ]; then
        SHUTDOWN_PATH="/usr/sbin/shutdown"
    elif [ -f "/sbin/shutdown" ]; then
        SHUTDOWN_PATH="/sbin/shutdown"
    fi
    
    # Configurar sudoers con configuración para evitar logs en sistemas read-only
    cat > "/etc/sudoers.d/hostberry" <<EOF
# Permisos para HostBerry
# Deshabilitar logging de sudo para evitar errores en sistemas read-only
Defaults!ALL !logfile
Defaults!ALL !syslog
$USER_NAME ALL=(ALL) NOPASSWD: $SAFE_DIR/set-timezone
EOF
    
    # Agregar permisos para shutdown si está disponible
    if [ -n "$SHUTDOWN_PATH" ]; then
        echo "$USER_NAME ALL=(ALL) NOPASSWD: $SHUTDOWN_PATH" >> "/etc/sudoers.d/hostberry"
    fi
    
    # Agregar permisos para reboot si está disponible
    if [ -n "$REBOOT_PATH" ]; then
        echo "$USER_NAME ALL=(ALL) NOPASSWD: $REBOOT_PATH" >> "/etc/sudoers.d/hostberry"
    fi
    
    # También agregar permisos para systemctl (más moderno y confiable)
    if command -v systemctl &> /dev/null; then
        SYSTEMCTL_PATH=$(command -v systemctl)
        echo "$USER_NAME ALL=(ALL) NOPASSWD: $SYSTEMCTL_PATH reboot" >> "/etc/sudoers.d/hostberry"
        echo "$USER_NAME ALL=(ALL) NOPASSWD: $SYSTEMCTL_PATH poweroff" >> "/etc/sudoers.d/hostberry"
        echo "$USER_NAME ALL=(ALL) NOPASSWD: $SYSTEMCTL_PATH shutdown" >> "/etc/sudoers.d/hostberry"
    fi
    
    # Agregar permisos WiFi si los comandos están disponibles
    if [ -n "$NMCLI_PATH" ]; then
        echo "$USER_NAME ALL=(ALL) NOPASSWD: $NMCLI_PATH" >> "/etc/sudoers.d/hostberry"
    fi
    
    if [ -n "$RFKILL_PATH" ]; then
        echo "$USER_NAME ALL=(ALL) NOPASSWD: $RFKILL_PATH" >> "/etc/sudoers.d/hostberry"
    fi
    
    if [ -n "$IFCONFIG_PATH" ]; then
        echo "$USER_NAME ALL=(ALL) NOPASSWD: $IFCONFIG_PATH" >> "/etc/sudoers.d/hostberry"
    fi
    
    if [ -n "$IW_PATH" ]; then
        echo "$USER_NAME ALL=(ALL) NOPASSWD: $IW_PATH" >> "/etc/sudoers.d/hostberry"
    fi
    
    if [ -n "$IWCONFIG_PATH" ]; then
        echo "$USER_NAME ALL=(ALL) NOPASSWD: $IWCONFIG_PATH" >> "/etc/sudoers.d/hostberry"
    fi
    
    # Agregar permisos para hostapd y systemctl hostapd
    if command -v hostapd &> /dev/null; then
        HOSTAPD_PATH=$(command -v hostapd)
        echo "$USER_NAME ALL=(ALL) NOPASSWD: $HOSTAPD_PATH" >> "/etc/sudoers.d/hostberry"
    fi
    
    if command -v hostapd_cli &> /dev/null; then
        HOSTAPD_CLI_PATH=$(command -v hostapd_cli)
        echo "$USER_NAME ALL=(ALL) NOPASSWD: $HOSTAPD_CLI_PATH" >> "/etc/sudoers.d/hostberry"
    fi
    
    # Agregar permisos para wpa_supplicant y wpa_cli (para modo STA)
    if command -v wpa_supplicant &> /dev/null; then
        WPA_SUPPLICANT_PATH=$(command -v wpa_supplicant)
        echo "$USER_NAME ALL=(ALL) NOPASSWD: $WPA_SUPPLICANT_PATH" >> "/etc/sudoers.d/hostberry"
    fi
    
    # Agregar permisos para rutas estándar de wpa_supplicant (por si no está en PATH)
    for wpa_path in "/usr/sbin/wpa_supplicant" "/sbin/wpa_supplicant" "/usr/bin/wpa_supplicant" "/bin/wpa_supplicant"; do
        if [ -f "$wpa_path" ]; then
            echo "$USER_NAME ALL=(ALL) NOPASSWD: $wpa_path" >> "/etc/sudoers.d/hostberry"
        fi
    done
    
    if command -v wpa_cli &> /dev/null; then
        WPA_CLI_PATH=$(command -v wpa_cli)
        echo "$USER_NAME ALL=(ALL) NOPASSWD: $WPA_CLI_PATH" >> "/etc/sudoers.d/hostberry"
    elif [ -f "/usr/sbin/wpa_cli" ]; then
        echo "$USER_NAME ALL=(ALL) NOPASSWD: /usr/sbin/wpa_cli" >> "/etc/sudoers.d/hostberry"
    elif [ -f "/sbin/wpa_cli" ]; then
        echo "$USER_NAME ALL=(ALL) NOPASSWD: /sbin/wpa_cli" >> "/etc/sudoers.d/hostberry"
    fi
    
    # Agregar permisos para systemctl con wpa_supplicant
    if command -v systemctl &> /dev/null; then
        SYSTEMCTL_PATH=$(command -v systemctl)
        echo "$USER_NAME ALL=(ALL) NOPASSWD: $SYSTEMCTL_PATH start wpa_supplicant" >> "/etc/sudoers.d/hostberry"
        echo "$USER_NAME ALL=(ALL) NOPASSWD: $SYSTEMCTL_PATH stop wpa_supplicant" >> "/etc/sudoers.d/hostberry"
        echo "$USER_NAME ALL=(ALL) NOPASSWD: $SYSTEMCTL_PATH restart wpa_supplicant" >> "/etc/sudoers.d/hostberry"
        echo "$USER_NAME ALL=(ALL) NOPASSWD: $SYSTEMCTL_PATH status wpa_supplicant" >> "/etc/sudoers.d/hostberry"
        echo "$USER_NAME ALL=(ALL) NOPASSWD: $SYSTEMCTL_PATH stop NetworkManager" >> "/etc/sudoers.d/hostberry"
    fi
    
    # Agregar permisos para systemctl con hostapd y dnsmasq
    if command -v systemctl &> /dev/null; then
        SYSTEMCTL_PATH=$(command -v systemctl)
        echo "$USER_NAME ALL=(ALL) NOPASSWD: $SYSTEMCTL_PATH start hostapd" >> "/etc/sudoers.d/hostberry"
        echo "$USER_NAME ALL=(ALL) NOPASSWD: $SYSTEMCTL_PATH stop hostapd" >> "/etc/sudoers.d/hostberry"
        echo "$USER_NAME ALL=(ALL) NOPASSWD: $SYSTEMCTL_PATH restart hostapd" >> "/etc/sudoers.d/hostberry"
        echo "$USER_NAME ALL=(ALL) NOPASSWD: $SYSTEMCTL_PATH status hostapd" >> "/etc/sudoers.d/hostberry"
        echo "$USER_NAME ALL=(ALL) NOPASSWD: $SYSTEMCTL_PATH enable hostapd" >> "/etc/sudoers.d/hostberry"
        echo "$USER_NAME ALL=(ALL) NOPASSWD: $SYSTEMCTL_PATH disable hostapd" >> "/etc/sudoers.d/hostberry"
        echo "$USER_NAME ALL=(ALL) NOPASSWD: $SYSTEMCTL_PATH unmask hostapd" >> "/etc/sudoers.d/hostberry"
        echo "$USER_NAME ALL=(ALL) NOPASSWD: $SYSTEMCTL_PATH start dnsmasq" >> "/etc/sudoers.d/hostberry"
        echo "$USER_NAME ALL=(ALL) NOPASSWD: $SYSTEMCTL_PATH stop dnsmasq" >> "/etc/sudoers.d/hostberry"
        echo "$USER_NAME ALL=(ALL) NOPASSWD: $SYSTEMCTL_PATH restart dnsmasq" >> "/etc/sudoers.d/hostberry"
        echo "$USER_NAME ALL=(ALL) NOPASSWD: $SYSTEMCTL_PATH enable dnsmasq" >> "/etc/sudoers.d/hostberry"
        echo "$USER_NAME ALL=(ALL) NOPASSWD: $SYSTEMCTL_PATH disable dnsmasq" >> "/etc/sudoers.d/hostberry"
        echo "$USER_NAME ALL=(ALL) NOPASSWD: $SYSTEMCTL_PATH unmask dnsmasq" >> "/etc/sudoers.d/hostberry"
        echo "$USER_NAME ALL=(ALL) NOPASSWD: $SYSTEMCTL_PATH daemon-reload" >> "/etc/sudoers.d/hostberry"
        # Blocky (Adblock DNS proxy)
        echo "$USER_NAME ALL=(ALL) NOPASSWD: $SYSTEMCTL_PATH start blocky" >> "/etc/sudoers.d/hostberry"
        echo "$USER_NAME ALL=(ALL) NOPASSWD: $SYSTEMCTL_PATH stop blocky" >> "/etc/sudoers.d/hostberry"
        echo "$USER_NAME ALL=(ALL) NOPASSWD: $SYSTEMCTL_PATH enable blocky" >> "/etc/sudoers.d/hostberry"
        echo "$USER_NAME ALL=(ALL) NOPASSWD: $SYSTEMCTL_PATH disable blocky" >> "/etc/sudoers.d/hostberry"
        echo "$USER_NAME ALL=(ALL) NOPASSWD: $SYSTEMCTL_PATH restart blocky" >> "/etc/sudoers.d/hostberry"
        # systemd-resolved (para resolv.conf al activar/desactivar Blocky)
        echo "$USER_NAME ALL=(ALL) NOPASSWD: $SYSTEMCTL_PATH restart systemd-resolved" >> "/etc/sudoers.d/hostberry"
    fi
    
    # Tor: permitir start/stop/enable/disable sin contraseña (habilitar Tor desde la web)
    echo "$USER_NAME ALL=(ALL) NOPASSWD: $SYSTEMCTL_PATH start tor" >> "/etc/sudoers.d/hostberry"
    echo "$USER_NAME ALL=(ALL) NOPASSWD: $SYSTEMCTL_PATH stop tor" >> "/etc/sudoers.d/hostberry"
    echo "$USER_NAME ALL=(ALL) NOPASSWD: $SYSTEMCTL_PATH enable tor" >> "/etc/sudoers.d/hostberry"
    echo "$USER_NAME ALL=(ALL) NOPASSWD: $SYSTEMCTL_PATH disable tor" >> "/etc/sudoers.d/hostberry"
    
    # Agregar permisos para hostnamectl y hostname (cambio de hostname)
    if command -v hostnamectl &> /dev/null; then
        HOSTNAMECTL_PATH=$(command -v hostnamectl)
        echo "$USER_NAME ALL=(ALL) NOPASSWD: $HOSTNAMECTL_PATH" >> "/etc/sudoers.d/hostberry"
    elif [ -f "/usr/bin/hostnamectl" ]; then
        echo "$USER_NAME ALL=(ALL) NOPASSWD: /usr/bin/hostnamectl" >> "/etc/sudoers.d/hostberry"
    fi
    
    if command -v hostname &> /dev/null; then
        HOSTNAME_PATH=$(command -v hostname)
        echo "$USER_NAME ALL=(ALL) NOPASSWD: $HOSTNAME_PATH" >> "/etc/sudoers.d/hostberry"
    elif [ -f "/bin/hostname" ]; then
        echo "$USER_NAME ALL=(ALL) NOPASSWD: /bin/hostname" >> "/etc/sudoers.d/hostberry"
    fi
    
    # Agregar permisos para ip (configuración de interfaces de red)
    if command -v ip &> /dev/null; then
        IP_PATH=$(command -v ip)
        echo "$USER_NAME ALL=(ALL) NOPASSWD: $IP_PATH" >> "/etc/sudoers.d/hostberry"
    elif [ -f "/usr/sbin/ip" ]; then
        echo "$USER_NAME ALL=(ALL) NOPASSWD: /usr/sbin/ip" >> "/etc/sudoers.d/hostberry"
    elif [ -f "/sbin/ip" ]; then
        echo "$USER_NAME ALL=(ALL) NOPASSWD: /sbin/ip" >> "/etc/sudoers.d/hostberry"
    fi
    
    # Agregar permisos para pkill (para detener procesos wpa_supplicant)
    if command -v pkill &> /dev/null; then
        PKILL_PATH=$(command -v pkill)
        echo "$USER_NAME ALL=(ALL) NOPASSWD: $PKILL_PATH" >> "/etc/sudoers.d/hostberry"
    elif [ -f "/usr/bin/pkill" ]; then
        echo "$USER_NAME ALL=(ALL) NOPASSWD: /usr/bin/pkill" >> "/etc/sudoers.d/hostberry"
    fi
    
    # Agregar permisos para pgrep (para verificar procesos)
    if command -v pgrep &> /dev/null; then
        PGREP_PATH=$(command -v pgrep)
        echo "$USER_NAME ALL=(ALL) NOPASSWD: $PGREP_PATH" >> "/etc/sudoers.d/hostberry"
    elif [ -f "/usr/bin/pgrep" ]; then
        echo "$USER_NAME ALL=(ALL) NOPASSWD: /usr/bin/pgrep" >> "/etc/sudoers.d/hostberry"
    fi
    
    # Agregar permisos para dhclient y udhcpc (para obtener IP)
    if command -v dhclient &> /dev/null; then
        DHCPCLIENT_PATH=$(command -v dhclient)
        echo "$USER_NAME ALL=(ALL) NOPASSWD: $DHCPCLIENT_PATH" >> "/etc/sudoers.d/hostberry"
    elif [ -f "/usr/sbin/dhclient" ]; then
        echo "$USER_NAME ALL=(ALL) NOPASSWD: /usr/sbin/dhclient" >> "/etc/sudoers.d/hostberry"
    fi
    
    if command -v udhcpc &> /dev/null; then
        UDHCPC_PATH=$(command -v udhcpc)
        echo "$USER_NAME ALL=(ALL) NOPASSWD: $UDHCPC_PATH" >> "/etc/sudoers.d/hostberry"
    elif [ -f "/usr/sbin/udhcpc" ]; then
        echo "$USER_NAME ALL=(ALL) NOPASSWD: /usr/sbin/udhcpc" >> "/etc/sudoers.d/hostberry"
    fi
    
    # Agregar permisos para sysctl (habilitar IP forwarding)
    if command -v sysctl &> /dev/null; then
        SYSCTL_PATH=$(command -v sysctl)
        echo "$USER_NAME ALL=(ALL) NOPASSWD: $SYSCTL_PATH" >> "/etc/sudoers.d/hostberry"
    elif [ -f "/usr/sbin/sysctl" ]; then
        echo "$USER_NAME ALL=(ALL) NOPASSWD: /usr/sbin/sysctl" >> "/etc/sudoers.d/hostberry"
    elif [ -f "/sbin/sysctl" ]; then
        echo "$USER_NAME ALL=(ALL) NOPASSWD: /sbin/sysctl" >> "/etc/sudoers.d/hostberry"
    fi
    
    # Agregar permisos para iptables (configuración de NAT)
    if command -v iptables &> /dev/null; then
        IPTABLES_PATH=$(command -v iptables)
        echo "$USER_NAME ALL=(ALL) NOPASSWD: $IPTABLES_PATH" >> "/etc/sudoers.d/hostberry"
    elif [ -f "/usr/sbin/iptables" ]; then
        echo "$USER_NAME ALL=(ALL) NOPASSWD: /usr/sbin/iptables" >> "/etc/sudoers.d/hostberry"
    elif [ -f "/sbin/iptables" ]; then
        echo "$USER_NAME ALL=(ALL) NOPASSWD: /sbin/iptables" >> "/etc/sudoers.d/hostberry"
    fi
    
    # Agregar permisos para comandos básicos necesarios para hostapd
    # cp (para copiar archivos de configuración)
    if command -v cp &> /dev/null; then
        CP_PATH=$(command -v cp)
        echo "$USER_NAME ALL=(ALL) NOPASSWD: $CP_PATH" >> "/etc/sudoers.d/hostberry"
    elif [ -f "/bin/cp" ]; then
        echo "$USER_NAME ALL=(ALL) NOPASSWD: /bin/cp" >> "/etc/sudoers.d/hostberry"
    fi
    
    # mkdir (para crear directorios de configuración)
    if command -v mkdir &> /dev/null; then
        MKDIR_PATH=$(command -v mkdir)
        echo "$USER_NAME ALL=(ALL) NOPASSWD: $MKDIR_PATH" >> "/etc/sudoers.d/hostberry"
    elif [ -f "/bin/mkdir" ]; then
        echo "$USER_NAME ALL=(ALL) NOPASSWD: /bin/mkdir" >> "/etc/sudoers.d/hostberry"
    fi
    
    # chmod (para establecer permisos de archivos)
    if command -v chmod &> /dev/null; then
        CHMOD_PATH=$(command -v chmod)
        echo "$USER_NAME ALL=(ALL) NOPASSWD: $CHMOD_PATH" >> "/etc/sudoers.d/hostberry"
    elif [ -f "/bin/chmod" ]; then
        echo "$USER_NAME ALL=(ALL) NOPASSWD: /bin/chmod" >> "/etc/sudoers.d/hostberry"
    fi
    
    # tee (para escribir archivos de configuración)
    if command -v tee &> /dev/null; then
        TEE_PATH=$(command -v tee)
        echo "$USER_NAME ALL=(ALL) NOPASSWD: $TEE_PATH" >> "/etc/sudoers.d/hostberry"
    elif [ -f "/usr/bin/tee" ]; then
        echo "$USER_NAME ALL=(ALL) NOPASSWD: /usr/bin/tee" >> "/etc/sudoers.d/hostberry"
    fi
    
    # cat (para leer archivos y pasarlos a tee)
    if command -v cat &> /dev/null; then
        CAT_PATH=$(command -v cat)
        echo "$USER_NAME ALL=(ALL) NOPASSWD: $CAT_PATH" >> "/etc/sudoers.d/hostberry"
    elif [ -f "/bin/cat" ]; then
        echo "$USER_NAME ALL=(ALL) NOPASSWD: /bin/cat" >> "/etc/sudoers.d/hostberry"
    fi
    
    # grep (para buscar en archivos como /etc/hosts)
    if command -v grep &> /dev/null; then
        GREP_PATH=$(command -v grep)
        echo "$USER_NAME ALL=(ALL) NOPASSWD: $GREP_PATH" >> "/etc/sudoers.d/hostberry"
    elif [ -f "/bin/grep" ]; then
        echo "$USER_NAME ALL=(ALL) NOPASSWD: /bin/grep" >> "/etc/sudoers.d/hostberry"
    elif [ -f "/usr/bin/grep" ]; then
        echo "$USER_NAME ALL=(ALL) NOPASSWD: /usr/bin/grep" >> "/etc/sudoers.d/hostberry"
    fi
    
    # sed (para reemplazar texto en archivos como /etc/hosts)
    if command -v sed &> /dev/null; then
        SED_PATH=$(command -v sed)
        echo "$USER_NAME ALL=(ALL) NOPASSWD: $SED_PATH" >> "/etc/sudoers.d/hostberry"
    elif [ -f "/bin/sed" ]; then
        echo "$USER_NAME ALL=(ALL) NOPASSWD: /bin/sed" >> "/etc/sudoers.d/hostberry"
    elif [ -f "/usr/bin/sed" ]; then
        echo "$USER_NAME ALL=(ALL) NOPASSWD: /usr/bin/sed" >> "/etc/sudoers.d/hostberry"
    fi
    
    # mount (para remontar sistemas de archivos de solo lectura como lectura-escritura)
    if command -v mount &> /dev/null; then
        MOUNT_PATH=$(command -v mount)
        echo "$USER_NAME ALL=(ALL) NOPASSWD: $MOUNT_PATH" >> "/etc/sudoers.d/hostberry"
    elif [ -f "/bin/mount" ]; then
        echo "$USER_NAME ALL=(ALL) NOPASSWD: /bin/mount" >> "/etc/sudoers.d/hostberry"
    elif [ -f "/usr/bin/mount" ]; then
        echo "$USER_NAME ALL=(ALL) NOPASSWD: /usr/bin/mount" >> "/etc/sudoers.d/hostberry"
    fi
    
    # Crear directorio /etc/hostapd con permisos correctos
    print_info "Creando directorio /etc/hostapd..."
    if [ ! -d "/etc/hostapd" ]; then
        mkdir -p /etc/hostapd
        chmod 755 /etc/hostapd
        print_success "Directorio /etc/hostapd creado con permisos 755"
    else
        chmod 755 /etc/hostapd 2>/dev/null || true
        print_info "Directorio /etc/hostapd ya existe, permisos verificados"
    fi
    
    # Crear también el directorio para systemd override si no existe
    if [ ! -d "/etc/systemd/system/hostapd.service.d" ]; then
        mkdir -p /etc/systemd/system/hostapd.service.d
        print_info "Directorio systemd override para hostapd creado"
    fi
    
    # Validar configuración de sudoers
    if visudo -c -f "/etc/sudoers.d/hostberry" 2>/dev/null; then
        chmod 440 "/etc/sudoers.d/hostberry"
        print_success "Permisos y sudoers configurados correctamente"
    else
        print_warning "Advertencia: Error al validar configuración de sudoers"
        print_info "Revisa manualmente: visudo -c -f /etc/sudoers.d/hostberry"
        chmod 440 "/etc/sudoers.d/hostberry"
    fi

    # Crear directorio temporal persistente para la configuración de wpa_supplicant
    FALLBACK_WPA_DIR="/tmp/hostberry/wpa_supplicant"
    if [ ! -d "$FALLBACK_WPA_DIR" ]; then
        mkdir -p "$FALLBACK_WPA_DIR"
        chown root:netdev "$FALLBACK_WPA_DIR"
        chmod 775 "$FALLBACK_WPA_DIR"
        print_info "Directorio temporal persistente creado: $FALLBACK_WPA_DIR"
    else
        chown root:netdev "$FALLBACK_WPA_DIR"
        chmod 775 "$FALLBACK_WPA_DIR"
    fi
}

# Crear configuración por defecto de HostAPD
create_hostapd_default_config() {
    print_info "Creando configuración por defecto de HostAPD…"
    # Install/update: siempre aplicar udev, create-ap0, hostapd, etc. (automatizado).
    # Si entras por SSH sobre la misma WiFi que usa la Pi, el AP en caliente puede cortar la sesión unos segundos; usa cable o consola si hace falta.
    if [ -n "${SSH_CONNECTION:-}" ] || [ -n "${SSH_TTY:-}" ]; then
        print_info "SSH detectado: se aplican igualmente ap0/hostapd/systemd (comportamiento automatizado)."
    fi
    if [ "$(is_default_route_over_wifi)" = "1" ]; then
        print_warning "Ruta por defecto por WiFi: al levantar el AP la conexión SSH por WiFi puede interrumpirse brevemente."
    fi
    
    # Valores por defecto (red "hostberry" abierta + portal cautivo hacia la web de Hostberry)
    HOSTAPD_INTERFACE="wlan0"
    HOSTAPD_SSID="hostberry"
    HOSTAPD_CHANNEL="6"
    HOSTAPD_GATEWAY="192.168.4.1"
    HOSTAPD_DHCP_START="192.168.4.2"
    HOSTAPD_DHCP_END="192.168.4.254"
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
        if command -v iw &> /dev/null; then
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
# Asegurar que wlan0 esté en modo managed (no AP)
# Esto se hace automáticamente cuando wpa_supplicant se ejecuta en wlan0
EOF
        chmod 644 "$HOSTAPD_CONFIG"
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
# Alinear canal/banda del AP (ap0) con la STA en la misma radio. Si el grep falla con el formato de iw,
# hostapd se queda en canal distinto y en muchas Pi no se ven beacons / SSID "hostberry".
CONF="${HOSTAPD_CONFIG}"
WLAN="${HOSTAPD_INTERFACE}"
[ -f "\$CONF" ] || exit 0
parse_channel_line() {
    echo "\$1" | sed -n 's/.*channel[[:space:]]\\+\\([0-9]\\+\\).*/\\1/p' | head -1
}
parse_freq_mhz() {
    echo "\$1" | sed -n 's/.*[(]\\([0-9]\\{4,\\}\\)[[:space:]]*MHz[)].*/\\1/p' | head -1
}
info_out=\$(iw dev "\$WLAN" info 2>/dev/null || true)
link_out=\$(iw dev "\$WLAN" link 2>/dev/null || true)
line=\$(echo "\$info_out" | grep -E 'channel[[:space:]]+[0-9]+' | head -1)
[ -n "\$line" ] || line=\$(echo "\$link_out" | grep -E 'channel[[:space:]]+[0-9]+' | head -1)
[ -n "\$line" ] || exit 0
ch=\$(parse_channel_line "\$line")
freq=\$(parse_freq_mhz "\$line")
[ -n "\$ch" ] || exit 0
if [ -z "\$freq" ]; then
    if [ "\$ch" -le 14 ] 2>/dev/null; then freq=2437; else freq=5180; fi
fi
if [ "\$freq" -lt 3000 ] 2>/dev/null; then
    sed -i 's/^hw_mode=.*/hw_mode=g/' "\$CONF"
    sed -i "s/^channel=.*/channel=\$ch/" "\$CONF"
    sed -i '/^ieee80211ac=/d' "\$CONF"
    sed -i '/^vht_/d' "\$CONF"
else
    sed -i 's/^hw_mode=.*/hw_mode=a/' "\$CONF"
    sed -i "s/^channel=.*/channel=\$ch/" "\$CONF"
    sed -i '/^ieee80211ac=/d' "\$CONF"
    sed -i '/^vht_/d' "\$CONF"
    grep -q '^ieee80211n=' "\$CONF" || echo 'ieee80211n=1' >> "\$CONF"
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
        [ -n "$DNSMASQ_NO_DHCP_LINE" ] && echo "$DNSMASQ_NO_DHCP_LINE"
        echo "dhcp-range=${HOSTAPD_DHCP_START},${HOSTAPD_DHCP_END},255.255.255.0,${HOSTAPD_LEASE_TIME}"
        echo "dhcp-option=3,${HOSTAPD_GATEWAY}"
        echo "dhcp-option=6,${HOSTAPD_GATEWAY}"
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
        systemctl start create-ap0.service 2>/dev/null || true
        print_success "Servicio systemd para ap0 actualizado, habilitado e iniciado"
        sleep 2
        if ip link show ap0 > /dev/null 2>&1; then
            print_success "Interfaz ap0 creada y verificada por el servicio systemd"
        else
            print_warning "El servicio se inició pero ap0 no está disponible aún (puede necesitar reinicio)"
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
        systemctl start create-ap0.service 2>/dev/null || true
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
# DHCP: el prep de dnsmasq pone la IPv4 en ap0; aquí sólo reintentamos dnsmasq por si hostapd arrancó antes.
ExecStartPost=-/bin/systemctl try-restart dnsmasq.service
PIDFile=/run/hostapd.pid
Type=forking
TimeoutStartSec=90
EOF
    chmod 644 "$OVERRIDE_FILE"
    print_success "Override de hostapd actualizado"
    
    print_info "Verificando estado del servicio hostapd…"
    if systemctl is-enabled hostapd 2>&1 | grep -q "masked"; then
        print_info "Desbloqueando servicio hostapd…"
        systemctl unmask hostapd 2>/dev/null || true
        print_success "Servicio hostapd desbloqueado"
    fi
    
    systemctl daemon-reload 2>/dev/null || true
    
    # Asegurar que dnsmasq esté instalado y el servicio systemd exista (DHCP/DNS para la red hostberry)
    if ! systemctl list-unit-files 2>/dev/null | grep -q 'dnsmasq\.service'; then
        print_info "Servicio dnsmasq no encontrado; instalando paquete dnsmasq..."
        if command -v apt-get &> /dev/null; then
            apt-get update -qq && apt-get install -y dnsmasq 2>/dev/null || true
            systemctl daemon-reload 2>/dev/null || true
        fi
    fi
    if ! systemctl list-unit-files 2>/dev/null | grep -q 'dnsmasq\.service'; then
        print_warning "dnsmasq no está instalado. Instálalo manualmente para DHCP en la red hostberry (p. ej. sudo apt-get install dnsmasq)"
    fi
    
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

    # dnsmasq: tras hostapd; sin Requires= (evita bloqueos en arranque si dnsmasq falla una vez).
    if systemctl list-unit-files 2>/dev/null | grep -q 'dnsmasq\.service'; then
        DNSMASQ_OVERRIDE_DIR="/etc/systemd/system/dnsmasq.service.d"
        DNSMASQ_OVERRIDE_FILE="${DNSMASQ_OVERRIDE_DIR}/hostberry.conf"
        print_info "Configurando override systemd para dnsmasq (prep ap0 + tras hostapd)…"
        mkdir -p "$DNSMASQ_OVERRIDE_DIR"
        cat > "$DNSMASQ_OVERRIDE_FILE" <<EOF
[Unit]
After=network.target create-ap0.service hostapd.service
Wants=create-ap0.service hostapd.service

[Service]
ExecStartPre=${DNSMASQ_PREP_SCRIPT}
EOF
        chmod 644 "$DNSMASQ_OVERRIDE_FILE"
        print_success "Override de dnsmasq actualizado"
    fi
    
    print_info "HostAPD y dnsmasq configurados; se habilitan para el arranque y enable_and_start_hostberry_wifi_ap inicia el AP al final."
    systemctl daemon-reload 2>/dev/null || true
    
    # Asegurar permisos correctos del archivo de configuración
    chmod 644 "$HOSTAPD_CONFIG" 2>/dev/null || true
    
    # ----- Portal cautivo: redirigir HTTP (80) desde ap0 al puerto de la web de Hostberry -----
    CAPTIVE_SCRIPT="${INSTALL_DIR}/scripts/captive-portal-setup.sh"
    CAPTIVE_SERVICE="/etc/systemd/system/hostberry-captive-portal.service"
    mkdir -p "${INSTALL_DIR}/scripts"
    print_info "Creando/actualizando script de portal cautivo (IP en ap0, DHCP, HTTP -> web Hostberry)..."
    cat > "$CAPTIVE_SCRIPT" <<'CAPTIVE_EOF'
#!/bin/bash
# HostBerry: asegurar IP en ap0, reiniciar dnsmasq (DHCP) y redirigir HTTP al portal
# Así los clientes reciben IP y al abrir el navegador ven la web de Hostberry

# 1) Asegurar que ap0 tenga la IP del gateway (necesaria para DHCP)
ip addr add 192.168.4.1/24 dev ap0 2>/dev/null || true

# 2) Reiniciar dnsmasq para que enlace con ap0 y reparta IPs a los clientes
systemctl restart dnsmasq 2>/dev/null || true

# 3) Portal cautivo: redirigir HTTP (80) al puerto HTTP de la web (redirección TLS o app en claro)
CONFIG_FILE="/opt/hostberry/config.yaml"
PORT=$(grep -E '^  http_redirect_port:' "$CONFIG_FILE" 2>/dev/null | head -1 | awk '{print $2}' | tr -d '"')
if [ -z "$PORT" ] || [ "$PORT" = "0" ]; then
  PORT=$(grep -E "^  port:" "$CONFIG_FILE" 2>/dev/null | awk '{print $2}' | tr -d '"' || echo "8000")
fi
iptables -t nat -D PREROUTING -i ap0 -p tcp --dport 80 -j REDIRECT --to-ports "$PORT" 2>/dev/null || true
iptables -t nat -A PREROUTING -i ap0 -p tcp --dport 80 -j REDIRECT --to-ports "$PORT"
exit 0
CAPTIVE_EOF
    chmod 755 "$CAPTIVE_SCRIPT"
    chown root:root "$CAPTIVE_SCRIPT"
    print_success "Script de portal cautivo actualizado: $CAPTIVE_SCRIPT"
    print_info "Actualizando unidad systemd del portal cautivo (tras HostBerry y dnsmasq)…"
    cat > "$CAPTIVE_SERVICE" <<EOF
[Unit]
Description=HostBerry captive portal - redirect HTTP to web UI on AP
After=network.target create-ap0.service hostapd.service dnsmasq.service ${SERVICE_NAME}.service
Wants=hostapd.service

[Service]
Type=oneshot
RemainAfterExit=yes
ExecStart=${CAPTIVE_SCRIPT}

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
hostberry_migrate_blocky_dns_loopback() {
    local BLOCKY_CONFIG_FILE="/etc/blocky/config.yml"
    [ -f "$BLOCKY_CONFIG_FILE" ] || return 0
    if ! grep -qE '^[[:space:]]*dns:[[:space:]]*("53"|53)[[:space:]]*$' "$BLOCKY_CONFIG_FILE" 2>/dev/null; then
        return 0
    fi
    sed -i 's/^\([[:space:]]*dns:\)[[:space:]]*"53"[[:space:]]*$/\1 127.0.0.1:53/' "$BLOCKY_CONFIG_FILE" 2>/dev/null || true
    sed -i 's/^\([[:space:]]*dns:\)[[:space:]]*53[[:space:]]*$/\1 127.0.0.1:53/' "$BLOCKY_CONFIG_FILE" 2>/dev/null || true
    print_info "Blocky: DNS restringido a 127.0.0.1:53 (compatible con dnsmasq en ap0)"
    if command -v systemctl &>/dev/null; then
        systemctl try-restart blocky.service 2>/dev/null || true
    fi
}

# Instalar Blocky (proxy DNS y ad-blocker para la página Adblock)
install_blocky() {
    local BLOCKY_VERSION="v0.28.2"
    local BLOCKY_CONFIG_DIR="/etc/blocky"
    local BLOCKY_CONFIG_FILE="${BLOCKY_CONFIG_DIR}/config.yml"
    local BLOCKY_SERVICE="/etc/systemd/system/blocky.service"

    hostberry_migrate_blocky_dns_loopback

    if [ -x "/usr/local/bin/blocky" ] || systemctl cat blocky &>/dev/null; then
        print_success "Blocky ya está instalado"
        return 0
    fi

    print_info "Instalando Blocky (DNS proxy y ad-blocker)..."

    local ARCH
    ARCH=$(uname -m)
    case "$ARCH" in
        x86_64|amd64) BLOCKY_ARCH="x86_64" ;;
        aarch64)      BLOCKY_ARCH="arm64" ;;
        armv7l|armhf) BLOCKY_ARCH="armv7" ;;
        armv6l)       BLOCKY_ARCH="armv6" ;;
        *)            BLOCKY_ARCH="x86_64" ;;
    esac

    local BLOCKY_URL="https://github.com/0xERR0R/blocky/releases/download/${BLOCKY_VERSION}/blocky_${BLOCKY_VERSION}_Linux_${BLOCKY_ARCH}.tar.gz"
    local TMP_BLOCKY="/tmp/blocky_install"
    rm -rf "$TMP_BLOCKY"
    mkdir -p "$TMP_BLOCKY"

    if command -v wget &>/dev/null; then
        wget -q -O "${TMP_BLOCKY}/blocky.tar.gz" "$BLOCKY_URL" || true
    else
        curl -sL -o "${TMP_BLOCKY}/blocky.tar.gz" "$BLOCKY_URL" || true
    fi

    if [ ! -s "${TMP_BLOCKY}/blocky.tar.gz" ]; then
        print_warning "No se pudo descargar Blocky (${BLOCKY_ARCH}). Puedes instalarlo desde la web Adblock más tarde."
        rm -rf "$TMP_BLOCKY"
        return 0
    fi

    export PATH="/usr/bin:/bin:$PATH"
    tar -xzf "${TMP_BLOCKY}/blocky.tar.gz" -C "$TMP_BLOCKY"
    cp "${TMP_BLOCKY}/blocky" /usr/local/bin/blocky
    chmod +x /usr/local/bin/blocky
    rm -rf "$TMP_BLOCKY"

    mkdir -p "$BLOCKY_CONFIG_DIR"
    if [ ! -f "$BLOCKY_CONFIG_FILE" ]; then
        cat > "$BLOCKY_CONFIG_FILE" <<'BLOCKYCONF'
# Blocky - Configuración por defecto HostBerry
upstreams:
  groups:
    default:
    - 1.1.1.1
    - 8.8.8.8
    - https://dns.cloudflare.com/dns-query

blocking:
  denylists:
    default:
    - https://raw.githubusercontent.com/StevenBlack/hosts/master/hosts
    - https://adguardteam.github.io/AdGuardSDNSFilter/Filters/filter.txt
  blockType: zeroIp
  refreshPeriod: 4h

ports:
  dns: 127.0.0.1:53
  http: 127.0.0.1:4000

log:
  level: warn
  format: text
BLOCKYCONF
        print_success "Configuración por defecto de Blocky creada: $BLOCKY_CONFIG_FILE"
    fi

    cat > "$BLOCKY_SERVICE" <<EOF
[Unit]
Description=Blocky DNS proxy and ad-blocker
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/blocky --config $BLOCKY_CONFIG_FILE
Restart=on-failure
RestartSec=3

[Install]
WantedBy=multi-user.target
EOF
    systemctl daemon-reload
    systemctl enable blocky
    print_success "Blocky instalado (arrancará con el sistema; configúralo desde la web Adblock)"
}

# Instalar LibreSpeed CLI (test de velocidad en la página Red) vía apt
install_librespeed_cli() {
    if command -v librespeed-cli &>/dev/null; then
        print_success "LibreSpeed CLI ya está instalado"
        return 0
    fi
    if ! command -v apt-get &>/dev/null; then
        print_warning "apt-get no disponible; instala librespeed-cli manualmente para el test de velocidad en Red"
        return 0
    fi
    print_info "Instalando LibreSpeed CLI (test de velocidad)..."
    if apt-get update -qq 2>/dev/null && apt-get install -y librespeed-cli >/dev/null 2>&1; then
        print_success "LibreSpeed CLI instalado (test de velocidad en la página Red)"
    else
        print_warning "No se pudo instalar librespeed-cli por apt. Instálalo con: sudo apt install librespeed-cli"
    fi
}

# Crear servicio systemd
create_systemd_service() {
    print_info "Creando servicio systemd..."
    
    cat > "$SERVICE_FILE" <<EOF
[Unit]
Description=HostBerry - Sistema de Gestión de Red
After=network.target

[Service]
Type=simple
User=${USER_NAME}
Group=${GROUP_NAME}
WorkingDirectory=${INSTALL_DIR}
Environment=HOSTBERRY_DEFAULT_ADMIN_PASSWORD=${GENERATED_ADMIN_PASSWORD}
ExecStart=${INSTALL_DIR}/hostberry
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal
SyslogIdentifier=${SERVICE_NAME}

# Seguridad
# NoNewPrivileges=true  # Deshabilitado para permitir sudo en comandos WiFi
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=${INSTALL_DIR} ${LOG_DIR} ${DATA_DIR}

# Puertos privilegiados 80/443 con usuario no root
AmbientCapabilities=CAP_NET_BIND_SERVICE
CapabilityBoundingSet=CAP_NET_BIND_SERVICE

# Recursos
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
EOF
    
    # Recargar systemd
    systemctl daemon-reload
    
    print_success "Servicio systemd creado: $SERVICE_FILE"
}

# Iniciar servicio
start_service() {
    print_info "Iniciando servicio ${SERVICE_NAME}..."
    
    systemctl enable "${SERVICE_NAME}"
    systemctl start "${SERVICE_NAME}"
    systemctl restart "${SERVICE_NAME}"
    
    # Esperar un momento y verificar
    sleep 2
    
    if systemctl is-active --quiet "${SERVICE_NAME}"; then
        print_success "Servicio iniciado correctamente"
        print_info "Estado: $(systemctl is-active ${SERVICE_NAME})"
        
        # Esperar un poco más para que se cree el usuario admin
        sleep 2
        
        # Verificar si se creó el usuario admin
        print_info "Verificando creación de usuario admin..."
        if journalctl -u "${SERVICE_NAME}" -n 20 --no-pager | grep -q "Usuario admin creado exitosamente"; then
            print_success "Usuario admin creado correctamente"
        elif journalctl -u "${SERVICE_NAME}" -n 20 --no-pager | grep -q "Error creando usuario admin"; then
            print_warning "Hubo un error al crear el usuario admin"
            print_info "Revisa los logs: sudo journalctl -u ${SERVICE_NAME} -n 50"
        else
            print_info "Revisa los logs para ver el estado del usuario admin:"
            print_info "  sudo journalctl -u ${SERVICE_NAME} -n 50 | grep -i admin"
        fi
    else
        print_warning "El servicio no se inició correctamente"
        print_info "Revisa los logs con: journalctl -u ${SERVICE_NAME} -f"
    fi
}

# URL sin :puerto si es HTTPS/443 o HTTP/80.
public_url() {
    local scheme="$1" host="$2" port="$3"
    case "${scheme}:${port}" in
        https:443) printf 'https://%s' "$host" ;;
        http:80)   printf 'http://%s' "$host" ;;
        *)         printf '%s://%s:%s' "$scheme" "$host" "$port" ;;
    esac
}

# Habilita e inicia AP WiFi + DHCP + portal cautivo (install y --update, también por SSH).
enable_and_start_hostberry_wifi_ap() {
    if ! command -v systemctl &>/dev/null; then
        return 0
    fi
    print_info "Habilitando en el arranque: ap0, hostapd (SSID hostberry), dnsmasq y portal cautivo…"
    systemctl daemon-reload 2>/dev/null || true
    rfkill unblock wifi 2>/dev/null || true
    systemctl unmask hostapd.service 2>/dev/null || true
    systemctl unmask dnsmasq.service 2>/dev/null || true
    systemctl enable create-ap0.service 2>/dev/null || true
    systemctl enable hostapd.service 2>/dev/null || true
    systemctl enable dnsmasq.service 2>/dev/null || true
    systemctl enable hostberry-captive-portal.service 2>/dev/null || true

    if [ "$(is_default_route_over_wifi)" = "1" ]; then
        print_warning "Ruta por defecto por WiFi: se inicia el AP igualmente (SSH por esa WiFi puede cortarse); preferible cable o consola."
    fi

    print_info "Iniciando ap0, hostapd, dnsmasq y reglas iptables del portal cautivo…"
    systemctl start create-ap0.service 2>/dev/null || true
    sleep 2
    systemctl start hostapd.service 2>/dev/null || true
    sleep 1
    systemctl start dnsmasq.service 2>/dev/null || true
    sleep 1
    systemctl start hostberry-captive-portal.service 2>/dev/null || true
    if systemctl is-active --quiet hostapd.service 2>/dev/null; then
        print_success "Punto de acceso WiFi (hostapd) activo"
    else
        print_warning "hostapd no está activo; tras reinicio comprueba: journalctl -u hostapd -u create-ap0 -b"
    fi
}

# Siempre tras install/--update: daemon-reload. Si no hay reinicio del sistema, reiniciar AP/DHCP/portal en caliente.
finalize_systemd_hostberry_network() {
    command -v systemctl &>/dev/null || return 0
    print_info "Aplicando systemd: daemon-reload…"
    systemctl daemon-reload 2>/dev/null || true
    if [ "${NEED_REBOOT_FOR_AP0:-0}" -eq 1 ]; then
        print_info "Habrá reinicio del sistema; se omiten reinicios en caliente de hostapd/dnsmasq."
        return 0
    fi
    print_info "Reiniciando hostapd, dnsmasq, ${SERVICE_NAME} y hostberry-captive-portal…"
    systemctl restart hostapd.service 2>/dev/null || true
    sleep 2
    systemctl restart dnsmasq.service 2>/dev/null || true
    systemctl try-restart "${SERVICE_NAME}.service" 2>/dev/null || true
    sleep 1
    systemctl restart hostberry-captive-portal.service 2>/dev/null || true
    print_success "Servicios HostBerry recargados"
}

# Mostrar información final
show_final_info() {
    echo ""
    case "$MODE" in
        update)    print_success "Actualización completa" ;;
        uninstall) print_success "Desinstalación completa" ;;
        *)         print_success "Instalación completa" ;;
    esac

    # Para desinstalación, no hay endpoints/paths que mostrar
    if [ "$MODE" = "uninstall" ]; then
        echo ""
        return 0
    fi

    local ip port web_url scheme http_redir mdns_name
    mdns_name="hostberry.local"
    ip="$(hostname -I 2>/dev/null | awk '{print $1}')"
    port="$(awk '/^[[:space:]]*port:/{gsub(/"/,"",$2); print $2; exit}' "$CONFIG_FILE" 2>/dev/null)"
    port="${port:-8000}"
    http_redir="$(awk '/^[[:space:]]*http_redirect_port:/{gsub(/"/,"",$2); print $2; exit}' "$CONFIG_FILE" 2>/dev/null)"
    scheme="http"
    if grep -qE '^[[:space:]]*tls_cert_file:' "$CONFIG_FILE" 2>/dev/null && grep -qE '^[[:space:]]*tls_key_file:' "$CONFIG_FILE" 2>/dev/null; then
        scheme="https"
    fi

    if [ -n "$ip" ] && [ "$ip" != "127.0.0.1" ]; then
        web_url="$(public_url "$scheme" "$ip" "$port")"
    else
        web_url="$(public_url "$scheme" "localhost" "$port")"
    fi

    print_info "Web:    ${web_url}"
    print_info "        $(public_url "$scheme" "$mdns_name" "$port")  (mDNS: hostberry.local)"
    print_info "WiFi AP: red abierta «hostberry» (sin contraseña). Tras asociarte: http://192.168.4.1:${port} (gateway del AP en ap0)."
    print_info "        Si no ves el SSID: reinicia y revisa journalctl -u hostapd -u create-ap0 -b (hace falta interfaz WiFi y paquetes iw, hostapd, dnsmasq)."
    if [ "$scheme" = "https" ] && [ -n "$http_redir" ] && [ "$http_redir" != "0" ] && [ "$http_redir" != "$port" ]; then
        print_info "HTTP:   $(public_url "http" "${ip:-localhost}" "$http_redir") (redirige a HTTPS)"
        print_info "        $(public_url "http" "$mdns_name" "$http_redir") (redirige a HTTPS)"
    fi
    print_info "Config: ${CONFIG_FILE}"
    print_info "Logs:   journalctl -u ${SERVICE_NAME} -f"

    # Contraseña admin: en install viene en memoria; en update u otras ejecuciones, leer INSTALL_CREDENTIALS.txt
    local saved_admin_password=""
    if [ -n "$GENERATED_ADMIN_PASSWORD" ]; then
        saved_admin_password="$GENERATED_ADMIN_PASSWORD"
    elif [ -r "${INSTALL_DIR}/INSTALL_CREDENTIALS.txt" ]; then
        saved_admin_password="$(grep -m1 '^Contraseña inicial admin: ' "${INSTALL_DIR}/INSTALL_CREDENTIALS.txt" 2>/dev/null | sed 's/^Contraseña inicial admin: //')"
    fi

    if [ -n "$saved_admin_password" ]; then
        print_warning "Login inicial: admin / ${saved_admin_password} (se ha guardado también en ${INSTALL_DIR}/INSTALL_CREDENTIALS.txt)"
        if [ "$(is_default_route_over_wifi)" = "1" ] && [ "${NEED_REBOOT_FOR_AP0:-0}" -eq 1 ]; then
            print_warning "El sistema reiniciará en segundos; si usas SSH por WiFi, vuelve a conectar cuando arranque la Pi."
        elif [ "$(is_default_route_over_wifi)" = "1" ] && [ "${NEED_REBOOT_FOR_AP0:-0}" -eq 0 ]; then
            print_warning "Reinicio automático omitido (p. ej. HOSTBERRY_SKIP_REBOOT=1). Reinicia manualmente cuando puedas."
        fi
    else
        print_warning "Login:  admin / admin (cámbiala)"
    fi
    echo ""
}

# Limpiar directorio temporal al finalizar
cleanup_temp() {
    if [ -d "$TEMP_CLONE_DIR" ] && [ "$TEMP_CLONE_DIR" != "$SCRIPT_DIR" ]; then
        print_info "Limpiando directorio temporal..."
        rm -rf "$TEMP_CLONE_DIR"
    fi
}

# Desinstalación completa: servicio, archivos, usuario y logs
do_uninstall() {
    print_info "Iniciando desinstalación de HostBerry..."

    # 1. Detener y deshabilitar el servicio
    if systemctl is-active --quiet "${SERVICE_NAME}" 2>/dev/null; then
        print_info "Deteniendo servicio ${SERVICE_NAME}..."
        systemctl stop "${SERVICE_NAME}" 2>/dev/null || true
        sleep 2
    fi
    if systemctl is-enabled --quiet "${SERVICE_NAME}" 2>/dev/null; then
        print_info "Deshabilitando servicio ${SERVICE_NAME}..."
        systemctl disable "${SERVICE_NAME}" 2>/dev/null || true
    fi

    # 2. Eliminar archivo de unidad systemd
    if [ -f "$SERVICE_FILE" ]; then
        print_info "Eliminando $SERVICE_FILE..."
        rm -f "$SERVICE_FILE"
        print_success "Archivo de servicio eliminado"
        systemctl daemon-reload 2>/dev/null || true
    fi

    # 3. Eliminar directorio de instalación
    if [ -d "$INSTALL_DIR" ]; then
        print_info "Eliminando $INSTALL_DIR..."
        rm -rf "$INSTALL_DIR"
        print_success "Directorio de instalación eliminado"
    fi

    # 4. Eliminar directorio de logs
    if [ -d "$LOG_DIR" ]; then
        print_info "Eliminando $LOG_DIR..."
        rm -rf "$LOG_DIR"
        print_success "Logs eliminados"
    fi

    # 5. Eliminar usuario y grupo del sistema (opcional, solo si existen)
    if id "$USER_NAME" &>/dev/null; then
        print_info "Eliminando usuario $USER_NAME..."
        userdel -r "$USER_NAME" 2>/dev/null || userdel "$USER_NAME" 2>/dev/null || true
        print_success "Usuario eliminado"
    fi
    if getent group "$GROUP_NAME" &>/dev/null; then
        print_info "Eliminando grupo $GROUP_NAME..."
        groupdel "$GROUP_NAME" 2>/dev/null || true
        print_success "Grupo eliminado"
    fi

    print_success "Desinstalación completada"
}

# Función principal
main() {
    local mode_label
    if [ "$HB_INSTALL_LANG" = "en" ]; then
        mode_label="INSTALLATION"
        [ "$MODE" = "update" ] && mode_label="UPDATE"
        [ "$MODE" = "uninstall" ] && mode_label="UNINSTALL"
    else
        mode_label="INSTALACIÓN"
        [ "$MODE" = "update" ] && mode_label="ACTUALIZACIÓN"
        [ "$MODE" = "uninstall" ] && mode_label="DESINSTALACIÓN"
    fi

    if [ "$MODE" = "install" ] || [ "$MODE" = "update" ]; then
        NEED_REBOOT_FOR_AP0=1
        if [ "${HOSTBERRY_SKIP_REBOOT:-0}" = "1" ]; then
            NEED_REBOOT_FOR_AP0=0
            print_warning "HOSTBERRY_SKIP_REBOOT=1: no se reiniciará el sistema al final."
        elif [ "$(is_default_route_over_wifi)" = "1" ]; then
            print_warning "Ruta por defecto por WiFi: al final se reiniciará el equipo (la sesión SSH por WiFi puede cortarse hasta que vuelva a arrancar)."
        fi
    fi

    print_banner "$mode_label"
    
    check_root
    
    # Desinstalación completa
    if [ "$MODE" = "uninstall" ]; then
        do_uninstall
        show_final_info
        return 0
    fi

    fix_hostname
    detect_os

    install_git
    download_project
    clean_previous_installation
    install_dependencies
    install_golang
    create_user
    install_files
    migrate_hostberry_tls_standard_ports
    setup_mkcert_tls
    configure_avahi_mdns
    build_project
    create_database
    configure_permissions
    create_hostapd_default_config
    configure_firewall
    create_systemd_service
    install_blocky
    install_librespeed_cli
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

# Ejecutar función principal
main
