#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
IMAGE="metricshell-inv001:prototype"
RESULTS_DIR="${ROOT_DIR}/results/$(date -u +%Y%m%dT%H%M%SZ)"
SUMMARY="${RESULTS_DIR}/summary.tsv"
SIGNAL_TO_EXIT_LATENCY="${RESULTS_DIR}/signal-to-exit-latency.tsv"
SIGNAL_TO_EXIT_LATENCY_STATS="${RESULTS_DIR}/signal-to-exit-latency-stats.tsv"
RESOURCES="${RESULTS_DIR}/resources.tsv"
SCRAPES="${RESULTS_DIR}/scrapes.tsv"
ZOMBIES="${RESULTS_DIR}/zombies.tsv"
ENVIRONMENT="${RESULTS_DIR}/environment.tsv"
EVENTS="${RESULTS_DIR}/events.jsonl"
SIGNAL_DELIVERY="${RESULTS_DIR}/signal-delivery.tsv"
ASSERTIONS="${RESULTS_DIR}/assertions.tsv"
REPEAT_COUNT="${INV001_REPEAT_COUNT:-30}"
RUN_HEAVY="${INV001_RUN_HEAVY:-0}"

mkdir -p "${RESULTS_DIR}"

docker build -t "${IMAGE}" "${ROOT_DIR}/prototype" >"${RESULTS_DIR}/docker-build.log"

printf 'case\tinit\tprocess_group\tsubreaper\tsignal\texpected\texit_code\tduration_ms\tresult\n' >"${SUMMARY}"
printf 'case\titeration\tsignal_to_exit_ms\n' >"${SIGNAL_TO_EXIT_LATENCY}"
printf 'case\tcount\tp50_ms\tp95_ms\tp99_ms\tmin_ms\tmax_ms\n' >"${SIGNAL_TO_EXIT_LATENCY_STATS}"
printf 'case\tphase\tpid\tclk_tck\tcpu_ticks_delta\tcpu_pct_one_core\trss_kb\thwm_kb\n' >"${RESOURCES}"
printf 'case\titeration\thttp_code\tvalue\n' >"${SCRAPES}"
printf 'case\titeration\tzombies\n' >"${ZOMBIES}"
printf 'key\tvalue\n' >"${ENVIRONMENT}"
: >"${EVENTS}"
printf 'case\tsubject\texpected_event\tobserved\tcount\n' >"${SIGNAL_DELIVERY}"
printf 'case\tassertion\texpected\tactual\tresult\n' >"${ASSERTIONS}"

cleanup() {
  docker ps -aq --filter 'name=^/inv001-' | xargs -r docker rm -f >/dev/null 2>&1 || true
}
trap cleanup EXIT
cleanup

hash_file() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  else
    shasum -a 256 "$1" | awk '{print $1}'
  fi
}

hash_stdin() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum | awk '{print $1}'
  else
    shasum -a 256 | awk '{print $1}'
  fi
}

benchmark_code_fingerprint() {
  {
    find "${ROOT_DIR}/prototype" -type f | LC_ALL=C sort
    printf '%s\n' "${ROOT_DIR}/run-bench.sh"
  } | while IFS= read -r path; do
    printf '%s  %s\n' "$(hash_file "${path}")" "${path#${ROOT_DIR}/}"
  done | hash_stdin
}

