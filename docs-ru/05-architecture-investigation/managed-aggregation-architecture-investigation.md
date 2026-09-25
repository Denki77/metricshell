# Архитектурное исследование Managed Aggregation

> Статус: запланировано
> Назначение: исследовать архитектуру опционального расширения Managed Aggregation до принятия ADR и реализации
> Scope: семантика managed registry, concurrent publishers, применимость legacy clients, performance/resource limits, lifecycle и интеграция с Core
> Зависит от: завершённых INV-001–INV-015, принятых ADR-001–ADR-015, требований и черновой спецификации Managed Aggregation

## 1. Назначение

Этот документ запускает новый цикл архитектурных исследований для опционального расширения Managed Aggregation.

Завершённая архитектура MetricShell Core этим исследованием не открывается заново. Расширение рассматривается как новый upstream producer полных application snapshots для существующей границы Core.

Исследование должно определить, можно ли реализовать требуемое managed-registry поведение, сохранив продуктовые ограничения:

- один продукт MetricShell;
- один исполняемый бинарник;
- один интеграционный слой container image;
- один runtime-процесс, управляющий одним логическим запуском workload;
- snapshot mode остаётся режимом по умолчанию;
- managed aggregation включается явно;
- не требуется sidecar, центральный daemon или внешний aggregation service;
- существующие контракты Core по lifecycle, snapshots, exposition и final scrape сохраняются.

## 2. Метод исследования

Каждый INV следует той же evidence-модели, которая использовалась для MetricShell Core:

1. **Вопрос** — один конкретный архитектурный вопрос.
2. **Контекст** — почему он важен и какие requirements/specification clauses затрагивает.
3. **Кандидаты** — реалистичные альтернативы, включая максимально простые варианты.
4. **Начальные гипотезы** — фальсифицируемые ожидания, записанные до эксперимента.
5. **Необходимые доказательства** — документация, prototype, integration test, fault injection, benchmark или source inspection.
6. **Эксперименты** — воспроизводимое окружение, workload, inputs, repetitions и measurements.
7. **Assertions** — переносимые correctness conditions, не зависящие от конкретных timing.
8. **Observations** — измерения и поведение, зависящие от окружения.
9. **Критерии оценки** — задаются до получения результатов.
10. **Результаты** — сохраняемые raw evidence.
11. **Вывод** — принять, отклонить, оставить fallback, отложить либо признать evidence недостаточным.
12. **Decision output** — ADR, обновление specification/requirements, benchmark record либо новое исследование.

Assertions должны описывать семантику или safety properties. Timing, throughput, CPU, RSS и scheduler behavior считаются observations, если requirement не задаёт жёсткую границу.

## 3. Существующая архитектурная граница

Принятая граница Core остаётся неизменной:

```text
полный candidate snapshot
→ валидация candidate целиком
→ атомарная замена active state
→ Prometheus/OpenMetrics exposition
```

Managed Aggregation может добавить только upstream path:

```text
instrumentation operations
→ managed registry
→ полный candidate snapshot
→ существующий Core
```

Исследования не должны неявно превращать Core в operation log, event store, distributed registry или merge engine.

## 4. Порядок исследований

