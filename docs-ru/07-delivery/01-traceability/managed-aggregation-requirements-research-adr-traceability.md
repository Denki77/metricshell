# MetricShell: Трассировка Managed Aggregation

**Статус:** Черновик для планирования реализации  
**Дата:** 2026-09-25

## Назначение

Этот документ связывает принятые требования Managed Aggregation с завершёнными исследованиями и архитектурными решениями,
а затем сопоставляет их с планом реализации.

Нормативная цепочка:

```text
требование -> INV-016...INV-020 -> ADR-016...ADR-020 -> спецификация -> delivery issue -> acceptance test
```

Scope документа:

```text
FR-MA / NFR-MA -> ADR -> Specification -> ISSUE
```

## Статусы

- **Архитектурно закрыто** — принятые ADR определяют архитектурную политику.
- **Ожидает реализации** — архитектура принята, production-код и delivery verification ещё не завершены.
- **Требует доработки спецификации** — финальная нормативная спецификация Managed Aggregation должна быть завершена при реализации без изменения принятых ADR.

## Матрица трассировки

| Требование | Тема                           | Связанные ADR                      | Спецификация / контракт                                                                                                                                                                                                                       | Delivery issues                                                                    | Статус               | Что зафиксировано                                                                                                                  |
|------------|--------------------------------|------------------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|------------------------------------------------------------------------------------|----------------------|------------------------------------------------------------------------------------------------------------------------------------|
| FR-MA-001  | Опциональность                 | ADR-016, ADR-020                   | [Требования Managed Aggregation](../../03-requirements/FR-NFR-managed-aggregation-requirements-extension.md); [Конфигурация](../../04-specification/configuration.md)                                                                         | ISSUE-MA-001, ISSUE-MA-016                                                         | Ожидает реализации   | Managed Aggregation включается явно; snapshot mode остаётся режимом по умолчанию и не меняется.                                    |
| FR-MA-002  | Владение registry              | ADR-016, ADR-017, ADR-020          | [Поведенческая модель Managed Aggregation](../../04-specification/managed-aggregation-behavioral-model-draft.md); [Runtime State Machine](../../04-specification/runtime-state-machine.md)                                                    | ISSUE-MA-002, ISSUE-MA-003, ISSUE-MA-012                                           | Ожидает реализации   | Один execution владеет одним in-memory registry одной workload epoch.                                                              |
| FR-MA-003  | Операции instrumentation       | ADR-016, ADR-017                   | [Поведенческая модель Managed Aggregation](../../04-specification/managed-aggregation-behavioral-model-draft.md)                                                                                                                              | ISSUE-MA-002, ISSUE-MA-003, ISSUE-MA-005                                           | Ожидает реализации   | Семантика counter/gauge/classic histogram детерминирована и задаётся descriptor.                                                   |
| FR-MA-004  | Тонкие клиенты                 | ADR-016, ADR-018                   | [Поведенческая модель Managed Aggregation](../../04-specification/managed-aggregation-behavioral-model-draft.md)                                                                                                                              | ISSUE-MA-005, ISSUE-MA-007                                                         | Ожидает реализации   | Клиент отправляет operations и не обязан владеть полным registry.                                                                  |
| FR-MA-005  | Legacy-совместимость           | ADR-018                            | [Поведенческая модель Managed Aggregation](../../04-specification/managed-aggregation-behavioral-model-draft.md)                                                                                                                              | ISSUE-MA-007, ISSUE-MA-015                                                         | Ожидает реализации   | Unix stream protocol пригоден для PHP 5.4 и shell/CLI.                                                                             |
| FR-MA-006  | Несколько local publishers     | ADR-017, ADR-018                   | [Поведенческая модель Managed Aggregation](../../04-specification/managed-aggregation-behavioral-model-draft.md)                                                                                                                              | ISSUE-MA-004, ISSUE-MA-006, ISSUE-MA-014                                           | Ожидает реализации   | Несколько publishers используют один bounded serialized commit order.                                                              |
| FR-MA-007  | Граница complete snapshot      | ADR-004, ADR-016, ADR-019, ADR-020 | [Application Snapshot Protocol](../../04-specification/application-snapshot-protocol.md); [поведенческая модель Managed Aggregation](../../04-specification/managed-aggregation-behavioral-model-draft.md)                                    | ISSUE-MA-009, ISSUE-MA-010                                                         | Ожидает реализации   | Managed state входит в Core только как complete candidate snapshot.                                                                |
| FR-MA-008  | Переиспользование Core         | ADR-004, ADR-019, ADR-020          | [Application Snapshot Protocol](../../04-specification/application-snapshot-protocol.md); [Runtime State Machine](../../04-specification/runtime-state-machine.md)                                                                            | ISSUE-MA-010, ISSUE-MA-011, ISSUE-MA-012                                           | Ожидает реализации   | Managed mode переиспользует validation, atomic replacement, exposition и final-scrape lifecycle.                                   |
| FR-MA-009  | Интеграция lifecycle           | ADR-020                            | [Runtime State Machine](../../04-specification/runtime-state-machine.md); [поведенческая модель Managed Aggregation](../../04-specification/managed-aggregation-behavioral-model-draft.md)                                                    | ISSUE-MA-011, ISSUE-MA-012, ISSUE-MA-014                                           | Ожидает реализации   | Последовательность: close admission -> bounded drain -> freeze -> одна final candidate/install attempt -> существующее final wait. |
| FR-MA-010  | Execution epoch                | ADR-016, ADR-020                   | [Runtime State Machine](../../04-specification/runtime-state-machine.md)                                                                                                                                                                      | ISSUE-MA-003, ISSUE-MA-012, ISSUE-MA-014                                           | Ожидает реализации   | Новый execution начинает пустой epoch; persistence/replay отсутствуют.                                                             |
| FR-MA-011  | Безопасное отклонение          | ADR-016, ADR-017, ADR-019, ADR-020 | [Поведенческая модель Managed Aggregation](../../04-specification/managed-aggregation-behavioral-model-draft.md); [Runtime Defaults and Resource Limits](../../04-specification/runtime-defaults-and-resource-limits.md)                      | ISSUE-MA-002, ISSUE-MA-003, ISSUE-MA-004, ISSUE-MA-008, ISSUE-MA-012               | Ожидает реализации   | Invalid/conflicting/late/over-limit operations не повреждают committed valid state.                                                |
| FR-MA-012  | Atomic batches                 | ADR-016                            | [Поведенческая модель Managed Aggregation](../../04-specification/managed-aggregation-behavioral-model-draft.md)                                                                                                                              | ISSUE-MA-002, ISSUE-MA-003                                                         | Ожидает реализации   | Поддержанный batch применяется по правилу all-or-nothing.                                                                          |
| FR-MA-013  | Descriptors                    | ADR-016, ADR-017                   | [Поведенческая модель Managed Aggregation](../../04-specification/managed-aggregation-behavioral-model-draft.md)                                                                                                                              | ISSUE-MA-002, ISSUE-MA-003                                                         | Ожидает реализации   | Descriptor/type/HELP/labels/histogram shape имеют детерминированные conflict rules.                                                |
| FR-MA-014  | Resource bounds                | ADR-017, ADR-018, ADR-019          | [Конфигурация](../../04-specification/configuration.md); [грамматика значений](../../04-specification/configuration-value-grammar.md); [Runtime Defaults and Resource Limits](../../04-specification/runtime-defaults-and-resource-limits.md) | ISSUE-MA-004, ISSUE-MA-005, ISSUE-MA-006, ISSUE-MA-008, ISSUE-MA-013               | Ожидает реализации   | Frame, series, histogram, queue и прочие ресурсы bounded и observable.                                                             |
| FR-MA-015  | Изоляция отказов               | ADR-017, ADR-018, ADR-019, ADR-020 | [Поведенческая модель Managed Aggregation](../../04-specification/managed-aggregation-behavioral-model-draft.md); [Runtime State Machine](../../04-specification/runtime-state-machine.md)                                                    | ISSUE-MA-006, ISSUE-MA-008, ISSUE-MA-010, ISSUE-MA-011, ISSUE-MA-012, ISSUE-MA-014 | Ожидает реализации   | Malformed/slow/disconnected/crashing/late publisher не повреждает registry/Core state.                                             |
| FR-MA-016  | Контракты Core                 | ADR-004, ADR-016, ADR-019, ADR-020 | [Application Snapshot Protocol](../../04-specification/application-snapshot-protocol.md); [Runtime State Machine](../../04-specification/runtime-state-machine.md)                                                                            | ISSUE-MA-010, ISSUE-MA-011, ISSUE-MA-012, ISSUE-MA-016                             | Ожидает реализации   | Managed Aggregation остаётся перед Core и не создаёт второй Core path.                                                             |
| NFR-MA-001 | Простота интеграции            | ADR-018, ADR-020                   | [Конфигурация](../../04-specification/configuration.md); [примеры Docker/Compose](../../04-specification/docker-compose-examples.md)                                                                                                          | ISSUE-MA-001, ISSUE-MA-006, ISSUE-MA-007, ISSUE-MA-016                             | Ожидает реализации   | Достаточно одного binary/process; обязательные daemon, sidecar или внешний service не нужны.                                       |
| NFR-MA-002 | Обратная совместимость         | ADR-016, ADR-020                   | [Конфигурация](../../04-specification/configuration.md)                                                                                                                                                                                       | ISSUE-MA-001, ISSUE-MA-016                                                         | Ожидает реализации   | Интеграции snapshot mode работают без изменений.                                                                                   |
| NFR-MA-003 | Архитектурная изоляция         | ADR-016, ADR-017, ADR-019          | [Поведенческая модель Managed Aggregation](../../04-specification/managed-aggregation-behavioral-model-draft.md)                                                                                                                              | ISSUE-MA-001...ISSUE-MA-010                                                        | Ожидает реализации   | Parsing, semantics, owner, materialization и Core bridge разделены.                                                                |
| NFR-MA-004 | Детерминированность            | ADR-016, ADR-017, ADR-020          | [Поведенческая модель Managed Aggregation](../../04-specification/managed-aggregation-behavioral-model-draft.md)                                                                                                                              | ISSUE-MA-002, ISSUE-MA-003, ISSUE-MA-004, ISSUE-MA-012, ISSUE-MA-014               | Ожидает реализации   | Одинаковые accepted order и configuration дают детерминированный результат.                                                        |
| NFR-MA-005 | Bounded resources              | ADR-017, ADR-018, ADR-019, ADR-020 | [Конфигурация](../../04-specification/configuration.md); [Runtime Defaults and Resource Limits](../../04-specification/runtime-defaults-and-resource-limits.md)                                                                               | ISSUE-MA-004, ISSUE-MA-005, ISSUE-MA-006, ISSUE-MA-008, ISSUE-MA-011, ISSUE-MA-013 | Ожидает реализации   | Memory, framing, queueing, processing и finalization waits ограничены.                                                             |
| NFR-MA-006 | Наблюдаемость                  | ADR-017, ADR-019, ADR-020          | [Self-Metrics Specification](../../04-specification/self-metrics.md); [Structured Logging Specification](../../04-specification/structured-logging.md)                                                                                        | ISSUE-MA-013                                                                       | Ожидает реализации   | Acceptance, rejection, overload, limits, generations и lifecycle наблюдаемы.                                                       |
| NFR-MA-007 | Тестируемость                  | ADR-016...ADR-020                  | [Требования Managed Aggregation](../../03-requirements/FR-NFR-managed-aggregation-requirements-extension.md); [поведенческая модель Managed Aggregation](../../04-specification/managed-aggregation-behavioral-model-draft.md)                | ISSUE-MA-002...ISSUE-MA-015                                                        | Ожидает реализации   | Семантика независимо тестируется и покрывается E2E через Core.                                                                     |
| NFR-MA-008 | Воспроизводимость исследований | ADR-016...ADR-020                  | [Архитектурное исследование](../../05-architecture-investigation/managed-aggregation-architecture-investigation.md); [материалы исследования](../../05-architecture-investigation/managed-aggregation-architecture-research.md)               | ISSUE-MA-015, ISSUE-MA-016                                                         | Архитектурно закрыто | Implementation validation сохраняет дисциплину evidence.                                                                           |

