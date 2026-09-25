# Архитектурные исследования Managed Aggregation

Этот документ определяет темы исследований расширения Managed Aggregation.

## INV-016

Семантика Managed Registry

### Вопрос

Какую точную семантику registry, descriptors, operations и snapshots должен предоставлять Managed Aggregation до выбора concurrency и transport?

### Контекст

Расширение нужно, чтобы простые и legacy clients могли публиковать instrumentation operations без владения полным Prometheus-совместимым registry.

### Кандидаты

- явная declaration + typed operations;
- implicit declaration при первом использовании;
- явный protocol с convenience clients, скрывающими declaration.

### Темы

- descriptor identity;
- lifetime family;
- series identity;
- canonicalization labels;
- counter initialization/delta;
- gauge SET и optional ADD/SUB;
- histogram buckets и atomic observation;
- conflicts;
- empty registry;
- deletion/staleness;
- atomicity operation/batch;
- registry epoch;
- generation полного Core snapshot.

### Начальные гипотезы

- descriptor semantics должна быть явной;
- counter должен сохранять накопительную неубывающую семантику метрики в пределах одной epoch registry;
- клиентская модель операций должна поддерживать максимально широкий практически полезный набор изменений counter, для которых можно определить корректную и детерминированную семантику;
- increment/add, установка абсолютного значения, initialization и поведение при reset/смене epoch должны быть исследованы и сравнены, а не запрещены заранее;
- возможность установки абсолютного значения требует отдельной проверки при наличии нескольких concurrent publishers;
- операция должна признаваться неподдерживаемой только в том случае, если исследование выявит проблемы с семантикой, корректностью, совместимостью, безопасностью или неоправданной сложностью;
- gauge требует SET;
- histogram buckets фиксируются до observations;
- observation атомарно обновляет count/sum/buckets;
- identity переиспользует Core rules;
- conflict отклоняется без изменения valid state;
- restart создаёт пустую epoch;
- disconnect не удаляет series;
- managed state становится полным Core snapshot.

### Необходимые доказательства

Prometheus/OpenMetrics review, compatibility с Application Snapshot Protocol, executable registry prototype, deterministic tests, conflict tests, snapshot tests, restart tests.

### Эксперименты

#### E-016.1 — Counter

Сравнить:

- increment;
- add(delta);
- установку абсолютного значения (absolute update/set);
- явную инициализацию (explicit initialization);
- reset в пределах той же epoch;
- reset с началом новой epoch;
- нулевые, положительные и отрицательные значения/delta;
- NaN/Inf;
- overflow;
- повторную установку того же абсолютного значения;
- установку меньшего абсолютного значения.

Отдельно определить:

- какие операции сохраняют корректную семантику counter;
- какие операции требуют явной семантики reset/epoch;
- какие операции остаются детерминированными при нескольких publishers;
- какие операции позволяют безопасно сформировать полный snapshot, совместимый с контрактом Core.

Assertions:

- accepted increments дают точную сумму;
- rejected input не меняет state;
- restart не сохраняет state;
- каждая принятая операция сохраняет выбранные инварианты counter;
- отклонённая операция не изменяет предыдущее валидное состояние;
- для каждой принятой операции явно определена её накопительная семантика;
- уменьшение значения counter не может происходить неявно: оно возможно только в соответствии с выбранной по результатам исследования семантикой reset/epoch;
- restart/new epoch соответствует выбранному lifecycle-контракту.

#### E-016.2 — Gauge

Проверить SET для поддерживаемых numeric classes; ADD/SUB — отдельно, если останутся candidate.

#### E-016.3 — Histogram

Сравнить candidate, разрешающий только неотрицательные значения, с candidates, допускающими signed observations.
Проверить отрицательные, нулевые, положительные, NaN, +Inf и -Inf observations; отрицательные и mixed-sign bucket
boundaries; переход sum в отрицательное значение и обратно через ноль; повторные отрицательные/положительные
observations. Сформировать репрезентативные snapshots и раздельно зафиксировать принятие prototype candidate и
существующим Core validator.

Assertions: принятая observation меняет count/sum/applicable buckets ровно один раз; +Inf=count; partial state
отсутствует; bucket schema после observations не меняется.

#### E-016.4 — Descriptor conflicts

Проверить type/HELP/label schema/bucket conflicts, derived-name collisions и reserved namespace.

#### E-016.5 — Label identity

Проверить reordered/missing/extra/duplicate labels.

#### E-016.6 — Batch semantics

Сравнить отсутствие batch и all-or-nothing batch.

#### E-016.7 — Snapshot materialization

Сформировать полный snapshot и прогнать существующим Core validator.

### Критерии оценки

Semantic clarity, Prometheus compatibility, простота legacy client, deterministic validation, minimal client state и возможность сформировать complete Core snapshot.

