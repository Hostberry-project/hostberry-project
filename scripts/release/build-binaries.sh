#!/usr/bin/env bash
# Compila binarios estáticos (pure Go, sin CGO) para varias arquitecturas Linux
# en dist/, listos para publicarse en GitHub Releases con upload-release.sh.
#
# Arquitecturas (nombre de asset -> objetivo Go):
#   amd64    -> GOARCH=amd64              PC/servidor x86 64 bits
#   arm64    -> GOARCH=arm64              Raspberry Pi 3/4/5 y Zero 2 (OS 64 bits)
#   armv7    -> GOARCH=arm GOARM=7        Raspberry Pi 2/3/4 (OS 32 bits)
#   armv6    -> GOARCH=arm GOARM=6        Raspberry Pi 1 / Pi Zero (32 bits)
#   386      -> GOARCH=386                PC x86 32 bits
#   riscv64  -> GOARCH=riscv64            placas RISC-V 64 bits
#
# Uso:
#   ./scripts/release/build-binaries.sh            # todas las arquitecturas
#   HOSTBERRY_RELEASE_ARCHES="amd64 arm64" ./scripts/release/build-binaries.sh  # subconjunto
set -euo pipefail

SELF_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SELF_DIR/../.." && pwd)"
OUT="$REPO_ROOT/dist"
cd "$REPO_ROOT"

export PATH="$PATH:/usr/local/go/bin"
command -v go >/dev/null 2>&1 || { echo "ERROR: go no está en el PATH"; exit 1; }

mkdir -p "$OUT"

# Pure Go (modernc/glebarez SQLite): CGO_ENABLED=0 permite cross-compilar sin toolchain C.
export CGO_ENABLED=0 GO111MODULE=on GOWORK=off GOTOOLCHAIN=local

# arch_asset  GOARCH  GOARM(opcional)
TARGETS=(
  "amd64    amd64   "
  "arm64    arm64   "
  "armv7    arm     7"
  "armv6    arm     6"
  "386      386     "
  "riscv64  riscv64 "
)

# Permite limitar el set con HOSTBERRY_RELEASE_ARCHES="amd64 arm64 ..."
SELECT="${HOSTBERRY_RELEASE_ARCHES:-}"

build() {
  local arch="$1" goarch="$2" goarm="${3:-}"
  local asset="hostberry-linux-$arch"
  echo ">> Compilando $asset (GOARCH=$goarch${goarm:+ GOARM=$goarm})..."
  # Limitar a 1 proceso de compilación para evitar sobrecarga de recursos
  GOOS=linux GOARCH="$goarch" GOARM="$goarm" go build -p 1 -trimpath -ldflags="-s -w" -o "$OUT/$asset" .
  chmod +x "$OUT/$asset"
  echo "   OK: $OUT/$asset"
}

for t in "${TARGETS[@]}"; do
  read -r arch goarch goarm <<<"$t"
  if [ -n "$SELECT" ] && [[ " $SELECT " != *" $arch "* ]]; then
    echo ">> Omitiendo $arch (no está en HOSTBERRY_RELEASE_ARCHES)"
    continue
  fi
  build "$arch" "$goarch" "$goarm"
done

echo ">> Binarios listos en $OUT:"
ls -la "$OUT"/hostberry-linux-*
echo ">> Publica con: GITHUB_TOKEN=... $SELF_DIR/upload-release.sh"
