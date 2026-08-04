# MetricShell: эпик реализации MetricShell Core

[English version](../../../docs/07-delivery/02-epics/EPIC-001-core.md)

**Статус:** Draft for implementation planning  
**Дата:** 2026-08-04

## Назначение

Этот документ является входом в реализацию MetricShell. Эпик декомпозирует реализацию на задачи, с учётом принятых АДР
для достижения результата, поставленного в начале проекта.

Нормативная цепочка:

```text
epic -> wave -> task -> acceptance test
```

## Архитектурные решения в порядке зависимости

| ADR     | Тема                    | Обязательный результат реализации                                                                        |
|---------|-------------------------|----------------------------------------------------------------------------------------------------------|
| ADR-001 | PID 1 and process model | MetricShell запускается как PID 1, владеет process group workload, пересылает сигналы и reap-ит потомков |
| ADR-002 | Workload lifecycle      | Формальная state machine запуска, работы, выхода workload и post-exit phase                              |
| ADR-003 | Shutdown budgeting      | Единый bounded shutdown budget, grace periods и escalation до принудительного завершения                 |
| ADR-004 | Metric-state semantics  | Последний валидный полный snapshot, whole-candidate validation и atomic replacement; без aggregation     |
| ADR-005 | Transport comparison    | Общий transport-independent publication contract и единые ошибки                                         |
| ADR-006 | File ingestion          | Atomic file publication, event notification и reconciliation fallback                                    |
| ADR-007 | Socket ingestion        | Framed local socket protocol, bounded requests и конкурентные complete candidates                        |
| ADR-008 | Local push              | Локальный HTTP/push adapter как дополнительный вход с тем же snapshot contract                           |
| ADR-009 | Shared memory/mmap      | mmap не является основным transport; отсутствие ABI-зависимости в core contract                          |
| ADR-010 | Prometheus exposition   | Один immutable complete snapshot на scrape; Prometheus text/OpenMetrics negotiation                      |
| ADR-011 | Final scrape semantics  | Freeze ingestion, N=1 по умолчанию, completed-response counting, finite timeout                          |
| ADR-012 | Kubernetes viability    | Bounded post-workload Pod lifetime, discovery и replica-specific validation                              |
| ADR-013 | Distribution            | Static amd64/arm64 artifacts, checksums, pinned multi-stage integration                                  |
| ADR-014 | Security and limits     | Non-root/local-only defaults и whole-candidate resource bounds                                           |
| ADR-015 | Final benchmark policy  | Bounded architecture, event-driven reconciliation и отдельная release performance certification          |

## Нормативный scope ADR-004

Одна успешная публикация означает один полный snapshot одного workload/application scope.

```text
complete candidate
-> complete validation
-> atomic replacement of last-valid snapshot
-> repeated safe scrapes
```

MetricShell не выполняет:

- merge независимых registries;
- summation snapshots;
- per-producer state;
- replay `increment`, `set`, `observe`;
- хранение истории snapshots;
- гарантированную доставку каждой версии;
- reconciliation нескольких producer timelines.

## Целевой production flow

```text
MetricShell PID 1
  |
  +-- start workload in owned process group
  |      |
  |      +-- client library / file / socket / local push
  |              |
  |              v
  |        complete candidate snapshot
  |              |
  |              v
  |        whole-candidate validation
  |              |
  |              v
  |        atomic active-state replacement
  |
  +-- Prometheus exposition of one immutable snapshot
  |
  +-- workload exits
         |
         +-- close ingestion
         +-- freeze last-valid application snapshot
         +-- bounded final scrape wait
         +-- drain, terminate descendants, return workload result
```

## Delivery waves

### Wave 1

Supervisor foundation and PID 1

**Цель:** создать минимальный корректный runtime, который уже может безопасно запускать workload как PID 1.

- [ISSUE-001. Инициализация production-модуля Go и команды](../03-issues/ISSUE-001/README.md)
- [ISSUE-002. Entrypoint PID 1 и разбор команды workload](../03-issues/ISSUE-002/README.md)
- [ISSUE-003. Управляемая process group/session](../03-issues/ISSUE-003/README.md)
- [ISSUE-004. Передача сигналов](../03-issues/ISSUE-004/README.md)
- [ISSUE-005. Reaping дочерних процессов и обработка orphan](../03-issues/ISSUE-005/README.md)
- [ISSUE-006. Сохранение результата workload](../03-issues/ISSUE-006/README.md)

