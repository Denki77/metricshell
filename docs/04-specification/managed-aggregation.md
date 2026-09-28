# Managed Aggregation Specification

[Russian version](../../docs-ru/04-specification/managed-aggregation.md)

> Status: Accepted normative specification
> Requirements: FR-MA-001–FR-MA-016, NFR-MA-001–NFR-MA-008
> Decisions: ADR-016–ADR-020; Core ADR-003 and ADR-004 remain authoritative

## Ownership and epoch

Managed Aggregation is selected explicitly with `mode=managed-registry`; `snapshot` remains the default. The two
ownership models cannot be combined. One MetricShell execution owns one in-memory registry beginning at generation
zero with no families or series. State is never persisted, replayed or shared across executions.

## Metric semantics

Every family has an explicit immutable descriptor: name, help, type, ordered label names and, for histograms,
non-negative strictly increasing buckets ending in `+Inf`. A repeated identical declaration is an accepted idempotent
no-op; conflicting type, metadata or buckets are
rejected without mutation. Label identity is canonical and must exactly match the descriptor.

Supported operations are `counter_initialize`, `counter_add`, `gauge_set`, `histogram_observe` and atomic bounded
batches. Counters are finite and non-negative and never decrease inside an epoch, matching the existing Core contract.
Gauges accept finite values, `NaN`, `+Inf` and `-Inf`. Histogram observations accept non-negative finite values and
`+Inf` and update count, sum and cumulative classic buckets atomically; `NaN` and negative values are rejected.

Registry generation versions state: a state-changing declaration or mutation advances it once, while an identical
declaration leaves it unchanged. Owner commit/order separately advances for every successfully processed accepted
operation, including an idempotent declaration.

## Ordering and acknowledgement

One bounded owner establishes the registry-wide commit order. Queue-full is an explicit overload outcome. A success
response is emitted only after commit and includes generation and commit order. Disconnect or response loss after
submission can leave the client outcome unknown; clients must not automatically retry a non-idempotent operation.
Cancellation after owner admission is also `unknown`, because the owner may still commit the operation. Protocol v1
has no idempotency key or exactly-once retry mechanism: `counter_add 1 → UNKNOWN` must not be retried blindly.

## Protocol and endpoint

Protocol version 1 is one bounded newline-terminated JSON operation and one JSON response per Unix stream connection.
Empty, partial, oversized, multiple, malformed, missing/invalid/unsupported-version and invalid-request frames have
stable protocol codes. The socket has an absolute path, private parent, mode `0600` or `0660`, bounded connections and
read/write deadlines. No TCP or remote managed endpoint exists.

## Resource and failure semantics

Family, active-series, label, histogram-bucket, batch, string, frame, connection and queue bounds are validated before
unsafe allocation or registry mutation. Semantic, resource, protocol, overload, late, cancelled and unknown-client
outcomes remain distinct. Every rejection preserves the complete committed registry and active Core snapshot.

## Materialization and Core

A generation is encoded as one deterministic immutable Application Snapshot Protocol document. Equal generations
reuse the cache; concurrent readers cannot observe mutation. The candidate enters the same Core parser, validation,
atomic holder and exposition path as snapshot transports. Conversion, validation or installation failure is never a
partial install and preserves the prior active Core state.

While the workload is running, one fixed periodic publisher materializes at most one generation per configured cycle
(`managed.publication_interval`, default `1s`). It has one goroutine, no work queue and no per-operation timer.
Mutations never wait for exposition or materialization. Several registry generations may coalesce; every installed
candidate is complete, and publication failure preserves the prior Core snapshot and is retried on a later cycle.
Successful managed commits do not by themselves guarantee that the resulting complete snapshot satisfies the
configured Core snapshot-size limit.

Visibility has three boundaries: accepted means committed to Managed Registry; publication means one complete registry
generation was installed in Core; scrape means one reader consumed one immutable Core generation. Accepted does not
mean immediately visible or already collected by Prometheus. Intermediate generations and exactly-once delivery after
an unknown client outcome are not guaranteed.

## Lifecycle

Workload exit or external termination first closes socket and owner admission, then drains only already admitted work
inside the existing finalization/shutdown budget. One logical winner freezes the registry; all later publishers are
rejected as `late`. Exactly one final generation is materialized and offered to Core. Natural completion delegates to
the unchanged immediate, duration or scrape-count final wait; external termination uses the existing immediate bounded
shutdown path. A restart begins a new empty epoch.

A non-zero workload exit does not discard valid committed metrics: bounded drain, final freeze, final install and the
configured final-wait policy still run, after which MetricShell preserves the workload exit code unless a
MetricShell-owned finalization failure takes precedence.

## When to use Managed Aggregation

The mode serves CLI, cron, Job/CronJob, batch, ETL/import/export, legacy PHP/shell, one-shot and long-running workers
that cannot conveniently own a Prometheus registry and endpoint. Snapshot mode instead accepts complete registry-owned
snapshots from the workload. A conventional HTTP service already exposing native Prometheus metrics may not need
MetricShell.

## Observability and security

Managed operation, bounded rejection class, protocol, queue/resource, generation, materialization, freeze and final
install outcomes use the closed registries in the self-metrics and structured-logging specifications. Application
metric names, labels, payloads, socket paths and client identities never become self-metric labels or diagnostic data.
The local filesystem permission boundary is the authentication boundary; protocol input is always untrusted.

## Compatibility

The shell command and PHP 5.4 client are stateless protocol adapters. Accepted means committed, not necessarily already
installed in Core. Rejected operations are safe with respect to committed state; an unknown result is not safe to
retry automatically. Snapshot mode behavior and the Core application-snapshot contract remain unchanged.
