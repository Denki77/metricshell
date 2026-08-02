#!/usr/bin/env bash
set -Eeuo pipefail
export LC_ALL=C

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(git -C "${ROOT_DIR}" rev-parse --show-toplevel)"
IMAGE="metricshell-inv011:prototype"
RUN_ID="${RUN_ID:-$(date -u +%Y%m%dT%H%M%SZ)}"
RESULTS_DIR="${ROOT_DIR}/results/${RUN_ID}"
ASSERTIONS="${RESULTS_DIR}/assertions.tsv"
OBSERVATIONS="${RESULTS_DIR}/observations.tsv"
SUMMARY="${RESULTS_DIR}/summary.tsv"
CONTAINER="inv011-bench-${RUN_ID}"
mkdir -p "${RESULTS_DIR}"
printf 'case\tassertion\texpected\tactual\tresult\n' >"${ASSERTIONS}"
printf 'case\tmetric\tvalue\tunit\tnote\n' >"${OBSERVATIONS}"
printf 'case\tresult\tdetails\n' >"${SUMMARY}"

hash_file() { if command -v sha256sum >/dev/null; then sha256sum "$1" | awk '{print $1}'; else shasum -a 256 "$1" | awk '{print $1}'; fi; }
hash_stdin() { if command -v sha256sum >/dev/null; then sha256sum | awk '{print $1}'; else shasum -a 256 | awk '{print $1}'; fi; }
fingerprint() {
  { find "${ROOT_DIR}/prototype" -type f | sort; printf '%s\n' "${ROOT_DIR}/run-bench.sh"; } |
    while IFS= read -r file; do printf '%s  %s\n' "$(hash_file "${file}")" "${file#${ROOT_DIR}/}"; done | hash_stdin
}
now_ns() { perl -MTime::HiRes=time -e 'printf "%.0f\n", time()*1000000000'; }
assert_eq() {
  local case_name="$1" assertion="$2" expected="$3" actual="$4" result=fail
  [[ "${expected}" == "${actual}" ]] && result=pass
  printf '%s\t%s\t%s\t%s\t%s\n' "${case_name}" "${assertion}" "${expected}" "${actual}" "${result}" >>"${ASSERTIONS}"
}
observe() { printf '%s\t%s\t%s\t%s\t%s\n' "$1" "$2" "$3" "$4" "${5:-}" >>"${OBSERVATIONS}"; }
case_result() {
  local case_name="$1" failed
  failed="$(awk -F '\t' -v c="${case_name}" 'NR>1&&$1==c&&$5!="pass"{n++} END{print n+0}' "${ASSERTIONS}")"
  if [[ "${failed}" == 0 ]]; then printf '%s\tpass\tall portable assertions passed\n' "${case_name}" >>"${SUMMARY}"; else printf '%s\tfail\t%s assertion(s) failed\n' "${case_name}" "${failed}" >>"${SUMMARY}"; fi
}
cleanup() { docker rm -f "${CONTAINER}" >/dev/null 2>&1 || true; }
trap cleanup EXIT
start_case() {
  local name="$1" started port running; shift
  cleanup
  started="$(now_ns)"
  docker run -d --name "${CONTAINER}" -p 127.0.0.1::19111 "${IMAGE}" "$@" >"${RESULTS_DIR}/${name}.cid"
  PORT=""; BASE=""
  for _ in $(seq 1 200); do
    running="$(docker inspect -f '{{.State.Running}}' "${CONTAINER}" 2>/dev/null || echo false)"
    if [[ "${running}" != true ]]; then
      docker logs "${CONTAINER}" >"${RESULTS_DIR}/${name}.startup.log" 2>&1 || true
      return 1
    fi
    port="$(docker inspect -f '{{with (index .NetworkSettings.Ports "19111/tcp")}}{{(index . 0).HostPort}}{{end}}' "${CONTAINER}" 2>/dev/null || true)"
    if [[ "${port}" =~ ^[0-9]+$ ]]; then
      PORT="${port}"; BASE="http://127.0.0.1:${PORT}"
      if curl -fsS --connect-timeout 1 --max-time 1 "${BASE}/healthz" >/dev/null 2>&1; then
        observe "${name}" published_host_port "${PORT}" port
        observe "${name}" http_ready_ns "$(( $(now_ns) - started ))" ns
        return 0
      fi
    fi
    sleep 0.05
  done
  docker logs "${CONTAINER}" >"${RESULTS_DIR}/${name}.startup.log" 2>&1 || true
  return 1
}
finish_case() {
  local name="$1" code
  code="$(docker wait "${CONTAINER}")"
  docker logs "${CONTAINER}" >"${RESULTS_DIR}/${name}.log" 2>&1
  printf '%s' "${code}"
}

