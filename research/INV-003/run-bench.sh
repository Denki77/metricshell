#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(cd "${ROOT_DIR}/../.." && pwd)"
IMAGE="metricshell-inv003:prototype"
STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
RESULTS_DIR="${ROOT_DIR}/results/${STAMP}"
REPETITIONS="${INV003_REPETITIONS:-30}"
mkdir -p "${RESULTS_DIR}"

cleanup() { docker ps -aq --filter 'name=inv003-' | xargs -r docker rm -f >/dev/null 2>&1 || true; }
trap cleanup EXIT
cleanup
docker build -t "${IMAGE}" "${ROOT_DIR}/prototype" >"${RESULTS_DIR}/docker-build.log"

fingerprint() {
  # Relative canonical paths keep benchmark identity independent of checkout location and host OS.
  (
    cd "${ROOT_DIR}"
    { find prototype -type f -print0 | LC_ALL=C sort -z | xargs -0 shasum -a 256; shasum -a 256 run-bench.sh; } |
      shasum -a 256 | awk '{print $1}'
  )
}
event_value() { sed -n "s/.*EVENT name=$2 .* $3=\([^ ]*\).*/\1/p" "$1" | tail -1; }
event_count() { grep -c "EVENT name=$2\( \|$\)" "$1" 2>/dev/null || true; }
wait_ready() { for _ in $(seq 1 200); do docker logs "$1" 2>&1 | grep -q WORKLOAD_READY && return; sleep .02; done; return 1; }
now_ms() { perl -MTime::HiRes=time -e 'printf "%.0f\n", time()*1000'; }

printf 'case\ttotal_s\tworkload_budget_ms\treserve_ms\tmode\texit_code\tforced\tshutdown_ms\tfinalize_ms\thttp_result\tresult\n' >"${RESULTS_DIR}/shutdown-grid.tsv"
printf 'case\tassertion\texpected\tactual\tresult\n' >"${RESULTS_DIR}/shutdown-grid-assertions.tsv"
run_grid_case() {
  local total="$1" workload="$2" reserve="$3" mode="$4" delay workload_arg reserve_arg budget_ms reserve_ms name code log forced elapsed finalized http http_elapsed expected_exit expected_forced expired_count complete_count finalize_count http_count case_result assertion_result
  workload_arg="${workload}s"; reserve_arg="${reserve}s"; budget_ms="$((workload*1000))"; reserve_ms="$((reserve*1000))"
  if [ "$total" = 1 ]; then workload_arg=750ms; reserve_arg=250ms; budget_ms=750; reserve_ms=250; fi
  case "$mode" in immediate) delay=0ms;; just_before) delay="$((budget_ms-100))ms";; after_deadline) delay="$((budget_ms+200))ms";; never) delay=-1ms;; esac
  name="inv003-grid-${total}-${mode}"; log="${RESULTS_DIR}/${name}.log"
  docker run -d --name "$name" "$IMAGE" --total-grace="${total}s" --policy=explicit --workload-timeout="$workload_arg" --reserve="$reserve_arg" --finalize-delay=20ms --http-timeout=250ms -- /usr/local/bin/workload --term-delay="$delay" >/dev/null
  wait_ready "$name"; docker kill --signal TERM "$name" >/dev/null; code="$(docker wait "$name")"; docker logs "$name" >"$log" 2>&1
  forced="$(event_value "$log" workload_exited forced)"; elapsed="$(event_value "$log" shutdown_complete total_elapsed_ms)"; finalized="$(event_value "$log" finalization_complete spent_ms)"; http="$(event_value "$log" http_shutdown result)"; http_elapsed="$(event_value "$log" http_shutdown elapsed_ms)"
  expired_count="$(event_count "$log" workload_budget_expired)"; complete_count="$(event_count "$log" shutdown_complete)"; finalize_count="$(event_count "$log" finalization_complete)"; http_count="$(event_count "$log" http_shutdown)"
  expected_exit=0; expected_forced=false; expected_expired=0
  if [ "$mode" = after_deadline ] || [ "$mode" = never ]; then expected_exit=137; expected_forced=true; expected_expired=1; fi
  : >"${RESULTS_DIR}/${name}.assertions"; case_result=pass
  assert_eq(){ assertion_result=fail; [ "$2" = "$3" ] && assertion_result=pass; [ "$assertion_result" = pass ] || case_result=fail; printf '%s\t%s\t%s\t%s\t%s\n' "${total}_${mode}" "$1" "$2" "${3:-missing}" "$assertion_result" >>"${RESULTS_DIR}/${name}.assertions"; }
  assert_num(){ assertion_result=fail; awk -v got="${3:-nan}" -v limit="$2" "BEGIN{exit !($4)}" && assertion_result=pass; [ "$assertion_result" = pass ] || case_result=fail; printf '%s\t%s\t%s\t%s\t%s\n' "${total}_${mode}" "$1" "$2" "${3:-missing}" "$assertion_result" >>"${RESULTS_DIR}/${name}.assertions"; }
  assert_eq exit_code "$expected_exit" "$code"; assert_eq forced "$expected_forced" "$forced"; assert_eq workload_budget_expired_count "$expected_expired" "$expired_count"; assert_eq shutdown_complete_count 1 "$complete_count"; assert_eq finalization_complete_count 1 "$finalize_count"; assert_eq http_shutdown_count 1 "$http_count"; assert_eq http_result drained "$http"
  assert_num shutdown_before_total "$((total*1000))" "$elapsed" 'got < limit'
  assert_num finalization_within_cap 20 "$finalized" 'got <= limit'
  assert_num http_within_cap 250 "$http_elapsed" 'got <= limit'
  if [ "$expected_forced" = true ]; then assert_num workload_not_killed_before_budget "$budget_ms" "$elapsed" 'got >= limit'; fi
  printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n' "${total}_${mode}" "$total" "$budget_ms" "$reserve_ms" "$mode" "$code" "${forced:-missing}" "${elapsed:-missing}" "${finalized:-missing}" "${http:-missing}" "$case_result" >"${RESULTS_DIR}/${name}.row"
  docker rm "$name" >/dev/null
}

