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
    docker build -t metricshell-example-standalone "$root/docker/standalone-copy"
    docker run -d --name metricshell-example-standalone -p 127.0.0.1:19100:9090 \
      --read-only --tmpfs /run/metricshell:uid=65532,gid=65532,mode=0700 \
      --cap-drop ALL --security-opt no-new-privileges metricshell-example-standalone >/dev/null
    output=$(curl --fail --retry 30 --retry-connrefused --retry-delay 1 http://127.0.0.1:19100/metrics)
    printf '%s\n' "$output" | grep -F 'example_jobs_total'
    docker stop --time 32 metricshell-example-standalone >/dev/null
    docker rm metricshell-example-standalone >/dev/null
    ;;
  *) echo "unknown EXAMPLE_FILTER: $filter" >&2; exit 2 ;;
esac
echo "DOCKER EXAMPLES: PASS"
