#!/usr/bin/env bash
set -euo pipefail
export LC_ALL=C

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(git -C "${ROOT_DIR}" rev-parse --show-toplevel)"
IMAGE="metricshell-inv007:prototype"
CONTAINER="inv007-benchmark"
RESULTS_DIR="${ROOT_DIR}/results/$(date -u +%Y%m%dT%H%M%SZ)"
REPETITIONS="${INV007_REPETITIONS:-3}"
MAX_PAYLOAD="${INV007_MAX_PAYLOAD:-65536}"

mkdir -p "${RESULTS_DIR}"

hash_file() {
  if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | awk '{print $1}'; else shasum -a 256 "$1" | awk '{print $1}'; fi
}
hash_stdin() {
  if command -v sha256sum >/dev/null 2>&1; then sha256sum | awk '{print $1}'; else shasum -a 256 | awk '{print $1}'; fi
}
benchmark_code_fingerprint() {
  {
    find "${ROOT_DIR}/prototype" -type f | LC_ALL=C sort
    printf '%s\n' "${ROOT_DIR}/run-bench.sh"
  } | while IFS= read -r path; do
    printf '%s  %s\n' "$(hash_file "${path}")" "${path#${ROOT_DIR}/}"
  done | hash_stdin
}
scope_diff_clean() {
  if git -C "${REPO_DIR}" diff --quiet -- research/INV-007/prototype research/INV-007/run-bench.sh \
    && git -C "${REPO_DIR}" diff --cached --quiet -- research/INV-007/prototype research/INV-007/run-bench.sh; then printf true; else printf false; fi
}
scope_untracked_count() {
  git -C "${REPO_DIR}" ls-files --others --exclude-standard -- research/INV-007/prototype research/INV-007/run-bench.sh | wc -l | tr -d ' '
}
cleanup() {
  docker rm -f "${CONTAINER}" >/dev/null 2>&1 || true
}
trap cleanup EXIT
cleanup

docker build -t "${IMAGE}" "${ROOT_DIR}/prototype" >"${RESULTS_DIR}/docker-build.log"
docker create --name "${CONTAINER}" "${IMAGE}" \
  --output-dir=/results --max-payload="${MAX_PAYLOAD}" --repetitions="${REPETITIONS}" >/dev/null
set +e
docker start -a "${CONTAINER}" >"${RESULTS_DIR}/benchmark.log" 2>&1
BENCH_EXIT=$?
set -e
docker cp "${CONTAINER}:/results/." "${RESULTS_DIR}"
for required in summary.tsv correctness.tsv performance.tsv pressure.tsv; do
  if [ ! -s "${RESULTS_DIR}/${required}" ]; then
    echo "benchmark did not produce ${required}" >&2
    BENCH_EXIT=1
  fi
done

{
  printf 'key\tvalue\n'
  printf 'run_date_utc\t%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  printf 'repository_head_sha\t%s\n' "$(git -C "${REPO_DIR}" rev-parse HEAD)"
  printf 'benchmark_scope_diff_clean\t%s\n' "$(scope_diff_clean)"
  printf 'benchmark_scope_untracked_count\t%s\n' "$(scope_untracked_count)"
  printf 'benchmark_code_fingerprint_sha256\t%s\n' "$(benchmark_code_fingerprint)"
  printf 'image_id\t%s\n' "$(docker image inspect "${IMAGE}" --format '{{.Id}}')"
  printf 'docker_server_version\t%s\n' "$(docker version --format '{{.Server.Version}}')"
  printf 'docker_os\t%s\n' "$(docker info --format '{{.OperatingSystem}}')"
  printf 'docker_architecture\t%s\n' "$(docker info --format '{{.Architecture}}')"
  printf 'container_kernel\t%s\n' "$(docker run --rm --entrypoint uname "${IMAGE}" -a)"
  printf 'container_image_architecture\t%s\n' "$(docker image inspect "${IMAGE}" --format '{{.Architecture}}')"
  printf 'repetitions\t%s\n' "${REPETITIONS}"
  printf 'max_payload_bytes\t%s\n' "${MAX_PAYLOAD}"
  printf 'benchmark_exit\t%s\n' "${BENCH_EXIT}"
} >"${RESULTS_DIR}/environment.tsv"

printf '%s\n' "${RESULTS_DIR}" >"${ROOT_DIR}/latest-results.txt"
if [ "${BENCH_EXIT}" -ne 0 ]; then
  echo "benchmark failed; inspect ${RESULTS_DIR}/benchmark.log" >&2
  exit "${BENCH_EXIT}"
fi
echo "${RESULTS_DIR}"
