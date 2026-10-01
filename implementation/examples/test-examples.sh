#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
implementation=$(CDPATH= cd -- "$root/.." && pwd)
project_root=$(CDPATH= cd -- "$implementation/.." && pwd)
release="$implementation/dist/release"
evidence="$implementation/dist/examples"
version=$(sed -n '1p' "$project_root/VERSION")
revision=$(git -C "$project_root" rev-parse --verify HEAD)
suffix=$$
runtime_image="metricshell-candidate-runtime:$suffix"
transport_image="metricshell-example-transport:$suffix"
standalone_image="metricshell-example-standalone:$suffix"
finite_image="metricshell-example-finite:$suffix"
multistage_image="metricshell-example-multistage:$suffix"
base_image="metricshell-example-base:$suffix"
clean_image="metricshell-clean-machine:$suffix"
long_project="metricshell-long-$suffix"
finite_project="metricshell-finite-$suffix"
transport_project="metricshell-transport-$suffix"
temporary=$(mktemp -d)
standalone_context="$temporary/standalone-copy"
finite_context="$temporary/finite-workload"

cleanup() {
  docker rm -f "metricshell-standalone-$suffix" "metricshell-multistage-$suffix" \
    "metricshell-base-$suffix" "metricshell-clean-$suffix" "metricshell-finite-unavailable-$suffix" \
    >/dev/null 2>&1 || true
  docker compose -p "$long_project" -f "$root/compose/long-running/compose.yaml" down -v --remove-orphans >/dev/null 2>&1 || true
  docker compose -p "$finite_project" -f "$root/compose/finite-workload/compose.yaml" down -v --remove-orphans >/dev/null 2>&1 || true
  docker compose -p "$transport_project" -f "$root/compose/transport-conformance/compose.yaml" down -v --remove-orphans >/dev/null 2>&1 || true
  rm -rf "$temporary"
}
trap cleanup EXIT INT TERM

fail() {
  printf 'EXAMPLE FAILURE: %s\n' "$*" >&2
  exit 1
}

wait_metric() {
  url=$1
  needle=$2
  attempt=0
  while [ "$attempt" -lt 40 ]; do
    output=$(curl --fail --silent --show-error "$url" 2>/dev/null || true)
    if printf '%s\n' "$output" | grep -F "$needle" >/dev/null; then
      printf '%s\n' "$output"
      return 0
    fi
    attempt=$((attempt + 1))
    sleep 1
  done
  fail "metric $needle did not appear at $url"
}

network_get() {
  network=$1
  url=$2
  docker run --rm --network "$network" curlimages/curl:8.16.0 --fail --silent --show-error "$url"
}

wait_network_metric() {
  network=$1
  url=$2
  needle=$3
  attempt=0
  while [ "$attempt" -lt 40 ]; do
    output=$(network_get "$network" "$url" 2>/dev/null || true)
    if printf '%s\n' "$output" | grep -F "$needle" >/dev/null; then
      printf '%s\n' "$output"
      return 0
    fi
    attempt=$((attempt + 1))
    sleep 1
  done
  fail "metric $needle did not appear at $url"
}

prepare_candidates() {
  test -s "$release/metricshell-linux-amd64" || fail "release candidate is missing"
  test -s "$release/SHA256SUMS" || fail "release checksums are missing"
  docker run --rm --platform linux/amd64 -v "$release:/release:ro" alpine:3.22 \
    sh -ec 'cd /release && sha256sum -c SHA256SUMS'
  docker build --platform linux/amd64 --target runtime --build-arg VERSION="$version" --build-arg REVISION="$revision" \
    -t "$runtime_image" -f "$implementation/Dockerfile" "$project_root"
  docker build --platform linux/amd64 --target integration --build-arg REVISION="$revision" \
    -t "$transport_image" -f "$implementation/Dockerfile" "$project_root"
  mkdir -p "$standalone_context" "$finite_context"
  cp "$root/docker/standalone-copy/Dockerfile" "$root/docker/standalone-copy/workload.sh" "$standalone_context/"
  cp "$root/compose/finite-workload/Dockerfile" "$root/compose/finite-workload/workload.sh" "$finite_context/"
  for directory in "$standalone_context" "$finite_context"; do
    mkdir -p "$directory/release"
    cp "$release/metricshell-linux-amd64" "$directory/release/"
    cp "$release/SHA256SUMS" "$directory/release/"
  done
  docker build --platform linux/amd64 -t "$standalone_image" "$standalone_context"
  docker build --platform linux/amd64 -t "$finite_image" "$finite_context"
}

