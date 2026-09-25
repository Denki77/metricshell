#!/usr/bin/env bash
set -euo pipefail
export LC_ALL=C

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(git -C "${ROOT_DIR}" rev-parse --show-toplevel)"
RESULTS_DIR="${INV017_RESULTS_DIR:-${ROOT_DIR}/results/$(date -u +%Y%m%dT%H%M%SZ)}"
IMAGE="metricshell-inv017:prototype"
OPS="${INV017_OPS_PER_PUBLISHER:-1000}"
REPETITIONS="${INV017_REPETITIONS:-3}"
RUN_ROLE="${INV017_RUN_ROLE:-standalone}"
RUN_ID="inv017-${RUN_ROLE}-$(date -u +%Y%m%d%H%M%S)-$$"
CONTAINER="${RUN_ID}-prototype"
mkdir -p "${RESULTS_DIR}"

cleanup() {
  docker rm -f "${CONTAINER}" >/dev/null 2>&1 || true
}
trap cleanup EXIT

hash_file() { if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | awk '{print $1}'; else shasum -a 256 "$1" | awk '{print $1}'; fi; }
hash_stdin() { if command -v sha256sum >/dev/null 2>&1; then sha256sum | awk '{print $1}'; else shasum -a 256 | awk '{print $1}'; fi; }
fingerprint() {
  { find "${ROOT_DIR}/prototype" -type f | LC_ALL=C sort; printf '%s\n' "${ROOT_DIR}/run-bench.sh" "${ROOT_DIR}/run-research.sh"; } |
    while IFS= read -r path; do printf '%s  %s\n' "$(hash_file "${path}")" "${path#${ROOT_DIR}/}"; done | hash_stdin
}

printf 'INV-017 results: %s\n' "${RESULTS_DIR}"
docker build --pull=false -t "${IMAGE}" "${ROOT_DIR}/prototype" >"${RESULTS_DIR}/docker-build.log" 2>&1
docker create --name "${CONTAINER}" "${IMAGE}" \
  --out=/results --ops="${OPS}" --repetitions="${REPETITIONS}" >/dev/null
prototype_status=0
docker start -a "${CONTAINER}" >"${RESULTS_DIR}/prototype.log" 2>&1 || prototype_status=$?
docker cp "${CONTAINER}:/results/." "${RESULTS_DIR}" >/dev/null
docker rm "${CONTAINER}" >/dev/null
if [ "${prototype_status}" -ne 0 ]; then
  printf 'INV-017 prototype failed with exit %s; evidence retained in %s\n' "${prototype_status}" "${RESULTS_DIR}" >&2
  exit "${prototype_status}"
fi

scope_diff_clean=true
git -C "${REPO_DIR}" diff --quiet -- research/INV-017 || scope_diff_clean=false
git -C "${REPO_DIR}" diff --cached --quiet -- research/INV-017 || scope_diff_clean=false
scope_untracked="$(git -C "${REPO_DIR}" ls-files --others --exclude-standard -- research/INV-017 | wc -l | tr -d ' ')"
{
  printf 'key\tvalue\n'
  printf 'run_date_utc\t%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  printf 'repository_head_sha\t%s\n' "$(git -C "${REPO_DIR}" rev-parse HEAD)"
  printf 'benchmark_scope_diff_clean\t%s\n' "${scope_diff_clean}"
  printf 'benchmark_scope_untracked_count\t%s\n' "${scope_untracked}"
  printf 'benchmark_code_fingerprint_sha256\t%s\n' "$(fingerprint)"
  printf 'docker_server_version\t%s\n' "$(docker version --format '{{.Server.Version}}')"
  printf 'docker_info\t%s\n' "$(docker info --format '{{.OSType}}/{{.Architecture}} ncpu={{.NCPU}} memory={{.MemTotal}}')"
  printf 'container_kernel\t%s\n' "$(docker run --rm --entrypoint uname "${IMAGE}" -a)"
  printf 'prototype_image_id\t%s\n' "$(docker image inspect -f '{{.Id}}' "${IMAGE}")"
  printf 'ops_per_publisher\t%s\n' "${OPS}"
  printf 'repetitions\t%s\n' "${REPETITIONS}"
  printf 'run_role\t%s\n' "${RUN_ROLE}"
} >"${RESULTS_DIR}/environment.tsv"

awk -F '\t' 'NR>1 && $5!="pass" {bad=1} END {exit bad}' "${RESULTS_DIR}/assertions.tsv"
printf '%s\n' "${RESULTS_DIR}" >"${ROOT_DIR}/latest-results.txt"
printf 'INV-017 completed: %s\n' "${RESULTS_DIR}"
