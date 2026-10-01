#!/bin/sh
set -eu

digest=${1:?usage: render-kubernetes-release.sh SHA256_DIGEST OUTPUT_DIRECTORY}
output=${2:?usage: render-kubernetes-release.sh SHA256_DIGEST OUTPUT_DIRECTORY}
case "$digest" in
  sha256:[0-9a-f][0-9a-f]*) ;;
  *) echo "invalid OCI digest" >&2; exit 1 ;;
esac
test "${#digest}" -eq 71

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
mkdir -p "$output"
for input in "$root"/examples/kubernetes/production/*.yaml; do
  name=$(basename "$input")
  sed -e '/^# TEMPLATE — NOT DIRECTLY DEPLOYABLE/d' -e "s/sha256:__METRICSHELL_IMAGE_DIGEST__/$digest/g" "$input" > "$output/$name"
done
if grep -R '__METRICSHELL_IMAGE_DIGEST__\|TEMPLATE — NOT DIRECTLY DEPLOYABLE' "$output"; then
  echo "unrendered Kubernetes release template" >&2
  exit 1
fi
printf '%s\n' "$digest" > "$output/IMAGE_DIGEST"
