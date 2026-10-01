# MetricShell

[![CI](https://github.com/Denki77/metricshell/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/Denki77/metricshell/actions/workflows/ci.yml)
[![Documentation](https://github.com/Denki77/metricshell/actions/workflows/docs.yml/badge.svg?branch=main)](https://github.com/Denki77/metricshell/actions/workflows/docs.yml)
 [![Release](https://github.com/Denki77/metricshell/actions/workflows/release.yml/badge.svg)](https://github.com/Denki77/metricshell/actions/workflows/release.yml)
![Version](https://img.shields.io/badge/version-0.2.0-blue)
![Go](https://img.shields.io/badge/go-1.26-00ADD8)
![Platforms](https://img.shields.io/badge/platform-linux%2Famd64%20%7C%20linux%2Farm64-lightgrey)
[![License: MIT](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

MetricShell is a small container-native runtime wrapper that runs one CLI workload and exposes its metrics through a
Prometheus-compatible endpoint.

[Русская версия](README_RU.md)

## Why

Batch jobs, one-shot workers and CLI tools often need Prometheus metrics without becoming HTTP servers themselves.
MetricShell keeps that responsibility outside the workload: it supervises the process, accepts complete metric
snapshots through bounded local transports, serves `/metrics`, and preserves final metrics long enough for a scrape.

MetricShell gives serverless and non-HTTP workloads a normal Prometheus pull target without making the workload own an
HTTP metrics server or monitoring-infrastructure push configuration. The workload knows only a local socket or snapshot
path. Prometheus discovery, credentials and routing remain infrastructure concerns; application configuration never
needs `PROMETHEUS_URL`, `PUSHGATEWAY_URL` or an equivalent destination.

## When should I use MetricShell?

Use it for CLI programs, cron, Kubernetes Jobs/CronJobs, workers, queue consumers, batch/ETL processes, legacy PHP,
shell scripts, and long-running daemons that do not expose HTTP metrics. Choose snapshot mode when the workload owns a
complete registry; choose Managed Registry for local counter, gauge and histogram operations.

## When should I NOT use MetricShell?

If an HTTP application already exposes `/metrics` correctly through a standard Prometheus client library, MetricShell
usually adds no value. It is not a replacement for normal in-process instrumentation or a universal Pushgateway.

## Current status

Core and the optional Managed Aggregation profile are implemented and production-validated. Snapshot mode remains the
default complete-replacement contract; `managed-registry` adds bounded local counter, gauge and histogram operations
through a private Unix socket. The project is still pre-1.0, so the public runtime contract may change between minor
versions.

## Quick start

All project commands run through Docker. A host Go toolchain is not required.

```sh
cd implementation
make ci
```

Build a local runtime image:

```sh
cd implementation
make build PLATFORM=linux/amd64 IMAGE=metricshell
```

Copy the released binary into an existing application image:

```dockerfile
FROM ghcr.io/denki77/metricshell-artifact:0.2.0 AS metricshell
FROM my-application
COPY --from=metricshell /metricshell /usr/local/bin/metricshell
ENTRYPOINT ["/usr/local/bin/metricshell", "--"]
CMD ["my-worker"]
```

For production, replace the version tag with the release digest published on GitHub. Standalone installation and
deployable Kubernetes examples are in the [Production Deployment Guide](docs/07-delivery/production-deployment.md).

### Managed Aggregation

Run MetricShell in managed-registry mode when the workload should emit simple operations instead of maintaining a
complete snapshot:

```sh
metricshell --mode=managed-registry --exposition-listen=0.0.0.0:9090 -- /path/to/workload
```

From that workload (the default private socket is `/run/metricshell/managed.sock`):

```sh
metricshell managed declare jobs counter "Processed jobs"
metricshell managed counter-add jobs 1
metricshell managed declare queue_depth gauge "Queued jobs"
metricshell managed gauge-set queue_depth 7
metricshell managed declare job_seconds histogram "Job duration" - 0.1 1 +Inf
metricshell managed histogram-observe job_seconds 0.42
```

The path is: workload operation → local Unix socket → Managed Registry → periodically published complete snapshot →
Core → `/metrics` → Prometheus. An accepted operation is committed to the current registry epoch; it is not a promise
that Prometheus has already scraped it.

## When to use Managed Aggregation

Use it for CLI, cron, Kubernetes Job/CronJob, batch, ETL/import/export, legacy PHP or shell, one-shot workers, and
long-running workers that cannot conveniently own a Prometheus registry and HTTP endpoint. Snapshot mode is different:
the workload owns its registry and sends each complete snapshot. If a normal long-running HTTP service already uses a
native Prometheus client and can expose `/metrics`, it may not need MetricShell at all.

## Documentation

- [Production implementation](implementation/README.md)
- [Production deployment guide](docs/07-delivery/production-deployment.md)
- [Configuration](docs/04-specification/configuration.md)
- [Runtime state machine](docs/04-specification/runtime-state-machine.md)
- [Application snapshot protocol](docs/04-specification/application-snapshot-protocol.md)
- [Managed Aggregation](docs/04-specification/managed-aggregation.md)
- [Docker and Compose examples](docs/04-specification/docker-compose-examples.md)
- [Architecture decisions](docs/06-architecture/adr/README.md)
- [Architecture investigation](docs/05-architecture-investigation/architecture-investigation.md)
- [Benchmark research: INV-015](research/INV-015/README.md)

## Versioning

The repository-wide version lives in [`VERSION`](VERSION). The current version is `0.2.0`. Runtime artifacts also embed
the source revision supplied by CI or Make.

## Contributing

Issues and pull requests are welcome. Please start with [CONTRIBUTING.md](CONTRIBUTING.md), keep implementation work
inside `implementation/`, and run the Docker-only checks before opening a PR.

## License

MetricShell is released under the [MIT License](LICENSE).