1. [INV-016 — Managed Registry Semantics](managed-aggregation-architecture-research.md#inv-016)
2. [INV-017 — Concurrent Publishers and Ordering](managed-aggregation-architecture-research.md#inv-017)
3. [INV-018 — Legacy Client and Transport Viability](managed-aggregation-architecture-research.md#inv-018)
4. [INV-019 — Performance, Snapshot Materialization and Resource Limits](managed-aggregation-architecture-research.md#inv-019)
5. [INV-020 — Lifecycle and Core Integration](managed-aggregation-architecture-research.md#inv-020)

INV-016 выполняется первым, потому что последующие исследования должны тестировать уже определённую семантическую модель.

INV-017 и INV-018 могут выполняться параллельно после того, как INV-016 достаточно зафиксирует operations и ownership для прототипов.

INV-019 требует репрезентативный candidate implementation из INV-016/017.

INV-020 проверяет полную lifecycle-интеграцию и должен использовать выбранную семантику предыдущих исследований.

## 5. Tracking исследований

| ID      | Тема                                                                                          | Статус    | Evidence            | Решение                                      |
|---------|-----------------------------------------------------------------------------------------------|-----------|---------------------|----------------------------------------------|
| INV-016 | [Managed Registry Semantics](managed-aggregation-architecture-research.md#inv-016)            | Завершено | `research/INV-016/` | [ADR-016](../06-architecture/adr/ADR-016.md) |
| INV-017 | [Concurrent Publishers and Ordering](managed-aggregation-architecture-research.md#inv-017)    | Завершено | `research/INV-017/` | [ADR-017](../06-architecture/adr/ADR-017.md) |
| INV-018 | [Legacy Client and Transport Viability](managed-aggregation-architecture-research.md#inv-018) | Planned   | `research/INV-018/` | ADR-018 либо rejected-alternative record     |
| INV-019 | [Performance and Resource Limits](managed-aggregation-architecture-research.md#inv-019)       | Planned   | `research/INV-019/` | ADR-019 / benchmark и limit specification    |
| INV-020 | [Lifecycle and Core Integration](managed-aggregation-architecture-research.md#inv-020)        | Planned   | `research/INV-020/` | ADR-020 / lifecycle specification update     |

Нумерация ADR предварительная. Отдельное исследование не обязано порождать отдельный ADR: несколько INV могут дать один ADR, а один INV может привести к нескольким независимым решениям.

## 6. Сквозные инварианты исследований

Следующие положения являются входными ограничениями, а не выводами исследований:

- snapshot mode остаётся режимом по умолчанию;
- managed aggregation включается явно;
- Core остаётся самостоятельно используемым без managed aggregation;
- один managed registry принадлежит одному логическому запуску workload;
- независимые приложения не разделяют один managed registry;
- новый запуск MetricShell по умолчанию создаёт новую managed-registry epoch;
- persistence/replay между рестартами MetricShell не требуется;
- managed state передаётся в Core как полный application snapshot;
- invalid managed input не должен повреждать ранее валидное state;
- расширение остаётся внутри того же MetricShell binary/process model;
- initial extension не требует hybrid ownership внешних snapshots и managed operations;
- нельзя предполагать гарантированное хранение каждой промежуточной operation или snapshot без явного изменения product scope.

Если evidence показывает, что одно из этих ограничений невозможно либо создаёт неприемлемую проблему, это не разрешает нарушить его неявно: исследование должно остановиться и предложить изменение requirements/scope.

## 7. Дисциплина Evidence

Каждый пакет `research/INV-016` — `research/INV-020` должен по необходимости сохранять:

- README с вопросом и способом запуска;
- prototype source;
- deterministic assertions;
- raw observations/results;
- environment fingerprint;
- commit SHA и benchmark-scope fingerprint;
- команду воспроизведения;
- report с разделением фактов и интерпретации;
- явные ссылки на затронутые requirements/specification clauses.

Зависящие от окружения результаты не должны объявляться переносимыми гарантиями.

Отрицательные результаты являются полноценным evidence и сохраняются, если они отвергают привлекательный candidate.

## 8. Критерии завершения

Цикл Managed Aggregation достаточно завершён для production implementation, когда:

- однозначно определена семантика operations для counter, gauge и histogram;
- зафиксированы descriptor и series identity rules;
- concurrency и ordering имеют детерминированную семантику;
- принято решение по duplicate/idempotency;
- интеграция legacy PHP/shell доказана либо явно отклонена evidence;
- выбор transport/protocol имеет воспроизводимые доказательства;
- определены resource bounds и overload behavior;
- определено mutation-to-snapshot поведение;
- acknowledgement semantics соответствуют реальной гарантии acceptance;
- определены lifecycle freeze и обработка in-flight operations;
- проверены restart/new-epoch semantics;
- final managed state попадает в существующий Core contract без его ослабления;
- необходимые ADR приняты;
- draft specification обновлена до accepted normative behavior;
- не осталось нерешённых вопросов, способных нарушить operating model Managed Aggregation.

Implementation spikes во время исследований допустимы, но являются evidence, а не production architecture, пока решения не зафиксированы.

---
[Исследования Managed Aggregation](managed-aggregation-architecture-research.md) | [Индекс документации](../README.md)
