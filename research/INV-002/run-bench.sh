#!/usr/bin/env bash
set -euo pipefail
export LC_ALL=C

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(git -C "${ROOT_DIR}" rev-parse --show-toplevel)"
IMAGE="metricshell-inv002:prototype"
RESULTS_DIR="${ROOT_DIR}/results/$(date -u +%Y%m%dT%H%M%SZ)"
mkdir -p "${RESULTS_DIR}"
SUMMARY="${RESULTS_DIR}/summary.tsv"
OBS="${RESULTS_DIR}/observations.tsv"
ASSERTIONS="${RESULTS_DIR}/assertions.tsv"
printf 'case\texpected_exit\tactual_exit\tattempts\tfinal_counter\tresult\n' >"${SUMMARY}"
printf 'case\tphase\tattempt\texit_code\tcounter\n' >"${OBS}"
printf 'case\tassertion\texpected\tactual\tresult\n' >"${ASSERTIONS}"

hash_file() {
  if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | awk '{print $1}'; else shasum -a 256 "$1" | awk '{print $1}'; fi
}
hash_stdin() {
  if command -v sha256sum >/dev/null 2>&1; then sha256sum | awk '{print $1}'; else shasum -a 256 | awk '{print $1}'; fi
}
benchmark_code_fingerprint() {
  {
    find "${ROOT_DIR}/prototype" -type f | LC_ALL=C sort
    printf '%s\n' "${ROOT_DIR}/run-bench.sh" "${ROOT_DIR}/run-extended-bench.sh" "${ROOT_DIR}/compose.yml"
  } | while IFS= read -r path; do printf '%s  %s\n' "$(hash_file "${path}")" "${path#${ROOT_DIR}/}"; done | hash_stdin
}
benchmark_scope_diff_clean() {
  if git -C "${REPO_DIR}" diff --quiet -- research/INV-002/prototype research/INV-002/run-bench.sh research/INV-002/run-extended-bench.sh research/INV-002/compose.yml \
    && git -C "${REPO_DIR}" diff --cached --quiet -- research/INV-002/prototype research/INV-002/run-bench.sh research/INV-002/run-extended-bench.sh research/INV-002/compose.yml; then
    printf 'true\n'
  else
    printf 'false\n'
  fi
}
benchmark_scope_untracked_count() {
  git -C "${REPO_DIR}" ls-files --others --exclude-standard -- research/INV-002/prototype research/INV-002/run-bench.sh research/INV-002/run-extended-bench.sh research/INV-002/compose.yml | wc -l | tr -d ' '
}
now_ms() { perl -MTime::HiRes=time -e 'printf "%.3f\n", time()*1000'; }
add_assertion() {
  local case_name="$1" assertion="$2" expected="$3" actual="$4" result=fail
  [ "${expected}" = "${actual}" ] && result=pass
  printf '%s\t%s\t%s\t%s\t%s\n' "${case_name}" "${assertion}" "${expected}" "${actual}" "${result}" >>"${ASSERTIONS}"
}

cleanup() {
  docker ps -aq --filter 'name=^/inv002-' | xargs -r docker rm -f >/dev/null 2>&1 || true
  docker volume rm inv002-external-state >/dev/null 2>&1 || true
}
trap cleanup EXIT
cleanup
docker build -t "${IMAGE}" "${ROOT_DIR}/prototype" >"${RESULTS_DIR}/docker-build.log"

