# Цели проекта

[English version](../../docs/02-project/project-goals.md)

## Назначение

Документ определяет долгосрочные цели MetricShell.

## Видение

MetricShell предоставляет переиспользуемый runtime-слой, который делает Prometheus-метрики доступными для CLI-нагрузок
без необходимости реализовывать HTTP endpoint в каждом приложении.

## Цели

- сохранить стандартную pull-модель Prometheus;
- упростить инструментирование CLI-приложений;
- уменьшить сложность развёртывания;
- одинаково работать в Docker, Docker Compose и Kubernetes;
- не зависеть от языка приложения;
- вынести наблюдаемость из бизнес-кода;
- сохранить сигналы, exit code и корректное завершение;
- подключаться как runtime-слой существующего образа.

## Не является целью

MetricShell не предназначен для того, чтобы:

- заменять Prometheus;
- заменять Pushgateway;
- заменять OpenTelemetry Collector;
- заменять sidecar exporters во всех сценариях;
- заменять application metric libraries.

Он предлагает другую deployment model с собственным набором компромиссов.

---
[Границы проекта](project-scope.md)

---
[README раздела](README.md) | [Общий README документации](../README.md)
