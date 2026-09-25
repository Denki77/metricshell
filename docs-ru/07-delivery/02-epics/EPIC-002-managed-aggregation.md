# EPIC-002: Реализация Managed Aggregation

**Статус:** Черновик для планирования реализации  
**Дата:** 2026-09-25

## Назначение

Этот эпик — полный план реализации опционального Managed Aggregation после завершения INV-016 — INV-020 и принятия
ADR-016 — ADR-020.

Нормативная цепочка:

```text
epic -> wave -> issue -> implementation -> acceptance test
```

## Архитектурные решения в порядке зависимостей

| ADR     | Тема                         | Обязательный результат                                                                                                           |
|---------|------------------------------|----------------------------------------------------------------------------------------------------------------------------------|
| ADR-016 | Managed Registry semantics   | Typed descriptor-driven registry, deterministic counter/gauge/histogram semantics, conflict rejection, empty epoch per execution |
| ADR-017 | Concurrency, ordering и ACK  | Один bounded single owner, registry-wide commit order, success ACK после commit, overload/unknown boundaries                     |
| ADR-018 | Local operation transport    | Versioned bounded NDJSON через Unix socket, one operation per connection, stateless clients                                      |
| ADR-019 | Materialization и limits     | Generation-based immutable cache, complete generation per representation, bounded controls                                       |
| ADR-020 | Lifecycle и Core integration | Close admission, bounded drain, single freeze, final materialization, existing Core install/final wait                           |

ADR-003 и ADR-004 Core остаются authoritative.

## Целевой production flow

```text
MetricShell PID 1
  |
  +-- snapshot mode (default, без изменений)
  |
  +-- managed-registry mode
  |      |
  |      +-- local publishers
  |             v
  |        Unix protocol v1
  |             v
  |        bounded owner queue
  |             v
  |        serialized Managed Registry
  |             v
  |        committed generation
  |             v
  |        immutable materialization
  |             v
  |        complete Core candidate
  |             v
  |        existing Core validation/install
  |
  +-- existing exposition
  |
  +-- workload exit
         +-- close admission
         +-- bounded drain
         +-- freeze
         +-- final generation
         +-- Core install
         +-- existing final wait
```

## Delivery waves

### Wave 1 — Mode boundary и semantic core

- [ISSUE-MA-001. Конфигурация и bootstrap managed mode](../03-issues/ISSUE-MA-001.md)
- [ISSUE-MA-002. Domain model и descriptor semantics Managed Aggregation](../03-issues/ISSUE-MA-002.md)
- [ISSUE-MA-003. Managed Registry и execution epoch](../03-issues/ISSUE-MA-003.md)

**Exit gate:** snapshot default не меняется, managed mode explicit, ADR-016 semantics реализованы и покрыты unit tests.

### Wave 2 — Serialized mutation ownership

- [ISSUE-MA-004. Bounded single-owner mutation loop](../03-issues/ISSUE-MA-004.md)

**Exit gate:** один commit order, bounded admission, observable overload, race-clean concurrency.

### Wave 3 — Protocol, Unix transport и thin clients

- [ISSUE-MA-005. Operation protocol v1 и bounded framing](../03-issues/ISSUE-MA-005.md)
- [ISSUE-MA-006. Unix socket server managed operations](../03-issues/ISSUE-MA-006.md)
- [ISSUE-MA-007. CLI helper и PHP 5.4 reference client](../03-issues/ISSUE-MA-007.md)

**Exit gate:** реальные clients работают без registry ownership; malformed/partial/version/error classes deterministic.

### Wave 4 — Resource controls и immutable materialization

- [ISSUE-MA-008. Resource controls Managed Aggregation](../03-issues/ISSUE-MA-008.md)
- [ISSUE-MA-009. Generation-based immutable materialization](../03-issues/ISSUE-MA-009.md)

**Exit gate:** series/histogram/queue/frame bounded; reject-before-mutation; response принадлежит одной complete generation.

### Wave 5 — Core bridge и runtime lifecycle

- [ISSUE-MA-010. Core candidate bridge и atomic installation](../03-issues/ISSUE-MA-010.md)
- [ISSUE-MA-011. Runtime admission barrier и bounded drain](../03-issues/ISSUE-MA-011.md)
- [ISSUE-MA-012. Freeze, final snapshot и final-scrape integration](../03-issues/ISSUE-MA-012.md)

