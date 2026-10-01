# Production deployment

[Russian version](../../docs-ru/07-delivery/production-deployment.md)

This is the operational entry point for MetricShell 0.2.0. MetricShell runs as PID 1, owns one workload process tree,
accepts instrumentation locally and exposes `GET /metrics` for Prometheus. The workload never needs a monitoring
destination: Prometheus discovery and scrape configuration belong to infrastructure.

## Install

Release assets contain `metricshell-linux-amd64`, `metricshell-linux-arm64`, `SHA256SUMS`, `SBOM.json`,
`PROVENANCE.json`, signatures and public verification material. Download an asset and `SHA256SUMS` from the same
release, run `sha256sum -c SHA256SUMS`, verify the signature as described by `VERIFY.md`, install as `metricshell`, and
confirm `metricshell --version` reports `0.2.0`.

The official images are `ghcr.io/denki77/metricshell:0.2.0` and
`ghcr.io/denki77/metricshell-artifact:0.2.0` for `linux/amd64` and `linux/arm64`. Tags are for discovery; production
manifests must use the immutable `image@sha256:<release-digest>` recorded in the release notes. Never use `latest`.

For Kubernetes, start from the Job, CronJob and Deployment manifests in `implementation/examples/kubernetes/production`.
The checked-in files are clearly marked templates; the release pipeline substitutes and validates the published digest.

## Runtime contract

MetricShell must be PID 1 and the workload follows the standalone `--`. It forwards signals to the workload process
group, reaps descendants and preserves workload exit status. Exposition defaults to port 9090. Snapshot and Managed
ingestion stay private: mount a writable directory for Unix sockets (normally `/run/metricshell`) or an atomic snapshot
file; the root filesystem can remain read-only. No Kubernetes API or service-account token is required.

Prometheus scrapes `GET /metrics`; `/healthz` and `/readyz` are probes and do not count as final scrapes. Use service
discovery, annotations or a PodMonitor. Choose a scrape timeout below the interval. Finite workloads should use final
wait mode `scrapes`, a finite timeout and enough pod lifetime for at least one eligible scrape.

## Security baseline

Run as a fixed non-root UID/GID, use a read-only root filesystem, drop all capabilities, set
`allowPrivilegeEscalation: false`, `no-new-privileges`, and `seccompProfile.type: RuntimeDefault`. Disable service-account
token mounting. Expose only port 9090. Keep the Managed Unix socket in a mode-0700 directory shared only with the
workload. Do not publish the HTTP ingestion listener; ingestion has no reason to be reachable from the network.

## Resources and lifecycle

Start with the requests/limits in the supplied manifests and tune from measurements. Bound memory, PIDs and `nofile`;
also configure snapshot/decoded byte limits, scrape concurrency, Managed connections, queue capacity, descriptors,
series, label bytes and histogram buckets. Rejection is preferable to unbounded growth.

`terminationGracePeriodSeconds` must exceed MetricShell's total shutdown grace. The supplied manifests use 32 seconds
outside and 30 seconds inside, reserving two seconds for cleanup. Workload shutdown, Managed admission close/drain,
freeze, final Core installation and final-scrape wait are all bounded. If Prometheus is unavailable, timeout ends the
wait and the original workload result is preserved.

## Local instrumentation

Managed clients default to `/run/metricshell/managed.sock`, so shell workloads can declare and update metrics without
passing a destination:

```sh
metricshell managed declare jobs counter "Processed jobs" worker
metricshell managed counter-initialize jobs 0 worker=batch
metricshell managed counter-add jobs 1 worker=batch
metricshell managed declare queue_depth gauge "Queued jobs"
metricshell managed gauge-set queue_depth 7
metricshell managed declare job_seconds histogram "Job duration" - 0.1 1 +Inf
metricshell managed histogram-observe job_seconds 0.42
```

PHP 5.4 copy/paste usage is in `implementation/examples/clients/php54`. Both clients know only the local Unix socket.

## Troubleshooting

- Workload did not start: inspect `workload.start_failed`, executable path, permissions and exit 73.
- Malformed metrics or Core candidate rejected: inspect the bounded rejection reason; the last valid snapshot remains.
- Resource rejection or overload: inspect self-metrics, then raise the specific bound only after measuring demand.
- Final scrape timeout: verify discovery, target readiness, scrape interval and timeout; the wait is intentionally finite.
- Bind failure: ensure exposition and ingestion addresses do not collide and the socket directory is writable/private.
- Managed `UNKNOWN`: the request may have committed after client acknowledgement was lost; reconcile deliberately.
- Prometheus does not discover the target: check labels/annotations or PodMonitor selectors and network policy to 9090.
- Restart: a new Managed process starts an empty epoch by design; MetricShell does not persist application metrics.

MetricShell correctness guarantees are defined by the specifications and tests. Resource limits are hard bounds;
benchmark observations are not an SLO.
