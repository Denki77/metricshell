#!/usr/bin/env bash
set -Eeuo pipefail
export LC_ALL=C
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(git -C "${ROOT_DIR}" rev-parse --show-toplevel)"
RESULTS_DIR="${INV020_RESULTS_DIR:-${ROOT_DIR}/results/$(date -u +%Y%m%dT%H%M%SZ)}"
IMAGE=metricshell-inv020:prototype
RUN_ID="inv020-$(date -u +%Y%m%d%H%M%S)-$$"
BENCH="${RUN_ID}-bench"
mkdir -p "${RESULTS_DIR}"
cleanup(){ docker rm -f "${BENCH}" "${RUN_ID}-natural" "${RUN_ID}-signal" "${RUN_ID}-job" >/dev/null 2>&1 || true; }
trap cleanup EXIT
hash_file(){ if command -v sha256sum >/dev/null 2>&1;then sha256sum "$1"|awk '{print $1}';else shasum -a 256 "$1"|awk '{print $1}';fi; }
hash_stdin(){ if command -v sha256sum >/dev/null 2>&1;then sha256sum|awk '{print $1}';else shasum -a 256|awk '{print $1}';fi; }
fingerprint(){ { find "${ROOT_DIR}/prototype" -type f|LC_ALL=C sort;printf '%s\n' "${ROOT_DIR}/run-bench.sh" "${ROOT_DIR}/export-stand.sh" "${ROOT_DIR}/verify-fingerprint.sh"; }|while IFS= read -r p;do printf '%s  %s\n' "$(hash_file "$p")" "${p#${ROOT_DIR}/}";done|hash_stdin; }

printf 'INV-020 results: %s\n' "${RESULTS_DIR}"
docker build --pull=false -t "${IMAGE}" "${ROOT_DIR}/prototype" >"${RESULTS_DIR}/docker-build.log" 2>&1
docker create --name "${BENCH}" --memory=256m --memory-swap=256m --cpus=2 "${IMAGE}" --out=/results >/dev/null
start_ns="$(perl -MTime::HiRes=time -e 'printf "%.0f",time*1000000000')"
docker start "${BENCH}" >/dev/null
printf 'timestamp_utc\tmemory_usage\tcpu_percent\tpids\n' >"${RESULTS_DIR}/container-stats.tsv"
while [ "$(docker inspect -f '{{.State.Running}}' "${BENCH}")" = true ]; do
  printf '%s\t' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" >>"${RESULTS_DIR}/container-stats.tsv"
  docker stats --no-stream --format '{{.MemUsage}}\t{{.CPUPerc}}\t{{.PIDs}}' "${BENCH}" >>"${RESULTS_DIR}/container-stats.tsv" 2>/dev/null || true
done
bench_exit="$(docker wait "${BENCH}")"
end_ns="$(perl -MTime::HiRes=time -e 'printf "%.0f",time*1000000000')"
docker logs "${BENCH}" >"${RESULTS_DIR}/prototype.log" 2>&1
[ "${bench_exit}" = 0 ]
docker cp "${BENCH}:/results/." "${RESULTS_DIR}" >/dev/null
docker inspect "${BENCH}" >"${RESULTS_DIR}/container.inspect.json"
printf 'wall_ms\t%s\n' "$(( (end_ns-start_ns)/1000000 ))" >"${RESULTS_DIR}/wall-time.tsv"

set +e
docker run --name "${RUN_ID}-natural" "${IMAGE}" --mode=lifecycle --exit-code=17 --workload-delay=1ms >"${RESULTS_DIR}/natural-exit.log" 2>&1
natural_exit=$?
docker create --name "${RUN_ID}-signal" "${IMAGE}" --mode=lifecycle --workload-delay=30s >/dev/null
docker start "${RUN_ID}-signal" >/dev/null
docker kill --signal TERM "${RUN_ID}-signal" >/dev/null
signal_exit="$(docker wait "${RUN_ID}-signal")"
docker logs "${RUN_ID}-signal" >"${RESULTS_DIR}/signal-exit.log" 2>&1
job_start="$(perl -MTime::HiRes=time -e 'printf "%.0f",time*1000000000')"
docker run --name "${RUN_ID}-job" "${IMAGE}" --mode=lifecycle --exit-code=17 --workload-delay=1ms --post-exit=250ms >"${RESULTS_DIR}/job-final-wait.log" 2>&1
job_exit=$?
job_end="$(perl -MTime::HiRes=time -e 'printf "%.0f",time*1000000000')"
set -e
printf 'case\texpected_exit\tactual_exit\tresult\nnatural\t17\t%s\t%s\nsignal\t143\t%s\t%s\nkubernetes_job_shape\t17\t%s\t%s\n' "$natural_exit" "$([ "$natural_exit" = 17 ]&&echo pass||echo fail)" "$signal_exit" "$([ "$signal_exit" = 143 ]&&echo pass||echo fail)" "$job_exit" "$([ "$job_exit" = 17 ]&&echo pass||echo fail)" >"${RESULTS_DIR}/process-lifecycle.tsv"
printf 'configured_ms\tobserved_container_ms\tresult\n250\t%s\t%s\n' "$(( (job_end-job_start)/1000000 ))" "$([ $(( (job_end-job_start)/1000000 )) -ge 250 ]&&echo pass||echo fail)" >"${RESULTS_DIR}/post-exit-duration.tsv"

scope_clean=true;git -C "${REPO_DIR}" diff --quiet -- research/INV-020||scope_clean=false;git -C "${REPO_DIR}" diff --cached --quiet -- research/INV-020||scope_clean=false
{
 printf 'key\tvalue\nrun_date_utc\t%s\nrepository_head_sha\t%s\nbenchmark_scope_diff_clean\t%s\nbenchmark_scope_untracked_count\t%s\nbenchmark_code_fingerprint_sha256\t%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$(git -C "${REPO_DIR}" rev-parse HEAD)" "$scope_clean" "$(git -C "${REPO_DIR}" ls-files --others --exclude-standard -- research/INV-020|wc -l|tr -d ' ')" "$(fingerprint)"
 printf 'docker_server_version\t%s\ndocker_info\t%s\ncontainer_kernel\t%s\nprototype_image_id\t%s\ncpu_limit\t2\nmemory_limit_bytes\t268435456\n' "$(docker version --format '{{.Server.Version}}')" "$(docker info --format '{{.OSType}}/{{.Architecture}} ncpu={{.NCPU}} memory={{.MemTotal}} kernel={{.KernelVersion}} os={{.OperatingSystem}}')" "$(docker run --rm --entrypoint uname "${IMAGE}" -a)" "$(docker image inspect -f '{{.Id}}' "${IMAGE}")"
} >"${RESULTS_DIR}/environment.tsv"
awk -F '\t' 'NR>1&&$5!="pass"{bad=1}END{exit bad}' "${RESULTS_DIR}/assertions.tsv"
awk -F '\t' 'NR>1&&$4!="pass"{bad=1}END{exit bad}' "${RESULTS_DIR}/process-lifecycle.tsv"
awk -F '\t' 'NR>1&&$3!="pass"{bad=1}END{exit bad}' "${RESULTS_DIR}/post-exit-duration.tsv"
printf '%s\n' "${RESULTS_DIR#${ROOT_DIR}/}" >"${ROOT_DIR}/latest-results.txt"
printf 'INV-020 completed: %s\n' "${RESULTS_DIR}"