## Трассировка архитектурных решений

| Capability                                         | Research | ADR     | Основные delivery issues                               |
|----------------------------------------------------|----------|---------|--------------------------------------------------------|
| Managed registry semantics                         | INV-016  | ADR-016 | ISSUE-MA-002, ISSUE-MA-003                             |
| Concurrent publishers, ordering, ACK, backpressure | INV-017  | ADR-017 | ISSUE-MA-004, ISSUE-MA-014                             |
| Legacy-клиент и локальный transport                | INV-018  | ADR-018 | ISSUE-MA-005, ISSUE-MA-006, ISSUE-MA-007               |
| Materialization и resource controls                | INV-019  | ADR-019 | ISSUE-MA-008, ISSUE-MA-009, ISSUE-MA-013, ISSUE-MA-015 |
| Lifecycle, freeze и интеграция Core                | INV-020  | ADR-020 | ISSUE-MA-010, ISSUE-MA-011, ISSUE-MA-012, ISSUE-MA-014 |

## Что нужно завершить в спецификациях

- promoted/update managed behavioral draft до accepted normative spec;
- configuration: explicit managed mode и bounded controls;
- protocol v1 и taxonomy ошибок;
- self-metrics и structured logging;
- runtime lifecycle по ADR-020;
- Docker/Compose examples;
- shell и PHP 5.4 usage.

