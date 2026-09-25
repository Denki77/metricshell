#!/usr/bin/env bash
set -euo pipefail
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
hash_file(){ if command -v sha256sum >/dev/null 2>&1;then sha256sum "$1"|awk '{print $1}';else shasum -a 256 "$1"|awk '{print $1}';fi; }
hash_stdin(){ if command -v sha256sum >/dev/null 2>&1;then sha256sum|awk '{print $1}';else shasum -a 256|awk '{print $1}';fi; }
actual="$({ find "${ROOT_DIR}/prototype" -type f|LC_ALL=C sort;printf '%s\n' "${ROOT_DIR}/run-bench.sh" "${ROOT_DIR}/export-stand.sh" "${ROOT_DIR}/verify-fingerprint.sh"; }|while IFS= read -r p;do printf '%s  %s\n' "$(hash_file "$p")" "${p#${ROOT_DIR}/}";done|hash_stdin)"
expected="$(tr -d '[:space:]' <"${ROOT_DIR}/benchmark-fingerprint.txt")"
printf 'expected\t%s\nactual\t%s\n' "$expected" "$actual"
[ "$expected" = "$actual" ]