write_environment_metadata() {
  {
    printf 'run_date_utc\t%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    printf 'repository_head_sha\t%s\n' "$(git rev-parse HEAD 2>/dev/null || printf 'unknown')"
    if git diff --quiet -- research/INV-001/prototype research/INV-001/run-bench.sh; then
      printf 'benchmark_scope_diff_clean\ttrue\n'
    else
      printf 'benchmark_scope_diff_clean\tfalse\n'
    fi
    printf 'benchmark_scope_untracked_count\t%s\n' "$(git ls-files --others --exclude-standard -- research/INV-001/prototype research/INV-001/run-bench.sh | wc -l | tr -d ' ')"
    printf 'benchmark_code_fingerprint_sha256\t%s\n' "$(benchmark_code_fingerprint)"
    printf 'host_uname_sanitized\t%s\n' "$(uname -a | awk '{$2="<hostname>"; print}')"
    printf 'host_kernel_name\t%s\n' "$(uname -s)"
    printf 'host_kernel_release\t%s\n' "$(uname -r)"
    printf 'host_kernel_version\t%s\n' "$(uname -v)"
    printf 'host_architecture\t%s\n' "$(uname -m)"
    printf 'docker_client_version\t%s\n' "$(docker version --format '{{.Client.Version}}' 2>/dev/null || printf 'unknown')"
    printf 'docker_server_version\t%s\n' "$(docker version --format '{{.Server.Version}}' 2>/dev/null || printf 'unknown')"
    printf 'docker_os_type\t%s\n' "$(docker info --format '{{.OSType}}' 2>/dev/null || printf 'unknown')"
    printf 'docker_architecture\t%s\n' "$(docker info --format '{{.Architecture}}' 2>/dev/null || printf 'unknown')"
    printf 'docker_cgroup_version\t%s\n' "$(docker info --format '{{.CgroupVersion}}' 2>/dev/null || printf 'unknown')"
    printf 'docker_security_options\t%s\n' "$(docker info --format '{{json .SecurityOptions}}' 2>/dev/null || printf 'unknown')"
    printf 'docker_ncpu\t%s\n' "$(docker info --format '{{.NCPU}}' 2>/dev/null || printf 'unknown')"
    printf 'docker_mem_total_bytes\t%s\n' "$(docker info --format '{{.MemTotal}}' 2>/dev/null || printf 'unknown')"
    printf 'container_architecture\t%s\n' "$(docker run --rm --entrypoint /bin/uname "${IMAGE}" -m 2>/dev/null || printf 'unknown')"
    printf 'container_kernel\t%s\n' "$(docker run --rm --entrypoint /bin/uname "${IMAGE}" -a 2>/dev/null || printf 'unknown')"
    printf 'container_nproc\t%s\n' "$(docker run --rm --entrypoint /usr/bin/nproc "${IMAGE}" 2>/dev/null || printf 'unknown')"
    printf 'container_mem_total_kb\t%s\n' "$(docker run --rm --entrypoint /bin/sh "${IMAGE}" -c "awk '/MemTotal:/ {print \$2}' /proc/meminfo" 2>/dev/null || printf 'unknown')"
    printf 'container_clk_tck\t%s\n' "$(docker run --rm --entrypoint /usr/bin/getconf "${IMAGE}" CLK_TCK 2>/dev/null || printf 'unknown')"
  } >>"${ENVIRONMENT}"
}

write_environment_metadata

docker_args_for_init() {
  local init="$1"
  if [ "${init}" = "docker-init" ]; then
    printf '%s\n' "--init"
  fi
}

metricshell_args() {
  local process_group="$1"
  local subreaper="$2"
  local post_exit="$3"
  local shutdown_grace="$4"
  local args=("--http=:9090")
  if [ "${process_group}" = "pg" ]; then
    args+=("--process-group")
  fi
  if [ "${subreaper}" = "subreaper" ]; then
    args+=("--subreaper")
  fi
  if [ "${post_exit}" != "none" ]; then
    args+=("--post-exit=${post_exit}")
  fi
  if [ "${shutdown_grace}" != "none" ]; then
    args+=("--shutdown-grace=${shutdown_grace}")
  fi
  printf '%s\n' "${args[@]}"
}

wait_for_log_event() {
  local container="$1"
  local event="$2"
  local timeout_seconds="${3:-10}"
  local deadline=$((SECONDS + timeout_seconds))
  while (( SECONDS < deadline )); do
    if docker logs "${container}" 2>&1 | grep -q "\"event\":\"${event}\""; then
      return 0
    fi
    sleep 0.05
  done
  return 1
}

