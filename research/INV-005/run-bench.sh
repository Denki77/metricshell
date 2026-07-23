#!/usr/bin/env bash
set -euo pipefail
export LC_ALL=C
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(git -C "${ROOT_DIR}" rev-parse --show-toplevel)"
IMAGE="metricshell-inv005:prototype"
STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
RESULTS_DIR="${ROOT_DIR}/results/${STAMP}"
STAMP_ID="$(printf '%s' "${STAMP}" | tr '[:upper:]' '[:lower:]')"
RESULTS_VOLUME="inv005-results-${STAMP_ID}"
EXPORT_CONTAINER="inv005-export-${STAMP_ID}"
rm -rf "${ROOT_DIR}/results"
mkdir -p "${RESULTS_DIR}"
hash_file(){ if command -v sha256sum >/dev/null;then sha256sum "$1"|awk '{print $1}';else shasum -a 256 "$1"|awk '{print $1}';fi; }
fingerprint(){
  {
    find "${ROOT_DIR}/prototype" -type f -print | LC_ALL=C sort
    printf '%s\n' "${ROOT_DIR}/run-bench.sh"
  } | while read -r f;do printf '%s  %s\n' "$(hash_file "$f")" "${f#${ROOT_DIR}/}";done |
    if command -v sha256sum >/dev/null;then sha256sum|awk '{print $1}';else shasum -a 256|awk '{print $1}';fi
}
scope_diff_clean(){
  if git -C "${REPO_DIR}" diff --quiet -- research/INV-005/prototype research/INV-005/run-bench.sh &&
     git -C "${REPO_DIR}" diff --cached --quiet -- research/INV-005/prototype research/INV-005/run-bench.sh;then printf true;else printf false;fi
}
scope_untracked_count(){ git -C "${REPO_DIR}" ls-files --others --exclude-standard -- research/INV-005/prototype research/INV-005/run-bench.sh | wc -l | tr -d ' '; }
cleanup(){
  docker rm -f inv005-php-unix inv005-php-http "${EXPORT_CONTAINER}" >/dev/null 2>&1 || true
  docker volume rm "${RESULTS_VOLUME}" >/dev/null 2>&1 || true
}
trap cleanup EXIT
cleanup
docker build -t "${IMAGE}" "${ROOT_DIR}/prototype" >"${RESULTS_DIR}/docker-build.log"
docker volume create "${RESULTS_VOLUME}" >/dev/null
docker run --rm --memory=512m --cpus=2 -v "${RESULTS_VOLUME}:/results" "${IMAGE}" --out=/results --count="${INV005_COUNT:-500}"
docker run --rm --memory=512m --cpus=2 -v "${RESULTS_VOLUME}:/results" "${IMAGE}" --out=/results --probes

# Export benchmark output without host bind mounts. This works the same with Docker
# Desktop, native/rootless Docker and hosts with restricted file-sharing paths.
docker create --name "${EXPORT_CONTAINER}" -v "${RESULTS_VOLUME}:/results" "${IMAGE}" >/dev/null
docker cp "${EXPORT_CONTAINER}:/results/." "${RESULTS_DIR}"
docker rm "${EXPORT_CONTAINER}" >/dev/null

# Turn observed counts into explicit, enforceable transport contracts.
awk -F '\t' 'BEGIN {
  OFS="\t"
  print "transport","scenario","sent","observed","lost","loss_ratio","policy","result"
}
NR > 1 && $2 == "consumer-observed" {
  sent=$4+0; seen=$5+0; lost=sent-seen; ratio=(sent ? lost/sent : 0)
  if ($1 == "unix-stream") policy="require_complete"
  else if (($1 == "file" || $1 == "shared-memory" || $1 == "mmap") && $3 != "multi4") policy="require_complete"
  else if (($1 == "file" || $1 == "shared-memory" || $1 == "mmap") && $3 == "multi4") policy="allow_superseded_record_loss"
  else if ($1 == "unix-dgram") policy="record_loss"
  else policy="unexpected"
  result="pass"
  if (policy == "require_complete" && lost != 0) result="fail"
  if (policy == "unexpected") result="fail"
  printf "%s\t%s\t%d\t%d\t%d\t%.9f\t%s\t%s\n",$1,$3,sent,seen,lost,ratio,policy,result
}' "${RESULTS_DIR}/summary.tsv" >"${RESULTS_DIR}/observation-contracts.tsv"

# Execute PHP producer paths against real consumers.
docker run --rm --entrypoint php84 -v "${RESULTS_VOLUME}:/shared" "${IMAGE}" /opt/inv005/php/client.php file /shared/php-file.actual php-file-value
docker run -d --name inv005-php-unix -v "${RESULTS_VOLUME}:/results" -v "${RESULTS_VOLUME}:/shared" "${IMAGE}" --out=/results --integration-server=unix-stream >/dev/null
for _ in $(seq 1 100);do
  docker exec inv005-php-unix test -S /shared/php.sock >/dev/null 2>&1 && break
  sleep 0.02
done
docker run --rm --network container:inv005-php-unix -v "${RESULTS_VOLUME}:/shared" --entrypoint php84 "${IMAGE}" /opt/inv005/php/client.php unix-stream /shared/php.sock php-unix-value
docker wait inv005-php-unix >/dev/null
docker rm inv005-php-unix >/dev/null
docker run -d --name inv005-php-http -v "${RESULTS_VOLUME}:/results" "${IMAGE}" --out=/results --integration-server=http >/dev/null
sleep 0.2
docker run --rm --network container:inv005-php-http --entrypoint php84 "${IMAGE}" /opt/inv005/php/client.php http 127.0.0.1:19090 php-http-value
docker wait inv005-php-http >/dev/null
docker rm inv005-php-http >/dev/null