**Wave 1 exit gate:** MetricShell может заменить простейший container entrypoint и корректно выполнять обязанности PID 1
даже без metrics ingestion.

### Wave 2

Lifecycle and shutdown coordination

- [ISSUE-007. State machine runtime lifecycle](../03-issues/ISSUE-007/README.md)
- [ISSUE-008. Модель shutdown budget](../03-issues/ISSUE-008/README.md)
- [ISSUE-009. Эскалация termination](../03-issues/ISSUE-009/README.md)
- [ISSUE-010. Контракт health и readiness](../03-issues/ISSUE-010/README.md)

**Wave 2 exit gate:** полный lifecycle deterministic, bounded и тестируемый до добавления transport adapters.

### Wave 3

Metric state core

- [ISSUE-011. Каноническая модель публикации](../03-issues/ISSUE-011/README.md)
- [ISSUE-012. Parser и validator полного candidate](../03-issues/ISSUE-012/README.md)
- [ISSUE-013. Atomic holder последнего валидного state](../03-issues/ISSUE-013/README.md)
- [ISSUE-014. Начальное zero-series state](../03-issues/ISSUE-014/README.md)
- [ISSUE-015. Отдельный домен self-metrics](../03-issues/ISSUE-015/README.md)

**Wave 3 exit gate:** production state core реализует ADR-004 независимо от транспорта.

### Wave 4

Transport contract and adapters

- [ISSUE-016. Общий transport-independent ingestion interface](../03-issues/ISSUE-016/README.md)
- [ISSUE-017. Protocol file publication](../03-issues/ISSUE-017/README.md)
- [ISSUE-018. Framed protocol Unix socket](../03-issues/ISSUE-018/README.md)
- [ISSUE-019. Сериализация writer официального client](../03-issues/ISSUE-019/README.md)
- [ISSUE-020. Local push HTTP adapter](../03-issues/ISSUE-020/README.md)
- [ISSUE-021. Закрепление mmap как non-primary](../03-issues/ISSUE-021/README.md)
- [ISSUE-022. Cross-adapter conformance suite](../03-issues/ISSUE-022/README.md)

**Wave 4 exit gate:** три заявленных способа интеграции работают на одном state core и не расходятся семантически.

### Wave 5

Exposition and final metrics

- [ISSUE-023. Сервер Prometheus exposition](../03-issues/ISSUE-023/README.md)
- [ISSUE-024. Pre-encoding response и limits](../03-issues/ISSUE-024/README.md)
- [ISSUE-025. Ingestion barrier finalization](../03-issues/ISSUE-025/README.md)
- [ISSUE-026. State machine final scrape](../03-issues/ISSUE-026/README.md)
- [ISSUE-027. Подсчёт complete responses и drain](../03-issues/ISSUE-027/README.md)
- [ISSUE-028. Наблюдаемость final wait](../03-issues/ISSUE-028/README.md)

**Wave 5 exit gate:** после workload exit MetricShell честно отдаёт frozen final snapshot и завершается в budget.

### Wave 6

Kubernetes, distribution, hardening and release evidence

- [ISSUE-029. Интеграция Kubernetes Job](../03-issues/ISSUE-029/README.md)
- [ISSUE-030. Lifecycle controls Kubernetes](../03-issues/ISSUE-030/README.md)
- [ISSUE-031. Integration test нескольких реплик Prometheus](../03-issues/ISSUE-031/README.md)
- [ISSUE-032. Статические multi-arch release artifacts](../03-issues/ISSUE-032/README.md)
- [ISSUE-033. Defaults container hardening](../03-issues/ISSUE-033/README.md)
- [ISSUE-034. Настраиваемые capacity и timeout limits](../03-issues/ISSUE-034/README.md)
- [ISSUE-035. Набор fault, soak и race tests](../03-issues/ISSUE-035/README.md)
- [ISSUE-036. Controlled release benchmark suite](../03-issues/ISSUE-036/README.md)
- [ISSUE-037. Release supply-chain pipeline](../03-issues/ISSUE-037/README.md)

**Wave 6 exit gate:** release candidate пригоден для production pilot и имеет доказуемую operational envelope.