# The 20 mandatory cases run concurrently; wall time is approximately the largest 60 s window.
for spec in '1 0 1' '5 4 1' '10 9 1' '30 28 2' '60 58 2'; do
  read -r total workload reserve <<<"$spec"
  # 1 s needs a non-zero useful workload slice while retaining a measurable reserve.
  for mode in immediate just_before after_deadline never; do run_grid_case "$total" "$workload" "$reserve" "$mode" & done
done
wait
for total in 1 5 10 30 60; do for mode in immediate just_before after_deadline never; do cat "${RESULTS_DIR}/inv003-grid-${total}-${mode}.row" >>"${RESULTS_DIR}/shutdown-grid.tsv"; cat "${RESULTS_DIR}/inv003-grid-${total}-${mode}.assertions" >>"${RESULTS_DIR}/shutdown-grid-assertions.tsv"; done; done

# Policy arithmetic and invalid/overflow behavior.
printf 'case\tpolicy\ttotal_ms\tworkload_ms\treserve_ms\texit_code\tresult\n' >"${RESULTS_DIR}/policy-comparison.tsv"
for spec in 'fixed fixed 5000 1000 4000' 'percentage percentage 5000 1000 4000' 'explicit explicit 5000 1000 4000'; do
  read -r label policy total reserve expected <<<"$spec"; name="inv003-policy-$label"
  args=(--total-grace="${total}ms" --policy="$policy" --reserve="${reserve}ms" --workload-timeout="${expected}ms" --workload-ratio=.8 --finalize-delay=5ms --http-timeout=5ms)
  docker run -d --name "$name" "$IMAGE" "${args[@]}" -- /usr/local/bin/workload --term-delay=0 >/dev/null; wait_ready "$name"; docker kill --signal TERM "$name" >/dev/null; code="$(docker wait "$name")"; docker logs "$name" >"${RESULTS_DIR}/${name}.log" 2>&1
  got="$(event_value "${RESULTS_DIR}/${name}.log" shutdown_started workload_budget_ms)"; result=fail; [ "$got" = "$expected" ] && result=pass
  printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\n' "$label" "$policy" "$total" "${got:-missing}" "$reserve" "$code" "$result" >>"${RESULTS_DIR}/policy-comparison.tsv"; docker rm "$name" >/dev/null
done
name=inv003-policy-overflow; docker run -d --name "$name" "$IMAGE" --total-grace=1s --policy=explicit --workload-timeout=900ms --reserve=200ms -- /usr/local/bin/workload --term-delay=0 >/dev/null; wait_ready "$name"; docker kill --signal TERM "$name" >/dev/null; code="$(docker wait "$name")"; docker logs "$name" >"${RESULTS_DIR}/${name}.log" 2>&1; result=fail; [ "$code" = 64 ] && grep -q 'name=budget_rejected' "${RESULTS_DIR}/${name}.log" && result=pass; printf 'overflow\texplicit\t1000\trejected\t200\t%s\t%s\n' "$code" "$result" >>"${RESULTS_DIR}/policy-comparison.tsv"; docker rm "$name" >/dev/null

