#!/bin/sh
set -eu

count="${1:-100}"
batch="${2:-0}"
batch_sleep="${3:-0}"
i=0
while [ "$i" -lt "$count" ]; do
  (exit 0) &
  i=$((i + 1))
  if [ "$batch" -gt 0 ] && [ $((i % batch)) -eq 0 ]; then
    sleep "$batch_sleep"
  fi
done

wait
exit 0
