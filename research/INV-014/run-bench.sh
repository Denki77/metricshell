#!/usr/bin/env bash
set -Eeuo pipefail
export LC_ALL=C
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")"&&pwd)"; REPO_DIR="$(git -C "$ROOT_DIR" rev-parse --show-toplevel)"; PROTO="${ROOT_DIR}/prototype"
RUN_ID="${RUN_ID:-$(date -u +%Y%m%dT%H%M%SZ)}"; RESULTS_DIR="${ROOT_DIR}/results/${RUN_ID}"; mkdir -p "$RESULTS_DIR"
ASSERTIONS="$RESULTS_DIR/assertions.tsv"; OBSERVATIONS="$RESULTS_DIR/observations.tsv"; SUMMARY="$RESULTS_DIR/summary.tsv"; IMAGE=metricshell-inv014:prototype; CONTAINER=inv014-bench
printf 'case\tassertion\texpected\tactual\tresult\n' >"$ASSERTIONS"; printf 'case\tmetric\tvalue\tunit\tnote\n' >"$OBSERVATIONS"; printf 'case\tresult\tdetails\n' >"$SUMMARY"
hash_file(){ if command -v sha256sum >/dev/null; then sha256sum "$1"|awk '{print $1}'; else shasum -a 256 "$1"|awk '{print $1}'; fi; }; hash_stdin(){ if command -v sha256sum >/dev/null; then sha256sum|awk '{print $1}'; else shasum -a 256|awk '{print $1}'; fi; }
fingerprint(){ { find "$PROTO" -type f|sort; printf '%s\n' "$ROOT_DIR/run-bench.sh"; }|while IFS= read -r f; do printf '%s  %s\n' "$(hash_file "$f")" "${f#$ROOT_DIR/}"; done|hash_stdin; }
assert_eq(){ local c="$1" a="$2" e="$3" v="$4" r=fail; [[ "$e" == "$v" ]]&&r=pass; printf '%s\t%s\t%s\t%s\t%s\n' "$c" "$a" "$e" "$v" "$r" >>"$ASSERTIONS"; }; observe(){ printf '%s\t%s\t%s\t%s\t%s\n' "$1" "$2" "$3" "$4" "${5:-}" >>"$OBSERVATIONS"; }
finish_case(){ local c="$1" n; n="$(awk -F '\t' -v c="$c" 'NR>1&&$1==c&&$5!="pass"{n++}END{print n+0}' "$ASSERTIONS")"; [[ "$n" == 0 ]]&&printf '%s\tpass\tall assertions passed\n' "$c" >>"$SUMMARY"||printf '%s\tfail\t%s assertion(s) failed\n' "$c" "$n" >>"$SUMMARY"; }
cleanup(){ docker rm -f "$CONTAINER" inv014-oom >/dev/null 2>&1||true; }; trap cleanup EXIT; cleanup
docker build --pull=false -t "$IMAGE" "$PROTO" >"$RESULTS_DIR/docker-build.log"
docker run -d --name "$CONTAINER" --read-only --security-opt no-new-privileges --cap-drop ALL --memory=64m --pids-limit=64 --ulimit nofile=64:64 -p 127.0.0.1::19114 "$IMAGE" --max-payload=65536 >"$RESULTS_DIR/container.id"
PORT="$(docker port "$CONTAINER" 19114/tcp|sed 's/.*://')"; BASE="http://127.0.0.1:$PORT"; ready=false
for _ in $(seq 1 100); do curl -fsS "$BASE/healthz" >/dev/null 2>&1&&{ ready=true; break; }; sleep .03; done; assert_eq startup ready true "$ready"
logs="$(docker logs "$CONTAINER" 2>&1)"; uid="$(printf '%s' "$logs"|sed -n 's/.*uid=\([0-9]*\).*/\1/p')"; assert_eq non_root uid_nonzero true "$([[ "$uid" -gt 0 ]]&&echo true||echo false)"
bind="$(docker inspect "$CONTAINER" -f '{{(index (index .NetworkSettings.Ports "19114/tcp") 0).HostIp}}')"; assert_eq binding host_loopback 127.0.0.1 "$bind"
readonly="$(docker inspect "$CONTAINER" -f '{{.HostConfig.ReadonlyRootfs}}')"; assert_eq sandbox read_only_rootfs true "$readonly"
nnp="$(docker inspect "$CONTAINER" -f '{{json .HostConfig.SecurityOpt}}')"; assert_eq sandbox no_new_privileges true "$([[ "$nnp" == *no-new-privileges* ]]&&echo true||echo false)"
caps="$(docker inspect "$CONTAINER" -f '{{json .HostConfig.CapDrop}}')"; assert_eq sandbox all_capabilities_dropped true "$([[ "$caps" == *ALL* ]]&&echo true||echo false)"

