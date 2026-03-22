#!/usr/bin/env python3
"""Extrae el literal entre comillas de cada print_info|success|warning|error."""
from __future__ import annotations

import re
import sys
from pathlib import Path


def extract_bash_double_quoted_after(s: str, start: int) -> tuple[str, int] | None:
    """Desde start (índice de la comilla de apertura), devuelve (contenido, índice tras cierre)."""
    if start >= len(s) or s[start] != '"':
        return None
    i = start + 1
    out: list[str] = []
    while i < len(s):
        c = s[i]
        if c == "\\":
            if i + 1 < len(s):
                out.append(s[i : i + 2])
                i += 2
            else:
                out.append(c)
                i += 1
            continue
        if c == '"':
            return "".join(out), i + 1
        if c == "$" and i + 1 < len(s) and s[i + 1] == "(":
            out.append("$(")
            i += 2
            depth = 1
            while i < len(s) and depth > 0:
                if s[i] == "$" and i + 1 < len(s) and s[i + 1] == "(":
                    out.append("$(")
                    i += 2
                    depth += 1
                    continue
                if s[i] == "(":
                    out.append("(")
                    i += 1
                    depth += 1
                    continue
                if s[i] == ")":
                    out.append(")")
                    i += 1
                    depth -= 1
                    continue
                out.append(s[i])
                i += 1
            continue
        out.append(c)
        i += 1
    return None


def main() -> None:
    path = Path(__file__).resolve().parent.parent / "install.sh"
    text = path.read_text(encoding="utf-8")
    pat = re.compile(r"print_(?:info|success|warning|error)\s+")
    seen: set[str] = set()
    ordered: list[str] = []
    for m in pat.finditer(text):
        j = m.end()
        while j < len(text) and text[j] in " \t":
            j += 1
        if j >= len(text) or text[j] != '"':
            continue
        got = extract_bash_double_quoted_after(text, j)
        if not got:
            continue
        inner, _ = got
        if not inner.strip():
            continue
        if inner in seen:
            continue
        seen.add(inner)
        ordered.append(inner)
    for line in ordered:
        print(line.replace("\n", "\\n"))
    print(f"# total: {len(ordered)}", file=sys.stderr)


if __name__ == "__main__":
    main()
