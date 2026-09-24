#!/usr/bin/env bash
set -euo pipefail
export LC_ALL=C

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(git -C "${ROOT_DIR}" rev-parse --show-toplevel)"
RESULTS_DIR="${ROOT_DIR}/results/$(date -u +%Y%m%dT%H%M%SZ)"
IMAGE="metricshell-inv016:prototype"
CORE_IMAGE="metricshell-inv016:corecheck"
REPETITIONS="${INV016_REPETITIONS:-30}"
RUN_ID="inv016-$(date -u +%Y%m%d%H%M%S)-$$"
PROTOTYPE_CONTAINER="${RUN_ID}-prototype"
CORE_CONTAINER="${RUN_ID}-corecheck"
mkdir -p "${RESULTS_DIR}"

hash_file() { if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | awk '{print $1}'; else shasum -a 256 "$1" | awk '{print $1}'; fi; }
hash_stdin() { if command -v sha256sum >/dev/null 2>&1; then sha256sum | awk '{print $1}'; else shasum -a 256 | awk '{print $1}'; fi; }
fingerprint() {
  { find "${ROOT_DIR}/prototype" "${ROOT_DIR}/corecheck" -type f | LC_ALL=C sort; printf '%s\n' "${ROOT_DIR}/run-bench.sh"; } |
    while IFS= read -r path; do printf '%s  %s\n' "$(hash_file "${path}")" "${path#${ROOT_DIR}/}"; done | hash_stdin
}

cleanup() {
  docker rm -f "${PROTOTYPE_CONTAINER}" "${CORE_CONTAINER}" >/dev/null 2>&1 || true
}
trap cleanup EXIT

printf 'INV-016 results: %s\n' "${RESULTS_DIR}"
printf 'Building semantic prototype...\n'
docker build -t "${IMAGE}" "${ROOT_DIR}/prototype" >"${RESULTS_DIR}/docker-build.log" 2>&1
printf 'Building Core compatibility checker...\n'
docker build -f "${ROOT_DIR}/corecheck/Dockerfile" -t "${CORE_IMAGE}" "${REPO_DIR}" >"${RESULTS_DIR}/corecheck-build.log" 2>&1

printf 'Running semantic candidates...\n'
docker create --name "${PROTOTYPE_CONTAINER}" "${IMAGE}" --out=/results --repetitions="${REPETITIONS}" >/dev/null
prototype_status=0
docker start -a "${PROTOTYPE_CONTAINER}" >"${RESULTS_DIR}/prototype.log" 2>&1 || prototype_status=$?
docker cp "${PROTOTYPE_CONTAINER}:/results/." "${RESULTS_DIR}" >/dev/null
docker rm "${PROTOTYPE_CONTAINER}" >/dev/null
if [ "${prototype_status}" -ne 0 ]; then
  printf 'Semantic prototype failed; evidence retained in %s\n' "${RESULTS_DIR}" >&2
  exit "${prototype_status}"
fi

printf 'Checking snapshots with Core validator...\n'
docker create --name "${CORE_CONTAINER}" "${CORE_IMAGE}" \
  /results/snapshot.json=pass \
  /results/empty-snapshot.json=pass \
  /results/histogram-negative-sum.json=reject:histogram_invalid \
  /results/histogram-cross-zero.json=pass \
  /results/histogram-negative-buckets.json=reject:histogram_invalid \
  /results/histogram-balanced-signed.json=reject:histogram_invalid \
  /results/histogram-negative-infinity.json=reject:histogram_invalid >/dev/null
docker cp "${RESULTS_DIR}/." "${CORE_CONTAINER}:/results" >/dev/null
core_status=0
docker start -a "${CORE_CONTAINER}" >"${RESULTS_DIR}/core-validation.tsv" 2>"${RESULTS_DIR}/corecheck.log" || core_status=$?
docker rm "${CORE_CONTAINER}" >/dev/null
if [ "${core_status}" -ne 0 ]; then
  printf 'Core compatibility check failed; evidence retained in %s\n' "${RESULTS_DIR}" >&2
  exit "${core_status}"
fi

scope_diff_clean=true
git -C "${REPO_DIR}" diff --quiet -- research/INV-016 || scope_diff_clean=false
git -C "${REPO_DIR}" diff --cached --quiet -- research/INV-016 || scope_diff_clean=false
scope_untracked="$(git -C "${REPO_DIR}" ls-files --others --exclude-standard -- research/INV-016 | wc -l | tr -d ' ')"
{
  printf 'key\tvalue\n'
  printf 'run_date_utc\t%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  printf 'repository_head_sha\t%s\n' "$(git -C "${REPO_DIR}" rev-parse HEAD)"
  printf 'benchmark_scope_diff_clean\t%s\n' "${scope_diff_clean}"
  printf 'benchmark_scope_untracked_count\t%s\n' "${scope_untracked}"
  printf 'benchmark_code_fingerprint_sha256\t%s\n' "$(fingerprint)"
  printf 'core_snapshot_source_sha256\t%s\n' "$(find "${REPO_DIR}/implementation/internal/snapshot" -type f | LC_ALL=C sort | while IFS= read -r path; do printf '%s  %s\n' "$(hash_file "${path}")" "${path#${REPO_DIR}/}"; done | hash_stdin)"
  printf 'docker_server_version\t%s\n' "$(docker version --format '{{.Server.Version}}')"
  printf 'docker_info\t%s\n' "$(docker info --format '{{.OSType}}/{{.Architecture}} ncpu={{.NCPU}} memory={{.MemTotal}}')"
  printf 'container_kernel\t%s\n' "$(docker run --rm --entrypoint uname "${IMAGE}" -a)"
  printf 'prototype_image_id\t%s\n' "$(docker image inspect -f '{{.Id}}' "${IMAGE}")"
  printf 'corecheck_image_id\t%s\n' "$(docker image inspect -f '{{.Id}}' "${CORE_IMAGE}")"
  printf 'repetitions\t%s\n' "${REPETITIONS}"
} >"${RESULTS_DIR}/environment.tsv"
printf '%s\n' "${RESULTS_DIR}" >"${ROOT_DIR}/latest-results.txt"

awk -F '\t' 'NR>1 && $5!="pass" {bad=1} END {exit bad}' "${RESULTS_DIR}/assertions.tsv"
awk -F '\t' 'NR>1 && $6!="pass" {bad=1} END {exit bad}' "${RESULTS_DIR}/core-validation.tsv"
printf 'INV-016 completed: %s\n' "${RESULTS_DIR}"
