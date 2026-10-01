# Standalone verified copy

Copy the release binaries and `SHA256SUMS` into `release/`, then build with `docker buildx build --load --platform
linux/amd64 -t metricshell-example-standalone .`. The Dockerfile verifies the selected binary before installing it.
Run on port 19100, wait for `example_jobs_total`, then stop with a timeout greater than MetricShell's shutdown budget.