run_case() {
  local name="$1" expected="$2" expected_attempts="$3" expected_counter="$4"; shift 4
  local log="${RESULTS_DIR}/${name}.log" code=0
  docker run --name "inv002-${name}" "${IMAGE}" "$@" >"${log}" 2>&1 || code=$?
  local final attempts counter result=fail
  final="$(grep 'EVENT lifecycle_finalized' "${log}" | tail -1)"
  attempts="$(printf '%s' "${final}" | sed -n 's/.*attempts=\([0-9]*\).*/\1/p')"
  counter="$(printf '%s' "${final}" | sed -n 's/.*value=\([0-9]*\).*/\1/p')"
  [ "${code}" = "${expected}" ] && [ "${attempts}" = "${expected_attempts}" ] && [ "${counter}" = "${expected_counter}" ] && result=pass
  printf '%s\t%s\t%s\t%s\t%s\t%s\n' "${name}" "${expected}" "${code}" "${attempts:-missing}" "${counter:-missing}" "${result}" >>"${SUMMARY}"
  awk -v c="${name}" '/EVENT attempt_exited/ {a=x=v=""; for(i=1;i<=NF;i++){if($i~/^attempt=/)a=substr($i,9);if($i~/^exit=/)x=substr($i,6);if($i~/^value=/)v=substr($i,7)}; print c"\tattempt_exit\t"a"\t"x"\t"v}' "${log}" >>"${OBS}"
  docker rm "inv002-${name}" >/dev/null
}

run_case single_success 0 1 5 --policy=single -- /usr/local/bin/workload --state=/tmp/a --exits=0 --increments=5 --hold=20ms
run_case single_failure 17 1 5 --policy=single -- /usr/local/bin/workload --state=/tmp/a --exits=17 --increments=5 --hold=20ms
run_case internal_reset 0 2 2 --policy=internal-restart --metric-state=reset --max-attempts=3 -- /usr/local/bin/workload --state=/tmp/a --exits=17,0 --increments=5,2 --hold=20ms
run_case internal_preserve 0 2 7 --policy=internal-restart --metric-state=preserve --max-attempts=3 -- /usr/local/bin/workload --state=/tmp/a --exits=17,0 --increments=5,2 --hold=20ms
run_case restart_limit 17 3 3 --policy=internal-restart --metric-state=preserve --max-attempts=3 -- /usr/local/bin/workload --state=/tmp/a --exits=17 --increments=1 --hold=20ms
run_case start_failure 127 0 0 --policy=single -- /missing/workload

# Docker owns the retry. Each MetricShell process still executes exactly once;
# /state only makes the synthetic workload fail once and then succeed.
docker volume create inv002-external-state >/dev/null
docker run -d --name inv002-external --restart=on-failure:2 -v inv002-external-state:/state "${IMAGE}" \
  --policy=single -- /usr/local/bin/workload --state=/state/attempt --exits=17,0 --increments=5,2 --hold=100ms \
  >"${RESULTS_DIR}/external_docker_restart.cid"
for _ in $(seq 1 100); do
  external_status="$(docker inspect -f '{{.State.Status}} {{.RestartCount}} {{.State.ExitCode}}' inv002-external)"
  [ "${external_status}" = "exited 1 0" ] && break
  sleep 0.05
done
docker logs inv002-external >"${RESULTS_DIR}/external_docker_restart.log" 2>&1
external_result=fail
[ "${external_status}" = "exited 1 0" ] && [ "$(grep -c 'EVENT lifecycle_finalized' "${RESULTS_DIR}/external_docker_restart.log")" = "2" ] && external_result=pass
printf 'external_docker_restart\t0\t%s\t2\t2\t%s\n' "$(printf '%s' "${external_status}" | awk '{print $3}')" "${external_result}" >>"${SUMMARY}"
awk -v c=external_docker_restart '/EVENT lifecycle_finalized/ {a++; x=v=""; for(i=1;i<=NF;i++){if($i~/^exit=/)x=substr($i,6);if($i~/^value=/)v=substr($i,7)}; print c"\tprocess_exit\t"a"\t"x"\t"v}' "${RESULTS_DIR}/external_docker_restart.log" >>"${OBS}"
docker rm inv002-external >/dev/null
docker volume rm inv002-external-state >/dev/null

