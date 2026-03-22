#!/usr/bin/env python3
"""
Genera locale/install.lang.en.tsv a partir de install.sh (mensajes en print_*).
Ejecutar desde la raíz del repo: python3 scripts/gen-install-lang-tsv.py
"""
from __future__ import annotations

import re
import subprocess
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
INSTALL_SH = ROOT / "install.sh"
OUT = ROOT / "locale" / "install.lang.en.tsv"

# Traducciones manuales cuando la heurística no basta (clave = mensaje español exacto)
MANUAL: dict[str, str] = {}


def heuristic_en(es: str) -> str:
    if es in MANUAL:
        return MANUAL[es]
    s = es
    reps = [
        ("Instalación completa", "Installation complete"),
        ("Actualización completa", "Update complete"),
        ("Desinstalación completa", "Uninstall complete"),
        ("Desinstalación completada", "Uninstall finished"),
        ("Ejecuta con sudo/root", "Run as root or sudo"),
        ("Git listo", "Git ready"),
        ("✅ ", "✅ "),
        ("ERROR CRÍTICO:", "CRITICAL ERROR:"),
        ("ERROR:", "ERROR:"),
        ("Error:", "Error:"),
        ("Advertencia:", "Warning:"),
        ("Instalando ", "Installing "),
        ("Instalación anterior eliminada", "Previous installation removed"),
        ("Configurando ", "Configuring "),
        ("Creando ", "Creating "),
        ("Creando/configurando ", "Creating/configuring "),
        ("Creando/actualizando ", "Creating/updating "),
        ("Descargando ", "Downloading "),
        ("Descarga", "Download"),
        ("Compilando ", "Building "),
        ("Compilación ", "Build "),
        ("Eliminando ", "Removing "),
        ("Deteniendo ", "Stopping "),
        ("Deshabilitando ", "Disabling "),
        ("Guardando backup", "Saving backup"),
        ("Restaurando ", "Restoring "),
        ("Moviendo ", "Moving "),
        ("Verificando ", "Checking "),
        ("Verificado:", "Verified:"),
        ("Revisa ", "Check "),
        ("Asegúrate de ", "Make sure to "),
        ("No se pudo ", "Could not "),
        ("No se encontró ", "Not found: "),
        ("No hay ", "There is no "),
        ("No existe ", "Does not exist: "),
        ("Ya ", "Already "),
        ("listo", "ready"),
        ("Listo", "Ready"),
        ("creado", "created"),
        ("Creado", "Created"),
        ("creada", "created"),
        ("configurado", "configured"),
        ("Configurado", "Configured"),
        ("eliminado", "removed"),
        ("Eliminado", "Removed"),
        ("actualizados", "updated"),
        ("preservados", "preserved"),
        ("completada", "complete"),
        ("completado", "complete"),
        ("abortando", "aborting"),
        ("Abortando", "Aborting"),
        ("Sistema:", "System:"),
        ("Usuario ", "User "),
        ("Grupo ", "Group "),
        ("Directorio ", "Directory "),
        ("Archivo ", "File "),
        ("Logs eliminados", "Logs removed"),
        ("Base de datos", "Database"),
        ("backup", "backup"),
        ("configuración", "configuration"),
        ("Desbloqueando ", "Unblocking "),
        ("Escribiendo ", "Writing "),
        ("Añadido ", "Added "),
        ("Añadiendo ", "Adding "),
        ("Ajustado ", "Adjusted "),
        ("Aplicando ", "Applying "),
        ("Habilitando ", "Enabling "),
        ("Iniciando ", "Starting "),
        ("Reiniciando ", "Restarting "),
        ("Limpiando ", "Cleaning "),
        ("Preparando ", "Preparing "),
        ("Copiando ", "Copying "),
        ("Generando ", "Generating "),
        ("Omitiendo ", "Skipping "),
        ("Proyecto ", "Project "),
        ("Modo actualización", "Update mode"),
        ("Modo ", "Mode "),
        ("Servicio ", "Service "),
        ("Servicios ", "Services "),
        ("Punto de acceso", "Wi-Fi access point"),
        ("hostapd no está activo", "hostapd is not active"),
        ("dnsmasq", "dnsmasq"),
        ("Override de", "Override for"),
        ("actualizado", "updated"),
        ("Portal cautivo", "Captive portal"),
        ("TLS listo", "TLS ready"),
        ("Login:", "Login:"),
        ("Login inicial", "Initial login"),
        ("Web:", "Web:"),
        ("Config:", "Config:"),
        ("HTTP:", "HTTP:"),
        ("Logs:", "Logs:"),
        ("Opción desconocida", "Unknown option"),
        ("Uso:", "Usage:"),
        ("Opciones:", "Options:"),
        ("Ejemplos:", "Examples:"),
        ("Instalar", "Install"),
        ("Actualizar", "Update"),
        ("Desinstalar", "Uninstall"),
        ("Mostrar esta ayuda", "Show this help"),
        ("sin opción", "no option"),
        ("preserva datos", "preserves data"),
        ("reinicio del sistema", "system reboot"),
        ("omitir reinicio", "skip reboot"),
    ]
    for a, b in reps:
        s = s.replace(a, b)
    # Frases comunes restantes
    s = (
        s.replace(" (primera instalación?)", " (first install?)")
        .replace(" (¿sin driver?)", " (no driver?)")
        .replace("…", "...")
        .replace("puede tardar", "may take")
        .replace("puede necesitar reinicio", "may need a reboot")
        .replace("Se usará ", "Will use ")
        .replace("se creará ", "will be created ")
        .replace("se omiten ", "skipping ")
        .replace("se ha guardado", "was also saved")
        .replace("cámbiala", "change it")
        .replace("Manual:", "Manual:")
        .replace("Sugerencia:", "Hint:")
        .replace("Verifica ", "Verify ")
        .replace("comprueba", "check")
        .replace("compruebe", "verify")
        .replace("redirige a HTTPS", "redirects to HTTPS")
        .replace("redirige a", "redirects to")
    )
    return s


def extract_strings(path: Path) -> list[str]:
    text = path.read_text(encoding="utf-8", errors="replace")
    pat = re.compile(
        r'print_(?:info|success|warning|error)\s+"(?:[^"\\]|\\.)*"',
        re.MULTILINE,
    )
    out: list[str] = []
    seen: set[str] = set()
    for m in pat.finditer(text):
        raw = m.group(0)
        inner = raw.split('"', 1)[1]
        if inner.endswith('"'):
            inner = inner[:-1]
        inner = inner.replace('\\"', '"').replace("\\$", "$")
        if not inner.strip() or inner.startswith("$(public_url"):
            continue
        if inner in seen:
            continue
        seen.add(inner)
        out.append(inner)
    return sorted(out)


def main() -> None:
    es_list = extract_strings(INSTALL_SH)
    OUT.parent.mkdir(parents=True, exist_ok=True)
    lines: list[str] = []
    for es in es_list:
        en = heuristic_en(es)
        es_esc = es.replace("\\", "\\\\").replace("\t", " ")
        en_esc = en.replace("\\", "\\\\").replace("\t", " ")
        lines.append(f"{es_esc}\t{en_esc}")
    OUT.write_text("\n".join(lines) + "\n", encoding="utf-8")
    print(f"Wrote {len(lines)} entries to {OUT}")


if __name__ == "__main__":
    main()
