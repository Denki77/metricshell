# MetricShell

[![CI](https://github.com/Denki77/metricshell/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/Denki77/metricshell/actions/workflows/ci.yml)
[![Documentation](https://github.com/Denki77/metricshell/actions/workflows/docs.yml/badge.svg?branch=main)](https://github.com/Denki77/metricshell/actions/workflows/docs.yml)
[![Release](https://github.com/Denki77/metricshell/actions/workflows/release.yml/badge.svg)](https://github.com/Denki77/metricshell/actions/workflows/release.yml)
![Version](https://img.shields.io/badge/version-0.2.0-blue)
![Go](https://img.shields.io/badge/go-1.26-00ADD8)
![Platforms](https://img.shields.io/badge/platform-linux%2Famd64%20%7C%20linux%2Farm64-lightgrey)
[![License: MIT](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

MetricShell — небольшой container-native runtime-wrapper: он запускает одну CLI-нагрузку и публикует её метрики через
Prometheus-compatible endpoint.

[English version](README.md)

## Зачем

Batch jobs, одноразовые workers и CLI-инструменты часто нуждаются в Prometheus-метриках, но не должны сами становиться
HTTP-серверами. MetricShell выносит эту обязанность наружу: управляет процессом, принимает complete metric snapshots
через bounded local transports, отдаёт `/metrics` и удерживает финальные метрики достаточно долго для scrape.

MetricShell даёт serverless- и non-HTTP-нагрузкам обычную pull-цель для Prometheus, не заставляя workload поднимать
HTTP-сервер или хранить конфигурацию push в monitoring infrastructure. Workload знает только локальный socket или путь
snapshot. Discovery, credentials и routing принадлежат инфраструктуре; приложению не нужны `PROMETHEUS_URL`,
`PUSHGATEWAY_URL` и аналогичные destination variables.

## Когда нужен MetricShell?

Для CLI, cron, Kubernetes Job/CronJob, worker, queue consumer, batch/ETL, legacy PHP, shell script и long-running daemon
без собственного HTTP metrics endpoint. Snapshot mode подходит workload с complete registry, Managed Registry — для
локальных операций counter, gauge и histogram.

## Когда MetricShell не нужен?

Если HTTP-приложение уже корректно публикует `/metrics` стандартной Prometheus client library, MetricShell обычно
избыточен. Это не замена обычной in-process instrumentation и не универсальная замена Pushgateway.

## Текущий статус

Core и optional Managed Aggregation profile реализованы и production-validated. Snapshot mode остаётся default
complete-replacement contract; `managed-registry` добавляет bounded local operations для counter, gauge и histogram
через private Unix socket. Проект всё ещё pre-1.0, поэтому публичный runtime contract может меняться между minor
versions.

## Быстрый старт

Все команды проекта выполняются через Docker. Host Go toolchain не требуется.

```sh
cd implementation
make ci
```

Собрать локальный runtime image:

```sh
cd implementation
make build PLATFORM=linux/amd64 IMAGE=metricshell
```

Добавить released binary в существующий application image:

```dockerfile
FROM ghcr.io/denki77/metricshell-artifact:0.2.0 AS metricshell
FROM my-application
COPY --from=metricshell /metricshell /usr/local/bin/metricshell
ENTRYPOINT ["/usr/local/bin/metricshell", "--"]
CMD ["my-worker"]
```

В production замените version tag на digest из GitHub Release. Standalone installation и deployable Kubernetes
examples описаны в [Production Deployment Guide](docs-ru/07-delivery/production-deployment.md).

### Managed Aggregation

Используйте managed-registry mode, когда workload должен отправлять простые операции вместо ведения полного snapshot:

```sh
metricshell --mode=managed-registry --exposition-listen=0.0.0.0:9090 -- /path/to/workload
```

Из workload (default private socket — `/run/metricshell/managed.sock`):

```sh
metricshell managed declare jobs counter "Processed jobs"
metricshell managed counter-add jobs 1
metricshell managed declare queue_depth gauge "Queued jobs"
metricshell managed gauge-set queue_depth 7
metricshell managed declare job_seconds histogram "Job duration" - 0.1 1 +Inf
metricshell managed histogram-observe job_seconds 0.42
```

Путь данных: workload operation → local Unix socket → Managed Registry → периодически публикуемый complete snapshot →
Core → `/metrics` → Prometheus. Accepted operation committed в registry текущей epoch, но это не обещание, что
Prometheus уже получил её через scrape.

## Когда использовать Managed Aggregation

Режим подходит для CLI, cron, Kubernetes Job/CronJob, batch, ETL/import/export, legacy PHP или shell, one-shot и
long-running workers, которым неудобно поддерживать собственный Prometheus registry и HTTP endpoint. В snapshot mode
workload сам владеет registry и отправляет каждый complete snapshot. Обычному long-running HTTP service с native
Prometheus client и собственным `/metrics` MetricShell может вообще не требоваться.

## Документация

- [Production implementation](implementation/README_RU.md)
- [Production deployment guide](docs-ru/07-delivery/production-deployment.md)
- [Configuration](docs-ru/04-specification/configuration.md)
- [Runtime state machine](docs-ru/04-specification/runtime-state-machine.md)
- [Application snapshot protocol](docs-ru/04-specification/application-snapshot-protocol.md)
- [Managed Aggregation](docs-ru/04-specification/managed-aggregation.md)
- [Docker and Compose examples](docs-ru/04-specification/docker-compose-examples.md)
- [Architecture decisions](docs-ru/06-architecture/adr/README.md)
- [Architecture investigation](docs-ru/05-architecture-investigation/architecture-investigation.md)
- [Benchmark research: INV-015](research/INV-015/README_ru.md)

## Версионирование

Версия всего репозитория находится в [`VERSION`](VERSION). Текущая версия — `0.2.0`. Runtime artifacts также встраивают
source revision, переданный CI или Make.

## Участие

Issues и pull requests приветствуются. Начните с [CONTRIBUTING.md](CONTRIBUTING.md), держите production-код в
`implementation/` и запускайте Docker-only checks перед PR.

## Лицензия

MetricShell распространяется под [MIT License](LICENSE).