# Verify the bounded post-exit endpoint while the supervisor is still alive.
post_started_ms="$(now_ms)"
docker run -d --name inv002-post -p 127.0.0.1::9090 "${IMAGE}" --policy=single --post-exit=2s -- /usr/local/bin/workload --state=/tmp/a --exits=17 --increments=5 --hold=20ms >"${RESULTS_DIR}/post.cid"
port="$(docker port inv002-post 9090/tcp | sed 's/.*://')"
for _ in $(seq 1 30); do curl -fsS "http://127.0.0.1:${port}/metrics" >"${RESULTS_DIR}/post-exit.metrics" 2>/dev/null && grep -q 'metricshell_workload_exit_code 17' "${RESULTS_DIR}/post-exit.metrics" && break; sleep 0.05; done
post_metrics_available=false; grep -q 'metricshell_workload_exit_code 17' "${RESULTS_DIR}/post-exit.metrics" && post_metrics_available=true
post_code="$(docker wait inv002-post)"
post_finished_ms="$(now_ms)"
post_elapsed_ms="$(awk -v a="${post_started_ms}" -v b="${post_finished_ms}" 'BEGIN {printf "%.3f", b-a}')"
docker logs inv002-post >"${RESULTS_DIR}/post-exit.log" 2>&1
post_attempts="$(grep -c 'EVENT attempt_started' "${RESULTS_DIR}/post-exit.log")"
post_lifecycles="$(grep -c 'EVENT lifecycle_finalized' "${RESULTS_DIR}/post-exit.log")"
post_counter="$(sed -n 's/.*EVENT lifecycle_finalized.*value=\([0-9]*\).*/\1/p' "${RESULTS_DIR}/post-exit.log" | tail -1)"
post_internal_elapsed_ms="$(sed -n 's/.*EVENT post_exit_end configured_ms=2000 elapsed_ms=\([0-9.]*\).*/\1/p' "${RESULTS_DIR}/post-exit.log" | tail -1)"
post_bounded=false; awk -v elapsed="${post_internal_elapsed_ms:-0}" 'BEGIN {exit !(elapsed >= 2000 && elapsed <= 2250)}' && post_bounded=true
add_assertion post_exit_endpoint actual_exit 17 "${post_code}"
add_assertion post_exit_endpoint attempt_started_count 1 "${post_attempts}"
add_assertion post_exit_endpoint lifecycle_finalized_count 1 "${post_lifecycles}"
add_assertion post_exit_endpoint final_counter 5 "${post_counter:-missing}"
add_assertion post_exit_endpoint metrics_available_during_window true "${post_metrics_available}"
add_assertion post_exit_endpoint internal_bounded_window_2000ms_to_2250ms true "${post_bounded}"
post_result=pass
awk -F '\t' '$1=="post_exit_endpoint"&&$5!="pass"{bad=1} END{exit bad?0:1}' "${ASSERTIONS}" && post_result=fail
printf 'post_exit_endpoint\t17\t%s\t%s\t%s\t%s\n' "${post_code}" "${post_attempts}" "${post_counter:-missing}" "${post_result}" >>"${SUMMARY}"
printf 'configured_ms\tinternal_elapsed_ms\thost_container_lifecycle_ms\n2000\t%s\t%s\n' "${post_internal_elapsed_ms:-missing}" "${post_elapsed_ms}" >"${RESULTS_DIR}/post-exit-duration.tsv"

{
  printf 'key\tvalue\n'
  printf 'run_date_utc\t%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  printf 'repository_head_sha\t%s\n' "$(git -C "${REPO_DIR}" rev-parse HEAD 2>/dev/null || printf unknown)"
  printf 'benchmark_scope_diff_clean\t%s\n' "$(benchmark_scope_diff_clean)"
  printf 'benchmark_scope_untracked_count\t%s\n' "$(benchmark_scope_untracked_count)"
  printf 'benchmark_code_fingerprint_sha256\t%s\n' "$(benchmark_code_fingerprint)"
  printf 'docker_server_version\t%s\n' "$(docker version --format '{{.Server.Version}}')"
  printf 'docker_architecture\t%s\n' "$(docker info --format '{{.Architecture}}')"
  printf 'container_kernel\t%s\n' "$(docker run --rm --entrypoint uname "${IMAGE}" -a)"
} >"${RESULTS_DIR}/environment.tsv"
printf '%s\n' "${RESULTS_DIR}" >"${ROOT_DIR}/latest-results.txt"
awk -F '\t' 'NR>1 && $6!="pass" {bad=1} END {exit bad}' "${SUMMARY}"
