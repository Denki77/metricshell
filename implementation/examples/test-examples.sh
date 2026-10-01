#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
version=$(sed -n '1p' "$root/../../VERSION")
revision=$(git -C "$root/../.." rev-parse --verify HEAD)
filter=${EXAMPLE_FILTER:-all}
keep=${KEEP_EXAMPLE_RESOURCES:-0}

cleanup() {
  if [ "$keep" != 1 ]; then
    docker rm -f metricshell-example-standalone >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT INT TERM

echo "MetricShell version=$version revision=$revision filter=$filter"
case "$filter" in
  all|docker)
    rm -rf "$root/docker/standalone-copy/release"
    mkdir -p "$root/docker/standalone-copy/release"
    cp "$root/../dist/release/metricshell-linux-amd64" "$root/docker/standalone-copy/release/"
    cp "$root/../dist/release/SHA256SUMS" "$root/docker/standalone-copy/release/"
    docker build --platform linux/amd64 -t metricshell-example-standalone "$root/docker/standalone-copy"
    docker run --platform linux/amd64 -d --name metricshell-example-standalone -p 127.0.0.1:19100:9090 \
      --read-only --tmpfs /run/metricshell:uid=65532,gid=65532,mode=0700 \
      --cap-drop ALL --security-opt no-new-privileges metricshell-example-standalone >/dev/null
    attempt=0
    output=
    until output=$(curl --fail --silent --show-error http://127.0.0.1:19100/metrics 2>/dev/null) && \
      printf '%s\n' "$output" | grep -F 'example_jobs_total' >/dev/null; do
      attempt=$((attempt + 1))
      test "$attempt" -lt 30
      sleep 1
    done
    printf '%s\n' "$output" | grep -F 'example_jobs_total' >/dev/null
    docker stop --time 32 metricshell-example-standalone >/dev/null
    docker rm metricshell-example-standalone >/dev/null
    ;;
  *) echo "unknown EXAMPLE_FILTER: $filter" >&2; exit 2 ;;
esac
echo "DOCKER EXAMPLES: PASS"