cleanup
docker build -t "${IMAGE}" "${ROOT_DIR}/prototype" >"${RESULTS_DIR}/docker-build.log"

# Repeatedly exercise Docker's ephemeral port publication and the complete threshold-triggered HTTP shutdown path.
printf 'iteration\thost_port\thttp_code\tcurl_exit_code\tcontainer_exit_code\n' >"${RESULTS_DIR}/startup-repetitions.tsv"
startup_success=0
for iteration in $(seq 1 10); do
  if start_case "startup-repeat-${iteration}" --mode=scrapes --required=1 --wait=15s; then
    curl_code=0
    http_code="$(curl -sS -o /dev/null -w '%{http_code}' "${BASE}/metrics")" || curl_code=$?
    container_code="$(finish_case "startup-repeat-${iteration}")"
    printf '%s\t%s\t%s\t%s\t%s\n' "${iteration}" "${PORT}" "${http_code}" "${curl_code}" "${container_code}" >>"${RESULTS_DIR}/startup-repetitions.tsv"
    if [[ "${http_code}" == 200 && "${curl_code}" == 0 && "${container_code}" == 0 ]]; then startup_success=$((startup_success+1)); fi
  else
    printf '%s\t%s\t%s\t%s\t%s\n' "${iteration}" "${PORT:-}" unavailable unavailable unavailable >>"${RESULTS_DIR}/startup-repetitions.tsv"
  fi
done
assert_eq startup_repetitions complete_cycles 10 "${startup_success}"

# Application state is frozen before the waiting phase; self-metrics remain live.
start_case frozen_state --mode=duration --wait=15s
curl -fsS "${BASE}/metrics" >"${RESULTS_DIR}/frozen-a.metrics"; sleep 0.15; curl -fsS "${BASE}/metrics" >"${RESULTS_DIR}/frozen-b.metrics"
app_a="$(awk '/^application_jobs_total /{print $2}' "${RESULTS_DIR}/frozen-a.metrics")"; app_b="$(awk '/^application_jobs_total /{print $2}' "${RESULTS_DIR}/frozen-b.metrics")"
self_a="$(awk '/^metricshell_scrape_attempts_total /{print $2}' "${RESULTS_DIR}/frozen-a.metrics")"; self_b="$(awk '/^metricshell_scrape_attempts_total /{print $2}' "${RESULTS_DIR}/frozen-b.metrics")"
freeze_code="$(curl -sS -o "${RESULTS_DIR}/frozen-snapshot.response" -w '%{http_code}' -X PUT --data 'application_jobs_total 99' "${BASE}/snapshot")"
assert_eq frozen_state application_value_immutable "${app_a}" "${app_b}"
assert_eq frozen_state self_metrics_continue true "$([[ "${self_b}" -gt "${self_a}" ]] && echo true || echo false)"
assert_eq frozen_state ingestion_closed_after_finalization 409 "${freeze_code}"
code="$(finish_case frozen_state)"; assert_eq frozen_state exit_code 0 "${code}"