scenario_standalone() {
  docker run --platform linux/amd64 -d --name "metricshell-standalone-$suffix" -p 127.0.0.1:19100:9090 \
    --read-only --tmpfs /run/metricshell:uid=65532,gid=65532,mode=0700 \
    --cap-drop ALL --security-opt no-new-privileges "$standalone_image" >/dev/null
  wait_metric http://127.0.0.1:19100/metrics 'example_jobs_total ' >/dev/null
  docker inspect -f '{{.State.Running}}' "metricshell-standalone-$suffix" | grep -Fx true >/dev/null
  docker stop --time 32 "metricshell-standalone-$suffix" >/dev/null
  docker rm "metricshell-standalone-$suffix" >/dev/null
  printf 'SCENARIO standalone-copy: PASS\n'
}

scenario_multistage() {
  docker build --platform linux/amd64 --build-arg METRICSHELL_ARTIFACT_REF="$runtime_image" \
    -t "$multistage_image" "$root/docker/multistage-copy"
  docker run --platform linux/amd64 -d --name "metricshell-multistage-$suffix" -p 127.0.0.1:19101:9090 \
    -e METRICSHELL_FINAL_WAIT_MODE=duration -e METRICSHELL_FINAL_WAIT_DURATION=8s "$multistage_image" >/dev/null
  wait_metric http://127.0.0.1:19101/metrics 'example_jobs_total 3' >/dev/null
  code=$(docker wait "metricshell-multistage-$suffix")
  test "$code" -eq 0 || fail "multistage-copy exit code was $code"
  docker rm "metricshell-multistage-$suffix" >/dev/null
  printf 'SCENARIO multistage-copy: PASS\n'
}

scenario_base_image() {
  docker build --platform linux/amd64 --build-arg METRICSHELL_IMAGE_REF="$runtime_image" \
    -t "$base_image" "$root/docker/base-image"
  docker run --platform linux/amd64 -d --name "metricshell-base-$suffix" -p 127.0.0.1:19102:9090 "$base_image" >/dev/null
  wait_metric http://127.0.0.1:19102/metrics 'queue_depth 1' >/dev/null
  docker inspect -f '{{.State.Running}}' "metricshell-base-$suffix" | grep -Fx true >/dev/null
  docker stop --time 32 "metricshell-base-$suffix" >/dev/null
  docker rm "metricshell-base-$suffix" >/dev/null
  printf 'SCENARIO base-image: PASS\n'
}

prometheus_query() {
  project=$1
  query=$2
  network_get "${project}_default" "http://prometheus:9090/api/v1/query?query=$query"
}

wait_prometheus_value() {
  project=$1
  metric=$2
  rejected=$3
  attempt=0
  while [ "$attempt" -lt 40 ]; do
    response=$(prometheus_query "$project" "$metric" 2>/dev/null || true)
    value=$(printf '%s' "$response" | sed -n 's/.*"value":\[[^,]*,"\([^"]*\)"\].*/\1/p')
    if [ -n "$value" ] && [ "$value" != "$rejected" ]; then
      printf '%s\n' "$value"
      return 0
    fi
    attempt=$((attempt + 1))
    sleep 1
  done
  fail "Prometheus did not observe a new $metric value"
}

scenario_long_running() {
  METRICSHELL_LONG_RUNNING_IMAGE="$standalone_image" docker compose -p "$long_project" \
    -f "$root/compose/long-running/compose.yaml" up -d
  application_id=$(docker compose -p "$long_project" -f "$root/compose/long-running/compose.yaml" ps -q application)
  test -n "$application_id" || fail "long-running application was not created"
  docker inspect -f '{{.State.Running}}' "$application_id" | grep -Fx true >/dev/null
  first=$(wait_prometheus_value "$long_project" example_jobs_total impossible)
  docker inspect -f '{{.State.Running}}' "$application_id" | grep -Fx true >/dev/null
  second=$(wait_prometheus_value "$long_project" example_jobs_total "$first")
  docker inspect -f '{{.State.Running}}' "$application_id" | grep -Fx true >/dev/null
  test "$first" != "$second" || fail "long-running metric did not change"
  printf 'SCENARIO compose/long-running: PASS (%s -> %s)\n' "$first" "$second"
}

