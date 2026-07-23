#!/usr/bin/env bash
set -euo pipefail
export LC_ALL=C

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(git -C "${ROOT_DIR}" rev-parse --show-toplevel)"
IMAGE="metricshell-inv006:prototype"
RESULTS_DIR="${ROOT_DIR}/results/$(date -u +%Y%m%dT%H%M%SZ)"
REPETITIONS="${INV006_REPETITIONS:-3}"
REQUIRE_REAL_OVERFLOW="${INV006_REQUIRE_REAL_OVERFLOW:-0}"
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
  } | while IFS= read -r path; do printf '%s  %s\n' "$(hash_file "${path}")" "${path#${ROOT_DIR}/}"; done | hash_stdin
}
scope_diff_clean() {
  if git -C "${REPO_DIR}" diff --quiet -- research/INV-006/prototype research/INV-006/run-bench.sh \
    && git -C "${REPO_DIR}" diff --cached --quiet -- research/INV-006/prototype research/INV-006/run-bench.sh; then printf true; else printf false; fi
}
scope_untracked_count() {
  git -C "${REPO_DIR}" ls-files --others --exclude-standard -- research/INV-006/prototype research/INV-006/run-bench.sh | wc -l | tr -d ' '
}
cleanup() {
  docker ps -aq --filter 'name=^/inv006-' | xargs -r docker rm -f >/dev/null 2>&1 || true
  docker volume rm inv006-data >/dev/null 2>&1 || true
}
trap cleanup EXIT
cleanup
docker build -t "${IMAGE}" "${ROOT_DIR}/prototype" >"${RESULTS_DIR}/docker-build.log"

run_one() {
  local filesystem="$1" name="$2"; shift 2
  case "${filesystem}" in
    layer)
      docker run --name "inv006-${name}" "${IMAGE}" "$@"
      ;;
    tmpfs)
      docker run --name "inv006-${name}" --tmpfs /data:rw,size=256m,mode=0755 "${IMAGE}" "$@"
      ;;
    volume)
      docker volume create inv006-data >/dev/null
      docker run --name "inv006-${name}" -v inv006-data:/data "${IMAGE}" "$@"
      ;;
  esac
  docker cp "inv006-${name}:/results/." "${RESULTS_DIR}"
  docker rm "inv006-${name}" >/dev/null
  if [ "${filesystem}" = volume ]; then
    docker volume rm inv006-data >/dev/null
  fi
}

printf 'filesystem\tstrategy\tinterval_ms\tcase\texpected\tactual\tresult\n' >"${RESULTS_DIR}/correctness.tsv"
for fs in layer tmpfs volume; do
  for spec in poll:10ms poll:100ms poll:1s inotify:1s hybrid:100ms hybrid:1s; do
    strategy="${spec%%:*}"; interval="${spec#*:}"; key="${fs}-${strategy}-${interval}"
    run_one "${fs}" "${key}" --mode=correctness --strategy="${strategy}" --interval="${interval}" --output="/results/${key}-correctness.tsv"
    awk -v fs="${fs}" -v s="${strategy}" -v i="${interval}" 'BEGIN{OFS="\t"} NR>1{print fs,s,i,$0}' \
      "${RESULTS_DIR}/${key}-correctness.tsv" >>"${RESULTS_DIR}/correctness.tsv"
  done
done

printf 'filesystem\tstrategy\tinitial_observed\tevent_dropped\tfinal_observed\texpected_final_observed\tresult\n' >"${RESULTS_DIR}/lost-event.tsv"
for fs in layer tmpfs volume; do
  for strategy in inotify hybrid; do
    key="${fs}-${strategy}-lost-event"
    run_one "${fs}" "${key}" --mode=lost-event --strategy="${strategy}" --interval=250ms --file-bytes=4096 --output="/results/${key}.tsv"
    awk -v fs="${fs}" 'BEGIN{OFS="\t"} NR==2{print fs,$0}' "${RESULTS_DIR}/${key}.tsv" >>"${RESULTS_DIR}/lost-event.tsv"
  done
done

printf 'filesystem\tstrategy\tupdates\tfinal_observed\tqueue_overflows\tresult\n' >"${RESULTS_DIR}/overflow.tsv"
for fs in layer tmpfs volume; do
  for strategy in inotify hybrid; do
    key="${fs}-${strategy}-overflow"
    run_one "${fs}" "${key}" --mode=burst --strategy="${strategy}" --interval=1s --initial-read-pause=750ms --updates=20000 --file-bytes=128 --output="/results/${key}.tsv"
    awk -v fs="${fs}" -v s="${strategy}" 'BEGIN{OFS="\t"} NR==2{result=($2=="true")?"pass":"fail"; print fs,s,$1,$2,$9,result}' \
      "${RESULTS_DIR}/${key}.tsv" >>"${RESULTS_DIR}/overflow.tsv"
  done
done

