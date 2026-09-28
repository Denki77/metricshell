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

Каждая family имеет явный immutable descriptor: name, help, type, ordered label names и non-negative строго
возрастающие buckets для histogram, заканчивающиеся `+Inf`. Повторная идентичная declaration — accepted idempotent
no-op; конфликт type, metadata или buckets отклоняется
без mutation. Label identity canonical и должна точно соответствовать descriptor.

Поддерживаются `counter_initialize`, `counter_add`, `gauge_set`, `histogram_observe` и atomic bounded batches. Counter
finite, non-negative и не уменьшается внутри epoch согласно существующему Core contract. Gauge принимает finite values,
`NaN`, `+Inf` и `-Inf`. Histogram observation принимает non-negative finite values и `+Inf`, атомарно обновляет count,
sum и cumulative classic buckets; `NaN` и отрицательные values отклоняются.

Registry generation версионирует state: изменяющая state declaration или mutation увеличивает её один раз, а identical
declaration оставляет неизменной. Owner commit/order отдельно увеличивается для каждой успешно обработанной accepted
operation, включая idempotent declaration.

## Ordering и acknowledgement

Один bounded owner задаёт registry-wide commit order. Полная queue даёт явный overload outcome. Success response
отправляется только после commit и содержит generation и commit order. Disconnect или потеря response после submission
может оставить client outcome unknown; non-idempotent operation нельзя автоматически повторять.
Cancellation после owner admission также даёт `unknown`, потому что owner ещё может commit operation. Protocol v1 не
имеет idempotency key или exactly-once retry: `counter_add 1 → UNKNOWN` нельзя повторять вслепую.

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

Пока workload работает, один fixed periodic publisher materializes не более одной generation за настроенный цикл
(`managed.publication_interval`, default `1s`). Он использует одну goroutine, не имеет work queue и per-operation timer.
Mutation не ждёт exposition или materialization. Несколько registry generations могут coalesce; каждый installed
candidate полный, publication failure сохраняет предыдущий Core snapshot и повторяется в следующем цикле. Успешные
managed commits сами по себе не гарантируют, что результирующий complete snapshot удовлетворяет настроенному Core
snapshot-size limit.

Visibility имеет три границы: accepted означает commit в Managed Registry; publication — install одной complete registry
generation в Core; scrape — чтение одной immutable Core generation. Accepted не означает немедленную visibility или
получение Prometheus. Публикация каждой промежуточной generation и exactly-once после unknown outcome не гарантируются.

## Lifecycle

Workload exit или external termination сначала закрывает socket и owner admission, затем drains только already admitted
work внутри существующего finalization/shutdown budget. Один logical winner freezes registry; все поздние publishers
отклоняются как `late`. Ровно одна final generation materializes и передаётся Core. Natural completion использует
неизменённый immediate, duration или scrape-count final wait; external termination — существующий immediate bounded
shutdown path. Restart начинает новую empty epoch.

Non-zero workload exit не уничтожает valid committed metrics: bounded drain, final freeze, final install и настроенный
final-wait выполняются, затем MetricShell сохраняет workload exit code, если MetricShell-owned finalization failure не
имеет больший приоритет.

## Когда использовать Managed Aggregation

Режим предназначен для CLI, cron, Job/CronJob, batch, ETL/import/export, legacy PHP/shell, one-shot и long-running
workers, которым неудобно владеть Prometheus registry и endpoint. Snapshot mode вместо этого принимает complete
registry-owned snapshots от workload. Обычному HTTP service с native Prometheus `/metrics` MetricShell может не требоваться.

## Observability и security

Managed operation, bounded rejection class, protocol, queue/resource, generation, materialization, freeze и final
install outcomes используют closed registries спецификаций self-metrics и structured logging. Application metric
names, labels, payloads, socket paths и client identities не попадают в self-metric labels или diagnostics. Local
filesystem permissions являются authentication boundary; protocol input всегда untrusted.

## Compatibility

Shell command и PHP 5.4 client — stateless protocol adapters. Accepted означает committed, но не обязательно уже
installed в Core. Rejected operations безопасны для committed state; unknown result нельзя автоматически retry.
Snapshot mode и Core application-snapshot contract не меняются.
