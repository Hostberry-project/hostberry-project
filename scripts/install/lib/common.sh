#!/bin/bash
# Utilidades compartidas del instalador HostBerry (colores, i18n, mensajes).
# Requiere: SCRIPT_DIR apuntando a la raíz del proyecto.

# Colores para output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Estilos
BOLD='\033[1m'
DIM='\033[2m'

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
        remove) accent="$RED" ;;
    esac

    echo ""
    print_logo
    printf "%b\n" "${accent}${BOLD}HostBerry${NC} ${DIM}${label}${NC}"
    echo ""
}

check_root() {
    if [ "$EUID" -ne 0 ]; then
        print_error "Ejecuta con sudo/root"
        exit 1
    fi
}
