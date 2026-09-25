#!/usr/bin/env bash
set -euo pipefail
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUT="${1:-/tmp/inv019-stand.tar.gz}"
tar -C "${ROOT_DIR}" -czf "${OUT}" prototype run-bench.sh export-stand.sh verify-fingerprint.sh benchmark-fingerprint.txt
printf '%s\n' "${OUT}"
