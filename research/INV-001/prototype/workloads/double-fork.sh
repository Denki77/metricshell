#!/bin/sh
set -eu

log() {
  printf '{"time":"%s","event":"%s","pid":%s,"ppid":%s}\n' "$(date -u +%Y-%m-%dT%H:%M:%S.%NZ)" "$1" "$$" "$(awk '/PPid:/ {print $2}' /proc/$$/status)"
}

log double_fork_parent_start

(
  (
    log daemonized_grandchild_start
    sleep 0.25
    log daemonized_grandchild_exit
    exit 23
  ) &
  exit 0
) &

wait || true
log double_fork_parent_exit
exit 0
