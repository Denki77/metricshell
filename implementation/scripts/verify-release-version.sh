#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
version=$(sed -n '1p' "$repo_root/VERSION")
test -n "$version"
test "$(wc -l < "$repo_root/VERSION" | tr -d ' ')" = 1
case "$version" in
  0|0.*[!0-9.]*|*[!0-9.]*) echo "invalid VERSION: $version" >&2; exit 1 ;;
esac

if [ -n "${GITHUB_REF_NAME:-}" ]; then
  test "$GITHUB_REF_NAME" = "v$version" || {
    echo "tag $GITHUB_REF_NAME does not match VERSION $version" >&2
    exit 1
  }
fi

release_dir=${1:-$repo_root/implementation/dist/release}
test "$(sed -n 's/^version=//p' "$release_dir/METADATA")" = "$version"
actual=$(docker run --rm --platform linux/amd64 -v "$release_dir:/release:ro" alpine:3.22 /release/metricshell-linux-amd64 --version)
case "$actual" in
  "metricshell version=$version revision="*) ;;
  *) echo "binary version mismatch: $actual" >&2; exit 1 ;;
esac
echo "RELEASE VERSION: $version"