## Трассировка specifications к issues

| Принятая specification или contract                                                                                          | Delivery issues                                                            |
|------------------------------------------------------------------------------------------------------------------------------|----------------------------------------------------------------------------|
| [Runtime State Machine](../../04-specification/runtime-state-machine.md)                                                     | ISSUE-007…ISSUE-010, ISSUE-025…ISSUE-028                                   |
| [Application Snapshot Protocol](../../04-specification/application-snapshot-protocol.md)                                     | ISSUE-011…ISSUE-014, ISSUE-016…ISSUE-022                                   |
| [Configuration](../../04-specification/configuration.md)                                                                     | ISSUE-001, ISSUE-008, ISSUE-010, ISSUE-016…ISSUE-020, ISSUE-023, ISSUE-034 |
| [Metric Filtering](../../04-specification/metrics-filtering.md)                                                              | ISSUE-023, ISSUE-024                                                       |
| [Self-Metrics](../../04-specification/self-metrics.md)                                                                       | ISSUE-015, ISSUE-023, ISSUE-028                                            |
| [Structured Logging](../../04-specification/structured-logging.md)                                                           | ISSUE-028, ISSUE-034, ISSUE-035                                            |
| [Defaults и resource limits](../../04-specification/runtime-defaults-and-resource-limits.md)                                 | ISSUE-008, ISSUE-017…ISSUE-020, ISSUE-023, ISSUE-026, ISSUE-034            |
| [Docker и Compose examples](../../04-specification/docker-compose-examples.md)                                               | ISSUE-029, ISSUE-030, ISSUE-032, ISSUE-035, ISSUE-037                      |
| Registry внутренних exit codes в [Configuration](../../04-specification/configuration.md#собственные-exit-codes-metricshell) | ISSUE-002, ISSUE-006, ISSUE-034                                            |
| [Спецификация грамматических значений конфигурации](../../04-specification/configuration-value-grammar.md)                   | ISSUE-001, ISSUE-008, ISSUE-034                                            |

## Requirement-to-delivery traceability

| Capability                  | INV     | ADR     | Tasks                           |
|-----------------------------|---------|---------|---------------------------------|
| PID 1/process ownership     | INV-001 | ADR-001 | ISSUE-002...ISSUE-005           |
| Workload lifecycle/result   | INV-002 | ADR-002 | ISSUE-006, ISSUE-007, ISSUE-010 |
| Shutdown budget/escalation  | INV-003 | ADR-003 | ISSUE-004, ISSUE-008, ISSUE-009 |
| Complete snapshot semantics | INV-004 | ADR-004 | ISSUE-011...ISSUE-015           |
| Common transport contract   | INV-005 | ADR-005 | ISSUE-016, ISSUE-022            |
| File ingestion              | INV-006 | ADR-006 | ISSUE-017, ISSUE-022            |
| Socket ingestion            | INV-007 | ADR-007 | ISSUE-018, ISSUE-019, ISSUE-022 |
| Local push                  | INV-008 | ADR-008 | ISSUE-020, ISSUE-022            |
| mmap/shared memory          | INV-009 | ADR-009 | ISSUE-021                       |
| Exposition                  | INV-010 | ADR-010 | ISSUE-023, ISSUE-024            |
| Final scrape                | INV-011 | ADR-011 | ISSUE-025...ISSUE-028           |
| Kubernetes                  | INV-012 | ADR-012 | ISSUE-029...ISSUE-031           |
| Distribution                | INV-013 | ADR-013 | ISSUE-032, ISSUE-037            |
| Security/limits             | INV-014 | ADR-014 | ISSUE-033...ISSUE-035           |
| Final benchmarks            | INV-015 | ADR-015 | ISSUE-036                       |

## Cross-cutting Definition of Done

Каждая production task считается завершённой только если:

- реализована без копирования prototype architecture как production shortcut;
- имеет unit tests и требуемые integration/e2e tests;
- проходит Go race detector для concurrent code;
- сохраняет ADR-004 semantics;
- имеет bounded errors/timeouts/resources;
- добавляет bounded diagnostics/self-metrics;
- синхронно обновляет английскую и русскую документацию;
- содержит ссылку `Requirement -> INV -> ADR -> specification -> task -> test`;
- не вводит aggregation без нового INV/ADR.
