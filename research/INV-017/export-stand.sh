#!/usr/bin/env bash
set -euo pipefail
export LC_ALL=C

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUT="${1:-${ROOT_DIR}/inv017-stand.tar.gz}"
TEMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TEMP_DIR}"' EXIT
mkdir -p "${TEMP_DIR}/research/INV-017"
cp -R "${ROOT_DIR}/prototype" "${TEMP_DIR}/research/INV-017/"
cp "${ROOT_DIR}/run-bench.sh" "${ROOT_DIR}/run-research.sh" "${ROOT_DIR}/verify-fingerprint.sh" "${ROOT_DIR}/benchmark-fingerprint.txt" "${TEMP_DIR}/research/INV-017/"
tar -C "${TEMP_DIR}" -czf "${OUT}" research/INV-017
printf '%s  %s\n' "$(if command -v sha256sum >/dev/null 2>&1; then sha256sum "${OUT}" | awk '{print $1}'; else shasum -a 256 "${OUT}" | awk '{print $1}'; fi)" "${OUT}"