Успех prototype assertions доказывает внутреннюю согласованность candidate и сохранение проверяемых инвариантов, но сам
по себе не выбирает product/architecture decision. Ordering, retry/idempotency, внешний protocol, resource limits и
lifecycle-dependent semantics отложены до INV-017–INV-020.

### Decision Output

[ADR-016](../06-architecture/adr/ADR-016.md) фиксирует semantic model managed registry. Внешние operations и детали
protocol остаются в scope INV-017–INV-020.

### Статус

Завершено.

---

## INV-017

Concurrent Publishers and Ordering

### Вопрос

Как один managed registry безопасно принимает operations от нескольких threads/processes/connections одного workload?

### Кандидаты

Single mutation loop; global lock; sharded locks; atomics + per-family locks; copy-on-write state.

### Начальные гипотезы

Single-owner loop наиболее прост для доказательства; per-connection order сохраняется; cross-connection mutations имеют один linearization order; increments не теряются; gauge SET требует deterministic ordering; histogram observation atomic; idempotency явная; queues bounded.

### Эксперименты

#### E-017.1 — Concurrent counters

1/2/8/32/128 publishers.

Assertions: final value = exact accepted sum; no lost/duplicate accepted increment.

#### E-017.2 — Concurrent gauge SET

Final value соответствует defined linearization order; torn state отсутствует.

#### E-017.3 — Concurrent histograms

Count/sum/buckets соответствуют accepted observations.

#### E-017.4 — Descriptor creation race

Compatible definitions converge; incompatible produce deterministic rejection/winner.

#### E-017.5 — Partial frame/crash

Incomplete frame не меняет registry; broken connection не повреждает others.

#### E-017.6 — Duplicate delivery

Исследовать retry after lost ACK, operation ID/idempotency key и explicit unknown outcome.

#### E-017.7 — Backpressure

Memory bounded; overload behavior deterministic; exposition остаётся operational.

#### E-017.8 — Registry-wide snapshot linearizability

Применить связанные cross-family mutations с известным commit order при concurrent чтении complete snapshots.

Assertions: каждый complete snapshot соответствует одному committed registry generation; family-local consistency
недостаточна, если snapshot смешивает state разных generations.

#### E-017.9 — Admission fairness contract

Проверить Managed Aggregation FR/NFR и измерить admission каждого publisher при bounded overload.

Assertions: policy остаётся bounded и observable. Equal-share/starvation-protection guarantee не выводится без
принятого product requirement; отсутствие гарантии фиксируется явно.

### Критерии оценки

Correctness, determinism, bounded memory, throughput, tail latency, failure recovery, PHP/shell compatibility.

### Decision Output

[ADR-017](../06-architecture/adr/ADR-017.md) фиксирует concurrency model, ordering/linearization contract, границы ACK и
idempotency, а также policy backpressure/fairness.

### Статус

Завершено.

---

## INV-018

Legacy Client and Transport Viability

### Вопрос

Могут ли PHP 5.4, shell и simple CLI использовать Managed Aggregation без complete registry, и какой local transport/protocol оправдан?

### Кандидаты

Unix stream socket + versioned operation protocol; local HTTP; reuse local adapter; отказ при чрезмерной сложности.

Client API: line/framed protocol, `metricshell metric ...`, PHP 5.4 client, reference Go client.

### Начальные гипотезы

Unix stream socket — сильнейший initial candidate; operation protocol отделён от snapshot payload; PHP 5.4 viability нужно доказать реальным runtime; shell удобнее через CLI helper; client не хранит full registry.

### Эксперименты

#### E-018.1 — PHP 5.4 basic client

Increment/set/observe без full registry/snapshot; errors detectable.

#### E-018.2 — PHP 5.4 multiprocess

Несколько workers публикуют в один registry без shared PHP memory.

#### E-018.3 — Shell/CLI

Helper commands; failure через exit code/stderr; никакого второго long-lived daemon.

#### E-018.4 — Startup race

Сравнить bounded retry, endpoint-before-workload-start, explicit readiness.

#### E-018.5 — Reconnect/restart

Reconnect без registry reconstruction; new epoch empty.

#### E-018.6 — Transport comparison

Сравнить dependencies, client LOC, same-container locality, filesystem/network security boundary, permissions, exposure,
framing robustness, debugging, startup/failure behavior и operational complexity. Persistent in-container generator
должен проверить Unix и HTTP при 1, 8, 32 и 128 clients и сохранить operations/sec, ACK p50/p95/p99 и errors как
environment-sensitive observations. Timings с Compose exec на каждую операцию классифицируются отдельно как end-to-end
short-lived helper observation.

