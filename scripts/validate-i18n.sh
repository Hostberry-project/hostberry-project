#!/bin/bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ES="${ROOT}/locales/es.json"
EN="${ROOT}/locales/en.json"

python3 << PY
import json, sys

def flatten(obj, prefix=""):
    keys = set()
    if isinstance(obj, dict):
        for k, v in obj.items():
            p = f"{prefix}.{k}" if prefix else k
            keys |= flatten(v, p)
    else:
        keys.add(prefix)
    return keys

with open("${ES}") as f:
    es = json.load(f)
with open("${EN}") as f:
    en = json.load(f)

es_keys = flatten(es)
en_keys = flatten(en)

missing_in_en = sorted(es_keys - en_keys)
missing_in_es = sorted(en_keys - es_keys)

ok = True
if missing_in_en:
    ok = False
    print("Claves en es.json ausentes en en.json:")
    for k in missing_in_en[:50]:
        print("  -", k)
    if len(missing_in_en) > 50:
        print(f"  ... y {len(missing_in_en)-50} más")

if missing_in_es:
    ok = False
    print("Claves en en.json ausentes en es.json:")
    for k in missing_in_es[:50]:
        print("  -", k)
    if len(missing_in_es) > 50:
        print(f"  ... y {len(missing_in_es)-50} más")

if ok:
    print(f"i18n OK: {len(es_keys)} claves coincidentes")
else:
    sys.exit(1)
PY
