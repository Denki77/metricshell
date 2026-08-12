# MetricShell

> Статус: черновик

**MetricShell** — контейнерный runtime-wrapper для публикации Prometheus-метрик CLI-нагрузок.

[English documentation](README.md)

## Что это такое?

MetricShell запускает произвольную команду как управляемый дочерний процесс и публикует метрики приложения через HTTP
endpoint, совместимый с Prometheus. Самому приложению при этом не требуется поднимать собственный HTTP-сервер.

## Текущий статус

Архитектура Core завершена. Production-реализация ведётся в каталоге [`implementation/`](implementation/README_RU.md).

## Документация

См. каталог [`docs-ru/`](docs-ru/README.md).
