# Нефункциональные требования

[English version](../../docs/03-requirements/non-functional-requirements.md)

> Статус: черновик для архитектурных исследований

## Назначение

Документ определяет атрибуты качества и измеримые инженерные ожидания. Итоговые числовые значения устанавливаются
прототипами и benchmarks; `TBD` обозначает значения, которые должно определить архитектурное исследование.

## Надёжность

### NFR-REL-001 — Независимость workload

Ошибка ingestion или exposition метрик по умолчанию НЕ ДОЛЖНА завершать workload.

### NFR-REL-002 — Детерминированный lifecycle

Одинаковый результат workload, configuration и последовательность внешних событий ДОЛЖНЫ приводить к одинаковому
lifecycle outcome.

### NFR-REL-003 — Ограниченное ожидание

Shutdown, ingestion flush, ожидание scrape и ожидание дочерних процессов ДОЛЖНЫ быть ограничены по времени.

### NFR-REL-004 — Локализация падений

Malformed input, client disconnects, concurrent scrapes и неподдерживаемые значения НЕ ДОЛЖНЫ приводить к падению
**MetricShell**.

### NFR-REL-005 — Согласованность финального состояния

Финальные application metrics ДОЛЖНЫ оставаться согласованными между post-workload scrapes.

### NFR-REL-006 — Наблюдаемость отказов

Ошибки ingestion и exposition ДОЛЖНЫ быть внешне наблюдаемы через документированное HTTP behavior, self-metrics,
логи либо их сочетание.

## Производительность

### NFR-PERF-001 — Startup overhead

Startup overhead без учёта запуска workload ДОЛЖЕН измеряться в документированной reference environment.

> Целевое значение для stable release: `TBD`.

### NFR-PERF-002 — Производительность ingestion

Для каждого transport ДОЛЖНЫ быть документированы sustained throughput и latency при низкой, средней и высокой
интенсивности updates.

> Целевые значения: `TBD`.

### NFR-PERF-003 — Scrape latency

Для registry в пределах поддерживаемых ограничений scrape latency ДОЛЖНА быть существенно ниже обычного scrape timeout
Prometheus.

> Точный размер registry и целевая latency: `TBD`.

### NFR-PERF-004 — Resource overhead

ДОЛЖНЫ быть измерены и опубликованы CPU и memory overhead для idle mode, reference registry, sustained ingestion
и concurrent scrapes.

### NFR-PERF-005 — Backpressure

При исчерпании capacity ДОЛЖНА применяться ограниченная policy отклонения, timeout, dropping либо replacement.
Неограниченные queues запрещены.

## Масштабируемость

### NFR-SCALE-001 — Принудительно применяемые limits

**MetricShell** ДОЛЖЕН применять настраиваемые limits для series, labels, payload, connections и memory-related
resources.

### NFR-SCALE-002 — Предсказуемая деградация

При достижении limit поведение ДОЛЖНО быть детерминированным и не допускать неконтролируемого роста ресурсов.

### NFR-SCALE-003 — Diagnostics cardinality

Отклонения из-за cardinality limits ДОЛЖНЫ быть наблюдаемыми.

## Переносимость

### NFR-PORT-001 — Независимость от orchestrator

Core executable НЕ ДОЛЖЕН зависеть от Kubernetes API или Kubernetes-only conventions.

### NFR-PORT-002 — OCI compatibility

**MetricShell** ДОЛЖЕН поддерживать OCI-compatible Linux containers. Другие платформы МОГУТ добавляться отдельно.

### NFR-PORT-003 — Минимальные предположения

Standalone integration СЛЕДУЕТ не требовать shell, init system, package manager или language runtime сверх явно
документированных требований.

### NFR-PORT-004 — Воспроизводимое происхождение

Опубликованные артефакты ДОЛЖНЫ содержать source revision и release version.

## Корректность процессов

### NFR-PROC-001 — Корректность PID 1

При работе как PID 1 MetricShell ДОЛЖЕН корректно выполнять обязанности выбранной process model.

### NFR-PROC-002 — Process groups

