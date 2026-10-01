#!/bin/sh
set -eu
metricshell managed declare queue_depth gauge "Queued jobs."
metricshell managed gauge-set queue_depth 1
exec sleep 3600
