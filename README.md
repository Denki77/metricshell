# MetricShell

[![CI](https://github.com/Denki77/metricshell/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/Denki77/metricshell/actions/workflows/ci.yml)
[![Documentation](https://github.com/Denki77/metricshell/actions/workflows/docs.yml/badge.svg?branch=main)](https://github.com/Denki77/metricshell/actions/workflows/docs.yml)
![Version](https://img.shields.io/badge/version-0.1.2-blue)
![Go](https://img.shields.io/badge/go-1.26-00ADD8)
![Docker](https://img.shields.io/badge/runtime-Docker-2496ED)
![Platforms](https://img.shields.io/badge/platform-linux%2Famd64%20%7C%20linux%2Farm64-lightgrey)
[![License: MIT](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

MetricShell is a small container-native runtime wrapper that runs one CLI workload and exposes its metrics through a
Prometheus-compatible endpoint.

[Русская версия](README_RU.md)

## Why

Batch jobs, one-shot workers and CLI tools often need Prometheus metrics without becoming HTTP servers themselves.
MetricShell keeps that responsibility outside the workload: it supervises the process, accepts complete metric
snapshots through bounded local transports, serves `/metrics`, and preserves final metrics long enough for a scrape.

## Current status

Core is implemented and ready for operational use in the complete-snapshot profile: publishers replace the whole metric
set at once, and partial metric updates are intentionally not part of this release. The project is still pre-1.0, so the
public runtime contract may change between minor versions.

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

Run a workload without shell interpretation:

```sh
docker run --rm metricshell:local -- /path/to/workload "argument with spaces"
```

Everything after the standalone `--` is passed directly to the workload. Use an explicit shell workload if shell
features are needed.

## Documentation

- [Production implementation](implementation/README.md)
- [Configuration](docs/04-specification/configuration.md)
- [Runtime state machine](docs/04-specification/runtime-state-machine.md)
- [Application snapshot protocol](docs/04-specification/application-snapshot-protocol.md)
- [Docker and Compose examples](docs/04-specification/docker-compose-examples.md)
- [Architecture decisions](docs/06-architecture/adr/README.md)
- [Architecture investigation](docs/05-architecture-investigation/architecture-investigation.md)
- [Benchmark research: INV-015](research/INV-015/README.md)

## Versioning

The repository-wide version lives in [`VERSION`](VERSION). The current version is `0.1.2`. Runtime artifacts also embed
the source revision supplied by CI or Make.

## Contributing

Issues and pull requests are welcome. Please start with [CONTRIBUTING.md](CONTRIBUTING.md), keep implementation work
inside `implementation/`, and run the Docker-only checks before opening a PR.

## License

MetricShell is released under the [MIT License](LICENSE).
