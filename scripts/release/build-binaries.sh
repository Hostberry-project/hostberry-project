#!/usr/bin/env bash
# Compila binarios estáticos (pure Go, sin CGO) para linux/amd64 y linux/arm64
# en dist/, listos para publicarse en GitHub Releases con upload-release.sh.
#
# Uso:
#   ./scripts/release/build-binaries.sh
#
# Salida: dist/hostberry-linux-amd64 y dist/hostberry-linux-arm64
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

build() {
  local goarch="$1" asset="$2"
  echo ">> Compilando $asset (GOARCH=$goarch)..."
  GOOS=linux GOARCH="$goarch" go build -trimpath -ldflags="-s -w" -o "$OUT/$asset" .
  chmod +x "$OUT/$asset"
  echo "   OK: $OUT/$asset"
}

build amd64 hostberry-linux-amd64
build arm64 hostberry-linux-arm64

echo ">> Binarios listos en $OUT:"
ls -la "$OUT"/hostberry-linux-*
echo ">> Publica con: GITHUB_TOKEN=... $SELF_DIR/upload-release.sh"