PHP 5.4 transport prototype должен различать accepted, transport, protocol и rejected outcomes. Bounded frame и explicit
version обязательны как research candidates, но точный production bound относится к INV-019. Safe pre-submit startup
retry отделяется от unknown-outcome retry после возможного принятия операции. Final ACK commit, deduplication и
idempotency semantics остаются scope INV-017.

### Критерии оценки

Client simplicity, PHP 5.4/shell viability, dependency count, deterministic errors, same-workload locality, local
security, network exposure, framing clarity, startup behavior и operational complexity. Performance observations —
secondary evidence, а не критерий выбора transport.

### Decision Output

[ADR-018](../06-architecture/adr/ADR-018.md) выбирает initial local managed-operation transport, направление versioned
bounded protocol и stateless legacy-client contract. Точная successful-ACK semantics остаётся зависимостью INV-017;
numeric resource limits остаются scope INV-019.

### Статус

Завершено.

---

## INV-019

Performance, Snapshot Materialization and Resource Limits

### Вопрос

Может ли выбранная архитектура обеспечить practical throughput/cardinality, consistent complete snapshots и bounded resources?

### Candidates snapshot strategy

Re-encode after each mutation; materialize on scrape; generation-based immutable cache; copy-on-write state.

### Workloads

100/1k/10k+ series; 1/10/100+ publishers; counter/gauge/histogram/mixed; sustained mutation with scrape; overload.

### Начальные гипотезы

Full re-encode per operation плохо масштабируется; immutable generations желательны; bounded mutation loop + snapshot cache перспективен; cardinality — главный memory risk; histograms требуют bucket limits.

### Эксперименты

#### E-019.1 — Throughput

Accepted/rejected ops, p50/p95/p99, CPU, RSS, allocations, queue depth, descriptors.

#### E-019.2 — Cardinality

Memory/series, snapshot bytes, mutation/materialization/scrape latency.

Assertions: limits enforce safely; rejection preserves state.

#### E-019.3 — Histogram cost

Vary buckets/series; enforce bucket limits before unsafe allocation.

#### E-019.4 — Snapshot strategy comparison

Раздельно измерять acknowledgement latency, generation timing, encoding, Core install, scrape latency, memory duplication.

#### E-019.5 — Concurrent scrape

Каждый response — одна consistent generation; torn state отсутствует.

#### E-019.6 — Overload

Memory bounded; behavior соответствует INV-017; positively acknowledged operations не теряются silently.

#### E-019.7 — Controlled resource exhaustion

Отделить normal limit rejection от fatal OS/container OOM.

### Decision Output

[ADR-019](../06-architecture/adr/ADR-019.md) фиксирует snapshot materialization, bounded resource controls и overload
boundaries. Точные production numeric defaults и maxima остаются отложенными.

### Статус

Завершено.

---

## INV-020

Lifecycle and Core Integration

### Вопрос

Как lifecycle связывает managed ingestion, workload termination, registry freeze, final snapshot и существующий Core final-scrape lifecycle?

### Candidates freeze boundary

Immediate freeze; drain fully received work; explicit flush/close; bounded hybrid drain.

### Начальные гипотезы

Один explicit freeze transition; post-freeze operations reject; partial frames не принимаются; committed-before-freeze входят в final state даже при ACK loss; bounded drain допустим; final managed snapshot устанавливается до final-scrape wait; self-metrics продолжают меняться; restart создаёт пустую epoch.

### Эксперименты

#### E-020.1 — Normal exit

Freeze ровно один раз; final snapshot содержит committed operations; late mutations невозможны.

#### E-020.2 — Signal shutdown

Проверить partial/received/queued/committing/committed-before-ACK/acknowledged stages и для каждого определить accepted/rejected/unknown-to-client.

#### E-020.3 — Late publisher

Reject after freeze; final snapshot unchanged.

#### E-020.4 — Final snapshot failure

Failure explicit; invalid snapshot never installs; Core failure semantics authoritative.

#### E-020.5 — Final scrape

Frozen application state immutable; self-metrics mutable; scrape counting unchanged; no TSDB persistence claim.

#### E-020.6 — Restart/new epoch

Один workload execution на process; registry empty; prior state not restored.

#### E-020.7 — Kubernetes Job/CronJob

Final managed snapshot доступен в bounded post-exit wait без Kubernetes API dependency.

### Критерии оценки

Deterministic freeze, bounded shutdown, acknowledgement clarity, Core lifecycle compatibility, exit semantics, final-scrape correctness.

### Decision Output

[ADR-020](../06-architecture/adr/ADR-020.md) фиксирует lifecycle, freeze/in-flight semantics, final materialization и
композицию с существующим lifecycle Core.

### Статус

Завершено.

---
[Обзор исследований](managed-aggregation-architecture-investigation.md) | [Индекс документации](../README.md)
