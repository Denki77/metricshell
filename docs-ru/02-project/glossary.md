# Глоссарий

[English version](../../docs/02-project/glossary.md)

## CLI

Command Line Interface — интерфейс командной строки.

## PID 1

Главный процесс внутри container, отвечающий за обработку signals и reaping child processes.

## Pull model

Модель monitoring, в которой система мониторинга периодически запрашивает metrics у target.

## Prometheus

Система monitoring и time-series database, которая преимущественно собирает metrics через scrape настроенных targets.

## Metrics endpoint

HTTP endpoint, публикующий metrics в формате Prometheus.

## Sidecar

Вспомогательный container, работающий рядом с основным application container.

## Pushgateway

Компонент Prometheus для отдельных push-based сценариев batch jobs.

## Runtime layer

Переиспользуемый execution layer, добавляемый в application image и предоставляющий runtime capabilities независимо от
business code.

## Оператор

Субъект, отвечающий за configuration, deployment и эксплуатацию MetricShell вместе с управляемым workload.

В зависимости от окружения оператором может быть:

- developer;
- DevOps engineer;
- CI/CD pipeline;
- Docker Compose;
- Kubernetes;
- другая orchestration platform.

Термин **Оператор** не означает Kubernetes Operator pattern, если это явно не указано.

---
[README раздела](README.md) | [Общий README документации](../README.md)
