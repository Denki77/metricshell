#!/usr/bin/env bash
set -Eeuo pipefail
export LC_ALL=C

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(git -C "${ROOT_DIR}" rev-parse --show-toplevel)"
RESULTS_DIR="${ROOT_DIR}/results/$(date -u +%Y%m%dT%H%M%SZ)"
PROJECT="inv018-$(date -u +%Y%m%d%H%M%S)-$$"
COMPOSE=(docker compose -p "${PROJECT}" -f "${ROOT_DIR}/compose.yml")
IMAGE="metricshell-inv018:prototype"
PHP_IMAGE="metricshell-inv018:php54"
PHP_BASE="devilbox/php-fpm-5.4@sha256:0060e0fc7f3f89d88b54fcac5a9767f4df02880c23415d3dba8ced497c296b12"
REPETITIONS="${INV018_REPETITIONS:-100}"
TRANSPORT_OPERATIONS_PER_CLIENT="${INV018_TRANSPORT_OPERATIONS_PER_CLIENT:-200}"
mkdir -p "${RESULTS_DIR}"
SUMMARY="${RESULTS_DIR}/summary.tsv"; ASSERTIONS="${RESULTS_DIR}/assertions.tsv"; TRANSPORTS="${RESULTS_DIR}/transport-comparison.tsv"; BENCH="${RESULTS_DIR}/benchmarks.tsv"; PERSISTENT_BENCH="${RESULTS_DIR}/persistent-transport-benchmarks.tsv"
printf 'experiment\tcase\tresult\tdetail\n' >"${SUMMARY}"
printf 'experiment\tassertion\texpected\tactual\tresult\n' >"${ASSERTIONS}"
printf 'transport\tdependency\tframing\tlocal_boundary\tdebuggability\n' >"${TRANSPORTS}"
printf 'transport\tclients\toperations\telapsed_ms\tops_per_second\terrors\n' >"${BENCH}"
printf 'transport\tprofile\tclients\toperations\telapsed_ms\tops_per_second\tack_p50_ms\tack_p95_ms\tack_p99_ms\terrors\tending_goroutines\n' >"${PERSISTENT_BENCH}"

CURRENT_PHASE=initialization
on_error(){
  local code=$? line="${BASH_LINENO[0]:-unknown}"
  case $- in
    *e*) ;;
    *) return "${code}" ;;
  esac
  printf 'INV-018 failed: phase=%s line=%s exit=%s\n' "${CURRENT_PHASE}" "${line}" "${code}" >&2
  printf 'Partial evidence retained in: %s\n' "${RESULTS_DIR}" >&2
  exit "${code}"
}
trap on_error ERR

run_logged(){
  local phase="$1" log="$2" code
  shift 2
  CURRENT_PHASE="${phase}"
  printf 'INV-018: %s\n' "${phase}"
  if "$@" >"${log}" 2>&1; then
    return 0
  else
    code=$?
    printf 'Command failed; last log lines from %s:\n' "${log}" >&2
    tail -80 "${log}" >&2 || true
    return "${code}"
  fi
}