readiness_event_for_case() {
  local case_name="$1"
  case "${case_name}" in
    signal_direct_pid1|signal_direct_pg|signal_direct_pg_init|signal_direct_pg_init_subreaper|repeat_signal_direct_pg_*)
      printf 'workload_ready\n'
      ;;
    signal_shell_script_pg|sigkill)
      printf 'workload_ready\n'
      ;;
    signal_shell_no_pg|signal_shell_pg|signal_shell_pg_init|signal_bash_pg)
      printf 'grandchild_ready\n'
      ;;
    shutdown_grace_forced_kill)
      printf 'stubborn_workload_ready\n'
      ;;
    *)
      printf 'workload_started\n'
      ;;
  esac
}

add_assertion() {
  local case_name="$1"
  local assertion="$2"
  local expected="$3"
  local actual="$4"
  local result="fail"
  if [ "${expected}" = "${actual}" ]; then
    result="pass"
  fi
  printf '%s\t%s\t%s\t%s\t%s\n' "${case_name}" "${assertion}" "${expected}" "${actual}" "${result}" >>"${ASSERTIONS}"
}

event_count() {
  local log_file="$1"
  local event="$2"
  grep -c "\"event\":\"${event}\"" "${log_file}" 2>/dev/null || true
}

event_present_value() {
  local log_file="$1"
  local event="$2"
  local count
  count="$(event_count "${log_file}" "${event}")"
  if [ "${count}" -gt 0 ]; then
    printf 'true\n'
  else
    printf 'false\n'
  fi
}

descendant_reaped_exit_value() {
  local log_file="$1"
  local exit_code="$2"
  if grep "\"event\":\"descendant_reaped\"" "${log_file}" 2>/dev/null | grep -q "\"exit_code\":${exit_code}"; then
    printf 'true\n'
  else
    printf 'false\n'
  fi
}

descendant_reaped_value() {
  local log_file="$1"
  event_present_value "${log_file}" "descendant_reaped"
}

case_has_failed_assertions() {
  local case_name="$1"
  awk -F '\t' -v c="${case_name}" 'NR > 1 && $1 == c && $5 != "pass" { failed = 1 } END { exit failed ? 0 : 1 }' "${ASSERTIONS}"
}

append_case_assertions() {
  local name="$1"
  local log_file="$2"
  local expected_exit="$3"
  local actual_exit="$4"
  local ready_expected="$5"
  local ready_actual="$6"
  local metrics_file="$7"
  local metrics_status_file="$8"

  add_assertion "${name}" "exit_code" "${expected_exit}" "${actual_exit}"
  if [ "${ready_expected}" != "none" ]; then
    add_assertion "${name}" "startup_ready" "${ready_expected}" "${ready_actual}"
  fi

  case "${name}" in
    signal_direct_pg|signal_direct_pid1|signal_direct_pg_init|signal_direct_pg_init_subreaper|repeat_signal_direct_pg_*)
      add_assertion "${name}" "workload_term" "true" "$(event_present_value "${log_file}" "workload_term")"
      ;;
    signal_shell_script_pg)
      add_assertion "${name}" "workload_term" "true" "$(event_present_value "${log_file}" "workload_term")"
      ;;
    signal_shell_pg|signal_shell_pg_init|signal_bash_pg)
      add_assertion "${name}" "shell_parent_term" "true" "$(event_present_value "${log_file}" "shell_parent_term")"
      add_assertion "${name}" "grandchild_term" "true" "$(event_present_value "${log_file}" "grandchild_term")"
      ;;
    double_fork_pid1_no_subreaper)
      add_assertion "${name}" "descendant_reaped_exit23" "true" "$(descendant_reaped_exit_value "${log_file}" "23")"
      ;;
    double_fork_no_subreaper_init)
      add_assertion "${name}" "descendant_reaped" "false" "$(descendant_reaped_value "${log_file}")"
      ;;
    double_fork_subreaper_init)
      add_assertion "${name}" "descendant_reaped_exit23" "true" "$(descendant_reaped_exit_value "${log_file}" "23")"
      ;;
    shutdown_grace_forced_kill)
      add_assertion "${name}" "stubborn_signal_ignored" "true" "$(event_present_value "${log_file}" "stubborn_signal_ignored")"
      add_assertion "${name}" "force_kill_sent" "true" "$(event_present_value "${log_file}" "force_kill_sent")"
      ;;
    post_exit_survival)
      local http_code metric_value
      http_code="$(cat "${metrics_status_file}" 2>/dev/null || printf '000')"
      metric_value="$(awk '/metricshell_workload_exit_code/ && $1 !~ /^#/ {print $2}' "${metrics_file}" 2>/dev/null | tail -n 1)"
      add_assertion "${name}" "metrics_http_code" "200" "${http_code}"
      add_assertion "${name}" "metrics_workload_exit_code" "17" "${metric_value:-missing}"
      ;;
  esac
}