# Immediate and duration candidates, with elapsed time recorded as observations.
cleanup; started="$(now_ns)"; code=0
docker run --name "${CONTAINER}" "${IMAGE}" --mode=immediate >"${RESULTS_DIR}/immediate.log" 2>&1 || code=$?
finished="$(now_ns)"
assert_eq immediate exit_code 0 "${code}"; assert_eq immediate reason immediate "$(grep -q 'reason=immediate' "${RESULTS_DIR}/immediate.log" && echo immediate || echo missing)"; observe immediate container_lifecycle_ns "$((finished-started))" ns
cleanup; started="$(now_ns)"; code=0
docker run --name "${CONTAINER}" "${IMAGE}" --mode=duration --wait=500ms >"${RESULTS_DIR}/duration.log" 2>&1 || code=$?
finished="$(now_ns)"
assert_eq duration exit_code 0 "${code}"; assert_eq duration reason duration_elapsed "$(grep -q 'reason=duration_elapsed' "${RESULTS_DIR}/duration.log" && echo duration_elapsed || echo missing)"; observe duration container_lifecycle_ns "$((finished-started))" ns "configured 500 ms"

# Health/readiness do not count; a normal manual curl is an eligible scrape.
start_case one_scrape --mode=scrapes --required=1 --wait=15s
for _ in $(seq 1 5); do curl -fsS "${BASE}/healthz" >/dev/null; curl -fsS "${BASE}/readyz" >/dev/null; done
state="$(curl -fsS "${BASE}/state")"; assert_eq one_scrape health_and_readiness_count "completed=0 attempted=0" "${state}"
curl -fsS "${BASE}/metrics" >"${RESULTS_DIR}/one-scrape.metrics"; code="$(finish_case one_scrape)"
assert_eq one_scrape manual_curl_counts 0 "${code}"
assert_eq one_scrape final_count 1 "$(grep -q 'final_scrapes=1' "${RESULTS_DIR}/one_scrape.log" && echo 1 || echo missing)"

# Repeated requests from the same client count independently; uniqueness is not required.
start_case n_scrapes --mode=scrapes --required=3 --wait=15s
curl -fsS "${BASE}/metrics" >/dev/null; curl -fsS "${BASE}/metrics" >/dev/null
state="$(curl -fsS "${BASE}/state")"; assert_eq n_scrapes before_n "completed=2 attempted=2" "${state}"
running="$(docker inspect -f '{{.State.Running}}' "${CONTAINER}")"; assert_eq n_scrapes still_running_before_n true "${running}"
curl -fsS "${BASE}/metrics" >/dev/null; code="$(finish_case n_scrapes)"; assert_eq n_scrapes exit_code 0 "${code}"
assert_eq n_scrapes same_client_counted_independently 3 "$(grep -q 'final_scrapes=3' "${RESULTS_DIR}/n_scrapes.log" && echo 3 || echo missing)"

# Concurrent completions are independent, while the configured counter saturates at N.
start_case concurrent_scrapes --mode=scrapes --required=10 --wait=15s --completion-grace=500ms
client_code=0
seq 1 20 | xargs -P20 -I{} curl -fsS "${BASE}/metrics" -o /dev/null >"${RESULTS_DIR}/concurrent-clients.log" 2>&1 || client_code=$?
code="$(finish_case concurrent_scrapes)"; assert_eq concurrent_scrapes exit_code 0 "${code}"
assert_eq concurrent_scrapes all_client_responses_complete 0 "${client_code}"
assert_eq concurrent_scrapes counter_saturates_at_n 10 "$(grep -q 'final_scrapes=10' "${RESULTS_DIR}/concurrent_scrapes.log" && echo 10 || echo missing)"

# Optional explicit eligibility is possible but is not proof of TSDB persistence.
start_case eligibility_token --mode=scrapes --required=1 --wait=15s --eligible-token=research-token
curl -fsS "${BASE}/metrics" >"${RESULTS_DIR}/token-missing.metrics"
state="$(curl -fsS "${BASE}/state")"; assert_eq eligibility_token ordinary_response_not_eligible "completed=0 attempted=1" "${state}"
curl -fsS -H 'X-Final-Scrape-Token: research-token' "${BASE}/metrics" >"${RESULTS_DIR}/token-present.metrics"; code="$(finish_case eligibility_token)"
assert_eq eligibility_token eligible_response_exits 0 "${code}"