# Export PHP evidence added after the benchmark/probe export.
docker create --name "${EXPORT_CONTAINER}" -v "${RESULTS_VOLUME}:/results" "${IMAGE}" >/dev/null
docker cp "${EXPORT_CONTAINER}:/results/." "${RESULTS_DIR}"
docker rm "${EXPORT_CONTAINER}" >/dev/null
{
 printf 'key\tvalue\n'
 printf 'run_date_utc\t%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
 printf 'repository_head_sha\t%s\n' "$(git -C "${REPO_DIR}" rev-parse HEAD)"
 printf 'benchmark_scope_diff_clean\t%s\n' "$(scope_diff_clean)"
 printf 'benchmark_scope_untracked_count\t%s\n' "$(scope_untracked_count)"
 printf 'benchmark_code_fingerprint_sha256\t%s\n' "$(fingerprint)"
 printf 'docker_server_version\t%s\n' "$(docker version --format '{{.Server.Version}}')"
 printf 'docker_os\t%s\n' "$(docker info --format '{{.OperatingSystem}}')"
 printf 'docker_architecture\t%s\n' "$(docker info --format '{{.Architecture}}')"
 printf 'container_kernel\t%s\n' "$(docker run --rm --entrypoint uname "${IMAGE}" -a)"
 printf 'cpu_limit\t2\nmemory_limit_mib\t512\n'
} >"${RESULTS_DIR}/environment.tsv"
{
 printf 'case\texpected\tactual\tresult\n'
 rows="$(awk -F '\t' 'NR>1{n++}END{print n}' "${RESULTS_DIR}/summary.tsv")"
 printf 'all_scenario_rows_emitted\t52\t%s\t%s\n' "$rows" "$([ "$rows" = 52 ] && printf pass || printf fail)"
 for profile in publish-only acknowledged;do
   complete="$(awk -F '\t' -v p="$profile" 'NR>1&&$2==p&&$4!=$5{bad=1}END{print bad?"false":"true"}' "${RESULTS_DIR}/summary.tsv")"
   printf '%s_operations_complete\ttrue\t%s\t%s\n' "${profile//-/_}" "$complete" "$([ "$complete" = true ] && printf pass || printf fail)"
 done
 stream_complete="$(awk -F '\t' 'NR>1&&$1=="unix-stream"&&($3!=$4||$8!="pass"){bad=1}END{print bad?"false":"true"}' "${RESULTS_DIR}/observation-contracts.tsv")"
 printf 'consumer_observed_unix_stream_complete\ttrue\t%s\t%s\n' "$stream_complete" "$([ "$stream_complete" = true ] && printf pass || printf fail)"
 single_complete="$(awk -F '\t' 'NR>1&&$7=="require_complete"&&$1!="unix-stream"&&($3!=$4||$8!="pass"){bad=1}END{print bad?"false":"true"}' "${RESULTS_DIR}/observation-contracts.tsv")"
 printf 'consumer_observed_single_producer_complete\ttrue\t%s\t%s\n' "$single_complete" "$([ "$single_complete" = true ] && printf pass || printf fail)"
 datagram_rows="$(awk -F '\t' 'NR>1&&$1=="unix-dgram"&&$7=="record_loss"&&$3==$4+$5{n++}END{print n}' "${RESULTS_DIR}/observation-contracts.tsv")"
 printf 'consumer_observed_datagram_loss_recorded\t4\t%s\t%s\n' "$datagram_rows" "$([ "$datagram_rows" = 4 ] && printf pass || printf fail)"
 snapshot_rows="$(awk -F '\t' 'NR>1&&$7=="allow_superseded_record_loss"&&$3==$4+$5{n++}END{print n}' "${RESULTS_DIR}/observation-contracts.tsv")"
 snapshot_lost="$(awk -F '\t' 'NR>1&&$7=="allow_superseded_record_loss"{lost+=$5}END{print lost+0}' "${RESULTS_DIR}/observation-contracts.tsv")"
 printf 'snapshot_superseded_count_recorded\t3_rows\t%s_rows:%s_lost\t%s\n' "$snapshot_rows" "$snapshot_lost" "$([ "$snapshot_rows" = 3 ] && printf pass || printf fail)"
 for kind in file unix http;do
   case "$kind" in file) actual="$(tr -d '\\n' <"${RESULTS_DIR}/php-file.actual")";;unix) actual="$(cat "${RESULTS_DIR}/php-unix.received")";;http) actual="$(cat "${RESULTS_DIR}/php-http.received")";;esac
   expected="php-${kind}-value";[ "$actual" = "$expected" ] && result=pass || result=fail
   printf 'php_%s_delivery\t%s\t%s\t%s\n' "$kind" "$expected" "$actual" "$result"
 done
 tail -n +2 "${RESULTS_DIR}/probes.tsv"
} >"${RESULTS_DIR}/assertions.tsv"
printf '%s\n' "${RESULTS_DIR}" >"${ROOT_DIR}/latest-results.txt"
awk -F '\t' 'NR>1&&$4!="pass"{bad=1}END{exit bad}' "${RESULTS_DIR}/assertions.tsv"
awk -F '\t' 'NR>1&&$8!="pass"{bad=1}END{exit bad}' "${RESULTS_DIR}/observation-contracts.tsv"
printf 'Results: %s\n' "${RESULTS_DIR}"