run_finite_case() {
  expected_exit=$1
  jobs=$2
  EXAMPLE_EXIT_CODE="$expected_exit" EXAMPLE_JOBS="$jobs" METRICSHELL_FINITE_IMAGE="$finite_image" \
    docker compose -p "$finite_project" -f "$root/compose/finite-workload/compose.yaml" up -d --force-recreate finite-application
  container_id=$(docker compose -p "$finite_project" -f "$root/compose/finite-workload/compose.yaml" ps -aq finite-application)
  code=$(docker wait "$container_id")
  test "$code" -eq "$expected_exit" || fail "finite workload exit $code, expected $expected_exit"
  response=$(prometheus_query "$finite_project" example_jobs_total)
  printf '%s' "$response" | grep -F "\"$jobs\"" >/dev/null || fail "Prometheus missed final finite metric $jobs"
  timestamp=$(printf '%s' "$response" | sed -n 's/.*"value":\[\([^,]*\),.*/\1/p')
  test -n "$timestamp" || fail "finite metric timestamp is missing"
  finished=$(docker inspect -f '{{.State.FinishedAt}}' "$container_id")
  printf '{"exit_code":%s,"metric":"example_jobs_total","value":"%s","sample_timestamp":%s,"finished_at":"%s"}\n' \
    "$expected_exit" "$jobs" "$timestamp" "$finished" >> "$evidence/finite-workload.jsonl"
}

scenario_finite_workload() {
  mkdir -p "$evidence"
  : > "$evidence/finite-workload.jsonl"
  METRICSHELL_FINITE_IMAGE="$finite_image" docker compose -p "$finite_project" \
    -f "$root/compose/finite-workload/compose.yaml" up -d prometheus
  run_finite_case 0 7
  run_finite_case 17 17
  started=$(date +%s)
  code=0
  docker run --platform linux/amd64 --name "metricshell-finite-unavailable-$suffix" \
    -e EXAMPLE_EXIT_CODE=17 -e EXAMPLE_JOBS=23 \
    -e METRICSHELL_FINAL_WAIT_MODE=scrapes -e METRICSHELL_FINAL_WAIT_REQUIRED_SCRAPES=1 \
    -e METRICSHELL_FINAL_WAIT_TIMEOUT=3s "$finite_image" >/dev/null 2>&1 || code=$?
  elapsed=$(( $(date +%s) - started ))
  test "$code" -eq 17 || fail "unavailable-Prometheus case lost workload exit 17 (got $code)"
  test "$elapsed" -ge 2 && test "$elapsed" -le 8 || fail "unavailable-Prometheus timeout was not bounded ($elapsed seconds)"
  printf 'SCENARIO compose/finite-workload: PASS (evidence %s)\n' "$evidence/finite-workload.jsonl"
}

publish_transport() {
  transport=$1
  payload=$2
  case "$transport" in
    file) docker run --rm -e FIXTURE_TRANSPORT=file -e FIXTURE_PAYLOAD="$payload" -v "${transport_project}_file-state:/run/metricshell" --entrypoint /snapshot-client-fixture "$transport_image" ;;
    unix) docker run --rm -e FIXTURE_TRANSPORT=unix -e FIXTURE_PAYLOAD="$payload" -v "${transport_project}_unix-state:/run/metricshell" --entrypoint /snapshot-client-fixture "$transport_image" ;;
    http) docker compose -p "$transport_project" -f "$root/compose/transport-conformance/compose.yaml" \
      exec -T -e FIXTURE_TRANSPORT=http -e FIXTURE_PAYLOAD="$payload" \
      -e FIXTURE_HTTP_URL=http://127.0.0.1:9091/v1/metrics http /snapshot-client-fixture ;;
  esac
}

