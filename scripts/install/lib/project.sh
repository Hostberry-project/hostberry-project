#!/bin/bash
# Módulo: project.sh
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