## Индекс delivery issues

| Issue        | Тема                                                    | Основные требования                                                 |
|--------------|---------------------------------------------------------|---------------------------------------------------------------------|
| ISSUE-MA-001 | Конфигурация и bootstrap managed mode                   | FR-MA-001, NFR-MA-001, NFR-MA-002, NFR-MA-003                       |
| ISSUE-MA-002 | Domain model и descriptor semantics                     | FR-MA-003, FR-MA-011, FR-MA-012, FR-MA-013, NFR-MA-004              |
| ISSUE-MA-003 | Managed Registry и execution epoch                      | FR-MA-002, FR-MA-003, FR-MA-010, FR-MA-011, FR-MA-013               |
| ISSUE-MA-004 | Bounded single-owner mutation loop                      | FR-MA-006, FR-MA-011, FR-MA-014, NFR-MA-004, NFR-MA-005             |
| ISSUE-MA-005 | Operation protocol v1 и framing                         | FR-MA-003, FR-MA-004, FR-MA-014, FR-MA-015                          |
| ISSUE-MA-006 | Unix socket managed-operation server                    | FR-MA-006, FR-MA-014, FR-MA-015, NFR-MA-001                         |
| ISSUE-MA-007 | CLI helper и reference client для PHP 5.4               | FR-MA-004, FR-MA-005, NFR-MA-001                                    |
| ISSUE-MA-008 | Managed resource controls                               | FR-MA-011, FR-MA-014, FR-MA-015, NFR-MA-005                         |
| ISSUE-MA-009 | Generation-based immutable materialization              | FR-MA-007, FR-MA-014, NFR-MA-004, NFR-MA-005                        |
| ISSUE-MA-010 | Core candidate bridge и atomic installation             | FR-MA-007, FR-MA-008, FR-MA-015, FR-MA-016                          |
| ISSUE-MA-011 | Runtime admission barrier и bounded drain               | FR-MA-008, FR-MA-009, FR-MA-015, NFR-MA-005                         |
| ISSUE-MA-012 | Freeze, final snapshot, final-scrape integration        | FR-MA-002, FR-MA-008, FR-MA-009, FR-MA-010, FR-MA-011, FR-MA-016    |
| ISSUE-MA-013 | Managed self-metrics и structured diagnostics           | FR-MA-014, NFR-MA-006                                               |
| ISSUE-MA-014 | E2E suite для concurrency, shutdown, restart и failures | FR-MA-006, FR-MA-009, FR-MA-010, FR-MA-015, NFR-MA-004, NFR-MA-007  |
| ISSUE-MA-015 | Legacy, performance, resource и production validation   | FR-MA-005, FR-MA-014, FR-MA-015, NFR-MA-005, NFR-MA-007, NFR-MA-008 |
| ISSUE-MA-016 | Документация, примеры и готовность к release            | FR-MA-001, FR-MA-016, NFR-MA-001, NFR-MA-002, NFR-MA-008            |

## Сквозные инварианты

- snapshot mode остаётся default;
- managed mode включается явно;
- hybrid ownership не вводится;
- одна execution = одна managed epoch;
- persistence/replay между epochs нет;
- один bounded owner сериализует accepted mutations;
- success ACK только после commit;
- unknown outcome остаётся явным;
- managed state входит в Core только complete candidate snapshot;
- invalid/over-limit input не мутирует committed valid state частично;
- scrape-visible bytes принадлежат одной complete generation;
- finalization bounded и использует existing Core budgets;
- second Core path/daemon/mandatory sidecar/Kubernetes API dependency не добавляются;
- research coverage numbers не становятся production defaults.

## Completion gate

Managed Aggregation завершён только после ISSUE-MA-001 — ISSUE-MA-016, полного FR-MA/NFR-MA evidence,
зелёного snapshot regression suite, managed unit/integration/race/fault/E2E suites и синхронизации EN/RU.
