#!/bin/sh
set -eu
metricshell managed declare example_jobs counter "Completed finite jobs."
metricshell managed counter-add example_jobs "${EXAMPLE_JOBS:-7}"
exit "${EXAMPLE_EXIT_CODE:-0}"