append_events_from_log() {
  local case_name="$1"
  local log_file="$2"
  perl -MJSON::PP -ne '
    BEGIN { $case = $ARGV[0]; shift @ARGV; }
    chomp;
    next unless /^\s*(\{.*\})\s*$/;
    print encode_json({case => $case, event => decode_json($1)}) . "\n";
  ' "${case_name}" "${log_file}" >>"${EVENTS}" 2>/dev/null || true
}

append_signal_delivery_row() {
  local case_name="$1"
  local subject="$2"
  local expected_event="$3"
  local log_file="$4"
  local count observed
  count="$(grep -c "\"event\":\"${expected_event}\"" "${log_file}" 2>/dev/null || true)"
  observed="false"
  if [ "${count}" -gt 0 ]; then
    observed="true"
  fi
  printf '%s\t%s\t%s\t%s\t%s\n' "${case_name}" "${subject}" "${expected_event}" "${observed}" "${count}" >>"${SIGNAL_DELIVERY}"
}

append_signal_delivery_from_log() {
  local case_name="$1"
  local log_file="$2"
  case "${case_name}" in
    signal_direct_pid1|signal_direct_pg|signal_direct_pg_init|signal_direct_pg_init_subreaper|repeat_signal_direct_pg_*)
      append_signal_delivery_row "${case_name}" "direct_workload" "workload_term" "${log_file}"
      ;;
    signal_shell_script_pg)
      append_signal_delivery_row "${case_name}" "shell_script_workload" "workload_term" "${log_file}"
      ;;
    signal_shell_pg|signal_shell_pg_init|signal_bash_pg)
      append_signal_delivery_row "${case_name}" "shell_parent" "shell_parent_term" "${log_file}"
      append_signal_delivery_row "${case_name}" "grandchild" "grandchild_term" "${log_file}"
      ;;
    shutdown_grace_forced_kill)
      append_signal_delivery_row "${case_name}" "stubborn_workload" "stubborn_signal_ignored" "${log_file}"
      ;;
  esac
}

append_signal_to_exit_latency_from_log() {
  local case_name="$1"
  local iteration="$2"
  local log_file="$3"
  local latency_case="${case_name}"
  if [[ "${case_name}" =~ ^(.+)_[0-9]+$ ]]; then
    latency_case="${BASH_REMATCH[1]}"
  fi
  perl -ne '
    if (/"event":"signal_forwarded"/ && /"mono_ns":([0-9]+)/) { $s=$1 }
    if (/"event":"workload_exited"/ && /"mono_ns":([0-9]+)/) { $e=$1 }
    END {
      if (defined $s && defined $e && $e >= $s) {
        printf "%.3f\n", ($e - $s) / 1000000;
      }
    }
  ' "${log_file}" | while IFS= read -r ms; do
    printf '%s\t%s\t%s\n' "${latency_case}" "${iteration}" "${ms}" >>"${SIGNAL_TO_EXIT_LATENCY}"
  done
}

