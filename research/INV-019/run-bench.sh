#!/usr/bin/env bash
set -Eeuo pipefail
export LC_ALL=C
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(git -C "${ROOT_DIR}" rev-parse --show-toplevel)"
RESULTS_DIR="${INV019_RESULTS_DIR:-${ROOT_DIR}/results/$(date -u +%Y%m%dT%H%M%SZ)}"
IMAGE=metricshell-inv019:prototype
RUN_ID="inv019-$(date -u +%Y%m%d%H%M%S)-$$"
CONTAINER="${RUN_ID}-bench"
mkdir -p "${RESULTS_DIR}"
cleanup(){ docker rm -f "${CONTAINER}" "${RUN_ID}-oom" "${RUN_ID}-fd" >/dev/null 2>&1 || true; }
trap cleanup EXIT
hash_file(){ if command -v sha256sum >/dev/null 2>&1;then sha256sum "$1"|awk '{print $1}';else shasum -a 256 "$1"|awk '{print $1}';fi; }
hash_stdin(){ if command -v sha256sum >/dev/null 2>&1;then sha256sum|awk '{print $1}';else shasum -a 256|awk '{print $1}';fi; }
fingerprint(){ { find "${ROOT_DIR}/prototype" -type f|LC_ALL=C sort;printf '%s\n' "${ROOT_DIR}/run-bench.sh" "${ROOT_DIR}/export-stand.sh" "${ROOT_DIR}/verify-fingerprint.sh"; }|while IFS= read -r p;do printf '%s  %s\n' "$(hash_file "$p")" "${p#${ROOT_DIR}/}";done|hash_stdin; }
printf 'INV-019 results: %s\n' "${RESULTS_DIR}"
docker build --pull=false -t "${IMAGE}" "${ROOT_DIR}/prototype" >"${RESULTS_DIR}/docker-build.log" 2>&1
docker create --name "${CONTAINER}" --memory=512m --memory-swap=512m --cpus=2 "${IMAGE}" --out=/results >/dev/null
start_ns="$(perl -MTime::HiRes=time -e 'printf "%.0f",time*1000000000')"
docker start "${CONTAINER}" >/dev/null
printf 'timestamp_utc\tmemory_usage\tcpu_percent\tpids\n' >"${RESULTS_DIR}/container-stats.tsv"
while [ "$(docker inspect -f '{{.State.Running}}' "${CONTAINER}")" = true ]; do
  printf '%s\t' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" >>"${RESULTS_DIR}/container-stats.tsv"
  docker stats --no-stream --format '{{.MemUsage}}\t{{.CPUPerc}}\t{{.PIDs}}' "${CONTAINER}" >>"${RESULTS_DIR}/container-stats.tsv" 2>/dev/null || true
done
bench_exit="$(docker wait "${CONTAINER}")"
end_ns="$(perl -MTime::HiRes=time -e 'printf "%.0f",time*1000000000')"
docker logs "${CONTAINER}" >"${RESULTS_DIR}/prototype.log" 2>&1
[ "${bench_exit}" = 0 ]
docker cp "${CONTAINER}:/results/." "${RESULTS_DIR}" >/dev/null
docker inspect "${CONTAINER}" >"${RESULTS_DIR}/container.inspect.json"
docker rm "${CONTAINER}" >/dev/null
printf 'wall_ms\t%s\n' "$(( (end_ns-start_ns)/1000000 ))" >"${RESULTS_DIR}/wall-time.tsv"

# E-019.7: distinguish a normal policy rejection from fatal cgroup OOM.
set +e
docker run --name "${RUN_ID}-oom" --memory=32m --memory-swap=32m "${IMAGE}" --mode=allocate --allocate-mb=128 >"${RESULTS_DIR}/oom.log" 2>&1
oom_exit=$?
set -e
oom_killed="$(docker inspect -f '{{.State.OOMKilled}}' "${RUN_ID}-oom")"
docker inspect "${RUN_ID}-oom" >"${RESULTS_DIR}/oom.inspect.json"
printf 'case\texpected\tactual\tresult\ncontainer_oom\texit=137,oom=true\texit=%s,oom=%s\t%s\n' "$oom_exit" "$oom_killed" "$([ "$oom_exit" = 137 ] && [ "$oom_killed" = true ] && echo pass || echo fail)" >"${RESULTS_DIR}/resource-exhaustion.tsv"

# The same image under a low FD ceiling; the benchmark needs no unbounded descriptors.
docker run --name "${RUN_ID}-fd" --ulimit nofile=64:64 --memory=512m --memory-swap=512m "${IMAGE}" --out=/results >/dev/null
fd_exit="$(docker inspect -f '{{.State.ExitCode}}' "${RUN_ID}-fd")"
printf 'fd_limit\texit_code\tresult\n64\t%s\t%s\n' "$fd_exit" "$([ "$fd_exit" = 0 ] && echo pass || echo fail)" >"${RESULTS_DIR}/fd-limit.tsv"

scope_clean=true;git -C "${REPO_DIR}" diff --quiet -- research/INV-019||scope_clean=false;git -C "${REPO_DIR}" diff --cached --quiet -- research/INV-019||scope_clean=false
{
 printf 'key\tvalue\nrun_date_utc\t%s\nrepository_head_sha\t%s\nbenchmark_scope_diff_clean\t%s\nbenchmark_scope_untracked_count\t%s\nbenchmark_code_fingerprint_sha256\t%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$(git -C "${REPO_DIR}" rev-parse HEAD)" "$scope_clean" "$(git -C "${REPO_DIR}" ls-files --others --exclude-standard -- research/INV-019|wc -l|tr -d ' ')" "$(fingerprint)"
 printf 'docker_server_version\t%s\ndocker_info\t%s\ncontainer_kernel\t%s\nprototype_image_id\t%s\ncpu_limit\t2\nmemory_limit_bytes\t536870912\noom_memory_limit_bytes\t33554432\nfd_limit\t64\n' "$(docker version --format '{{.Server.Version}}')" "$(docker info --format '{{.OSType}}/{{.Architecture}} ncpu={{.NCPU}} memory={{.MemTotal}} kernel={{.KernelVersion}} os={{.OperatingSystem}}')" "$(docker run --rm --entrypoint uname "${IMAGE}" -a)" "$(docker image inspect -f '{{.Id}}' "${IMAGE}")"
} >"${RESULTS_DIR}/environment.tsv"
awk -F '\t' 'NR>1&&$5!="pass"{bad=1}END{exit bad}' "${RESULTS_DIR}/assertions.tsv"
awk -F '\t' 'NR>1&&$4!="pass"{bad=1}END{exit bad}' "${RESULTS_DIR}/resource-exhaustion.tsv"
awk -F '\t' 'NR>1&&$3!="pass"{bad=1}END{exit bad}' "${RESULTS_DIR}/fd-limit.tsv"
printf '%s\n' "${RESULTS_DIR#${ROOT_DIR}/}" >"${ROOT_DIR}/latest-results.txt"
printf 'INV-019 completed: %s\n' "${RESULTS_DIR}"
