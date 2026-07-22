#!/usr/bin/env bash
set -euo pipefail
export LC_ALL=C

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(git -C "${ROOT_DIR}" rev-parse --show-toplevel)"
IMAGE="metricshell-inv004:prototype"
RESULTS_DIR="${ROOT_DIR}/results/$(date -u +%Y%m%dT%H%M%SZ)"
REPETITIONS="${INV004_REPEAT_COUNT:-30}"
mkdir -p "${RESULTS_DIR}"

hash_file() { if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | awk '{print $1}'; else shasum -a 256 "$1" | awk '{print $1}'; fi; }
hash_stdin() { if command -v sha256sum >/dev/null 2>&1; then sha256sum | awk '{print $1}'; else shasum -a 256 | awk '{print $1}'; fi; }
fingerprint() {
  { find "${ROOT_DIR}/prototype" -type f | LC_ALL=C sort; printf '%s\n' "${ROOT_DIR}/run-bench.sh"; } |
    while IFS= read -r file; do printf '%s  %s\n' "$(hash_file "${file}")" "${file#${ROOT_DIR}/}"; done | hash_stdin
}
cleanup() { docker ps -aq --filter 'name=^/inv004-' | xargs -r docker rm -f >/dev/null 2>&1 || true; }
trap cleanup EXIT
cleanup

docker build --pull=false -t "${IMAGE}" "${ROOT_DIR}/prototype" >"${RESULTS_DIR}/docker-build.log"
docker run --rm "${IMAGE}" --mode=scenarios >"${RESULTS_DIR}/semantics.tsv"

printf 'candidate\tproducers\tseries\tupdates\telapsed_us\tupdates_per_second\tchecksum\tallocated_bytes\tbytes_per_update\treconciliation_interval\treconciliations\treconciliation_share_percent\trepetition\n' >"${RESULTS_DIR}/benchmarks.tsv"
run_bench() {
  local candidate="$1" producers="$2" series="$3" updates="$4" interval="$5" repetition="$6"
  row="$(docker run --rm "${IMAGE}" --mode=benchmark --candidate="${candidate}" --producers="${producers}" --series="${series}" --updates="${updates}" --reconciliation-interval="${interval}")"
  printf '%s\t%s\n' "${row}" "${repetition}" >>"${RESULTS_DIR}/benchmarks.tsv"
}

# All candidate, producer-count and scale variants. Snapshot update count is lower because every update is a full registry.
for candidate in snapshot operations hybrid_amortized; do
  for producers in 1 4 16; do
    for series in 1 100 1000 10000; do
      if [ "${candidate}" = snapshot ]; then updates=100; else updates=100000; fi
      run_bench "${candidate}" "${producers}" "${series}" "${updates}" 1000 0
    done
  done
done

# Distribution for the representative 1-producer/100-series case.
for repetition in $(seq 1 "${REPETITIONS}"); do
  run_bench snapshot 1 100 100 1000 "${repetition}"
  run_bench operations 1 100 100000 1000 "${repetition}"
  run_bench hybrid_amortized 1 100 100000 1000 "${repetition}"
done

for interval in 100 1000 10000; do
  run_bench hybrid_amortized 4 1000 100000 "${interval}" 0
done

awk -F '\t' 'BEGIN{OFS="\t"; print "candidate","count","p50_updates_per_second","p95_updates_per_second","p99_updates_per_second","p50_bytes_per_update"}
  NR>1 && $13>0 {key=$1; rate[key,++n[key]]=$6; bytes[key,n[key]]=$9}
  END {for (k in n) {for(i=1;i<=n[k];i++){for(j=i+1;j<=n[k];j++)if(rate[k,i]>rate[k,j]){t=rate[k,i];rate[k,i]=rate[k,j];rate[k,j]=t}; for(j=i+1;j<=n[k];j++)if(bytes[k,i]>bytes[k,j]){t=bytes[k,i];bytes[k,i]=bytes[k,j];bytes[k,j]=t}}; p50=int((n[k]-1)*.50+1);p95=int((n[k]-1)*.95+1);p99=int((n[k]-1)*.99+1);print k,n[k],rate[k,p50],rate[k,p95],rate[k,p99],bytes[k,p50]}}' \
  "${RESULTS_DIR}/benchmarks.tsv" | sort >"${RESULTS_DIR}/benchmark-stats.tsv"

printf '%s\n' \
  'snapshot_drop_recovery pass' 'snapshot_producer_restart pass' 'snapshot_stale_removal pass' \
  'snapshot_multi_producer_counter pass' 'snapshot_duplicate_sequence pass' \
  'snapshot_counter_decrease_rejected pass' 'snapshot_type_change_rejected pass' \
  'snapshot_new_epoch_lower_counter pass' 'snapshot_histogram_schema_change_rejected pass' \
  'snapshot_histogram_cumulative_decrease_rejected pass' 'snapshot_histogram_new_epoch_reset pass' \
  'multi_histogram_compatible pass' 'multi_histogram_schema_mismatch_rejected pass' \
  'multi_gauge_without_policy_rejected pass' 'multi_gauge_sum_policy pass' \
  'multi_owner_type_conflict_rejected pass' 'absolute_drop_recovery pass' \
  'absolute_counter_decrease fail' 'absolute_multi_producer_collision fail' 'operations_drop fail' \
  'operations_gap_detected pass' 'operations_late_does_not_repair_gap pass' \
  'operations_duplicate_no_gap pass' 'operations_conflict_sequence_reusable pass' \
  'operations_receiver_restart fail' 'operations_multi_producer pass' \
  'operation_old_epoch_rejected pass' 'operation_new_epoch_requires_snapshot pass' \
  'operation_after_new_epoch_snapshot_accepted pass' \
  'hybrid_snapshot_clears_incomplete pass' 'hybrid_loss_reconciliation pass' \
  'hybrid_receiver_restart pass' 'hybrid_stale_removal pass' >"${RESULTS_DIR}/expected-results.txt"