**Exit gate:** один Core path; close admission first; bounded drain; single freeze; existing Core install/final wait unchanged.

### Wave 6 — Observability и production verification

- [ISSUE-MA-013. Managed self-metrics и structured diagnostics](../03-issues/ISSUE-MA-013.md)
- [ISSUE-MA-014. E2E suite concurrency, shutdown, restart и failures](../03-issues/ISSUE-MA-014.md)

**Exit gate:** bounded observability, production E2E, race detector, failure/shutdown coverage.

### Wave 7 — Compatibility, performance validation и release readiness

- [ISSUE-MA-015. Legacy, performance, resource и production validation](../03-issues/ISSUE-MA-015.md)
- [ISSUE-MA-016. Документация, примеры и готовность к release](../03-issues/ISSUE-MA-016.md)

**Exit gate:** PHP 5.4/shell compatibility, production validation, normative spec, EN/RU parity, snapshot regression green.

## Описание issues

### [ISSUE-MA-001](../03-issues/ISSUE-MA-001.md)

Explicit managed mode, snapshot default, invalid/hybrid config rejection, bootstrap managed subsystem only when enabled.

### [ISSUE-MA-002](../03-issues/ISSUE-MA-002.md)

Descriptor/family/series types, canonical labels, counter/gauge/histogram semantics, conflicts, ADR-016 batch decision.

### [ISSUE-MA-003](../03-issues/ISSUE-MA-003.md)

One in-memory registry per execution, generation tracking, new empty epoch, no persistence/replay.

### [ISSUE-MA-004](../03-issues/ISSUE-MA-004.md)

One owner loop, bounded queue, per-connection order, global commit order, post-commit success, overload rejection.

### [ISSUE-MA-005](../03-issues/ISSUE-MA-005.md)

Versioned NDJSON request/response, one op/connection, bounded parser, deterministic protocol errors.

### [ISSUE-MA-006](../03-issues/ISSUE-MA-006.md)

Unix socket endpoint, permissions/lifecycle, startup readiness, connection handling, close-admission.

### [ISSUE-MA-007](../03-issues/ISSUE-MA-007.md)

Предоставить удобный для shell CLI и stateless-клиент для PHP 5.4 с правилами retry и unknown outcome.

### [ISSUE-MA-008](../03-issues/ISSUE-MA-008.md)

Configurable series, histogram bucket, queue, frame limits with reject-before-mutation.

### [ISSUE-MA-009](../03-issues/ISSUE-MA-009.md)

Immutable generation cache, reuse/invalidation, concurrent slow-reader safety.

### [ISSUE-MA-010](../03-issues/ISSUE-MA-010.md)

Преобразовать complete managed generation в существующий Core candidate и переиспользовать validation/atomic install.

### [ISSUE-MA-011](../03-issues/ISSUE-MA-011.md)

Close admission on exit/termination, drain only admitted work within remaining ADR-003 budget.

### [ISSUE-MA-012](../03-issues/ISSUE-MA-012.md)

Single freeze, late rejection, final generation, existing Core final-wait delegation.

### [ISSUE-MA-013](../03-issues/ISSUE-MA-013.md)

Bounded self-metrics/logging for operations, rejection, overload, limits, queue, generations, materialization, freeze/finalization.

### [ISSUE-MA-014](../03-issues/ISSUE-MA-014.md)

E2E: publishers, ACK loss, malformed/partial, overload, natural exit, SIGTERM, budget exhaustion, late publishers, restart, final scrape.

### [ISSUE-MA-015](../03-issues/ISSUE-MA-015.md)

Проверить PHP 5.4/shell и ключевые сценарии INV-019/020 на production implementation; сохранить raw evidence и environment fingerprint.

### [ISSUE-MA-016](../03-issues/ISSUE-MA-016.md)

Завершить нормативную managed-спецификацию и обновить configuration, limits, self-metrics, logging, lifecycle, примеры, EN/RU traceability и release notes.

## Трассировка спецификаций к задачам

