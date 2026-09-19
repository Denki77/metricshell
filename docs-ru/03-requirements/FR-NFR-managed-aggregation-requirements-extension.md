# Расширение Scope и Requirements — Managed Aggregation

- **Статус:** предлагаемое расширение scope
- **Относится к:** MetricShell
- **Зависит от:** завершённого scope MetricShell Core и ADR-001 — ADR-015
- **Режим по умолчанию:** Core snapshot mode
- **Опциональный режим:** managed aggregation

## 1. Назначение

Этот документ расширяет MetricShell опциональной управляемой агрегацией, не изменяя завершённую архитектуру MetricShell Core.

Core остаётся самостоятельно используемым и является режимом по умолчанию. Расширение предназначено для workloads, которые не могут или не должны самостоятельно поддерживать полный Prometheus-совместимый registry: legacy-приложений, CLI, shell-скриптов и multiprocess workers.

## 2. Проблема

MetricShell Core принимает полные application metric snapshots. Это подходит, когда workload уже владеет полным состоянием метрик.

Для legacy- и простых клиентов требование самостоятельно хранить counters, gauges, histogram buckets, descriptors, labels, обеспечивать concurrency и сериализовать полный snapshot переносит слишком большую ответственность в workload.

Таким клиентам естественно отправлять операции:

```text
increment processed_total
set queue_depth 12
observe request_duration_seconds 0.42
```

Поэтому MetricShell нужен опциональный владелец агрегированного состояния application metrics.

## 3. Расширение Scope

MetricShell ДОЛЖЕН поддерживать опциональный managed-registry mode.

В этом режиме MetricShell владеет in-memory application metric registry одного логического запуска workload. Клиенты отправляют instrumentation operations. MetricShell применяет принятые операции и формирует полные application snapshots для существующего пути Core.

```text
workload
    ↓ instrumentation operations
managed aggregation
    ↓ полный application snapshot
MetricShell Core
    ↓
Prometheus / OpenMetrics
```

Агрегация располагается перед существующей snapshot-границей Core.

## 4. Продуктовая модель и развёртывание

MetricShell ДОЛЖЕН оставаться:

- одним продуктом;
- одним исполняемым бинарником;
- одним интеграционным слоем container image;
- одним runtime-процессом, управляющим одним логическим запуском workload;
- небольшим модульным монолитом с явными внутренними архитектурными границами.

Managed aggregation НЕ ДОЛЖЕН требовать второго бинарника, отдельного daemon, обязательного sidecar или внешнего центрального сервиса агрегации.

## 5. Режимы работы

### 5.1 Snapshot mode по умолчанию

```text
metricshell
```

по поведению эквивалентен явному запуску:

```text
metricshell --mode=snapshot
```

Workload владеет application registry и публикует полные snapshots.

### 5.2 Managed-registry mode

Агрегация включается явно, концептуально:

```text
metricshell --mode=managed-registry
```

Точный CLI/configuration contract определяется после архитектурных исследований и проектирования реализации.

### 5.3 Изоляция режимов

Первое расширение НЕ ДОЛЖНО требовать hybrid registry, смешивающего внешние полные snapshots и managed operations. Такая композиция требует отдельного исследования и изменения scope.

## 6. Функциональные требования

### FR-MA-001 — Опциональность

Managed aggregation ДОЛЖЕН быть опциональным и НЕ ДОЛЖЕН менять поведение snapshot mode по умолчанию.

### FR-MA-002 — Владение registry

В managed mode MetricShell ДОЛЖЕН владеть application metric registry в течение одного запуска MetricShell и одного логического workload.

### FR-MA-003 — Instrumentation operations

Managed mode ДОЛЖЕН поддерживать операции, достаточные для counters, gauges и histogram observations. Точная семантика и wire representation являются предметом архитектурных исследований.

### FR-MA-004 — Тонкие клиенты

Клиенты НЕ ДОЛЖНЫ быть обязаны поддерживать полный metric registry или формировать полные Prometheus snapshots.

### FR-MA-005 — Legacy-совместимость

Архитектура ДОЛЖНА явно проверить интеграцию PHP 5.4 и shell/CLI без обязательной современной Prometheus client library.

### FR-MA-006 — Несколько локальных publishers

Managed mode ДОЛЖЕН поддерживать несколько взаимодействующих publishers одного логического workload с учётом исследованных правил ordering, concurrency и ownership. Совместное использование registry независимыми приложениями вне scope.

### FR-MA-007 — Граница полного snapshot

Managed aggregation ДОЛЖЕН сформировать полный application snapshot до передачи состояния в существующий snapshot-путь Core.

### FR-MA-008 — Повторное использование Core

Managed mode ДОЛЖЕН повторно использовать существующие validation, atomic replacement, exposition, lifecycle, post-exit и final-scrape механизмы Core.

### FR-MA-009 — Интеграция с lifecycle

Архитектура ДОЛЖНА определить startup, normal exit, signal shutdown, in-flight operations, registry freeze, final snapshot, final scrape и restart behavior.

### FR-MA-010 — Epoch запуска

