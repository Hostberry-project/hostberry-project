#!/bin/bash
# HostBerry - Instalador modular para Linux (Debian, Ubuntu, Raspberry Pi OS)
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/install/lib/loader.sh
source "${SCRIPT_DIR}/scripts/install/lib/loader.sh"

main