printf 'alpha{id="1"} 1\nbeta 2\n' >"$RESULTS_DIR/snapshot-a.txt"; printf 'beta 7\n' >"$RESULTS_DIR/snapshot-b.txt"
code="$(curl -sS -o /dev/null -w '%{http_code}' -X PUT --data-binary @"$RESULTS_DIR/snapshot-a.txt" "$BASE/ingest")"; assert_eq complete_snapshot first_install 204 "$code"
code="$(curl -sS -o /dev/null -w '%{http_code}' -X PUT --data-binary @"$RESULTS_DIR/snapshot-b.txt" "$BASE/ingest")"; assert_eq complete_snapshot replacement 204 "$code"
curl -fsS "$BASE/metrics" >"$RESULTS_DIR/after-replacement.metrics"; assert_eq complete_snapshot omitted_series_removed true "$(! grep -q '^alpha' "$RESULTS_DIR/after-replacement.metrics"&&grep -q '^beta 7$' "$RESULTS_DIR/after-replacement.metrics"&&echo true||echo false)"

printf 'broken{ 1\n' >"$RESULTS_DIR/malformed.txt"; code="$(curl -sS -o /dev/null -w '%{http_code}' -X PUT --data-binary @"$RESULTS_DIR/malformed.txt" "$BASE/ingest")"; assert_eq validation malformed_rejected 400 "$code"
curl -fsS "$BASE/metrics" >"$RESULTS_DIR/after-malformed.metrics"; assert_eq validation last_valid_retained true "$(grep -q '^beta 7$' "$RESULTS_DIR/after-malformed.metrics"&&echo true||echo false)"
printf 'x 1\nx 1\n' >"$RESULTS_DIR/duplicate.txt"; code="$(curl -sS -o /dev/null -w '%{http_code}' -X PUT --data-binary @"$RESULTS_DIR/duplicate.txt" "$BASE/ingest")"; assert_eq validation duplicate_rejected 400 "$code"
printf 'metric{secret_token="value"} 1\n' >"$RESULTS_DIR/secret.txt"; code="$(curl -sS -o /dev/null -w '%{http_code}' -X PUT --data-binary @"$RESULTS_DIR/secret.txt" "$BASE/ingest")"; assert_eq secret_policy secret_like_label_rejected 400 "$code"
printf 'metric{a="1",b="2",c="3",d="4",e="5",f="6",g="7",h="8",i="9"} 1\n' >"$RESULTS_DIR/labels.txt"; code="$(curl -sS -o /dev/null -w '%{http_code}' -X PUT --data-binary @"$RESULTS_DIR/labels.txt" "$BASE/ingest")"; assert_eq label_limit too_many_labels 422 "$code"
long="$(printf 'x%.0s' $(seq 1 65))"; printf 'metric{label="%s"} 1\n' "$long" >"$RESULTS_DIR/label-value.txt"; code="$(curl -sS -o /dev/null -w '%{http_code}' -X PUT --data-binary @"$RESULTS_DIR/label-value.txt" "$BASE/ingest")"; assert_eq label_limit long_value 422 "$code"
seq 1 1001|awk '{print "series_"$1" 1"}' >"$RESULTS_DIR/series-limit.txt"; code="$(curl -sS -o /dev/null -w '%{http_code}' -X PUT --data-binary @"$RESULTS_DIR/series-limit.txt" "$BASE/ingest")"; assert_eq series_limit too_many_series 422 "$code"
head -c 70000 /dev/zero|tr '\0' x >"$RESULTS_DIR/payload-limit.txt"; code="$(curl -sS -o /dev/null -w '%{http_code}' -X PUT --data-binary @"$RESULTS_DIR/payload-limit.txt" "$BASE/ingest")"; assert_eq payload_limit oversized 413 "$code"

