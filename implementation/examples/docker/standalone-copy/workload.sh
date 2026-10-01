#!/bin/sh
set -eu
metricshell managed declare example_jobs counter "Completed example jobs."
metricshell managed counter-initialize example_jobs 0
while :; do
  metricshell managed counter-add example_jobs 1 >/dev/null
  sleep 2
done
