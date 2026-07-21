#!/usr/bin/env bash
set -euo pipefail
export LC_ALL=C

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(git -C "${ROOT_DIR}" rev-parse --show-toplevel)"
IMAGE="metricshell-inv002:prototype"
RESULTS_DIR="${ROOT_DIR}/results/$(date -u +%Y%m%dT%H%M%SZ)-extended"
REPETITIONS="${INV002_REPEAT_COUNT:-30}"
mkdir -p "${RESULTS_DIR}"

hash_file() {
  if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | awk '{print $1}'; else shasum -a 256 "$1" | awk '{print $1}'; fi
}
hash_stdin() {
  if command -v sha256sum >/dev/null 2>&1; then sha256sum | awk '{print $1}'; else shasum -a 256 "$1" | awk '{print $1}'; fi
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

cleanup() {
  docker ps -aq --filter 'name=^/inv002x-' | xargs -r docker rm -f >/dev/null 2>&1 || true
  docker compose -p inv002x -f "${ROOT_DIR}/compose.yml" down -v >/dev/null 2>&1 || true
}
trap cleanup EXIT
cleanup
docker build -t "${IMAGE}" "${ROOT_DIR}/prototype" >"${RESULTS_DIR}/docker-build.log"

now_ms() { perl -MTime::HiRes=time -e 'printf "%.3f\n", time()*1000'; }
percentile() {
  local file="$1" p="$2"
  sort -n "${file}" | awk -v p="${p}" '{a[NR]=$1} END {i=int((NR-1)*p+1); print a[i]}'
}

# 30-run lifecycle latency and reliability.
printf 'iteration\tduration_ms\texit_code\tresult\n' >"${RESULTS_DIR}/repetitions.tsv"
: >"${RESULTS_DIR}/repetition-values.tmp"
for i in $(seq 1 "${REPETITIONS}"); do
  start="$(now_ms)"; code=0
  docker run --rm "${IMAGE}" --policy=single -- /usr/local/bin/workload --state=/tmp/a --exits=0 --increments=1 --hold=1ms >/dev/null 2>&1 || code=$?
  end="$(now_ms)"; duration="$(awk -v a="${start}" -v b="${end}" 'BEGIN {printf "%.3f", b-a}')"
  result=fail; [ "${code}" = 0 ] && result=pass
  printf '%s\t%s\t%s\t%s\n' "${i}" "${duration}" "${code}" "${result}" >>"${RESULTS_DIR}/repetitions.tsv"
  printf '%s\n' "${duration}" >>"${RESULTS_DIR}/repetition-values.tmp"
done
printf 'count\tpassed\tp50_ms\tp95_ms\tp99_ms\tmin_ms\tmax_ms\n' >"${RESULTS_DIR}/latency-stats.tsv"
printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\n' "${REPETITIONS}" \
  "$(awk -F '\t' 'NR>1&&$4=="pass"{n++} END{print n+0}' "${RESULTS_DIR}/repetitions.tsv")" \
  "$(percentile "${RESULTS_DIR}/repetition-values.tmp" .50)" "$(percentile "${RESULTS_DIR}/repetition-values.tmp" .95)" \
  "$(percentile "${RESULTS_DIR}/repetition-values.tmp" .99)" "$(sort -n "${RESULTS_DIR}/repetition-values.tmp" | head -1)" \
  "$(sort -n "${RESULTS_DIR}/repetition-values.tmp" | tail -1)" >>"${RESULTS_DIR}/latency-stats.tsv"

# Compose-owned restart.
docker compose -p inv002x -f "${ROOT_DIR}/compose.yml" up -d >"${RESULTS_DIR}/compose-up.log" 2>&1
compose_cid="$(docker compose -p inv002x -f "${ROOT_DIR}/compose.yml" ps -q metricshell)"
for _ in $(seq 1 100); do
  compose_status="$(docker inspect -f '{{.State.Status}} {{.RestartCount}} {{.State.ExitCode}}' "${compose_cid}")"
  [ "${compose_status}" = "exited 1 0" ] && break
  sleep 0.05
done
docker logs "${compose_cid}" >"${RESULTS_DIR}/compose-restart.log" 2>&1
printf 'implementation\tstatus\trestart_count\texit_code\tlifecycles\tresult\n' >"${RESULTS_DIR}/runtime-restarts.tsv"
compose_result=fail
[ "${compose_status}" = "exited 1 0" ] && [ "$(grep -c 'EVENT lifecycle_finalized' "${RESULTS_DIR}/compose-restart.log")" = 2 ] && compose_result=pass
printf 'compose\t%s\t%s\t%s\t%s\t%s\n' $(printf '%s' "${compose_status}" | tr ' ' '\n') \
  "$(grep -c 'EVENT lifecycle_finalized' "${RESULTS_DIR}/compose-restart.log")" "${compose_result}" >>"${RESULTS_DIR}/runtime-restarts.tsv"
docker compose -p inv002x -f "${ROOT_DIR}/compose.yml" down -v >/dev/null

# Scrape at 50 ms across an external Docker restart.
docker volume create inv002x-scrape-state >/dev/null
docker run -d --name inv002x-scrape --restart=on-failure:2 -p 127.0.0.1::9090 -v inv002x-scrape-state:/state "${IMAGE}" \
  --policy=single --post-exit=250ms -- /usr/local/bin/workload --state=/state/attempt --exits=17,0 --increments=5,2 --hold=500ms >/dev/null
scrape_port="$(docker port inv002x-scrape 9090/tcp | sed 's/.*://')"
printf 'sample\ttimestamp_ms\thttp_code\tcounter\tattempt\texit_code\n' >"${RESULTS_DIR}/restart-scrapes.tsv"
for i in $(seq 1 150); do
  body="${RESULTS_DIR}/scrape.tmp"; : >"${body}"
  http="$(curl -sS -o "${body}" -w '%{http_code}' "http://127.0.0.1:${scrape_port}/metrics" 2>/dev/null || true)"
  printf '%s\t%s\t%s\t%s\t%s\t%s\n' "${i}" "$(now_ms)" "${http:-000}" \
    "$(awk '$1=="app_events_total"{print $2}' "${body}" 2>/dev/null || true)" \
    "$(awk '$1=="metricshell_workload_attempt"{print $2}' "${body}" 2>/dev/null || true)" \
    "$(awk '$1=="metricshell_workload_exit_code"{print $2}' "${body}" 2>/dev/null || true)" >>"${RESULTS_DIR}/restart-scrapes.tsv"
  sleep 0.05
done
docker logs inv002x-scrape >"${RESULTS_DIR}/restart-scrape.log" 2>&1
printf 'assertion\texpected\tactual\tresult\n' >"${RESULTS_DIR}/restart-scrape-assertions.tsv"
printf 'observation\tvalue\n' >"${RESULTS_DIR}/restart-scrape-observations.tsv"
printf 'samples\thttp_200\thttp_gap\tcounter_5\tcounter_2\tlifecycles\tresult\n' >"${RESULTS_DIR}/restart-scrape-stats.tsv"
scrape_ok="$(awk -F '\t' 'NR>1&&$3==200{n++} END{print n+0}' "${RESULTS_DIR}/restart-scrapes.tsv")"
scrape_gap="$(awk -F '\t' 'NR>1&&$3!=200{n++} END{print n+0}' "${RESULTS_DIR}/restart-scrapes.tsv")"
counter_5="$(awk -F '\t' 'NR>1&&$4==5{n++} END{print n+0}' "${RESULTS_DIR}/restart-scrapes.tsv")"
counter_2="$(awk -F '\t' 'NR>1&&$4==2{n++} END{print n+0}' "${RESULTS_DIR}/restart-scrapes.tsv")"
scrape_lifecycles="$(grep -c 'EVENT lifecycle_finalized' "${RESULTS_DIR}/restart-scrape.log")"
first_lifecycle_exit="$(sed -n 's/.*EVENT lifecycle_finalized attempts=[0-9]* exit=\([0-9]*\).*/\1/p' "${RESULTS_DIR}/restart-scrape.log" | sed -n '1p')"
second_lifecycle_exit="$(sed -n 's/.*EVENT lifecycle_finalized attempts=[0-9]* exit=\([0-9]*\).*/\1/p' "${RESULTS_DIR}/restart-scrape.log" | sed -n '2p')"
single_attempt_lifecycles="$(grep -c 'EVENT lifecycle_finalized attempts=1 ' "${RESULTS_DIR}/restart-scrape.log")"
scrape_restart_count="$(docker inspect -f '{{.RestartCount}}' inv002x-scrape)"
restart_gap_observed=false; [ "${scrape_gap}" -gt 0 ] && restart_gap_observed=true
first_epoch_observed=false; [ "${counter_5}" -gt 0 ] && first_epoch_observed=true
second_epoch_observed=false; [ "${counter_2}" -gt 0 ] && second_epoch_observed=true
for assertion in \
  "two_lifecycles_completed:2:${scrape_lifecycles}" \
  "first_lifecycle_exit:17:${first_lifecycle_exit:-missing}" \
  "second_lifecycle_exit:0:${second_lifecycle_exit:-missing}" \
  "each_metricshell_process_executed_once:2:${single_attempt_lifecycles}" \
  "runtime_restart_count:1:${scrape_restart_count}"; do
  IFS=: read -r name expected actual <<<"${assertion}"; result=fail; [ "${expected}" = "${actual}" ] && result=pass
  printf '%s\t%s\t%s\t%s\n' "${name}" "${expected}" "${actual}" "${result}" >>"${RESULTS_DIR}/restart-scrape-assertions.tsv"
done
printf 'restart_gap_observed\t%s\n' "${restart_gap_observed}" >>"${RESULTS_DIR}/restart-scrape-observations.tsv"
printf 'first_epoch_observed\t%s\n' "${first_epoch_observed}" >>"${RESULTS_DIR}/restart-scrape-observations.tsv"
printf 'second_epoch_observed\t%s\n' "${second_epoch_observed}" >>"${RESULTS_DIR}/restart-scrape-observations.tsv"
printf 'http_200_count\t%s\n' "${scrape_ok}" >>"${RESULTS_DIR}/restart-scrape-observations.tsv"
printf 'http_gap_count\t%s\n' "${scrape_gap}" >>"${RESULTS_DIR}/restart-scrape-observations.tsv"
scrape_result=pass; awk -F '\t' 'NR>1&&$4!="pass"{bad=1} END{exit bad?0:1}' "${RESULTS_DIR}/restart-scrape-assertions.tsv" && scrape_result=fail
printf '150\t%s\t%s\t%s\t%s\t%s\t%s\n' "${scrape_ok}" "${scrape_gap}" "${counter_5}" "${counter_2}" "${scrape_lifecycles}" "${scrape_result}" >>"${RESULTS_DIR}/restart-scrape-stats.tsv"
docker rm -f inv002x-scrape >/dev/null; docker volume rm inv002x-scrape-state >/dev/null

# Signal-to-exit latency measured inside MetricShell, excluding Docker CLI transport latency.
printf 'iteration\tsignal_to_exit_ms\texit_code\tresult\n' >"${RESULTS_DIR}/signal-to-exit-latency.tsv"
: >"${RESULTS_DIR}/signal-to-exit-values.tmp"
for i in $(seq 1 "${REPETITIONS}"); do
  name="inv002x-signal-${i}"
  docker run -d --name "${name}" "${IMAGE}" --policy=single -- /usr/local/bin/workload --state=/tmp/a --exits=0 --increments=1 --hold=30s >/dev/null
  ready=false
  for _ in $(seq 1 100); do docker logs "${name}" 2>&1 | grep -q 'WORKLOAD_READY' && { ready=true; break; }; sleep 0.02; done
  docker kill --signal TERM "${name}" >/dev/null
  code="$(docker wait "${name}")"
  docker logs "${name}" >"${RESULTS_DIR}/signal-${i}.log" 2>&1
  latency="$(sed -n 's/.*EVENT signal_to_exit elapsed_ms=\([0-9.]*\).*/\1/p' "${RESULTS_DIR}/signal-${i}.log" | tail -1)"
  result=fail; [ "${ready}" = true ] && [ "${code}" = 143 ] && [ -n "${latency}" ] && result=pass
  printf '%s\t%s\t%s\t%s\n' "${i}" "${latency:-missing}" "${code}" "${result}" >>"${RESULTS_DIR}/signal-to-exit-latency.tsv"
  [ -n "${latency}" ] && printf '%s\n' "${latency}" >>"${RESULTS_DIR}/signal-to-exit-values.tmp"
  docker rm "${name}" >/dev/null
done
printf 'count\tpassed\tp50_ms\tp95_ms\tp99_ms\tmin_ms\tmax_ms\n' >"${RESULTS_DIR}/signal-to-exit-latency-stats.tsv"
printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\n' "${REPETITIONS}" \
  "$(awk -F '\t' 'NR>1&&$4=="pass"{n++} END{print n+0}' "${RESULTS_DIR}/signal-to-exit-latency.tsv")" \
  "$(percentile "${RESULTS_DIR}/signal-to-exit-values.tmp" .50)" "$(percentile "${RESULTS_DIR}/signal-to-exit-values.tmp" .95)" \
  "$(percentile "${RESULTS_DIR}/signal-to-exit-values.tmp" .99)" "$(sort -n "${RESULTS_DIR}/signal-to-exit-values.tmp" | head -1)" \
  "$(sort -n "${RESULTS_DIR}/signal-to-exit-values.tmp" | tail -1)" >>"${RESULTS_DIR}/signal-to-exit-latency-stats.tsv"

# Signal and OOM fault injection.
printf 'case\texpected_exit\tactual_exit\tresult\n' >"${RESULTS_DIR}/faults.tsv"
for spec in term:TERM:143 kill:KILL:137; do
  IFS=: read -r name signal expected <<<"${spec}"
  docker run -d --name "inv002x-${name}" "${IMAGE}" --policy=single -- /usr/local/bin/workload --state=/tmp/a --exits=0 --increments=1 --hold=10s >/dev/null
  sleep 0.2; docker kill --signal "${signal}" "inv002x-${name}" >/dev/null; code="$(docker wait "inv002x-${name}")"
  result=fail; [ "${code}" = "${expected}" ] && result=pass
  printf '%s\t%s\t%s\t%s\n' "${name}" "${expected}" "${code}" "${result}" >>"${RESULTS_DIR}/faults.tsv"
  docker logs "inv002x-${name}" >"${RESULTS_DIR}/${name}.log" 2>&1; docker rm "inv002x-${name}" >/dev/null
done
docker run -d --name inv002x-container-oom --memory=32m --memory-swap=32m "${IMAGE}" --policy=single -- /usr/local/bin/workload --state=/tmp/a --exits=0 --increments=1 --allocate-mb=128 --hold=1s >/dev/null
oom_code="$(docker wait inv002x-container-oom)"; oom_killed="$(docker inspect -f '{{.State.OOMKilled}}' inv002x-container-oom)"
docker inspect inv002x-container-oom >"${RESULTS_DIR}/container-oom.inspect.json"
docker logs inv002x-container-oom >"${RESULTS_DIR}/container-oom.log" 2>&1
oom_observed=false; grep -q 'EVENT attempt_exited attempt=1 exit=137' "${RESULTS_DIR}/container-oom.log" && oom_observed=true
oom_finalized=false; grep -q 'EVENT lifecycle_finalized attempts=1 exit=137' "${RESULTS_DIR}/container-oom.log" && oom_finalized=true
oom_result=fail
[ "${oom_code}" = 137 ] && [ "${oom_killed}" = true ] && [ "${oom_observed}" = true ] && [ "${oom_finalized}" = true ] && oom_result=pass
printf 'container_oom\t137\t%s\t%s\n' "${oom_code}" "${oom_result}" >>"${RESULTS_DIR}/faults.tsv"
printf 'assertion\texpected\tactual\tresult\n' >"${RESULTS_DIR}/container-oom-assertions.tsv"
for assertion in "container_exit:137:${oom_code}" "container_oom_killed:true:${oom_killed}" "metricshell_observed_exit_137:true:${oom_observed}" "lifecycle_finalized_137:true:${oom_finalized}"; do
  IFS=: read -r name expected actual <<<"${assertion}"; result=fail; [ "${expected}" = "${actual}" ] && result=pass
  printf '%s\t%s\t%s\t%s\n' "${name}" "${expected}" "${actual}" "${result}" >>"${RESULTS_DIR}/container-oom-assertions.tsv"
done
docker rm inv002x-container-oom >/dev/null

# Post-exit grid, run concurrently so wall time is bounded by 30 seconds. Pass/fail uses
# the in-process timer; host Docker lifecycle time is retained only as an observation.
printf 'configured_seconds\tinternal_elapsed_ms\thost_lifecycle_ms\texit_code\tresult\n' >"${RESULTS_DIR}/post-exit-grid.tsv"
for seconds in 0 1 2 5 10 30; do
  (
    start="$(now_ms)"; code=0
    docker run --rm "${IMAGE}" --policy=single --post-exit="${seconds}s" -- /usr/local/bin/workload --state=/tmp/a --exits=17 --increments=1 --hold=1ms >"${RESULTS_DIR}/post-${seconds}.log" 2>&1 || code=$?
    end="$(now_ms)"; host_elapsed="$(awk -v a="${start}" -v b="${end}" 'BEGIN {printf "%.3f", b-a}')"
    internal_elapsed="$(sed -n 's/.*EVENT post_exit_end configured_ms=[0-9]* elapsed_ms=\([0-9.]*\).*/\1/p' "${RESULTS_DIR}/post-${seconds}.log" | tail -1)"
    result=fail
    awk -v got="${internal_elapsed:-0}" -v want="${seconds}" 'BEGIN {exit !(got >= want*1000 && got <= want*1000+250)}' \
      && [ "${code}" = 17 ] && result=pass
    printf '%s\t%s\t%s\t%s\t%s\n' "${seconds}" "${internal_elapsed:-missing}" "${host_elapsed}" "${code}" "${result}" >"${RESULTS_DIR}/post-${seconds}.row"
  ) &
done
wait
for seconds in 0 1 2 5 10 30; do cat "${RESULTS_DIR}/post-${seconds}.row" >>"${RESULTS_DIR}/post-exit-grid.tsv"; done

# Internal restart storm quantifies the extra supervisor scope without Docker's intentional exponential backoff.
printf 'attempts\tduration_ms\texit_code\tlog_bytes\tresult\n' >"${RESULTS_DIR}/restart-storm.tsv"
for attempts in 10 100 1000; do
  start="$(now_ms)"; code=0
  docker run --name "inv002x-storm-${attempts}" "${IMAGE}" --policy=internal-restart --metric-state=preserve --max-attempts="${attempts}" -- \
    /usr/local/bin/workload --state=/tmp/a --exits=17 --increments=1 --hold=0s >"${RESULTS_DIR}/storm-${attempts}.log" 2>&1 || code=$?
  end="$(now_ms)"; duration="$(awk -v a="${start}" -v b="${end}" 'BEGIN {printf "%.3f", b-a}')"
  count="$(grep -c 'EVENT attempt_exited' "${RESULTS_DIR}/storm-${attempts}.log")"; result=fail
  [ "${code}" = 17 ] && [ "${count}" = "${attempts}" ] && result=pass
  printf '%s\t%s\t%s\t%s\t%s\n' "${attempts}" "${duration}" "${code}" "$(wc -c <"${RESULTS_DIR}/storm-${attempts}.log" | tr -d ' ')" "${result}" >>"${RESULTS_DIR}/restart-storm.tsv"
  docker rm "inv002x-storm-${attempts}" >/dev/null
done

printf 'benchmark\tstatus\treason\n' >"${RESULTS_DIR}/coverage.tsv"
container_arch="$(docker info --format '{{.Architecture}}')"
printf 'native_linux_x86_64\tnot_run\tcurrent container environment is LinuxKit %s, not native Linux\n' "${container_arch}" >>"${RESULTS_DIR}/coverage.tsv"
printf 'native_linux_arm64\tnot_run\tcurrent container environment is LinuxKit %s, not native Linux\n' "${container_arch}" >>"${RESULTS_DIR}/coverage.tsv"
printf 'kubernetes_job\tnot_run\tconfigured cluster OAuth refresh token is invalid\n' >>"${RESULTS_DIR}/coverage.tsv"
printf 'docker_daemon_restart\tnot_run\twould disrupt unrelated local containers and requires explicit separate authorization\n' >>"${RESULTS_DIR}/coverage.tsv"
printf 'disk_full\tnot_run\tout of INV-002 lifecycle scope and unsafe on shared Docker daemon\n' >>"${RESULTS_DIR}/coverage.tsv"
printf 'network_partition\tnot_run\tprototype has no remote network dependency\n' >>"${RESULTS_DIR}/coverage.tsv"

{
  printf 'key\tvalue\n'; printf 'run_date_utc\t%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  printf 'repository_head_sha\t%s\n' "$(git -C "${REPO_DIR}" rev-parse HEAD 2>/dev/null || printf unknown)"
  printf 'benchmark_scope_diff_clean\t%s\n' "$(benchmark_scope_diff_clean)"
  printf 'benchmark_scope_untracked_count\t%s\n' "$(benchmark_scope_untracked_count)"
  printf 'benchmark_code_fingerprint_sha256\t%s\n' "$(benchmark_code_fingerprint)"
  printf 'docker_server_version\t%s\n' "$(docker version --format '{{.Server.Version}}')"
  printf 'docker_info\t%s\n' "$(docker info --format '{{.OSType}}/{{.Architecture}} ncpu={{.NCPU}} memory={{.MemTotal}}')"
  printf 'container_kernel\t%s\n' "$(docker run --rm --entrypoint uname "${IMAGE}" -a)"
  printf 'repeat_count\t%s\n' "${REPETITIONS}"
} >"${RESULTS_DIR}/environment.tsv"
rm -f "${RESULTS_DIR}"/*.tmp "${RESULTS_DIR}"/post-*.row
printf '%s\n' "${RESULTS_DIR}" >"${ROOT_DIR}/latest-extended-results.txt"

awk -F '\t' 'NR>1 && $4!="pass" {bad=1} END {exit bad}' "${RESULTS_DIR}/repetitions.tsv"
awk -F '\t' 'NR>1 && $6!="pass" {bad=1} END {exit bad}' "${RESULTS_DIR}/runtime-restarts.tsv"
awk -F '\t' 'NR>1 && $7!="pass" {bad=1} END {exit bad}' "${RESULTS_DIR}/restart-scrape-stats.tsv"
awk -F '\t' 'NR>1 && $4!="pass" {bad=1} END {exit bad}' "${RESULTS_DIR}/signal-to-exit-latency.tsv"
awk -F '\t' 'NR>1 && $4!="pass" {bad=1} END {exit bad}' "${RESULTS_DIR}/faults.tsv"
awk -F '\t' 'NR>1 && $5!="pass" {bad=1} END {exit bad}' "${RESULTS_DIR}/post-exit-grid.tsv"
awk -F '\t' 'NR>1 && $5!="pass" {bad=1} END {exit bad}' "${RESULTS_DIR}/restart-storm.tsv"