# True absolute deadline: remaining time is sampled when TERM is received.
printf 'case\tdeadline_offset_ms\tpre_term_delay_ms\treserve_ms\tremaining_ms\tworkload_budget_ms\texit_code\tresult\n' >"${RESULTS_DIR}/absolute-deadline.tsv"
for spec in 'full 5000 0 1000' 'partially_spent 5000 1000 1000' 'nearly_expired 500 250 1000' 'already_expired -100 0 1000' 'reserve_exceeds_remaining 700 200 1000'; do
  read -r label offset pre reserve <<<"$spec"; deadline_file="${RESULTS_DIR}/deadline-${label}.txt"; name="inv003-deadline-$label"; docker run -d --name "$name" "$IMAGE" --policy=deadline --shutdown-deadline-file=/tmp/shutdown-deadline --reserve="${reserve}ms" --finalize-delay=0 --http-timeout=0 -- /usr/local/bin/workload --term-delay=0 >/dev/null; wait_ready "$name"; deadline="$(( $(now_ms) + offset ))"; printf '%s\n' "$deadline" >"$deadline_file"; docker cp "$deadline_file" "$name:/tmp/shutdown-deadline" >/dev/null; [ "$pre" = 0 ] || sleep "$(awk -v ms="$pre" 'BEGIN{print ms/1000}')"; docker kill --signal TERM "$name" >/dev/null; code="$(docker wait "$name")"; log="${RESULTS_DIR}/${name}.log"; docker logs "$name" >"$log" 2>&1; remaining="$(event_value "$log" shutdown_started remaining_total_ms)"; budget="$(event_value "$log" shutdown_started workload_budget_ms)"; result=pass
  [ -n "$remaining" ] && [ -n "$budget" ] || result=fail
  awk -v r="${remaining:-999999}" -v o="$offset" -v p="$pre" 'BEGIN{max=o-p; if(max<0)max=0; exit !(r <= max && r >= 0)}' || result=fail
  expected_budget=$(( remaining > reserve ? remaining-reserve : 0 )); [ "$budget" = "$expected_budget" ] || result=fail
  printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n' "$label" "$offset" "$pre" "$reserve" "${remaining:-missing}" "${budget:-missing}" "$code" "$result" >>"${RESULTS_DIR}/absolute-deadline.tsv"; docker rm "$name" >/dev/null
done

# Repetition/latency distribution with an immediate workload.
printf 'iteration\tshutdown_ms\texit_code\tresult\n' >"${RESULTS_DIR}/repetitions.tsv"; : >"${RESULTS_DIR}/latency.tmp"
for i in $(seq 1 "$REPETITIONS"); do name="inv003-repeat-$i"; docker run -d --name "$name" "$IMAGE" --total-grace=1s --policy=fixed --reserve=500ms --finalize-delay=5ms --http-timeout=5ms -- /usr/local/bin/workload --term-delay=0 >/dev/null; wait_ready "$name"; docker kill --signal TERM "$name" >/dev/null; code="$(docker wait "$name")"; docker logs "$name" >"${RESULTS_DIR}/${name}.log" 2>&1; ms="$(event_value "${RESULTS_DIR}/${name}.log" shutdown_complete total_elapsed_ms)"; result=fail; [ -n "$ms" ] && awk -v x="$ms" 'BEGIN{exit !(x<500)}' && result=pass; printf '%s\t%s\t%s\t%s\n' "$i" "$ms" "$code" "$result" >>"${RESULTS_DIR}/repetitions.tsv"; printf '%s\n' "$ms" >>"${RESULTS_DIR}/latency.tmp"; docker rm "$name" >/dev/null; done
percentile(){ sort -n "$1" | awk -v p="$2" '{a[NR]=$1} END{i=int((NR-1)*p+1); print a[i]}'; }
printf 'count\tpassed\tp50_ms\tp95_ms\tp99_ms\tmin_ms\tmax_ms\n' >"${RESULTS_DIR}/latency-stats.tsv"
printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\n' "$REPETITIONS" "$(awk -F '\t' 'NR>1&&$4=="pass"{n++}END{print n+0}' "${RESULTS_DIR}/repetitions.tsv")" "$(percentile "${RESULTS_DIR}/latency.tmp" .5)" "$(percentile "${RESULTS_DIR}/latency.tmp" .95)" "$(percentile "${RESULTS_DIR}/latency.tmp" .99)" "$(sort -n "${RESULTS_DIR}/latency.tmp"|head -1)" "$(sort -n "${RESULTS_DIR}/latency.tmp"|tail -1)" >>"${RESULTS_DIR}/latency-stats.tsv"