write_signal_to_exit_latency_stats() {
  perl -F'\t' -ane '
    next if $. == 1;
    chomp $F[2];
    push @{$v{$F[0]}}, 0 + $F[2];
    END {
      for my $key (sort keys %v) {
        my @a = sort { $a <=> $b } @{$v{$key}};
        my $n = @a;
        my $p50 = $a[int(($n - 1) * 0.50)];
        my $p95 = $a[int(($n - 1) * 0.95)];
        my $p99 = $a[int(($n - 1) * 0.99)];
        printf "%s\t%d\t%.3f\t%.3f\t%.3f\t%.3f\t%.3f\n", $key, $n, $p50, $p95, $p99, $a[0], $a[-1];
      }
    }
  ' "${SIGNAL_TO_EXIT_LATENCY}" >>"${SIGNAL_TO_EXIT_LATENCY_STATS}"
}

sample_metricshell_resource() {
  local cname="$1"
  local case_name="$2"
  local phase="$3"
  local interval="${4:-0.25}"
  local pid
  pid="$(docker exec "${cname}" sh -c 'cat /tmp/metricshell.pid' 2>/dev/null || true)"
  if [ -z "${pid}" ]; then
    printf '%s\t%s\tunknown\tunknown\t0\t0\t0\t0\n' "${case_name}" "${phase}" >>"${RESOURCES}"
    return
  fi
  local first second rss hwm clk_tck cpu_pct
  first="$(docker exec "${cname}" sh -c "awk '{print \$14+\$15}' /proc/${pid}/stat" 2>/dev/null || printf '0')"
  sleep "${interval}"
  second="$(docker exec "${cname}" sh -c "awk '{print \$14+\$15}' /proc/${pid}/stat" 2>/dev/null || printf '0')"
  rss="$(docker exec "${cname}" sh -c "awk '/VmRSS:/ {print \$2}' /proc/${pid}/status" 2>/dev/null || printf '0')"
  hwm="$(docker exec "${cname}" sh -c "awk '/VmHWM:/ {print \$2}' /proc/${pid}/status" 2>/dev/null || printf '0')"
  clk_tck="$(docker exec "${cname}" getconf CLK_TCK 2>/dev/null || printf '100')"
  cpu_pct="$(awk -v a="${first}" -v b="${second}" -v hz="${clk_tck}" -v sec="${interval}" 'BEGIN { if (sec > 0 && hz > 0) printf "%.3f", ((b-a)/(hz*sec))*100; else printf "0.000" }')"
  printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n' "${case_name}" "${phase}" "${pid}" "${clk_tck}" "$((second - first))" "${cpu_pct}" "${rss:-0}" "${hwm:-0}" >>"${RESOURCES}"
}

