#!/usr/bin/env bash
set -Eeuo pipefail
export LC_ALL=C

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(git -C "${ROOT_DIR}" rev-parse --show-toplevel)"
IMAGE="metricshell-inv010:prototype"
PROMTOOL_IMAGE="prom/prometheus@sha256:63805ebb8d2b3920190daf1cb14a60871b16fd38bed42b857a3182bc621f4996"
RUN_ID="${RUN_ID:-$(date -u +%Y%m%dT%H%M%SZ)}"
RESULTS_DIR="${ROOT_DIR}/results/${RUN_ID}"
RESULTS_REF="research/INV-010/results/${RUN_ID}"
ASSERTIONS="${RESULTS_DIR}/assertions.tsv"
OBSERVATIONS="${RESULTS_DIR}/observations.tsv"
SUMMARY="${RESULTS_DIR}/summary.tsv"
CONTAINER="inv010-bench"
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
cleanup

docker build -t "${IMAGE}" "${ROOT_DIR}/prototype" >"${RESULTS_DIR}/docker-build.log"
docker run -d --name "${CONTAINER}" -p 127.0.0.1::19100 "${IMAGE}" --series=1000 >"${RESULTS_DIR}/container.id"
PORT="$(docker port "${CONTAINER}" 19100/tcp | sed 's/.*://')"
BASE="http://127.0.0.1:${PORT}"
ready=false
for _ in $(seq 1 100); do if curl -fsS "${BASE}/healthz" >/dev/null; then ready=true; break; fi; sleep 0.05; done
assert_eq startup health_ready true "${ready}"

# Formats, negotiation, metadata, classic histogram, timestamps, and fallback.
curl -fsS -D "${RESULTS_DIR}/prometheus.headers" -H 'Accept: text/plain; version=0.0.4' "${BASE}/metrics" -o "${RESULTS_DIR}/prometheus.txt"
curl -fsS -D "${RESULTS_DIR}/openmetrics.headers" -H 'Accept: application/openmetrics-text; version=1.0.0' "${BASE}/metrics" -o "${RESULTS_DIR}/openmetrics.txt"
prom_ct="$(awk -F': ' 'tolower($1)=="content-type"{print tolower($2)}' "${RESULTS_DIR}/prometheus.headers" | tr -d '\r')"
open_ct="$(awk -F': ' 'tolower($1)=="content-type"{print tolower($2)}' "${RESULTS_DIR}/openmetrics.headers" | tr -d '\r')"
assert_eq formats prometheus_content_type true "$([[ "${prom_ct}" == text/plain* ]] && echo true || echo false)"
assert_eq formats openmetrics_content_type true "$([[ "${open_ct}" == application/openmetrics-text* ]] && echo true || echo false)"
assert_eq formats openmetrics_eof true "$(tail -1 "${RESULTS_DIR}/openmetrics.txt" | grep -qx '# EOF' && echo true || echo false)"
assert_eq formats help_and_type true "$(grep -q '^# HELP inv010_series ' "${RESULTS_DIR}/prometheus.txt" && grep -q '^# TYPE inv010_series gauge' "${RESULTS_DIR}/prometheus.txt" && echo true || echo false)"
assert_eq formats classic_histogram true "$(grep -q 'inv010_duration_seconds_bucket{le="+Inf"} 5' "${RESULTS_DIR}/prometheus.txt" && grep -q 'inv010_duration_seconds_count 5' "${RESULTS_DIR}/prometheus.txt" && echo true || echo false)"
promtool_code=0
docker run --rm -i --entrypoint=/bin/promtool "${PROMTOOL_IMAGE}" check metrics <"${RESULTS_DIR}/prometheus.txt" >"${RESULTS_DIR}/promtool.log" 2>&1 || promtool_code=$?
assert_eq formats promtool_check_metrics 0 "${promtool_code}"

printf '# HELP timestamped Optional timestamp.\n# TYPE timestamped gauge\ntimestamped 7 1700000000000\n' >"${RESULTS_DIR}/timestamp.snapshot"
timestamp_code="$(curl -sS -o /dev/null -w '%{http_code}' -X PUT --data-binary @"${RESULTS_DIR}/timestamp.snapshot" "${BASE}/snapshot")"
curl -fsS "${BASE}/metrics" >"${RESULTS_DIR}/timestamp.metrics"
assert_eq optional_timestamp install_status 204 "${timestamp_code}"
assert_eq optional_timestamp preserved true "$(grep -q '^timestamped 7 1700000000000$' "${RESULTS_DIR}/timestamp.metrics" && echo true || echo false)"

