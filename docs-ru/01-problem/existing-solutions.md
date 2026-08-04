# Существующие решения

[English version](../../docs/01-problem/existing-solutions.md)

> Статус: черновик

## Назначение

Документ рассматривает существующие подходы к публикации метрик CLI-приложений, workers и конечных workloads.

## Проблема

Prometheus основан на pull model: collectors периодически выполняют scrape HTTP endpoints наблюдаемых targets
[P-01](references.md#p-01--writing-exporters).

Для service-level batch jobs Prometheus рекомендует Pushgateway только в отдельных сценариях и документирует его
ограничения как общей замены pull scraping [P-02](references.md#p-02--when-to-use-the-pushgateway).

Для instance-level batch workloads Prometheus прямо указывает, что однозначного рекомендуемого pattern пока нет, и
перечисляет несколько альтернатив [P-01](references.md#p-01--writing-exporters). Поэтому требуется архитектурное
исследование, а не предположение об одном универсальном решении.

## Существующие подходы

### Встроенный HTTP endpoint

Ссылки: [P-01](references.md#p-01--writing-exporters), [P-03](references.md#p-03--exposition-formats).

Преимущества:

- нативная модель Prometheus;
- зрелые библиотеки;
- отсутствие дополнительной инфраструктуры.

Ограничения:

- monitoring становится ответственностью application;
- CLI application должно поднимать HTTP.

### Pushgateway

Ссылка: [P-02](references.md#p-02--when-to-use-the-pushgateway).

Преимущество: рекомендован для отдельных service-level batch jobs.

Ограничения:

- дополнительная инфраструктура;
- отдельный lifecycle;
- управление stale metrics;
- потеря обычной семантики `up`.

### Node Exporter Textfile Collector

Ссылка: [P-01](references.md#p-01--writing-exporters).

Преимущество: простота.

Ограничения: node scope и обязательный collector.

### Sidecar exporter

Ссылка: [K-01](references.md#k-01--sidecar-containers).

Преимущество: разделение ответственности.

Минусы: добавляет артефакт развёртывания и чаще связан с Kubernetes.

### OpenTelemetry

Ссылки: [O-01](references.md#o-01--specification), [O-02](references.md#o-02--metrics-specification).

Преимущества: vendor-neutral model и богатая telemetry.

Минусы: решает более широкую задачу, чем публикация Prometheus-метрик.

## Вывод

Универсально лучшего решения нет. Все варианты должны сравниваться по единой матрице критериев в ходе архитектурных
исследований.

См. также:

- [Источники](references.md);
- [Критерии оценки существующих решений](../03-requirements/existing-solutions-evaluation.md).