| Нормативная спецификация                                                                                                                                      | Задачи реализации                                                                                                                                                                                                                                                                                                                |
|---------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [Требования Managed Aggregation](../../03-requirements/FR-NFR-managed-aggregation-requirements-extension.md)                                                  | [ISSUE-MA-001](../03-issues/ISSUE-MA-001.md) — [ISSUE-MA-016](../03-issues/ISSUE-MA-016.md)                                                                                                                                                                                                                                      |
| [Поведенческая модель Managed Aggregation](../../04-specification/managed-aggregation-behavioral-model-draft.md)                                              | [ISSUE-MA-002](../03-issues/ISSUE-MA-002.md), [ISSUE-MA-003](../03-issues/ISSUE-MA-003.md), [ISSUE-MA-004](../03-issues/ISSUE-MA-004.md), [ISSUE-MA-005](../03-issues/ISSUE-MA-005.md), [ISSUE-MA-008](../03-issues/ISSUE-MA-008.md), [ISSUE-MA-009](../03-issues/ISSUE-MA-009.md), [ISSUE-MA-012](../03-issues/ISSUE-MA-012.md) |
| [Application Snapshot Protocol](../../04-specification/application-snapshot-protocol.md)                                                                      | [ISSUE-MA-009](../03-issues/ISSUE-MA-009.md), [ISSUE-MA-010](../03-issues/ISSUE-MA-010.md), [ISSUE-MA-012](../03-issues/ISSUE-MA-012.md)                                                                                                                                                                                         |
| [Runtime State Machine](../../04-specification/runtime-state-machine.md)                                                                                      | [ISSUE-MA-001](../03-issues/ISSUE-MA-001.md), [ISSUE-MA-003](../03-issues/ISSUE-MA-003.md), [ISSUE-MA-006](../03-issues/ISSUE-MA-006.md), [ISSUE-MA-011](../03-issues/ISSUE-MA-011.md), [ISSUE-MA-012](../03-issues/ISSUE-MA-012.md), [ISSUE-MA-014](../03-issues/ISSUE-MA-014.md)                                               |
| [Configuration Specification](../../04-specification/configuration.md) и [Configuration Value Grammar](../../04-specification/configuration-value-grammar.md) | [ISSUE-MA-001](../03-issues/ISSUE-MA-001.md), [ISSUE-MA-004](../03-issues/ISSUE-MA-004.md), [ISSUE-MA-005](../03-issues/ISSUE-MA-005.md), [ISSUE-MA-006](../03-issues/ISSUE-MA-006.md), [ISSUE-MA-008](../03-issues/ISSUE-MA-008.md), [ISSUE-MA-011](../03-issues/ISSUE-MA-011.md), [ISSUE-MA-016](../03-issues/ISSUE-MA-016.md) |
| [Runtime Defaults and Resource Limits](../../04-specification/runtime-defaults-and-resource-limits.md)                                                        | [ISSUE-MA-004](../03-issues/ISSUE-MA-004.md), [ISSUE-MA-005](../03-issues/ISSUE-MA-005.md), [ISSUE-MA-006](../03-issues/ISSUE-MA-006.md), [ISSUE-MA-008](../03-issues/ISSUE-MA-008.md), [ISSUE-MA-011](../03-issues/ISSUE-MA-011.md), [ISSUE-MA-015](../03-issues/ISSUE-MA-015.md)                                               |
| [Self-Metrics Specification](../../04-specification/self-metrics.md) и [Structured Logging Specification](../../04-specification/structured-logging.md)       | [ISSUE-MA-013](../03-issues/ISSUE-MA-013.md), [ISSUE-MA-014](../03-issues/ISSUE-MA-014.md), [ISSUE-MA-016](../03-issues/ISSUE-MA-016.md)                                                                                                                                                                                         |
| [Примеры Docker/Compose](../../04-specification/docker-compose-examples.md)                                                                                   | [ISSUE-MA-006](../03-issues/ISSUE-MA-006.md), [ISSUE-MA-007](../03-issues/ISSUE-MA-007.md), [ISSUE-MA-016](../03-issues/ISSUE-MA-016.md)                                                                                                                                                                                         |

## Трассировка требований к реализации