printf 'request\thttp_code\n' >"$RESULTS_DIR/concurrency.tsv"; export BASE RESULTS_DIR
seq 1 12|xargs -P12 -I{} sh -c 'code=$(printf "c{} 1\n"|curl -sS -o /dev/null -w "%{http_code}" -X PUT -H "X-Research-Hold-Millis: 500" --data-binary @- "$BASE/ingest"); printf "%s\t%s\n" "{}" "$code" >"$RESULTS_DIR/concurrent-{}.tsv"'
for f in "$RESULTS_DIR"/concurrent-*.tsv; do cat "$f" >>"$RESULTS_DIR/concurrency.tsv"; rm -f "$f"; done
busy="$(awk -F '\t' '$2==429{n++}END{print n+0}' "$RESULTS_DIR/concurrency.tsv")"; accepted="$(awk -F '\t' '$2==204{n++}END{print n+0}' "$RESULTS_DIR/concurrency.tsv")"
assert_eq concurrency_limit overload_rejected true "$([[ "$busy" -ge 1 ]]&&echo true||echo false)"; assert_eq concurrency_limit bounded_acceptance true "$([[ "$accepted" -le 4 ]]&&echo true||echo false)"; observe concurrency_limit accepted "$accepted" requests; observe concurrency_limit rejected_429 "$busy" requests
( exec 3<>"/dev/tcp/127.0.0.1/${PORT}"; printf 'PUT /ingest HTTP/1.1\r\nHost:x\r\nContent-Length:100\r\n\r\nx' >&3; sleep 2; exec 3>&- ) || true
assert_eq slow_client server_survives true "$(curl -fsS "$BASE/healthz" >/dev/null&&echo true||echo false)"

fd="$(docker exec "$CONTAINER" sh -c 'ulimit -n')"; assert_eq fd_limit soft_limit 64 "$fd"
mem="$(docker inspect "$CONTAINER" -f '{{.HostConfig.Memory}}')"; assert_eq memory_limit configured_bytes 67108864 "$mem"
perm="$(docker exec "$CONTAINER" stat -c '%a:%u:%g' /run/metricshell)"; observe path_permissions mode_uid_gid "$perm" text; assert_eq path_permissions private_mode 700 "${perm%%:*}"

bind_code=0; docker run --rm "$IMAGE" --addr=not-an-address >"$RESULTS_DIR/bind-failure.log" 2>&1||bind_code=$?; assert_eq bind_failure internal_exit 70 "$bind_code"
oom_code=0; docker run --name inv014-oom --memory=32m --memory-swap=32m "$IMAGE" --mode=allocate --allocate-bytes=134217728 >"$RESULTS_DIR/oom.log" 2>&1||oom_code=$?
oom="$(docker inspect inv014-oom -f '{{.State.OOMKilled}}')"; assert_eq memory_exhaustion oom_killed true "$oom"; assert_eq memory_exhaustion signal_exit 137 "$oom_code"; docker rm inv014-oom >/dev/null

for i in $(seq 1 100); do printf 'bad%%%s\n' "$i"|curl -sS -o /dev/null -X PUT --data-binary @- "$BASE/ingest"||true; done
assert_eq malicious_producer server_survives_fuzz true "$(curl -fsS "$BASE/healthz" >/dev/null&&echo true||echo false)"
for c in startup non_root binding sandbox complete_snapshot validation secret_policy label_limit series_limit payload_limit concurrency_limit slow_client fd_limit memory_limit path_permissions bind_failure memory_exhaustion malicious_producer; do finish_case "$c"; done
docker logs "$CONTAINER" >"$RESULTS_DIR/container.log" 2>&1||true
{
 printf 'key\tvalue\n'; printf 'investigation\tINV-014\n'; printf 'run_date_utc\t%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"; printf 'repository_head_sha\t%s\n' "$(git -C "$REPO_DIR" rev-parse HEAD)"; printf 'benchmark_scope_diff_clean\t%s\n' "$(git -C "$REPO_DIR" diff --quiet -- research/INV-014/prototype research/INV-014/run-bench.sh&&git -C "$REPO_DIR" diff --cached --quiet -- research/INV-014/prototype research/INV-014/run-bench.sh&&echo true||echo false)"; printf 'benchmark_code_fingerprint_sha256\t%s\n' "$(fingerprint)"; printf 'host_uname\t%s\n' "$(uname -a|tr '\t\n' ' ')"; printf 'docker_server_version\t%s\n' "$(docker version --format '{{.Server.Version}}')"; printf 'docker_architecture\t%s\n' "$(docker info --format '{{.Architecture}}')"; printf 'container_kernel\t%s\n' "$(docker run --rm alpine:3.20 uname -a)";
} >"$RESULTS_DIR/environment.tsv"; printf '%s\n' "$RESULTS_DIR" >"$ROOT_DIR/latest-results.txt"; failed="$(awk -F '\t' 'NR>1&&$5!="pass"{n++}END{print n+0}' "$ASSERTIONS")"; printf 'failed_assertions\t%s\nresults_dir\t%s\n' "$failed" "$RESULTS_DIR" >"$RESULTS_DIR/run-summary.tsv"; [[ "$failed" == 0 ]]
