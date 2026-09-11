# MetricShell

[![CI](https://github.com/Denki77/metricshell/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/Denki77/metricshell/actions/workflows/ci.yml)
[![Documentation](https://github.com/Denki77/metricshell/actions/workflows/docs.yml/badge.svg?branch=main)](https://github.com/Denki77/metricshell/actions/workflows/docs.yml)
![Version](https://img.shields.io/badge/version-0.1.2-blue)
![Go](https://img.shields.io/badge/go-1.26-00ADD8)
![Docker](https://img.shields.io/badge/runtime-Docker-2496ED)
![Platforms](https://img.shields.io/badge/platform-linux%2Famd64%20%7C%20linux%2Farm64-lightgrey)
[![License: MIT](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

MetricShell — небольшой container-native runtime-wrapper: он запускает одну CLI-нагрузку и публикует её метрики через
Prometheus-compatible endpoint.

[English version](README.md)

## Зачем

Batch jobs, одноразовые workers и CLI-инструменты часто нуждаются в Prometheus-метриках, но не должны сами становиться
HTTP-серверами. MetricShell выносит эту обязанность наружу: управляет процессом, принимает complete metric snapshots
через bounded local transports, отдаёт `/metrics` и удерживает финальные метрики достаточно долго для scrape.

## Текущий статус

Core реализован и готов к эксплуатации в complete-snapshot profile: publishers заменяют весь набор метрик целиком, а
partial metric updates намеренно не входят в этот release. Проект всё ещё pre-1.0, поэтому публичный runtime contract
может меняться между minor versions.

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

Запустить workload без shell interpretation:

```sh
docker run --rm metricshell:local -- /path/to/workload "argument with spaces"
```

Всё после отдельного `--` передаётся напрямую в workload. Если нужны shell features, используйте shell как явный
workload.

## Документация

- [Production implementation](implementation/README_RU.md)
- [Configuration](docs-ru/04-specification/configuration.md)
- [Runtime state machine](docs-ru/04-specification/runtime-state-machine.md)
- [Application snapshot protocol](docs-ru/04-specification/application-snapshot-protocol.md)
- [Docker and Compose examples](docs-ru/04-specification/docker-compose-examples.md)
- [Architecture decisions](docs-ru/06-architecture/adr/README.md)
- [Architecture investigation](docs-ru/05-architecture-investigation/architecture-investigation.md)
- [Benchmark research: INV-015](research/INV-015/README_ru.md)

## Версионирование

Версия всего репозитория находится в [`VERSION`](VERSION). Текущая версия — `0.1.2`. Runtime artifacts также встраивают
source revision, переданный CI или Make.

## Участие

Issues и pull requests приветствуются. Начните с [CONTRIBUTING.md](CONTRIBUTING.md), держите production-код в
`implementation/` и запускайте Docker-only checks перед PR.

## Лицензия

MetricShell распространяется под [MIT License](LICENSE).