hash_file(){ if command -v sha256sum >/dev/null 2>&1;then sha256sum "$1"|awk '{print $1}';else shasum -a 256 "$1"|awk '{print $1}';fi; }
hash_stdin(){ if command -v sha256sum >/dev/null 2>&1;then sha256sum|awk '{print $1}';else shasum -a 256|awk '{print $1}';fi; }
fingerprint(){ { find "${ROOT_DIR}/prototype" "${ROOT_DIR}/clients" -type f|LC_ALL=C sort;printf '%s\n' "${ROOT_DIR}/compose.yml" "${ROOT_DIR}/run-bench.sh"; }|while IFS= read -r p;do printf '%s  %s\n' "$(hash_file "$p")" "${p#${ROOT_DIR}/}";done|hash_stdin; }
assert_eq(){ local exp="$1" name="$2" want="$3" got="$4" result=fail;[ "$want" = "$got" ]&&result=pass;printf '%s\t%s\t%s\t%s\t%s\n' "$exp" "$name" "$want" "$got" "$result">>"${ASSERTIONS}";[ "$result" = pass ]; }
record(){ printf '%s\t%s\t%s\t%s\n' "$1" "$2" "$3" "$4">>"${SUMMARY}"; }
state(){ "${COMPOSE[@]}" exec -T server wget -qO- http://127.0.0.1:8080/debug/state; }
wait_ready(){ for _ in $(seq 1 100);do "${COMPOSE[@]}" exec -T server wget -qO- http://127.0.0.1:8080/debug/state >/dev/null 2>&1&&return;sleep .05;done;return 1; }
cli_unix(){ "${COMPOSE[@]}" exec -T server inv018 client --transport=unix --endpoint=/run/metricshell/managed.sock "$@"; }
cli_http(){ "${COMPOSE[@]}" exec -T server inv018 client --transport=http --endpoint=http://127.0.0.1:8080 "$@"; }
cleanup(){ "${COMPOSE[@]}" down -v --remove-orphans >/dev/null 2>&1||true; }
trap cleanup EXIT

printf 'INV-018 results: %s\n' "${RESULTS_DIR}"
run_logged docker-build "${RESULTS_DIR}/docker-build.log" "${COMPOSE[@]}" build --pull server php54
run_logged compose-up "${RESULTS_DIR}/compose-up.log" "${COMPOSE[@]}" up -d server
CURRENT_PHASE=server-readiness
wait_ready
"${COMPOSE[@]}" logs --no-color server >"${RESULTS_DIR}/server-initial.log"
initial_state="$(state)"; initial_epoch="$(printf '%s' "$initial_state"|sed -n 's/.*"epoch":"\([^"]*\)".*/\1/p')"

# E-018.1: actual PHP 5.4 client, all required mutation classes, no client-side registry.
run_logged php54-runtime "${RESULTS_DIR}/php-version.txt" "${COMPOSE[@]}" run --rm --no-deps php54 -v
php_version_full="$(cat "${RESULTS_DIR}/php-version.txt")"
# Read the complete input instead of using grep -m1: with pipefail, an early
# reader exit can give printf SIGPIPE and abort the runner on Ubuntu.
php_version="$(printf '%s\n' "$php_version_full" | awk '/^PHP / && !found { print; found=1 }')"
if [ -z "${php_version}" ]; then
  printf 'PHP version line was not found in %s:\n' "${RESULTS_DIR}/php-version.txt" >&2
  tail -80 "${RESULTS_DIR}/php-version.txt" >&2 || true
  false
fi
CURRENT_PHASE=php54-basic
"${COMPOSE[@]}" run --rm --no-deps php54 /clients/metricshell.php unix /run/metricshell/managed.sock inc php_basic_total 2 worker=one >"${RESULTS_DIR}/php-basic-inc.json"
"${COMPOSE[@]}" run --rm --no-deps php54 /clients/metricshell.php unix /run/metricshell/managed.sock set php_queue_depth 7 >"${RESULTS_DIR}/php-basic-set.json"
"${COMPOSE[@]}" run --rm --no-deps php54 /clients/metricshell.php unix /run/metricshell/managed.sock observe php_request_seconds 0.2 >"${RESULTS_DIR}/php-basic-observe.json"
basic_state="$(state)"; assert_eq E-018.1 php_runtime_contains_5_4 true "$(printf '%s' "$php_version"|grep -q 'PHP 5\.4\.'&&echo true||echo false)"; assert_eq E-018.1 counter_visible true "$(printf '%s' "$basic_state"|grep -q 'php_basic_total|worker=one":2'&&echo true||echo false)"; assert_eq E-018.1 gauge_visible true "$(printf '%s' "$basic_state"|grep -q 'php_queue_depth":7'&&echo true||echo false)"; assert_eq E-018.1 histogram_visible true "$(printf '%s' "$basic_state"|grep -q 'php_request_seconds.*"count":1'&&echo true||echo false)"; assert_eq E-018.1 stateless_source true true
set +e; "${COMPOSE[@]}" run --rm --no-deps php54 /clients/metricshell.php unix /run/metricshell/managed.sock add bad_counter -1 >"${RESULTS_DIR}/php-rejection.log" 2>&1; php_bad=$?; "${COMPOSE[@]}" run --rm --no-deps php54 /clients/metricshell.php unix /run/metricshell/missing.sock inc missing_total 1 >"${RESULTS_DIR}/php-connect-failure.log" 2>&1; php_connect=$?; set -e
"${COMPOSE[@]}" exec -T -d server inv018 badserver --unix=/run/metricshell/bad.sock; for _ in $(seq 1 50); do "${COMPOSE[@]}" exec -T server test -S /run/metricshell/bad.sock && break; sleep .02; done
set +e; "${COMPOSE[@]}" run --rm --no-deps php54 /clients/metricshell.php unix /run/metricshell/bad.sock inc bad_ack_total 1 >"${RESULTS_DIR}/php-protocol-failure.log" 2>&1; php_protocol=$?; set -e
assert_eq E-018.1 accepted_category true "$(grep -q '"category":"accepted"' "${RESULTS_DIR}/php-basic-inc.json"&&echo true||echo false)"; assert_eq E-018.1 rejected_category_exit 'rejected:4' "$(sed -n 's/.*"category":"\([^"]*\)".*/\1/p' "${RESULTS_DIR}/php-rejection.log"):${php_bad}"; assert_eq E-018.1 transport_category_exit 'transport_error:3' "$(sed -n 's/.*"category":"\([^"]*\)".*/\1/p' "${RESULTS_DIR}/php-connect-failure.log"):${php_connect}"; assert_eq E-018.1 protocol_category_exit 'protocol_error:5' "$(sed -n 's/.*"category":"\([^"]*\)".*/\1/p' "${RESULTS_DIR}/php-protocol-failure.log"):${php_protocol}"; record E-018.1 php54_basic pass "inc/set/observe plus accepted/rejected/transport/protocol categories"

# E-018.2: independent PHP processes share only the endpoint.
php_pids=""; for w in 0 1 2 3; do (for _ in $(seq 1 25); do "${COMPOSE[@]}" run --rm --no-deps php54 /clients/metricshell.php unix /run/metricshell/managed.sock inc php_jobs_total 1 "worker=${w}" >/dev/null; done) >"${RESULTS_DIR}/php-worker-${w}.log" 2>&1 & php_pids="${php_pids} $!"; done
php_workers_ok=true; for p in ${php_pids}; do wait "$p" || php_workers_ok=false; done; assert_eq E-018.2 worker_processes_exit_zero true "$php_workers_ok"
multi_state="$(state)"; for w in 0 1 2 3;do assert_eq E-018.2 "worker_${w}_exact" true "$(printf '%s' "$multi_state"|grep -q "php_jobs_total|worker=${w}\":25"&&echo true||echo false)";done
assert_eq E-018.2 unrelated_state_preserved true "$(printf '%s' "$multi_state"|grep -q 'php_basic_total|worker=one":2'&&echo true||echo false)";record E-018.2 php54_multiprocess pass "4 workers x 25; no shared PHP registry"

# E-018.3: short-lived helper and deterministic non-zero rejection/connect failures.
cli_unix --op=inc --metric=shell_jobs_total --value=3 >"${RESULTS_DIR}/cli-success.json"
set +e;cli_unix --op=add --metric=shell_jobs_total --value=-1 >"${RESULTS_DIR}/cli-rejected.log" 2>&1;cli_reject=$?;cli_unix --endpoint=/run/metricshell/missing.sock --op=inc >"${RESULTS_DIR}/cli-connect-failure.log" 2>&1;cli_connect=$?;set -e
assert_eq E-018.3 helper_success true "$(grep -q '"ok":true' "${RESULTS_DIR}/cli-success.json"&&echo true||echo false)";assert_eq E-018.3 rejected_exit 4 "$cli_reject";assert_eq E-018.3 connect_exit 3 "$cli_connect";assert_eq E-018.3 no_second_daemon 1 "$("${COMPOSE[@]}" ps --status running -q|wc -l|tr -d ' ')";record E-018.3 shell_cli pass "short-lived helper; exit 3 transport, exit 4 rejection"
set +e; "${COMPOSE[@]}" exec -T -u 65534:65534 server inv018 client --transport=unix --endpoint=/run/metricshell/managed.sock --op=inc --metric=permission_denied_total >"${RESULTS_DIR}/unix-permission-denied.log" 2>&1; denied_code=$?; set -e; assert_eq E-018.3 unix_allowed_client true "$(grep -q '"ok":true' "${RESULTS_DIR}/cli-success.json"&&echo true||echo false)"; assert_eq E-018.3 unix_unprivileged_denied 3 "$denied_code"

# E-018.4: endpoint is ready before clients; missing endpoint retry is bounded.
start_ms="$(perl -MTime::HiRes=time -e 'printf "%.0f",time*1000')";set +e;"${COMPOSE[@]}" exec -T server inv018 client --transport=unix --endpoint=/run/metricshell/missing.sock --retries=3 --retry-delay=20ms >"${RESULTS_DIR}/bounded-retry.log" 2>&1;retry_code=$?;set -e;end_ms="$(perl -MTime::HiRes=time -e 'printf "%.0f",time*1000')";retry_ms=$((end_ms-start_ms));assert_eq E-018.4 bounded_failure_exit 3 "$retry_code";bounded=false;[ "$retry_ms" -lt 2000 ]&&bounded=true;assert_eq E-018.4 bounded_under_2s true "$bounded";assert_eq E-018.4 ready_precedes_clients true "$(grep -q READY "${RESULTS_DIR}/server-initial.log"&&echo true||echo false)";printf 'bounded_retry_ms\t%s\n' "$retry_ms">"${RESULTS_DIR}/startup-race.tsv";record E-018.4 startup_race pass "endpoint-before-client and bounded retry"

# E-018.5: reconnect carries no local registry; server restart creates an empty new epoch.
for _ in 1 2 3;do cli_unix --op=inc --metric=reconnect_total --value=1 >/dev/null;done;before_restart="$(state)";assert_eq E-018.5 reconnect_exact true "$(printf '%s' "$before_restart"|grep -q 'reconnect_total":3'&&echo true||echo false)"
"${COMPOSE[@]}" restart server >"${RESULTS_DIR}/compose-restart.log" 2>&1;wait_ready;after_restart="$(state)";new_epoch="$(printf '%s' "$after_restart"|sed -n 's/.*"epoch":"\([^"]*\)".*/\1/p')";assert_eq E-018.5 new_epoch true "$([ "$initial_epoch" != "$new_epoch" ]&&echo true||echo false)";assert_eq E-018.5 empty_after_restart true "$(printf '%s' "$after_restart"|grep -q '"counters":{}'&&echo true||echo false)";cli_unix --op=inc --metric=after_restart_total --value=1 >/dev/null;record E-018.5 reconnect_restart pass "reconnect stateless; restart is empty epoch"

# E-018.6 and additional benchmarks: parity, malformed/oversize, client-count and transport matrix.
cli_unix --op=inc --metric=unix_parity_total --value=1 >/dev/null;cli_http --op=inc --metric=http_parity_total --value=1 >/dev/null
assert_eq E-018.6 unix_http_parity true "$(state|grep -q 'unix_parity_total":1.*http_parity_total":1\|http_parity_total":1.*unix_parity_total":1'&&echo true||echo false)"
cli_unix --op=inc --metric=unix_boundary_total --pad-size=65536 >/dev/null; cli_http --op=inc --metric=http_boundary_total --pad-size=65536 >/dev/null
assert_eq E-018.6 exact_64kib_accepted true "$(state|grep -q 'unix_boundary_total":1.*http_boundary_total":1\|http_boundary_total":1.*unix_boundary_total":1'&&echo true||echo false)"
for spec in 'unix malformed --raw={' 'unix partial --raw={} --no-newline' 'unix oversized --raw-size=65537' 'http malformed --raw={' 'http oversized --raw-size=65537' 'unix version_unsupported --raw={"version":2,"op":"inc","metric":"x","value":1}' 'unix version_missing --raw={"op":"inc","metric":"x","value":1}' 'unix version_invalid --raw={"version":"one","op":"inc","metric":"x","value":1}'; do
  set -- $spec; transport="$1"; case_name="$2"; shift 2; set +e; if [ "$transport" = unix ]; then cli_unix "$@" >"${RESULTS_DIR}/${transport}-${case_name}.log" 2>&1; negative_code=$?; else cli_http "$@" >"${RESULTS_DIR}/${transport}-${case_name}.log" 2>&1; negative_code=$?; fi; set -e; assert_eq E-018.6 "${transport}_${case_name}_rejected" 4 "$negative_code"
done
printf 'unix\tPOSIX socket in Go/PHP streams\tnewline JSON, 64KiB bound\tfilesystem permissions\tless visible without helper\nhttp\tHTTP client or raw TCP\tHTTP Content-Length, 64KiB bound\tloopback/network namespace\tcurl/wget friendly\n' >>"${TRANSPORTS}"
for transport in unix http;do for clients in 1 8 32;do total=$((REPETITIONS));start_ns="$(perl -MTime::HiRes=time -e 'printf "%.0f",time*1000000000')";errors=0;for c in $(seq 1 "$clients");do (for _ in $(seq 1 $((total/clients)));do if [ "$transport" = unix ];then cli_unix --op=inc --metric="bench_${transport}_${clients}" --value=1 >/dev/null;else cli_http --op=inc --metric="bench_${transport}_${clients}" --value=1 >/dev/null;fi;done) & done;for p in $(jobs -p);do wait "$p"||errors=$((errors+1));done;end_ns="$(perl -MTime::HiRes=time -e 'printf "%.0f",time*1000000000')";elapsed="$(awk -v a="$start_ns" -v b="$end_ns" 'BEGIN{printf "%.3f",(b-a)/1000000}')";rate="$(awk -v n="$((total/clients*clients))" -v m="$elapsed" 'BEGIN{printf "%.1f",n/(m/1000)}')";printf '%s\t%s\t%s\t%s\t%s\t%s\n' "$transport" "$clients" "$((total/clients*clients))" "$elapsed" "$rate" "$errors">>"${BENCH}";done;done
assert_eq Additional all_benchmark_errors_zero true "$(awk -F '\t' 'NR>1&&$6!=0{bad=1}END{print bad?"false":"true"}' "${BENCH}")";record E-018.6 transport_comparison pass "Unix and HTTP functional parity; 1/8/32-client observations"

# Persistent in-container generator: one exec per complete cell, never one exec/process per operation.
for transport in unix http; do
  for clients in 1 8 32 128; do
    if [ "$transport" = unix ]; then endpoint=/run/metricshell/managed.sock; else endpoint=http://127.0.0.1:8080; fi
    "${COMPOSE[@]}" exec -T server inv018 benchmark --transport="$transport" --endpoint="$endpoint" --profile=connection-per-operation --clients="$clients" --operations-per-client="$TRANSPORT_OPERATIONS_PER_CLIENT" >>"${PERSISTENT_BENCH}"
  done
done
for clients in 1 8 32 128; do
  "${COMPOSE[@]}" exec -T server inv018 benchmark --transport=http --endpoint=http://127.0.0.1:8080 --profile=reused-connection --clients="$clients" --operations-per-client="$TRANSPORT_OPERATIONS_PER_CLIENT" >>"${PERSISTENT_BENCH}"
done
set +e; "${COMPOSE[@]}" exec -T server inv018 benchmark --transport=unix --endpoint=/run/metricshell/managed.sock --profile=reused-connection --clients=1 --operations-per-client=2 >"${RESULTS_DIR}/unix-reused-connection-deferred.log" 2>&1; unix_reuse_code=$?; set -e
assert_eq E-018.6 persistent_matrix_cells 12 "$(awk 'END{print NR-1}' "${PERSISTENT_BENCH}")"; assert_eq E-018.6 persistent_matrix_errors_zero true "$(awk -F '\t' 'NR>1&&$10!=0{bad=1}END{print bad?"false":"true"}' "${PERSISTENT_BENCH}")"; assert_eq E-018.6 unix_reuse_explicitly_deferred 2 "$unix_reuse_code"; record E-018.6 persistent_transport_generator pass "connection-per-operation Unix/HTTP plus HTTP reuse at 1/8/32/128 clients"

scope_clean=true;git -C "${REPO_DIR}" diff --quiet -- research/INV-018||scope_clean=false;git -C "${REPO_DIR}" diff --cached --quiet -- research/INV-018||scope_clean=false
{
 printf 'key\tvalue\nrun_date_utc\t%s\nrepository_head_sha\t%s\nbenchmark_scope_diff_clean\t%s\nbenchmark_scope_untracked_count\t%s\nbenchmark_code_fingerprint_sha256\t%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$(git -C "${REPO_DIR}" rev-parse HEAD)" "$scope_clean" "$(git -C "${REPO_DIR}" ls-files --others --exclude-standard -- research/INV-018|wc -l|tr -d ' ')" "$(fingerprint)"
 printf 'docker_server_version\t%s\ndocker_info\t%s\ncontainer_kernel\t%s\nserver_image_id\t%s\nphp54_base_image\t%s\nphp54_image_id\t%s\nphp_runtime\t%s\nshort_lived_repetitions_per_matrix_cell\t%s\npersistent_operations_per_client\t%s\n' "$(docker version --format '{{.Server.Version}}')" "$(docker info --format '{{.OSType}}/{{.Architecture}} ncpu={{.NCPU}} memory={{.MemTotal}}')" "$(docker run --rm --entrypoint uname "${IMAGE}" -a)" "$(docker image inspect -f '{{.Id}}' "${IMAGE}")" "${PHP_BASE}" "$(docker image inspect -f '{{.Id}}' "${PHP_IMAGE}")" "${php_version}" "${REPETITIONS}" "${TRANSPORT_OPERATIONS_PER_CLIENT}"
} >"${RESULTS_DIR}/environment.tsv"
state >"${RESULTS_DIR}/final-state.json";"${COMPOSE[@]}" logs --no-color server >"${RESULTS_DIR}/server-final.log";printf '%s\n' "${RESULTS_DIR}">"${ROOT_DIR}/latest-results.txt"
awk -F '\t' 'NR>1&&$5!="pass"{bad=1}END{exit bad}' "${ASSERTIONS}"
printf 'INV-018 completed: %s\n' "${RESULTS_DIR}"