run_case() {
  local name="$1"
  local init="$2"
  local process_group="$3"
  local subreaper="$4"
  local signal_name="$5"
  local expected="$6"
  local post_exit="$7"
  local shutdown_grace="$8"
  shift 8

  local cname="inv001-${name}"
  local log_file="${RESULTS_DIR}/${name}.log"
  local inspect_file="${RESULTS_DIR}/${name}.inspect.json"
  local metrics_file="${RESULTS_DIR}/${name}.metrics.txt"
  local metrics_status_file="${RESULTS_DIR}/${name}.metrics.status"

  local docker_cmd=(docker run -d --name "${cname}")
  while IFS= read -r arg; do
    [ -n "${arg}" ] && docker_cmd+=("${arg}")
  done < <(docker_args_for_init "${init}")
  docker_cmd+=("${IMAGE}")
  while IFS= read -r arg; do
    [ -n "${arg}" ] && docker_cmd+=("${arg}")
  done < <(metricshell_args "${process_group}" "${subreaper}" "${post_exit}" "${shutdown_grace}")

  if [ "${name}" = "internal_failure" ]; then
    docker_cmd+=(--internal-fail)
  else
    docker_cmd+=(-- "$@")
  fi

  local start_ns
  start_ns="$(date +%s%N)"
  "${docker_cmd[@]}" >"${RESULTS_DIR}/${name}.cid"
  local ready_expected="none"
  local ready_actual="none"

  if [ "${signal_name}" != "none" ]; then
    local ready_event
    ready_event="$(readiness_event_for_case "${name}")"
    ready_expected="true"
    if wait_for_log_event "${cname}" "${ready_event}" 10; then
      ready_actual="true"
    else
      ready_actual="false"
    fi
    docker kill --signal "${signal_name}" "${cname}" >/dev/null
  fi

  if [ "${post_exit}" != "none" ]; then
    if wait_for_log_event "${cname}" "post_exit_begin" 10; then
      if docker exec "${cname}" wget -q -O - http://127.0.0.1:9090/metrics >"${metrics_file}"; then
        printf '200\n' >"${metrics_status_file}"
      else
        printf '000\n' >"${metrics_status_file}"
      fi
    else
      printf '000\n' >"${metrics_status_file}"
    fi
  fi

  set +e
  docker wait "${cname}" >"${RESULTS_DIR}/${name}.exit"
  local wait_status=$?
  set -e
  local end_ns
  end_ns="$(date +%s%N)"

  docker logs "${cname}" >"${log_file}" 2>&1 || true
  docker inspect "${cname}" >"${inspect_file}" || true
  local exit_code
  exit_code="$(cat "${RESULTS_DIR}/${name}.exit" 2>/dev/null || printf 'docker-wait-failed-%s' "${wait_status}")"
  local duration_ms=$(( (end_ns - start_ns) / 1000000 ))
  append_events_from_log "${name}" "${log_file}"
  append_signal_delivery_from_log "${name}" "${log_file}"
  append_signal_to_exit_latency_from_log "${name}" "1" "${log_file}"
  append_case_assertions "${name}" "${log_file}" "${expected}" "${exit_code}" "${ready_expected}" "${ready_actual}" "${metrics_file}" "${metrics_status_file}"
  local result="pass"
  if case_has_failed_assertions "${name}"; then
    result="fail"
  fi
  printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n' "${name}" "${init}" "${process_group}" "${subreaper}" "${signal_name}" "${expected}" "${exit_code}" "${duration_ms}" "${result}" >>"${SUMMARY}"
  docker rm -f "${cname}" >/dev/null 2>&1 || true
}

run_resource_case() {
  local name="$1"
  local workload="$2"
  shift 2
  local cname="inv001-${name}"
  docker run -d --name "${cname}" "${IMAGE}" --http=:9090 --post-exit=3s -- "${workload}" "$@" >"${RESULTS_DIR}/${name}.cid"
  sleep 0.5
  sample_metricshell_resource "${cname}" "${name}" "active" "0.5"
  docker wait "${cname}" >"${RESULTS_DIR}/${name}.exit" || true &
  sleep 1
  sample_metricshell_resource "${cname}" "${name}" "post_exit" "0.5"
  wait || true
  docker logs "${cname}" >"${RESULTS_DIR}/${name}.log" 2>&1 || true
  append_events_from_log "${name}" "${RESULTS_DIR}/${name}.log"
  docker inspect "${cname}" >"${RESULTS_DIR}/${name}.inspect.json" || true
  docker rm -f "${cname}" >/dev/null 2>&1 || true
}