Новый запуск MetricShell по умолчанию ДОЛЖЕН создавать новую epoch managed registry. Persistent restoration между рестартами находится вне этого расширения.

### FR-MA-011 — Безопасное отклонение

Некорректные или конфликтующие операции НЕ ДОЛЖНЫ повреждать уже валидное состояние registry. Семантика acceptance/rejection ДОЛЖНА быть детерминированной.

### FR-MA-012 — Atomic batches

Архитектурные исследования ДОЛЖНЫ определить необходимость atomic batch для связанных операций. Если batches поддерживаются, отклонённый batch НЕ ДОЛЖЕН применяться частично.

### FR-MA-013 — Descriptors

Архитектура ДОЛЖНА определить ownership и lifecycle metric type, HELP metadata, label schema и histogram configuration, включая детерминированную обработку конфликтов.

### FR-MA-014 — Ограничение ресурсов

Managed mode ДОЛЖЕН защищаться от неограниченных payload, series/cardinality, labels, histogram configuration, concurrent publishers, queued work и memory consumption.

### FR-MA-015 — Изоляция отказов

Malformed, slow, disconnected или crashing publisher НЕ ДОЛЖЕН повреждать валидный registry или нарушать Core exposition последнего валидного состояния.

### FR-MA-016 — Контракты Core

Managed aggregation ДОЛЖЕН сохранять принятые контракты Core. Любое необходимое изменение принятого Core-контракта требует явного изменения scope и superseding ADR.

## 7. Нефункциональные требования

### NFR-MA-001 — Простота интеграции

Расширение ДОЛЖНО сохранять существующую low-friction модель: один слой MetricShell в image workload и явный выбор режима. Для локальной агрегации не требуется дополнительный сервис.

### NFR-MA-002 — Обратная совместимость

Существующие snapshot-mode интеграции ДОЛЖНЫ продолжить работать без изменений.

### NFR-MA-003 — Архитектурная изоляция

Parsing операций, mutation registry и aggregation semantics ДОЛЖНЫ быть отделены явными внутренними границами модулей.

### NFR-MA-004 — Детерминированность

При одинаковом порядке принятых операций и конфигурации семантика registry ДОЛЖНА быть детерминированной. Правила concurrency ordering ДОЛЖНЫ быть явными.

### NFR-MA-005 — Ограниченное потребление ресурсов

Memory, buffering, concurrency и input processing ДОЛЖНЫ иметь явные ограничения либо доказуемо ограниченное поведение.

### NFR-MA-006 — Наблюдаемость

MetricShell ДОЛЖЕН предоставлять достаточные self-metrics для диагностики acceptance, rejection, overload и resource limits managed mode без переноса неограниченной cardinality application labels в self-metrics.

### NFR-MA-007 — Тестируемость

Aggregation semantics ДОЛЖНЫ независимо тестироваться и иметь end-to-end покрытие через Core.

### NFR-MA-008 — Воспроизводимость исследований

Архитектурные решения ДОЛЖНЫ подтверждаться воспроизводимыми исследованиями с дисциплиной evidence, принятой для Core.

## 8. Явно вне Scope

Расширение не добавляет:

- distributed aggregation между экземплярами MetricShell;
- центральный aggregation cluster;
- persistent application metric state;
- гарантированную историю/доставку каждой instrumentation operation;
- remote write;
- Prometheus HA deduplication;
- совместный registry для независимых workloads;
- автоматическое объединение независимых полных snapshots;
- второй исполняемый файл MetricShell;
- обязательный sidecar;
- hybrid ownership snapshots + operations.

## 9. Совместимость с Core

Существующая граница Core сохраняется:

```text
полный candidate snapshot
→ валидация candidate целиком
→ атомарная замена active state
→ exposition
```

Managed aggregation добавляет только нового producer:

```text
instrumentation operations
→ managed registry
→ полный candidate snapshot
→ существующий Core
```

Core не становится обработчиком instrumentation operations и не объединяет независимые registries.

## 10. Необходимые архитектурные исследования

До фиксации реализации отдельные исследования ДОЛЖНЫ покрыть как минимум:

- managed-registry semantics;
- concurrent publishers;
- legacy-client viability;
- performance и resource limits;
- lifecycle integration.

Исследования отслеживаются в Managed Aggregation Research Plan. Их выводы должны быть зафиксированы отдельными ADR.

## 11. Критерии принятия расширения Scope

Расширение принимается, когда проект фиксирует:

1. MetricShell Core остаётся архитектурно завершённым и не меняет ответственность.
2. Snapshot mode остаётся режимом по умолчанию.
3. Managed aggregation является явно включаемым режимом того же бинарника MetricShell.
4. Aggregation архитектурно изолирован внутри небольшого модульного монолита.
5. Клиенты могут публиковать instrumentation operations без владения полным metric state.
6. Aggregation формирует полные snapshots для существующего Core-контракта.
7. Нерешённая семантика определяется архитектурными исследованиями, а не преждевременно продуктовыми требованиями.

Принятие документа разрешает начать цикл исследований managed aggregation. Сам документ не выбирает synchronization algorithms, framing, структуры хранения или детали протокола.