printf 'filesystem\tstrategy\tinterval_ms\tfile_bytes\trepetition\tupdates\tobserved\tmissed\tp50_ms\tp95_ms\tp99_ms\tmax_ms\twall_ms\tcpu_ms\tcpu_percent\treads\tparse_errors\twatch_invalidations\tqueue_overflows\n' >"${RESULTS_DIR}/performance.tsv"
for fs in layer tmpfs volume; do
  for spec in poll:10ms poll:100ms inotify:1s hybrid:100ms hybrid:1s; do
    strategy="${spec%%:*}"; interval="${spec#*:}"
    for size in 128 4096 1048576; do
      updates=30
      [ "${size}" = 1048576 ] && updates=5
      for repetition in $(seq 1 "${REPETITIONS}"); do
        key="${fs}-${strategy}-${interval}-${size}-${repetition}"
        run_one "${fs}" "${key}" --mode=performance --strategy="${strategy}" --interval="${interval}" --updates="${updates}" --file-bytes="${size}" --output="/results/${key}.tsv"
        awk -v fs="${fs}" -v s="${strategy}" -v i="${interval}" -v z="${size}" -v r="${repetition}" 'BEGIN{OFS="\t"} NR==2{print fs,s,i,z,r,$0}' \
          "${RESULTS_DIR}/${key}.tsv" >>"${RESULTS_DIR}/performance.tsv"
      done
    done
  done
done

printf 'filesystem\tstrategy\tinterval_ms\tupdates\tfinal_observed\tproduce_ms\ttotal_ms\tcpu_ms\treads\tparse_errors\twatch_invalidations\tqueue_overflows\tresult\n' >"${RESULTS_DIR}/burst.tsv"
for fs in layer tmpfs volume; do
  for spec in poll:100ms inotify:1s hybrid:100ms hybrid:1s; do
    strategy="${spec%%:*}"; interval="${spec#*:}"; key="${fs}-${strategy}-${interval}-burst"
    run_one "${fs}" "${key}" --mode=burst --strategy="${strategy}" --interval="${interval}" --updates=10000 --file-bytes=128 --output="/results/${key}.tsv"
    awk -v fs="${fs}" -v s="${strategy}" -v i="${interval}" 'BEGIN{OFS="\t"} NR==2{print fs,s,i,$0}' "${RESULTS_DIR}/${key}.tsv" >>"${RESULTS_DIR}/burst.tsv"
  done
done

printf 'filesystem\tstrategy\tinterval_ms\tduration_ms\tcpu_ms\tcpu_percent\treads\tparse_errors\n' >"${RESULTS_DIR}/idle.tsv"
for fs in layer tmpfs volume; do
  for spec in poll:10ms poll:100ms poll:1s inotify:1s hybrid:100ms hybrid:1s; do
    strategy="${spec%%:*}"; interval="${spec#*:}"; key="${fs}-${strategy}-${interval}-idle"
    run_one "${fs}" "${key}" --mode=idle --strategy="${strategy}" --interval="${interval}" --duration=5s --output="/results/${key}.tsv"
    awk -v fs="${fs}" -v s="${strategy}" -v i="${interval}" 'BEGIN{OFS="\t"} NR==2{print fs,s,i,$0}' "${RESULTS_DIR}/${key}.tsv" >>"${RESULTS_DIR}/idle.tsv"
  done
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
  printf 'require_real_overflow\t%s\n' "${REQUIRE_REAL_OVERFLOW}"
} >"${RESULTS_DIR}/environment.tsv"

awk -F '\t' 'NR>1{n++; if($7=="pass")p++} END{printf "metric\tvalue\ncorrectness_cases\t%d\ncorrectness_passed\t%d\ncorrectness_failed\t%d\n",n,p,n-p}' \
  "${RESULTS_DIR}/correctness.tsv" >"${RESULTS_DIR}/summary.tsv"
awk -F '\t' 'NR>1&&$8>0{bad++} END{printf "performance_rows_with_misses\t%d\n",bad+0}' "${RESULTS_DIR}/performance.tsv" >>"${RESULTS_DIR}/summary.tsv"
awk -F '\t' 'NR>1&&$13!="pass"{bad++} END{printf "burst_rows_failed\t%d\n",bad+0}' "${RESULTS_DIR}/burst.tsv" >>"${RESULTS_DIR}/summary.tsv"
awk -F '\t' 'NR>1&&$6!="pass"{bad++} END{printf "overflow_rows_failed\t%d\n",bad+0}' "${RESULTS_DIR}/overflow.tsv" >>"${RESULTS_DIR}/summary.tsv"
awk -F '\t' 'NR>1&&$5>0{n++} END{printf "overflow_rows_observed\t%d\n",n+0}' "${RESULTS_DIR}/overflow.tsv" >>"${RESULTS_DIR}/summary.tsv"
awk -F '\t' 'NR>1&&$7!="pass"{bad++} END{printf "lost_event_rows_failed\t%d\n",bad+0}' "${RESULTS_DIR}/lost-event.tsv" >>"${RESULTS_DIR}/summary.tsv"
printf '%s\n' "${RESULTS_DIR}" >"${ROOT_DIR}/latest-results.txt"
awk -F '\t' 'NR>1&&$7!="pass"{bad=1} END{exit bad}' "${RESULTS_DIR}/correctness.tsv"
awk -F '\t' 'NR>1&&$13!="pass"{bad=1} END{exit bad}' "${RESULTS_DIR}/burst.tsv"
awk -F '\t' 'NR>1&&$6!="pass"{bad=1} END{exit bad}' "${RESULTS_DIR}/overflow.tsv"
awk -F '\t' 'NR>1&&$7!="pass"{bad=1} END{exit bad}' "${RESULTS_DIR}/lost-event.tsv"
if [ "${REQUIRE_REAL_OVERFLOW}" = 1 ]; then
  for strategy in inotify hybrid; do
    awk -F '\t' -v s="${strategy}" 'NR>1&&$2==s&&$5>0{found=1} END{exit !found}' "${RESULTS_DIR}/overflow.tsv"
  done
fi