run_external_scrape_case() {
  local name="external_scrape_post_exit"
  local cname="inv001-${name}"
  docker run -d --name "${cname}" -p 127.0.0.1::9090 "${IMAGE}" --http=:9090 --post-exit=4s -- /prototype/workloads/exit-code.sh 17 >"${RESULTS_DIR}/${name}.cid"
  sleep 0.75
  local port
  port="$(docker port "${cname}" 9090/tcp | sed 's/.*://')"
  for i in 1 2 3 4 5; do
    local body http_code value
    body="$(curl -sS -w '\n%{http_code}' "http://127.0.0.1:${port}/metrics" || printf '\n000')"
    http_code="$(printf '%s\n' "${body}" | tail -n 1)"
    value="$(printf '%s\n' "${body}" | awk '/metricshell_workload_exit_code/ && $1 !~ /^#/ {print $2}' | tail -n 1)"
    printf '%s\t%s\t%s\t%s\n' "${name}" "${i}" "${http_code}" "${value:-missing}" >>"${SCRAPES}"
    sleep 0.75
  done
  docker wait "${cname}" >"${RESULTS_DIR}/${name}.exit" || true
  docker logs "${cname}" >"${RESULTS_DIR}/${name}.log" 2>&1 || true
  append_events_from_log "${name}" "${RESULTS_DIR}/${name}.log"
  docker inspect "${cname}" >"${RESULTS_DIR}/${name}.inspect.json" || true
  docker rm -f "${cname}" >/dev/null 2>&1 || true
}

run_zombie_scan_case() {
  local name="zombie_scan_child_churn_10000"
  local cname="inv001-${name}"
  docker run -d --name "${cname}" "${IMAGE}" --http=:9090 -- /prototype/workloads/child-spawner.sh 10000 100 0.02 >"${RESULTS_DIR}/${name}.cid"
  local i
  for i in $(seq 1 30); do
    local running zombies
    running="$(docker inspect -f '{{.State.Running}}' "${cname}" 2>/dev/null || printf 'false')"
    if [ "${running}" != "true" ]; then
      printf '%s\t%s\tcontainer-exited\n' "${name}" "${i}" >>"${ZOMBIES}"
      break
    fi
    zombies="$(docker exec "${cname}" sh -c 'z=0; for f in /proc/[0-9]*/status; do [ -r "$f" ] || continue; if grep -q "^State:.*Z" "$f" 2>/dev/null; then z=$((z + 1)); fi; done; printf "%s\n" "$z"' 2>/dev/null || true)"
    if ! [[ "${zombies}" =~ ^[0-9]+$ ]]; then
      running="$(docker inspect -f '{{.State.Running}}' "${cname}" 2>/dev/null || printf 'false')"
      if [ "${running}" = "true" ]; then
        zombies="scan-failed"
      else
        zombies="container-exited"
      fi
    fi
    printf '%s\t%s\t%s\n' "${name}" "${i}" "${zombies}" >>"${ZOMBIES}"
    sleep 0.05
  done
  docker wait "${cname}" >"${RESULTS_DIR}/${name}.exit" || true
  docker logs "${cname}" >"${RESULTS_DIR}/${name}.log" 2>&1 || true
  append_events_from_log "${name}" "${RESULTS_DIR}/${name}.log"
  docker inspect "${cname}" >"${RESULTS_DIR}/${name}.inspect.json" || true
  docker rm -f "${cname}" >/dev/null 2>&1 || true
}

run_repeat_latency_case() {
  local base_name="$1"
  local init="$2"
  local process_group="$3"
  local subreaper="$4"
  local signal_name="$5"
  local expected="$6"
  shift 6

  local i
  for i in $(seq 1 "${REPEAT_COUNT}"); do
    run_case "${base_name}_${i}" "${init}" "${process_group}" "${subreaper}" "${signal_name}" "${expected}" none none "$@"
  done
}

