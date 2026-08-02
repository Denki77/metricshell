#!/usr/bin/env bash
set -euo pipefail
export LC_ALL=C

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(git -C "${ROOT_DIR}" rev-parse --show-toplevel)"
IMAGE="metricshell-inv009:prototype"
STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
RESULTS_DIR="${ROOT_DIR}/results/${STAMP}"
CONTAINER="inv009-${STAMP}"
OOM_CONTAINER="inv009-oom-${STAMP}"
mkdir -p "${RESULTS_DIR}"

hash_file() { if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | awk '{print $1}'; else shasum -a 256 "$1" | awk '{print $1}'; fi; }
hash_stdin() { if command -v sha256sum >/dev/null 2>&1; then sha256sum | awk '{print $1}'; else shasum -a 256 | awk '{print $1}'; fi; }
fingerprint() {
  { find "${ROOT_DIR}/prototype" -type f | LC_ALL=C sort; printf '%s\n' "${ROOT_DIR}/run-bench.sh"; } |
    while IFS= read -r path; do printf '%s  %s\n' "$(hash_file "${path}")" "${path#${ROOT_DIR}/}"; done | hash_stdin
}
scope_diff_clean() {
  if git -C "${REPO_DIR}" diff --quiet -- research/INV-009/prototype research/INV-009/run-bench.sh \
    && git -C "${REPO_DIR}" diff --cached --quiet -- research/INV-009/prototype research/INV-009/run-bench.sh; then printf 'true\n'; else printf 'false\n'; fi
}
cleanup() { docker rm -f "${CONTAINER}" "${OOM_CONTAINER}" >/dev/null 2>&1 || true; }
trap cleanup EXIT
cleanup

docker build --pull=false -t "${IMAGE}" "${ROOT_DIR}/prototype" >"${RESULTS_DIR}/docker-build.log" 2>&1
docker create --name "${CONTAINER}" "${IMAGE}" /out >/dev/null
docker start -a "${CONTAINER}" >"${RESULTS_DIR}/container.log"
docker cp "${CONTAINER}:/out/." "${RESULTS_DIR}/"

docker create --name "${OOM_CONTAINER}" --memory=32m "${IMAGE}" allocate 134217728 >/dev/null
docker start "${OOM_CONTAINER}" >/dev/null
memory_rc="$(docker wait "${OOM_CONTAINER}")"
oom_killed="$(docker inspect "${OOM_CONTAINER}" --format '{{.State.OOMKilled}}')"
docker logs "${OOM_CONTAINER}" >"${RESULTS_DIR}/memory-limit.log" 2>&1
printf 'case\texpected\tactual\tresult\n' >"${RESULTS_DIR}/external-assertions.tsv"
if [[ "${memory_rc}" == 137 ]]; then memory_result=pass; else memory_result=fail; fi
if [[ "${oom_killed}" == true ]]; then oom_result=pass; else oom_result=fail; fi
printf 'memory_limit_exit_code\t137\t%s\t%s\n' "${memory_rc}" "${memory_result}" >>"${RESULTS_DIR}/external-assertions.tsv"
printf 'memory_limit_oom_killed\ttrue\t%s\t%s\n' "${oom_killed}" "${oom_result}" >>"${RESULTS_DIR}/external-assertions.tsv"

{
  printf 'key\tvalue\n'
  printf 'run_date_utc\t%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  printf 'repository_head_sha\t%s\n' "$(git -C "${REPO_DIR}" rev-parse HEAD 2>/dev/null || printf unknown)"
  printf 'benchmark_scope_diff_clean\t%s\n' "$(scope_diff_clean)"
  printf 'benchmark_code_fingerprint_sha256\t%s\n' "$(fingerprint)"
  printf 'docker_server_version\t%s\n' "$(docker version --format '{{.Server.Version}}')"
  printf 'docker_os\t%s\n' "$(docker info --format '{{.OSType}}')"
  printf 'docker_architecture\t%s\n' "$(docker info --format '{{.Architecture}}')"
  printf 'container_kernel\t%s\n' "$(docker run --rm --entrypoint uname "${IMAGE}" -a)"
  printf 'image_id\t%s\n' "$(docker image inspect "${IMAGE}" --format '{{.Id}}')"
} >"${RESULTS_DIR}/environment.tsv"
tail -n +2 "${RESULTS_DIR}/container-environment.tsv" >>"${RESULTS_DIR}/environment.tsv"

awk -F '\t' 'NR>1 && $4!="pass"{bad=1} END{exit bad}' "${RESULTS_DIR}/assertions.tsv"
awk -F '\t' 'NR>1 && $4!="pass"{bad=1} END{exit bad}' "${RESULTS_DIR}/external-assertions.tsv"
printf '%s\n' "${RESULTS_DIR}" >"${ROOT_DIR}/latest-results.txt"
printf 'Results: %s\n' "${RESULTS_DIR}"