# A disconnected large/chunked response must not count; a later complete response does.
start_case aborted_scrape --mode=scrapes --required=1 --wait=15s --padding-bytes=8388608 --chunk-delay=1ms
( exec 3<>"/dev/tcp/127.0.0.1/${PORT}"; printf 'GET /metrics HTTP/1.1\r\nHost: localhost\r\n\r\n' >&3; sleep 0.01; exec 3>&- ) || true
sleep 0.25
state="$(curl -fsS "${BASE}/state")"; completed="$(printf '%s' "${state}" | sed -n 's/.*completed=\([0-9]*\).*/\1/p')"; attempted="$(printf '%s' "${state}" | sed -n 's/.*attempted=\([0-9]*\).*/\1/p')"
assert_eq aborted_scrape disconnected_not_counted 0 "${completed}"
assert_eq aborted_scrape attempt_observed true "$([[ "${attempted}" -ge 1 ]] && echo true || echo false)"
curl -fsS "${BASE}/metrics" -o "${RESULTS_DIR}/complete-large.metrics"; code="$(finish_case aborted_scrape)"
assert_eq aborted_scrape later_complete_response_counts 0 "${code}"

# Timeout is bounded and does not manufacture a completed scrape.
cleanup; started="$(now_ns)"; code=0
docker run --name "${CONTAINER}" "${IMAGE}" --mode=scrapes --required=2 --wait=500ms >"${RESULTS_DIR}/timeout.log" 2>&1 || code=$?
finished="$(now_ns)"
assert_eq timeout exit_code 0 "${code}"; assert_eq timeout zero_completed 0 "$(grep -q 'reason=timeout final_scrapes=0' "${RESULTS_DIR}/timeout.log" && echo 0 || echo missing)"; observe timeout container_lifecycle_ns "$((finished-started))" ns "configured 500 ms"

for case_name in startup_repetitions frozen_state immediate duration one_scrape n_scrapes concurrent_scrapes eligibility_token aborted_scrape timeout; do case_result "${case_name}"; done
{
  printf 'key\tvalue\n'
  printf 'investigation\tINV-011\n'
  printf 'run_date_utc\t%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  printf 'repository_head_sha\t%s\n' "$(git -C "${REPO_DIR}" rev-parse HEAD 2>/dev/null || printf unknown)"
  printf 'benchmark_scope_diff_clean\t%s\n' "$(git -C "${REPO_DIR}" diff --quiet -- research/INV-011/prototype research/INV-011/run-bench.sh && git -C "${REPO_DIR}" diff --cached --quiet -- research/INV-011/prototype research/INV-011/run-bench.sh && echo true || echo false)"
  printf 'benchmark_code_fingerprint_sha256\t%s\n' "$(fingerprint)"
  printf 'host_uname\t%s\n' "$(uname -a | tr '\t\n' ' ')"
  printf 'docker_server_version\t%s\n' "$(docker version --format '{{.Server.Version}}')"
  printf 'docker_architecture\t%s\n' "$(docker info --format '{{.Architecture}}')"
  printf 'docker_operating_system\t%s\n' "$(docker info --format '{{.OperatingSystem}}' | tr '\t\n' ' ')"
  printf 'container_kernel\t%s\n' "$(docker run --rm --entrypoint uname "${IMAGE}" -a)"
} >"${RESULTS_DIR}/environment.tsv"
printf '%s\n' "${RESULTS_DIR}" >"${ROOT_DIR}/latest-results.txt"
failed="$(awk -F '\t' 'NR>1&&$5!="pass"{n++} END{print n+0}' "${ASSERTIONS}")"
printf 'failed_assertions\t%s\nresults_dir\t%s\n' "${failed}" "${RESULTS_DIR}" >"${RESULTS_DIR}/run-summary.tsv"
[[ "${failed}" == 0 ]]