run_case signal_direct_pid1 no-init no-pg no-subreaper TERM 0 none none /usr/local/bin/workload-trap
run_case signal_direct_pg no-init pg no-subreaper TERM 0 none none /usr/local/bin/workload-trap
run_case signal_direct_pg_init docker-init pg no-subreaper TERM 0 none none /usr/local/bin/workload-trap
run_case signal_direct_pg_init_subreaper docker-init pg subreaper TERM 0 none none /usr/local/bin/workload-trap
run_case signal_shell_script_pg no-init pg no-subreaper TERM 143 none none /prototype/workloads/trap-direct.sh
run_case signal_shell_no_pg no-init no-pg no-subreaper TERM 143 none none /bin/sh -c /prototype/workloads/shell-grandchild.sh
run_case signal_shell_pg no-init pg no-subreaper TERM 143 none none /bin/sh -c /prototype/workloads/shell-grandchild.sh
run_case signal_shell_pg_init docker-init pg no-subreaper TERM 143 none none /bin/sh -c /prototype/workloads/shell-grandchild.sh
run_case signal_bash_pg no-init pg no-subreaper TERM 143 none none /bin/bash -c /prototype/workloads/shell-grandchild.sh
run_case reap_short_children_200 no-init no-pg no-subreaper none 0 none none /prototype/workloads/child-spawner.sh 200
run_case child_churn_1000 no-init no-pg no-subreaper none 0 none none /prototype/workloads/child-spawner.sh 1000
run_case child_churn_10000 no-init no-pg no-subreaper none 0 none none /prototype/workloads/child-spawner.sh 10000
if [ "${RUN_HEAVY}" = "1" ]; then
  run_case child_churn_100000 no-init no-pg no-subreaper none 0 none none /prototype/workloads/child-spawner.sh 100000
fi
run_case double_fork_pid1_no_subreaper no-init no-pg no-subreaper none 0 none none /prototype/workloads/double-fork.sh
run_case double_fork_no_subreaper_init docker-init no-pg no-subreaper none 0 none none /prototype/workloads/double-fork.sh
run_case double_fork_subreaper_init docker-init no-pg subreaper none 0 none none /prototype/workloads/double-fork.sh
run_case exit_zero no-init no-pg no-subreaper none 0 none none /prototype/workloads/exit-code.sh 0
run_case exit_17 no-init no-pg no-subreaper none 17 none none /prototype/workloads/exit-code.sh 17
run_case sigkill no-init no-pg no-subreaper KILL 137 none none /prototype/workloads/trap-direct.sh
run_case shutdown_grace_forced_kill no-init no-pg no-subreaper TERM 137 none 500ms /usr/local/bin/workload-stubborn
run_case start_failure no-init no-pg no-subreaper none 127 none none /prototype/workloads/missing.sh
run_case internal_failure no-init no-pg no-subreaper none 70 none none /prototype/workloads/exit-code.sh 0
run_case post_exit_survival no-init no-pg no-subreaper none 17 3s none /prototype/workloads/exit-code.sh 17

run_repeat_latency_case repeat_signal_direct_pg no-init pg no-subreaper TERM 0 /usr/local/bin/workload-trap
run_resource_case resource_idle_post_exit /prototype/workloads/exit-code.sh 0
run_resource_case resource_active_cpu /usr/local/bin/workload-cpu 3
run_resource_case resource_child_churn /prototype/workloads/child-spawner.sh 1000
run_external_scrape_case
run_zombie_scan_case
write_signal_to_exit_latency_stats

printf '%s\n' "${RESULTS_DIR}" >"${ROOT_DIR}/latest-results.txt"
cat "${SUMMARY}"
printf '\nSignal-to-exit latency stats:\n'
cat "${SIGNAL_TO_EXIT_LATENCY_STATS}"
printf '\nResource samples:\n'
cat "${RESOURCES}"
printf '\nScrape samples:\n'
cat "${SCRAPES}"
printf '\nZombie samples:\n'
cat "${ZOMBIES}"
printf '\nSignal delivery:\n'
cat "${SIGNAL_DELIVERY}"
printf '\nAssertions:\n'
cat "${ASSERTIONS}"
printf '\nEnvironment:\n'
cat "${ENVIRONMENT}"
