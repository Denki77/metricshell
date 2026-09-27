# Спецификация Managed Aggregation

[English version](../../docs/04-specification/managed-aggregation.md)

> Статус: Accepted normative specification
> Требования: FR-MA-001–FR-MA-016, NFR-MA-001–NFR-MA-008
> Решения: ADR-016–ADR-020; Core ADR-003 и ADR-004 остаются authoritative

## Ownership и epoch

Managed Aggregation выбирается явно через `mode=managed-registry`; `snapshot` остаётся default. Две модели ownership
нельзя комбинировать. Один запуск MetricShell владеет одним in-memory registry, начинающимся с generation zero без
families и series. State не сохраняется, не replay и не разделяется между запусками.

## Семантика метрик

Каждая family имеет явный immutable descriptor: name, help, type, ordered label names и finite строго возрастающие
buckets для histogram. Повторная идентичная declaration идемпотентна; конфликт type, metadata или buckets отклоняется
без mutation. Label identity canonical и должна точно соответствовать descriptor.

Поддерживаются `counter_initialize`, `counter_add`, `gauge_set`, `histogram_observe` и atomic bounded batches. Counter
finite, non-negative и не уменьшается внутри epoch. Gauge принимает finite values. Histogram observation finite и
non-negative и атомарно обновляет count, sum и cumulative classic buckets. Успешная declaration или mutation
увеличивает registry generation ровно один раз; rejection её не меняет.

## Ordering и acknowledgement

Один bounded owner задаёт registry-wide commit order. Полная queue даёт явный overload outcome. Success response
отправляется только после commit и содержит generation и commit order. Disconnect или потеря response после submission
может оставить client outcome unknown; non-idempotent operation нельзя автоматически повторять.

## Protocol и endpoint

Protocol version 1 — одна bounded newline-terminated JSON operation и один JSON response на Unix stream connection.
Empty, partial, oversized, multiple, malformed, missing/invalid/unsupported-version и invalid-request frames имеют
stable protocol codes. Socket использует absolute path, private parent, mode `0600` или `0660`, bounded connections и
read/write deadlines. TCP или remote managed endpoint отсутствует.

## Resource и failure semantics

Bounds для families, active series, labels, histogram buckets, batch, strings, frame, connections и queue проверяются
до unsafe allocation или registry mutation. Semantic, resource, protocol, overload, late, cancelled и unknown-client
outcomes различаются. Любой rejection сохраняет complete committed registry и active Core snapshot.

## Materialization и Core

Generation кодируется в один deterministic immutable Application Snapshot Protocol document. Equal generations
переиспользуют cache; concurrent readers не видят mutation. Candidate проходит тот же Core parser, validation, atomic
holder и exposition path, что snapshot transports. Ошибка conversion, validation или installation не создаёт partial
install и сохраняет prior active Core state.

## Lifecycle

Workload exit или external termination сначала закрывает socket и owner admission, затем drains только already admitted
work внутри существующего finalization/shutdown budget. Один logical winner freezes registry; все поздние publishers
отклоняются как `late`. Ровно одна final generation materializes и передаётся Core. Natural completion использует
неизменённый immediate, duration или scrape-count final wait; external termination — существующий immediate bounded
shutdown path. Restart начинает новую empty epoch.

## Observability и security

Managed operation, bounded rejection class, protocol, queue/resource, generation, materialization, freeze и final
install outcomes используют closed registries спецификаций self-metrics и structured logging. Application metric
names, labels, payloads, socket paths и client identities не попадают в self-metric labels или diagnostics. Local
filesystem permissions являются authentication boundary; protocol input всегда untrusted.

## Compatibility

Shell command и PHP 5.4 client — stateless protocol adapters. Accepted означает committed, но не обязательно уже
installed в Core. Rejected operations безопасны для committed state; unknown result нельзя автоматически retry.
Snapshot mode и Core application-snapshot contract не меняются.
