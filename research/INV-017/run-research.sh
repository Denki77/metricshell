#!/usr/bin/env bash
set -Eeuo pipefail
export LC_ALL=C

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RUN_NAME="$(date -u +%Y%m%dT%H%M%SZ)"
RUN_DIR="${ROOT_DIR}/results/${RUN_NAME}"
REFERENCE_DIR="${RUN_DIR}/reference"
EXTENDED_DIR="${RUN_DIR}/extended"
RACE_DIR="${RUN_DIR}/race"
PHASE="initialization"
mkdir -p "${REFERENCE_DIR}" "${EXTENDED_DIR}" "${RACE_DIR}"

on_error() {
  status=$?
  trap - ERR
  {
    printf 'key\tvalue\n'
    printf 'status\tfailed\n'
    printf 'phase\t%s\n' "${PHASE}"
    printf 'exit_code\t%s\n' "${status}"
    printf 'line\t%s\n' "${BASH_LINENO[0]:-unknown}"
  } >"${RUN_DIR}/failure.tsv"
  printf 'INV-017 failed: phase=%s exit=%s\nEvidence retained in: %s\n' "${PHASE}" "${status}" "${RUN_DIR}" >&2
  while IFS= read -r log; do
    printf '\n--- %s (last 80 lines) ---\n' "${log#${RUN_DIR}/}" >&2
    tail -n 80 "${log}" >&2 || true
  done < <(find "${RUN_DIR}" -type f -name '*.log' | LC_ALL=C sort)
  exit "${status}"
}
trap on_error ERR

PHASE="fingerprint"
"${ROOT_DIR}/verify-fingerprint.sh" | tee "${RUN_DIR}/fingerprint.tsv"

PHASE="environment"
HOST_OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
DOCKER_PLATFORM="$(docker info --format '{{.OSType}}-{{.Architecture}}' | tr '/:' '--')"
ENVIRONMENT_ID="${HOST_OS}-${DOCKER_PLATFORM}"
{
  printf 'key\tvalue\n'
  printf 'environment_id\t%s\n' "${ENVIRONMENT_ID}"
  printf 'host_os\t%s\n' "${HOST_OS}"
  printf 'docker_platform\t%s\n' "${DOCKER_PLATFORM}"
} >"${RUN_DIR}/run-environment.tsv"

PHASE="reference"
printf 'Running INV-017 reference matrix...\n'
INV017_RESULTS_DIR="${REFERENCE_DIR}" \
INV017_RUN_ROLE="reference" \
INV017_OPS_PER_PUBLISHER="${INV017_REFERENCE_OPS:-1000}" \
INV017_REPETITIONS="${INV017_REFERENCE_REPETITIONS:-3}" \
  "${ROOT_DIR}/run-bench.sh"

PHASE="extended"
printf 'Running INV-017 extended matrix...\n'
INV017_RESULTS_DIR="${EXTENDED_DIR}" \
INV017_RUN_ROLE="extended" \
INV017_OPS_PER_PUBLISHER="${INV017_EXTENDED_OPS:-10000}" \
INV017_REPETITIONS="${INV017_EXTENDED_REPETITIONS:-10}" \
  "${ROOT_DIR}/run-bench.sh"

PHASE="race-detector"
printf 'Running INV-017 Go race detector...\n'
docker build --pull=false -f "${ROOT_DIR}/prototype/Dockerfile.race" \
  -t metricshell-inv017:race "${ROOT_DIR}/prototype" \
  >"${RACE_DIR}/race-build.log" 2>&1
docker run --rm metricshell-inv017:race \
  --out=/tmp/results --ops=100 --repetitions=1 \
  >"${RACE_DIR}/race-detector.log" 2>&1
{
  printf 'command\tresult\treported_races\n'
  printf 'go run -race ./cmd/inv017 --out=/tmp/results --ops=100 --repetitions=1\tpass\t0\n'
} >"${RACE_DIR}/race-detector.tsv"

PHASE="finalization"
{
  printf 'role\tresult_set\n'
  printf 'reference\tresults/%s/reference\n' "${RUN_NAME}"
  printf 'extended\tresults/%s/extended\n' "${RUN_NAME}"
  printf 'race\tresults/%s/race\n' "${RUN_NAME}"
} >"${RUN_DIR}/run-set.tsv"
printf 'results/%s\n' "${RUN_NAME}" >"${ROOT_DIR}/latest-results.txt"
printf 'results/%s/reference\n' "${RUN_NAME}" >"${ROOT_DIR}/latest-reference-results.txt"
printf 'results/%s/extended\n' "${RUN_NAME}" >"${ROOT_DIR}/latest-extended-results.txt"
printf 'status\tpassed\n' >"${RUN_DIR}/summary.tsv"

trap - ERR
printf 'INV-017 research completed: %s\n' "${RUN_DIR}"
