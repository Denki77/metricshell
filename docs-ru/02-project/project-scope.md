# Границы проекта

[English version](../../docs/02-project/project-scope.md)

## Входит в проект

- runtime-wrapper контейнерной нагрузки;
- управление дочерним процессом;
- Prometheus-совместимый endpoint;
- локальный приём метрик;
- самонаблюдаемость runtime;
- настраиваемое завершение;
- Docker, Compose и Kubernetes.

## Не входит

- хранение метрик;
- service discovery;
- конфигурация Prometheus;
- алертинг;
- локальная или распределённая агрегация значений метрик между producers;
- сбор логов и трассировок;
- мониторинг хоста;
- проектирование бизнес-метрик.

MetricShell не хранит producer-scoped contributions и не агрегирует значения метрик. Workload и его libraries должны
публиковать один полный, не содержащий конфликтов application snapshot.