# Malformed candidates are rejected atomically and never replace the active complete snapshot.
printf 'broken_metric{ 1\n' >"${RESULTS_DIR}/malformed.snapshot"
malformed_code="$(curl -sS -o "${RESULTS_DIR}/malformed.response" -w '%{http_code}' -X PUT --data-binary @"${RESULTS_DIR}/malformed.snapshot" "${BASE}/snapshot")"
curl -fsS "${BASE}/metrics" >"${RESULTS_DIR}/after-malformed.metrics"
assert_eq malformed_candidate rejected 400 "${malformed_code}"
assert_eq malformed_candidate last_valid_retained true "$(grep -q '^timestamped 7 1700000000000$' "${RESULTS_DIR}/after-malformed.metrics" && echo true || echo false)"

# Every concurrent scrape must contain exactly one complete A or B snapshot, never a merge.
printf 'sample\tgeneration_header\tgeneration_labels\tseries\tresult\n' >"${RESULTS_DIR}/concurrent-scrapes.tsv"
curl -fsS -X PUT "${BASE}/generate?series=250&generation=A" >/dev/null
(
  for i in $(seq 1 120); do
    generation=A; (( i % 2 == 0 )) && generation=B
    curl -fsS -X PUT "${BASE}/generate?series=250&generation=${generation}" >/dev/null
  done
) & installer_pid=$!
seq 1 120 | xargs -P16 -I{} sh -c 'curl -fsS -D "$1/h.{}" "$2/metrics" -o "$1/b.{}"' _ "${RESULTS_DIR}" "${BASE}"
wait "${installer_pid}"
concurrent_ok=true
for i in $(seq 1 120); do
  header="$(awk -F': ' 'tolower($1)=="x-application-snapshot-generation"{print $2}' "${RESULTS_DIR}/h.${i}" | tr -d '\r')"
  labels="$(sed -n 's/.*inv010_series{generation="\([^"]*\)".*/\1/p' "${RESULTS_DIR}/b.${i}" | sort -u | tr '\n' ',' | sed 's/,$//')"
  count="$(grep -c '^inv010_series{' "${RESULTS_DIR}/b.${i}" || true)"
  result=pass
  if [[ "${header}" != A && "${header}" != B ]] || [[ "${labels}" != "${header}" ]] || [[ "${count}" != 250 ]]; then result=fail; concurrent_ok=false; fi
  printf '%s\t%s\t%s\t%s\t%s\n' "${i}" "${header}" "${labels}" "${count}" "${result}" >>"${RESULTS_DIR}/concurrent-scrapes.tsv"
  rm -f "${RESULTS_DIR}/h.${i}" "${RESULTS_DIR}/b.${i}"
done
assert_eq concurrent_replace complete_single_snapshot true "${concurrent_ok}"

# Cardinality matrix and response throughput. These are observations, not portable thresholds.
printf 'series\tresponse_bytes\tinstall_ns\tscrape_ns\n' >"${RESULTS_DIR}/cardinality.tsv"
for series in 0 1000 10000 100000; do
  started="$(now_ns)"; curl -fsS -X PUT "${BASE}/generate?series=${series}&generation=cardinality-${series}" >/dev/null; installed="$(now_ns)"
  curl -fsS "${BASE}/metrics" -o "${RESULTS_DIR}/cardinality-${series}.metrics"; scraped="$(now_ns)"
  bytes="$(wc -c <"${RESULTS_DIR}/cardinality-${series}.metrics" | tr -d ' ')"
  printf '%s\t%s\t%s\t%s\n' "${series}" "${bytes}" "$((installed-started))" "$((scraped-installed))" >>"${RESULTS_DIR}/cardinality.tsv"
  observe cardinality "response_bytes_${series}" "${bytes}" bytes "complete application snapshot"
  observe cardinality "install_ns_${series}" "$((installed-started))" ns "host-side wall time"
  observe cardinality "scrape_ns_${series}" "$((scraped-installed))" ns "host-side wall time"
done

# Multiple scrapers, gzip, slow and disconnected clients.
started="$(now_ns)"; seq 1 32 | xargs -P32 -I{} curl -fsS "${BASE}/metrics" -o /dev/null; multi_code=$?; finished="$(now_ns)"
assert_eq multiple_scrapers all_32_succeeded 0 "${multi_code}"
observe multiple_scrapers wall_time_ns "$((finished-started))" ns "32 concurrent complete responses"
curl -fsS --compressed -D "${RESULTS_DIR}/gzip.headers" -H 'Accept-Encoding: gzip' "${BASE}/metrics" -o "${RESULTS_DIR}/gzip.metrics"
assert_eq compression negotiated true "$(grep -qi '^Content-Encoding: gzip' "${RESULTS_DIR}/gzip.headers" && echo true || echo false)"
curl --limit-rate 1k --max-time 1 "${BASE}/metrics" -o /dev/null 2>"${RESULTS_DIR}/slow-client.log" || true
assert_eq slow_client server_survives true "$(curl -fsS "${BASE}/healthz" >/dev/null && echo true || echo false)"
( exec 3<>"/dev/tcp/127.0.0.1/${PORT}"; printf 'GET /metrics HTTP/1.1\r\nHost: localhost\r\n\r\n' >&3; sleep 0.02; exec 3>&- ) || true
assert_eq aborted_client server_survives true "$(curl -fsS "${BASE}/healthz" >/dev/null && echo true || echo false)"

# Response bound is checked before status/body headers are committed.
cleanup
docker run -d --name "${CONTAINER}" -p 127.0.0.1::19100 "${IMAGE}" --series=10000 --response-limit=1024 >/dev/null
PORT="$(docker port "${CONTAINER}" 19100/tcp | sed 's/.*://')"; BASE="http://127.0.0.1:${PORT}"
for _ in $(seq 1 100); do curl -fsS "${BASE}/healthz" >/dev/null 2>&1 && break; sleep 0.05; done
limit_code="$(curl -sS -o "${RESULTS_DIR}/limited-response.txt" -w '%{http_code}' "${BASE}/metrics")"
assert_eq response_limit preflight_503 503 "${limit_code}"
assert_eq response_limit bounded_body true "$([[ "$(wc -c <"${RESULTS_DIR}/limited-response.txt")" -lt 1024 ]] && echo true || echo false)"

for case_name in startup formats optional_timestamp malformed_candidate concurrent_replace cardinality multiple_scrapers compression slow_client aborted_client response_limit; do case_result "${case_name}"; done
docker logs "${CONTAINER}" >"${RESULTS_DIR}/container.log" 2>&1 || true
{
  printf 'key\tvalue\n'
  printf 'investigation\tINV-010\n'
  printf 'run_date_utc\t%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  printf 'repository_head_sha\t%s\n' "$(git -C "${REPO_DIR}" rev-parse HEAD 2>/dev/null || printf unknown)"
  printf 'benchmark_scope_diff_clean\t%s\n' "$(git -C "${REPO_DIR}" diff --quiet -- research/INV-010/prototype research/INV-010/run-bench.sh && git -C "${REPO_DIR}" diff --cached --quiet -- research/INV-010/prototype research/INV-010/run-bench.sh && echo true || echo false)"
  printf 'benchmark_code_fingerprint_sha256\t%s\n' "$(fingerprint)"
  printf 'host_uname\t%s\n' "$(uname -a | tr '\t\n' ' ')"
  printf 'docker_server_version\t%s\n' "$(docker version --format '{{.Server.Version}}')"
  printf 'docker_architecture\t%s\n' "$(docker info --format '{{.Architecture}}')"
  printf 'docker_operating_system\t%s\n' "$(docker info --format '{{.OperatingSystem}}' | tr '\t\n' ' ')"
  printf 'container_kernel\t%s\n' "$(docker run --rm --entrypoint uname "${IMAGE}" -a)"
} >"${RESULTS_DIR}/environment.tsv"
printf '%s\n' "${RESULTS_REF}" >"${ROOT_DIR}/latest-results.txt"
failed="$(awk -F '\t' 'NR>1&&$5!="pass"{n++} END{print n+0}' "${ASSERTIONS}")"
printf 'failed_assertions\t%s\nresults_dir\t%s\n' "${failed}" "${RESULTS_REF}" >"${RESULTS_DIR}/run-summary.tsv"
[[ "${failed}" == 0 ]]