printf 'assertion\texpected\tactual\tresult\n' >"${RESULTS_DIR}/assertions.tsv"
while read -r name expected; do
  actual="$(awk -F '\t' -v name="${name}" '$1==name{print $6}' "${RESULTS_DIR}/semantics.tsv")"
  matches="$(awk -F '\t' -v name="${name}" '$1==name{n++} END{print n+0}' "${RESULTS_DIR}/semantics.tsv")"
  result=fail; [ "${expected}" = "${actual}" ] && [ "${matches}" = 1 ] && result=pass
  printf '%s\t%s\t%s\t%s\n' "${name}" "${expected}" "${actual:-missing}" "${result}" >>"${RESULTS_DIR}/assertions.tsv"
done <"${RESULTS_DIR}/expected-results.txt"
expected_count="$(wc -l <"${RESULTS_DIR}/expected-results.txt" | tr -d ' ')"
total="$(awk -F '\t' 'NR>1{n++} END{print n+0}' "${RESULTS_DIR}/semantics.tsv")"
result=fail; [ "${expected_count}" = "${total}" ] && result=pass
printf 'scenario_set_cardinality\t%s\t%s\t%s\n' "${expected_count}" "${total}" "${result}" >>"${RESULTS_DIR}/assertions.tsv"
passed="$(awk -F '\t' 'NR>1&&$6=="pass"{n++} END{print n+0}' "${RESULTS_DIR}/semantics.tsv")"
failed="$((total-passed))"

scope_untracked="$(git -C "${REPO_DIR}" ls-files --others --exclude-standard -- research/INV-004/prototype research/INV-004/run-bench.sh | wc -l | tr -d ' ')"
scope_clean=false
if git -C "${REPO_DIR}" diff --quiet -- research/INV-004/prototype research/INV-004/run-bench.sh \
  && git -C "${REPO_DIR}" diff --cached --quiet -- research/INV-004/prototype research/INV-004/run-bench.sh; then scope_clean=true; fi
printf 'key\tvalue\n' >"${RESULTS_DIR}/environment.tsv"
printf 'run_date_utc\t%s\nrepository_head_sha\t%s\nbenchmark_code_fingerprint_sha256\t%s\nbenchmark_scope_diff_clean\t%s\nbenchmark_scope_untracked_count\t%s\ndocker_server_version\t%s\ndocker_platform\t%s\ncontainer_kernel\t%s\ncontainer_go_version\t%s\nimage_id\t%s\nimage_repo_digest\t%s\nrepeat_count\t%s\n' \
  "$(date -u +%FT%TZ)" "$(git -C "${REPO_DIR}" rev-parse HEAD)" "$(fingerprint)" "${scope_clean}" "${scope_untracked}" \
  "$(docker version --format '{{.Server.Version}}')" "$(docker info --format '{{.OSType}}/{{.Architecture}} ncpu={{.NCPU}} memory={{.MemTotal}}')" \
  "$(docker run --rm --entrypoint uname "${IMAGE}" -a)" "$(docker run --rm --entrypoint /usr/local/bin/inv004 "${IMAGE}" --help 2>&1 | head -1)" \
  "$(docker image inspect -f '{{.Id}}' "${IMAGE}")" "$(docker image inspect -f '{{join .RepoDigests ","}}' "${IMAGE}")" "${REPETITIONS}" >>"${RESULTS_DIR}/environment.tsv"

printf 'benchmark\tstatus\treason\nubuntu_same_fingerprint\tprepared\trun the same script unchanged and compare benchmark_code_fingerprint_sha256\nnative_linux\tnot_run\tcurrent daemon platform is recorded in environment.tsv\nkubernetes\tnot_applicable\tmetric-state semantics are transport/runtime independent in this experiment\ncrash_between_operations\tcovered\toperations_receiver_restart scenario\ndropped_reordered_duplicate_updates\tcovered\tsemantics.tsv\nproducer_restart_stale_conflict_multi_producer\tcovered\tsemantics.tsv\nscale_and_distribution\tcovered\tbenchmarks.tsv and benchmark-stats.tsv\n' >"${RESULTS_DIR}/coverage.tsv"
printf 'metric\tvalue\nscenarios\t%s\nconfirmations\t%s\ncounterexamples\t%s\nassertions_failed\t%s\nbenchmark_rows\t%s\n' "${total}" "${passed}" "${failed}" \
  "$(awk -F '\t' 'NR>1&&$4!="pass"{n++} END{print n+0}' "${RESULTS_DIR}/assertions.tsv")" \
  "$(awk -F '\t' 'NR>1{n++} END{print n+0}' "${RESULTS_DIR}/benchmarks.tsv")" >"${RESULTS_DIR}/summary.tsv"

printf '%s\n' "${RESULTS_DIR}" >"${ROOT_DIR}/latest-results.txt"
awk -F '\t' 'NR>1&&$4!="pass"{bad=1} END{exit bad}' "${RESULTS_DIR}/assertions.tsv"
echo "Results: ${RESULTS_DIR}"