# Active scrape draining: one request fits; one exceeds its remaining reserve.
printf 'case\tscrape_delay_ms\thttp_result\thttp_elapsed_ms\tpost_term_http_code\tnormal_post_term_scrape\tshutdown_ms\tresult\n' >"${RESULTS_DIR}/http-drain.tsv"
for spec in 'fits 100 drained' 'overruns 1500 timeout'; do read -r label delay expected <<<"$spec"; name="inv003-http-$label"; docker run -d --name "$name" -p 127.0.0.1::9090 "$IMAGE" --total-grace=2s --policy=explicit --workload-timeout=500ms --reserve=1500ms --finalize-delay=20ms --http-timeout=700ms -- /usr/local/bin/workload --term-delay=400ms >/dev/null; wait_ready "$name"; port="$(docker port "$name" 9090/tcp|sed 's/.*://')"; curl -sS "http://127.0.0.1:$port/metrics?delay=${delay}ms" >"${RESULTS_DIR}/http-${label}.body" 2>/dev/null & curlpid=$!; sleep .05; docker kill --signal TERM "$name" >/dev/null; for _ in $(seq 1 100); do docker logs "$name" 2>&1 | grep -q 'name=shutdown_started' && break; sleep .01; done; post_code="$(curl -sS -o "${RESULTS_DIR}/http-${label}-post.body" -w '%{http_code}' "http://127.0.0.1:$port/metrics" 2>/dev/null || true)"; docker wait "$name" >/dev/null || true; wait "$curlpid" || true; docker logs "$name" >"${RESULTS_DIR}/${name}.log" 2>&1; got="$(event_value "${RESULTS_DIR}/${name}.log" http_shutdown result)"; http_elapsed="$(event_value "${RESULTS_DIR}/${name}.log" http_shutdown elapsed_ms)"; ms="$(event_value "${RESULTS_DIR}/${name}.log" shutdown_complete total_elapsed_ms)"; normal_post=false; [ "$post_code" = 200 ] && normal_post=true; result=fail; [ "$got" = "$expected" ] && [ "$normal_post" = false ] && awk -v x="$ms" 'BEGIN{exit !(x<2000)}' && awk -v x="$http_elapsed" 'BEGIN{exit !(x<=710)}' && result=pass; printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n' "$label" "$delay" "${got:-missing}" "${http_elapsed:-missing}" "${post_code:-000}" "$normal_post" "${ms:-missing}" "$result" >>"${RESULTS_DIR}/http-drain.tsv"; docker rm "$name" >/dev/null; done

# Docker is the independent deadline enforcer here. A completed marker proves the runtime did not SIGKILL MetricShell.
printf 'external_timeout_ms\tinternal_total_grace_ms\tshutdown_complete_elapsed_ms\tdocker_stop_elapsed_ms\tsafety_margin_ms\tcontainer_exit\tshutdown_complete_count\tdocker_oom_killed\tresult\n' >"${RESULTS_DIR}/external-deadline.tsv"
for spec in '1000 750 250' '5000 4000 500'; do read -r external internal reserve <<<"$spec"; name="inv003-external-$external"; docker run -d --name "$name" "$IMAGE" --total-grace="${internal}ms" --policy=fixed --reserve="${reserve}ms" --finalize-delay=20ms --http-timeout=100ms -- /usr/local/bin/workload --term-delay=-1ms >/dev/null; wait_ready "$name"; host_start="$(now_ms)"; docker stop --time "$((external/1000))" "$name" >/dev/null; host_end="$(now_ms)"; host_elapsed=$((host_end-host_start)); code="$(docker inspect -f '{{.State.ExitCode}}' "$name")"; oom="$(docker inspect -f '{{.State.OOMKilled}}' "$name")"; log="${RESULTS_DIR}/${name}.log"; docker logs "$name" >"$log" 2>&1; complete_count="$(event_count "$log" shutdown_complete)"; shutdown_elapsed="$(event_value "$log" shutdown_complete total_elapsed_ms)"; margin="$(awk -v ext="$external" -v got="${shutdown_elapsed:-999999}" 'BEGIN{printf "%.3f", ext-got}')"; result=fail; [ "$complete_count" = 1 ] && [ "$oom" = false ] && [ "$code" = 137 ] && awk -v got="$shutdown_elapsed" -v internal="$internal" -v external="$external" 'BEGIN{exit !(got < internal && internal < external)}' && result=pass; printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n' "$external" "$internal" "${shutdown_elapsed:-missing}" "$host_elapsed" "$margin" "$code" "$complete_count" "$oom" "$result" >>"${RESULTS_DIR}/external-deadline.tsv"; docker rm "$name" >/dev/null; done

