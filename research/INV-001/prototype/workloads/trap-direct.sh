#!/bin/sh
set -eu

log() {
  printf '{"time":"%s","event":"%s","pid":%s,"ppid":%s}\n' "$(date -u +%Y-%m-%dT%H:%M:%S.%NZ)" "$1" "$$" "$(awk '/PPid:/ {print $2}' /proc/$$/status)"
}

term_seen=0
trap 'log workload_term; term_seen=1' TERM
trap 'log workload_int; exit 130' INT
trap 'log workload_hup' HUP

log workload_start
log workload_ready

while [ "$term_seen" -eq 0 ]; do
  sleep 0.05
done

log workload_exit_after_term
exit 0
