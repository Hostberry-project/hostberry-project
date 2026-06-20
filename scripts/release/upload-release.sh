#!/usr/bin/env bash
# Crea una release en GitHub y sube los binarios precompilados como assets.
#
# Uso:
#   GITHUB_TOKEN=ghp_tuToken ./scripts/release/upload-release.sh [TAG]
#
# - GITHUB_TOKEN: token con permiso de escritura (scope 'repo' clásico, o fine-grained
#   con Contents: read/write sobre este repositorio).
# - TAG (opcional): etiqueta de la release. Por defecto se deriva de
#   internal/constants/constants.go (Version) -> vX.Y.Z, para que coincida con
#   lo que busca el instalador (scripts/install/lib/release_binary.sh).
#
# Los binarios deben estar compilados en dist/ (hostberry-linux-amd64 y
# hostberry-linux-arm64). Genera y sube un .sha256 por asset (el nombre exacto
# que descarga el instalador para verificar el checksum) y un SHA256SUMS combinado.
#
# Requisitos: curl, python3 y sha256sum.
set -euo pipefail

REPO="Hostberry-project/hostberry-project"
SELF_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SELF_DIR/../.." && pwd)"
DIR="$REPO_ROOT/dist"
CONSTANTS="$REPO_ROOT/internal/constants/constants.go"

# Tag por defecto: vVERSION leído de constants.go (lo que el instalador espera).
default_tag() {
  local ver=""
  if [ -f "$CONSTANTS" ]; then
    ver="$(grep -E 'Version[[:space:]]*=' "$CONSTANTS" 2>/dev/null | head -1 | sed -E 's/.*"([^"]+)".*/\1/')"
  fi
  if [ -n "$ver" ]; then
    printf 'v%s' "$ver"
  else
    printf 'v2.1.0'
  fi
}

TAG="${1:-$(default_tag)}"
TARGET="main"   # commit/rama sobre la que se crea la etiqueta si no existe
BIN_ASSETS=("hostberry-linux-amd64" "hostberry-linux-arm64")

: "${GITHUB_TOKEN:?Define GITHUB_TOKEN con tu token de GitHub}"

API="https://api.github.com/repos/$REPO"
UPLOADS="https://uploads.github.com/repos/$REPO"
AUTH=(-H "Authorization: token $GITHUB_TOKEN" -H "Accept: application/vnd.github+json")

echo ">> Comprobando binarios en $DIR"
for a in "${BIN_ASSETS[@]}"; do
  [ -f "$DIR/$a" ] || { echo "ERROR: falta $DIR/$a (compila los binarios primero)"; exit 1; }
done

# Genera un .sha256 por binario (nombre exacto que descarga release_binary.sh)
# y un SHA256SUMS combinado para verificación manual.
echo ">> Generando checksums (.sha256 por asset + SHA256SUMS)"
( cd "$DIR"
  : > SHA256SUMS
  for a in "${BIN_ASSETS[@]}"; do
    sum="$(sha256sum "$a" | awk '{print $1}')"
    printf '%s' "$sum" > "$a.sha256"
    printf '%s  %s\n' "$sum" "$a" >> SHA256SUMS
  done
)

ASSETS=("${BIN_ASSETS[@]}")
for a in "${BIN_ASSETS[@]}"; do ASSETS+=("$a.sha256"); done
ASSETS+=("SHA256SUMS")

echo ">> Creando release $TAG en $REPO"
BODY=$(cat <<EOF
{
  "tag_name": "$TAG",
  "target_commitish": "$TARGET",
  "name": "$TAG",
  "body": "Binarios precompilados (linux estáticos, sin dependencias).\\n\\n- hostberry-linux-amd64 (+ .sha256)\\n- hostberry-linux-arm64 (+ .sha256)\\n\\nEl instalador (install.sh / --update) descarga el binario según arquitectura y verifica el .sha256. Verificación manual: sha256sum -c SHA256SUMS",
  "draft": false,
  "prerelease": false
}
EOF
)

RESP=$(curl -fsS "${AUTH[@]}" -X POST "$API/releases" -d "$BODY" 2>/dev/null || true)
REL_ID=$(printf '%s' "$RESP" | python3 -c 'import sys,json;
try:
    d=json.load(sys.stdin); print(d.get("id",""))
except Exception:
    print("")')

if [ -z "$REL_ID" ]; then
  echo ">> No se pudo crear (¿ya existe la etiqueta $TAG?). Intentando reutilizarla..."
  RESP=$(curl -fsS "${AUTH[@]}" "$API/releases/tags/$TAG")
  REL_ID=$(printf '%s' "$RESP" | python3 -c 'import sys,json;print(json.load(sys.stdin).get("id",""))')
fi
[ -n "$REL_ID" ] || { echo "ERROR: no se pudo obtener el id de la release. Respuesta:"; echo "$RESP"; exit 1; }
echo ">> Release id: $REL_ID"

for a in "${ASSETS[@]}"; do
  echo ">> Subiendo $a"
  # Borra el asset previo con el mismo nombre si existe (para re-subidas)
  EXIST=$(curl -fsS "${AUTH[@]}" "$API/releases/$REL_ID/assets" | python3 -c "import sys,json;
[print(x['id']) for x in json.load(sys.stdin) if x.get('name')=='$a']" 2>/dev/null || true)
  for id in $EXIST; do curl -fsS "${AUTH[@]}" -X DELETE "$API/releases/assets/$id" >/dev/null || true; done
  curl -fsS "${AUTH[@]}" -H "Content-Type: application/octet-stream" \
    --data-binary @"$DIR/$a" \
    "$UPLOADS/releases/$REL_ID/assets?name=$a" >/dev/null
  echo "   OK"
done

echo ">> Listo. Release $TAG con assets subidos:"
echo "   https://github.com/$REPO/releases/tag/$TAG"