Семантика завершения child и descendant processes ДОЛЖНА быть определена и протестирована.

### NFR-PROC-003 — Целостность exit outcome

Ошибка workload НЕ ДОЛЖНА неявно превращаться в success. Приоритет runtime failure и forced termination ДОЛЖЕН быть
документирован.

## Безопасность

### NFR-SEC-001 — Работа без root

**MetricShell** ДОЛЖЕН поддерживать non-root operation.

### NFR-SEC-002 — Локальный ingestion по умолчанию

Socket- и push-ingestion по умолчанию ДОЛЖНЫ быть недоступны за пределами контейнера workload, если внешняя доступность
не включена явно.

### NFR-SEC-003 — Недоверенные входные данные

Имена метрик, labels, values, files, frames и requests ДОЛЖНЫ валидироваться как недоверенные входные данные.

### NFR-SEC-004 — Защита от исчерпания ресурсов

Oversized payloads, чрезмерное число connections, series и labels, а также slow clients ДОЛЖНЫ быть ограничены.

### NFR-SEC-005 — Безопасность secrets

**MetricShell** НЕ ДОЛЖЕН намеренно публиковать environment variables, arguments, credentials или file contents
как labels либо в обычных логах.

### NFR-SEC-006 — Настраиваемый HTTP bind

Bind address scrape endpoint ДОЛЖЕН настраиваться. Внешний bind ДОЛЖЕН быть документирован как security decision.

## Совместимость

### NFR-COMP-001 — Совместимость exposition

Stable releases ДОЛЖНЫ указывать поддерживаемые Prometheus/OpenMetrics formats и проверять output официальными
либо standards-compatible tools.

### NFR-COMP-002 — Versioning protocols

Socket- и push-протоколы ДОЛЖНЫ иметь compatibility strategy до stable release.

### NFR-COMP-003 — Versioning file format

Собственный structured file format ДОЛЖЕН иметь явную schema version. Прямое использование внешнего versioned standard
освобождается от этого требования.

### NFR-COMP-004 — Client compatibility matrix

Проект ДОЛЖЕН публиковать compatibility matrix runtime/clients.

## Сопровождаемость

### NFR-MAINT-001 — Разделение документации

Requirements, behavioral specification, architecture и ADR ДОЛЖНЫ оставаться отдельными слоями.

### NFR-MAINT-002 — Traceability

Каждое обязательное функциональное требование ДОЛЖНО быть связано с автоматизированным acceptance coverage.

### NFR-MAINT-003 — Покрытие ADR

Process model, protocols, final-scrape semantics и failure precedence ДОЛЖНЫ быть зафиксированы в ADR.

### NFR-MAINT-004 — Stable surface

Configuration, protocols и self-metrics НЕ ДОЛЖНЫ объявляться stable до появления compatibility policy.

## Эксплуатационная пригодность

### NFR-OPS-001 — Причина ожидания

Операторы ДОЛЖНЫ иметь возможность определить, почему **MetricShell** остаётся активным после завершения workload.

### NFR-OPS-002 — Effective configuration

Effective non-secret configuration СЛЕДУЕТ делать наблюдаемой.

### NFR-OPS-003 — Health semantics

Health/readiness behavior ДОЛЖНО быть документировано для каждой lifecycle phase.

### NFR-OPS-004 — Troubleshooting

Для каждого transport и shutdown mode ДОЛЖНА существовать документация по отказам и troubleshooting.

## Тестирование

### NFR-TEST-001 — Concurrency

Concurrent ingestion, scrape, завершение workload и signals ДОЛЖНЫ иметь автоматизированное race/concurrency coverage.

### NFR-TEST-002 — Container E2E

Docker- и Kubernetes-модели использования ДОЛЖНЫ иметь end-to-end tests до stable release.

### NFR-TEST-003 — Fault injection

Tests ДОЛЖНЫ покрывать malformed input, disconnects, stalled shutdown, bind failure, races замены файла и scrape
timeout.

### NFR-TEST-004 — Benchmarks

Release documentation СЛЕДУЕТ публиковать benchmark methodology и results.
