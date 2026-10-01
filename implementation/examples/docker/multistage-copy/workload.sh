#!/bin/sh
set -eu
metricshell managed declare example_jobs counter "Completed example jobs."
metricshell managed counter-add example_jobs 3