# Post-exit waiting is intentionally skipped once external termination has started.
printf 'assertion\texpected\tactual\tresult\n' >"${RESULTS_DIR}/termination-policy.tsv"
log="${RESULTS_DIR}/inv003-external-5000.log"; post_events="$(grep -c 'name=post_exit_' "$log" || true)"; result=fail; [ "$post_events" = 0 ] && result=pass; printf 'post_exit_wait_during_termination\t0\t%s\t%s\n' "$post_events" "$result" >>"${RESULTS_DIR}/termination-policy.tsv"

printf 'benchmark\tstatus\tevidence_or_reason\n' >"${RESULTS_DIR}/coverage.tsv"
for item in mandatory_shutdown_grid grid_explicit_assertions policy_comparison absolute_deadline budget_overflow repeated_latency http_admission_and_active_drain forced_kill finalization_timing external_docker_deadline termination_post_exit_policy; do printf '%s\tcovered\tgenerated TSV and raw logs\n' "$item" >>"${RESULTS_DIR}/coverage.tsv"; done
printf 'native_ubuntu\tprepared_not_run\trun the same run-bench.sh command; current daemon fingerprint is recorded\n' >>"${RESULTS_DIR}/coverage.tsv"
printf 'kubernetes\tnot_run\tINV-003 asks for Docker; Kubernetes deadline injection is runtime-specific\n' >>"${RESULTS_DIR}/coverage.tsv"

{
  printf 'key\tvalue\n'; printf 'run_date_utc\t%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"; printf 'host_os\t%s\n' "$(uname -s)"; printf 'repository_head_sha\t%s\n' "$(git -C "$REPO_DIR" rev-parse HEAD 2>/dev/null || printf unknown)"; printf 'benchmark_code_fingerprint_sha256\t%s\n' "$(fingerprint)"; printf 'docker_server_version\t%s\n' "$(docker version --format '{{.Server.Version}}')"; printf 'docker_info\t%s\n' "$(docker info --format '{{.OSType}}/{{.Architecture}} kernel={{.KernelVersion}} ncpu={{.NCPU}} memory={{.MemTotal}} driver={{.Driver}}')"; printf 'container_kernel\t%s\n' "$(docker run --rm --entrypoint uname "$IMAGE" -a)"; printf 'image_id\t%s\n' "$(docker image inspect "$IMAGE" --format '{{.Id}}')"; printf 'image_repo_digest\t%s\n' "$(docker image inspect "$IMAGE" --format '{{join .RepoDigests ","}}')"; printf 'repeat_count\t%s\n' "$REPETITIONS";
} >"${RESULTS_DIR}/environment.tsv"
rm -f "${RESULTS_DIR}/latency.tmp" "${RESULTS_DIR}"/*.row
printf '%s\n' "$RESULTS_DIR" >"${ROOT_DIR}/latest-results.txt"
awk -F '\t' 'NR>1&&$NF!="pass"{bad=1}END{exit bad}' "${RESULTS_DIR}/shutdown-grid.tsv"
awk -F '\t' 'NR>1&&$NF!="pass"{bad=1}END{exit bad}' "${RESULTS_DIR}/shutdown-grid-assertions.tsv"
awk -F '\t' 'NR>1&&$NF!="pass"{bad=1}END{exit bad}' "${RESULTS_DIR}/policy-comparison.tsv"
awk -F '\t' 'NR>1&&$NF!="pass"{bad=1}END{exit bad}' "${RESULTS_DIR}/absolute-deadline.tsv"
awk -F '\t' 'NR>1&&$NF!="pass"{bad=1}END{exit bad}' "${RESULTS_DIR}/repetitions.tsv"
awk -F '\t' 'NR>1&&$NF!="pass"{bad=1}END{exit bad}' "${RESULTS_DIR}/http-drain.tsv"
awk -F '\t' 'NR>1&&$NF!="pass"{bad=1}END{exit bad}' "${RESULTS_DIR}/external-deadline.tsv"
awk -F '\t' 'NR>1&&$NF!="pass"{bad=1}END{exit bad}' "${RESULTS_DIR}/termination-policy.tsv"
echo "results: ${RESULTS_DIR}"