transport_exposition() {
  transport=$1
  network_get "${transport_project}_default" "http://$transport:9090/metrics" | \
    sed '/^# HELP metricshell_/d;/^# TYPE metricshell_/d;/^metricshell_/d;/^# EOF$/d;/^$/d'
}

wait_transport_state() {
  transport=$1
  state=$2
  attempt=0
  while [ "$attempt" -lt 40 ]; do
    exposition=$(transport_exposition "$transport" 2>/dev/null || true)
    case "$state" in
      application) printf '%s\n' "$exposition" | grep -F 'example_jobs_total 3' >/dev/null && { printf '%s\n' "$exposition"; return 0; } ;;
      zero) ! printf '%s\n' "$exposition" | grep -F 'example_' >/dev/null && { printf '%s\n' "$exposition"; return 0; } ;;
    esac
    attempt=$((attempt + 1))
    sleep 1
  done
  fail "$transport did not reach $state application state"
}

scenario_transport_conformance() {
  METRICSHELL_TRANSPORT_IMAGE="$transport_image" docker compose -p "$transport_project" \
    -f "$root/compose/transport-conformance/compose.yaml" up -d
  reference=
  for transport in file unix http; do
    publish_transport "$transport" application >/dev/null
    current="$temporary/$transport.application"
    wait_transport_state "$transport" application > "$current"
    if [ -z "$reference" ]; then reference=$current; else cmp "$reference" "$current"; fi
    publish_transport "$transport" malformed >/dev/null
    sleep 1
    retained="$temporary/$transport.retained"
    transport_exposition "$transport" > "$retained"
    cmp "$current" "$retained"
    publish_transport "$transport" zero >/dev/null
    wait_transport_state "$transport" zero >/dev/null
  done
  printf 'SCENARIO compose/transport-conformance: PASS\n'
}

scenario_clean_machine() {
  context="$temporary/clean-machine"
  mkdir -p "$context"
  cp "$release/metricshell-linux-amd64" "$release/SHA256SUMS" "$context/"
  cat > "$context/Dockerfile" <<'EOF'
FROM alpine:3.22
COPY metricshell-linux-amd64 SHA256SUMS /candidate/
RUN cd /candidate && grep 'metricshell-linux-amd64$' SHA256SUMS | sha256sum -c - && \
    install -m 0755 metricshell-linux-amd64 /usr/local/bin/metricshell && \
    printf '%s\n' '#!/bin/sh' 'set -eu' 'metricshell managed declare clean_machine gauge "Clean machine acceptance."' 'metricshell managed gauge-set clean_machine 1' 'exec sleep 30' > /usr/local/bin/workload && chmod 0755 /usr/local/bin/workload
RUN mkdir -p /run/metricshell && chown -R 65532:65532 /run/metricshell && chmod 0700 /run/metricshell
USER 65532:65532
ENTRYPOINT ["/usr/local/bin/metricshell", "--mode=managed-registry", "--exposition-listen=0.0.0.0:9090", "--", "/usr/local/bin/workload"]
EOF
  test "$(find "$context" -type f | wc -l | tr -d ' ')" -eq 3 || fail "clean-machine context contains developer artifacts"
  docker build --platform linux/amd64 -t "$clean_image" "$context"
  docker run --platform linux/amd64 -d --name "metricshell-clean-$suffix" -p 127.0.0.1:19103:9090 "$clean_image" >/dev/null
  wait_metric http://127.0.0.1:19103/metrics 'clean_machine 1' >/dev/null
  docker exec "metricshell-clean-$suffix" /usr/local/bin/metricshell --version | grep -F "metricshell version=$version revision=$revision" >/dev/null
  docker stop --time 32 "metricshell-clean-$suffix" >/dev/null
  docker rm "metricshell-clean-$suffix" >/dev/null
  printf 'SCENARIO clean-machine: PASS\n'
}

prepare_candidates
scenario_standalone
scenario_multistage
scenario_base_image
scenario_long_running
scenario_finite_workload
scenario_transport_conformance
scenario_clean_machine
printf 'ALL REQUIRED EXAMPLE SCENARIOS: PASS\n'
