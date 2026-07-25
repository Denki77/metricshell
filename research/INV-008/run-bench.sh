#!/usr/bin/env bash
set -euo pipefail
export LC_ALL=C

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(git -C "${ROOT_DIR}" rev-parse --show-toplevel)"
IMAGE="metricshell-inv008:prototype"
STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
RESULTS_DIR="${ROOT_DIR}/results/${STAMP}"
CONTAINER="inv008-${STAMP}"
mkdir -p "${RESULTS_DIR}"

hash_file() {
  if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | awk '{print $1}'; else shasum -a 256 "$1" | awk '{print $1}'; fi
}
hash_stdin() {
  if command -v sha256sum >/dev/null 2>&1; then sha256sum | awk '{print $1}'; else shasum -a 256 | awk '{print $1}'; fi
}
fingerprint() {
  {
    find "${ROOT_DIR}/prototype" -type f | LC_ALL=C sort
    printf '%s\n' "${ROOT_DIR}/run-bench.sh"
  } | while IFS= read -r path; do printf '%s  %s\n' "$(hash_file "${path}")" "${path#${ROOT_DIR}/}"; done | hash_stdin
}
benchmark_scope_diff_clean() {
  if git -C "${REPO_DIR}" diff --quiet -- research/INV-008/prototype research/INV-008/run-bench.sh \
    && git -C "${REPO_DIR}" diff --cached --quiet -- research/INV-008/prototype research/INV-008/run-bench.sh; then
    printf 'true\n'
  else
    printf 'false\n'
  fi
}
benchmark_scope_untracked_count() {
  git -C "${REPO_DIR}" ls-files --others --exclude-standard -- \
    research/INV-008/prototype research/INV-008/run-bench.sh | wc -l | tr -d ' '
}
cleanup() { docker rm -f "${CONTAINER}" >/dev/null 2>&1 || true; }
trap cleanup EXIT
cleanup

docker build --pull=false -t "${IMAGE}" "${ROOT_DIR}/prototype" >"${RESULTS_DIR}/docker-build.log" 2>&1
docker create --name "${CONTAINER}" "${IMAGE}" /out >/dev/null
docker start -a "${CONTAINER}" >"${RESULTS_DIR}/container.log"
docker cp "${CONTAINER}:/out/." "${RESULTS_DIR}/"

{
  printf 'key\tvalue\n'
  printf 'run_date_utc\t%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  printf 'repository_head_sha\t%s\n' "$(git -C "${REPO_DIR}" rev-parse HEAD 2>/dev/null || printf unknown)"
  printf 'benchmark_scope_diff_clean\t%s\n' "$(benchmark_scope_diff_clean)"
  printf 'benchmark_scope_untracked_count\t%s\n' "$(benchmark_scope_untracked_count)"
  printf 'benchmark_code_fingerprint_sha256\t%s\n' "$(fingerprint)"
  printf 'docker_server_version\t%s\n' "$(docker version --format '{{.Server.Version}}')"
  printf 'docker_os\t%s\n' "$(docker info --format '{{.OSType}}')"
  printf 'docker_architecture\t%s\n' "$(docker info --format '{{.Architecture}}')"
  printf 'container_kernel\t%s\n' "$(docker run --rm --entrypoint uname "${IMAGE}" -a)"
  printf 'image_repo_digest\t%s\n' "$(docker image inspect "${IMAGE}" --format '{{.Id}}')"
} >"${RESULTS_DIR}/environment.tsv"

tail -n +2 "${RESULTS_DIR}/container-environment.tsv" >>"${RESULTS_DIR}/environment.tsv"
awk -F '\t' 'NR>1 && $4!="pass"{bad=1} END{exit bad}' "${RESULTS_DIR}/correctness.tsv"
awk -F '\t' 'NR>1 && $7!=0{bad=1} END{exit bad}' "${RESULTS_DIR}/performance.tsv"
printf '%s\n' "${RESULTS_DIR}" >"${ROOT_DIR}/latest-results.txt"
printf 'Results: %s\n' "${RESULTS_DIR}"
