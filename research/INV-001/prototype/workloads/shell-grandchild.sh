#!/bin/sh
set -eu

log() {
  printf '{"time":"%s","event":"%s","pid":%s,"ppid":%s}\n' "$(date -u +%Y-%m-%dT%H:%M:%S.%NZ)" "$1" "$$" "$(awk '/PPid:/ {print $2}' /proc/$$/status)"
}

log shell_parent_start

(
  child_term=0
  trap 'log grandchild_term; child_term=1' TERM
  trap 'log grandchild_hup' HUP
  log grandchild_start
  log grandchild_ready
  while [ "$child_term" -eq 0 ]; do
    sleep 0.05
  done
  log grandchild_exit_after_term
) &
child=$!

trap 'log shell_parent_term; wait "$child"; exit 0' TERM
trap 'log shell_parent_hup' HUP
log shell_parent_ready

wait "$child"