| Требование | Задачи реализации                                                                                                                                                                                                                                                                                                                                                                                                                                                          |
|------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| FR-MA-001  | [ISSUE-MA-001](../03-issues/ISSUE-MA-001.md), [ISSUE-MA-016](../03-issues/ISSUE-MA-016.md)                                                                                                                                                                                                                                                                                                                                                                                 |
| FR-MA-002  | [ISSUE-MA-002](../03-issues/ISSUE-MA-002.md), [ISSUE-MA-003](../03-issues/ISSUE-MA-003.md), [ISSUE-MA-012](../03-issues/ISSUE-MA-012.md)                                                                                                                                                                                                                                                                                                                                   |
| FR-MA-003  | [ISSUE-MA-002](../03-issues/ISSUE-MA-002.md), [ISSUE-MA-003](../03-issues/ISSUE-MA-003.md), [ISSUE-MA-005](../03-issues/ISSUE-MA-005.md)                                                                                                                                                                                                                                                                                                                                   |
| FR-MA-004  | [ISSUE-MA-005](../03-issues/ISSUE-MA-005.md), [ISSUE-MA-007](../03-issues/ISSUE-MA-007.md)                                                                                                                                                                                                                                                                                                                                                                                 |
| FR-MA-005  | [ISSUE-MA-007](../03-issues/ISSUE-MA-007.md), [ISSUE-MA-015](../03-issues/ISSUE-MA-015.md)                                                                                                                                                                                                                                                                                                                                                                                 |
| FR-MA-006  | [ISSUE-MA-004](../03-issues/ISSUE-MA-004.md), [ISSUE-MA-006](../03-issues/ISSUE-MA-006.md), [ISSUE-MA-014](../03-issues/ISSUE-MA-014.md)                                                                                                                                                                                                                                                                                                                                   |
| FR-MA-007  | [ISSUE-MA-009](../03-issues/ISSUE-MA-009.md), [ISSUE-MA-010](../03-issues/ISSUE-MA-010.md)                                                                                                                                                                                                                                                                                                                                                                                 |
| FR-MA-008  | [ISSUE-MA-010](../03-issues/ISSUE-MA-010.md), [ISSUE-MA-011](../03-issues/ISSUE-MA-011.md), [ISSUE-MA-012](../03-issues/ISSUE-MA-012.md)                                                                                                                                                                                                                                                                                                                                   |
| FR-MA-009  | [ISSUE-MA-011](../03-issues/ISSUE-MA-011.md), [ISSUE-MA-012](../03-issues/ISSUE-MA-012.md), [ISSUE-MA-014](../03-issues/ISSUE-MA-014.md)                                                                                                                                                                                                                                                                                                                                   |
| FR-MA-010  | [ISSUE-MA-003](../03-issues/ISSUE-MA-003.md), [ISSUE-MA-012](../03-issues/ISSUE-MA-012.md), [ISSUE-MA-014](../03-issues/ISSUE-MA-014.md)                                                                                                                                                                                                                                                                                                                                   |
| FR-MA-011  | [ISSUE-MA-002](../03-issues/ISSUE-MA-002.md), [ISSUE-MA-003](../03-issues/ISSUE-MA-003.md), [ISSUE-MA-004](../03-issues/ISSUE-MA-004.md), [ISSUE-MA-008](../03-issues/ISSUE-MA-008.md), [ISSUE-MA-012](../03-issues/ISSUE-MA-012.md)                                                                                                                                                                                                                                       |
| FR-MA-012  | [ISSUE-MA-002](../03-issues/ISSUE-MA-002.md), [ISSUE-MA-003](../03-issues/ISSUE-MA-003.md)                                                                                                                                                                                                                                                                                                                                                                                 |
| FR-MA-013  | [ISSUE-MA-002](../03-issues/ISSUE-MA-002.md), [ISSUE-MA-003](../03-issues/ISSUE-MA-003.md)                                                                                                                                                                                                                                                                                                                                                                                 |
| FR-MA-014  | [ISSUE-MA-004](../03-issues/ISSUE-MA-004.md), [ISSUE-MA-005](../03-issues/ISSUE-MA-005.md), [ISSUE-MA-006](../03-issues/ISSUE-MA-006.md), [ISSUE-MA-008](../03-issues/ISSUE-MA-008.md), [ISSUE-MA-013](../03-issues/ISSUE-MA-013.md)                                                                                                                                                                                                                                       |
| FR-MA-015  | [ISSUE-MA-006](../03-issues/ISSUE-MA-006.md), [ISSUE-MA-008](../03-issues/ISSUE-MA-008.md), [ISSUE-MA-010](../03-issues/ISSUE-MA-010.md), [ISSUE-MA-011](../03-issues/ISSUE-MA-011.md), [ISSUE-MA-012](../03-issues/ISSUE-MA-012.md), [ISSUE-MA-014](../03-issues/ISSUE-MA-014.md)                                                                                                                                                                                         |
| FR-MA-016  | [ISSUE-MA-010](../03-issues/ISSUE-MA-010.md), [ISSUE-MA-011](../03-issues/ISSUE-MA-011.md), [ISSUE-MA-012](../03-issues/ISSUE-MA-012.md), [ISSUE-MA-016](../03-issues/ISSUE-MA-016.md)                                                                                                                                                                                                                                                                                     |
| NFR-MA-001 | [ISSUE-MA-001](../03-issues/ISSUE-MA-001.md), [ISSUE-MA-006](../03-issues/ISSUE-MA-006.md), [ISSUE-MA-007](../03-issues/ISSUE-MA-007.md), [ISSUE-MA-016](../03-issues/ISSUE-MA-016.md)                                                                                                                                                                                                                                                                                     |
| NFR-MA-002 | [ISSUE-MA-001](../03-issues/ISSUE-MA-001.md), [ISSUE-MA-016](../03-issues/ISSUE-MA-016.md)                                                                                                                                                                                                                                                                                                                                                                                 |
| NFR-MA-003 | [ISSUE-MA-001](../03-issues/ISSUE-MA-001.md), [ISSUE-MA-002](../03-issues/ISSUE-MA-002.md), [ISSUE-MA-003](../03-issues/ISSUE-MA-003.md), [ISSUE-MA-004](../03-issues/ISSUE-MA-004.md), [ISSUE-MA-005](../03-issues/ISSUE-MA-005.md), [ISSUE-MA-006](../03-issues/ISSUE-MA-006.md), [ISSUE-MA-007](../03-issues/ISSUE-MA-007.md), [ISSUE-MA-008](../03-issues/ISSUE-MA-008.md), [ISSUE-MA-009](../03-issues/ISSUE-MA-009.md), [ISSUE-MA-010](../03-issues/ISSUE-MA-010.md) |
| NFR-MA-004 | [ISSUE-MA-002](../03-issues/ISSUE-MA-002.md), [ISSUE-MA-003](../03-issues/ISSUE-MA-003.md), [ISSUE-MA-004](../03-issues/ISSUE-MA-004.md), [ISSUE-MA-012](../03-issues/ISSUE-MA-012.md), [ISSUE-MA-014](../03-issues/ISSUE-MA-014.md)                                                                                                                                                                                                                                       |
| NFR-MA-005 | [ISSUE-MA-004](../03-issues/ISSUE-MA-004.md), [ISSUE-MA-005](../03-issues/ISSUE-MA-005.md), [ISSUE-MA-006](../03-issues/ISSUE-MA-006.md), [ISSUE-MA-008](../03-issues/ISSUE-MA-008.md), [ISSUE-MA-011](../03-issues/ISSUE-MA-011.md), [ISSUE-MA-013](../03-issues/ISSUE-MA-013.md)                                                                                                                                                                                         |
| NFR-MA-006 | [ISSUE-MA-013](../03-issues/ISSUE-MA-013.md)                                                                                                                                                                                                                                                                                                                                                                                                                               |
| NFR-MA-007 | [ISSUE-MA-002](../03-issues/ISSUE-MA-002.md) — [ISSUE-MA-015](../03-issues/ISSUE-MA-015.md)                                                                                                                                                                                                                                                                                                                                                                                |
| NFR-MA-008 | [ISSUE-MA-015](../03-issues/ISSUE-MA-015.md), [ISSUE-MA-016](../03-issues/ISSUE-MA-016.md)                                                                                                                                                                                                                                                                                                                                                                                 |

## Сквозной Definition of Done

Каждая задача сохраняет snapshot mode default, принятые ADR semantics, bounded resources/waits, Core ADR-004 semantics,
race-safety, bounded observability, EN/RU parity и цепочку requirement -> INV -> ADR -> specification -> issue -> test.
