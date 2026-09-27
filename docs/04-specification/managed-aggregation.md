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

Every family has an explicit immutable descriptor: name, help, type, ordered label names and, for histograms, finite
strictly increasing buckets. A repeated identical declaration is idempotent; conflicting type, metadata or buckets are
rejected without mutation. Label identity is canonical and must exactly match the descriptor.

Supported operations are `counter_initialize`, `counter_add`, `gauge_set`, `histogram_observe` and atomic bounded
batches. Counters are finite and non-negative and never decrease inside an epoch. Gauges accept finite values.
Histogram observations are finite and non-negative and update count, sum and cumulative classic buckets atomically.
Every successful declaration or mutation advances the registry generation exactly once; rejection does not.

## Ordering and acknowledgement

One bounded owner establishes the registry-wide commit order. Queue-full is an explicit overload outcome. A success
response is emitted only after commit and includes generation and commit order. Disconnect or response loss after
submission can leave the client outcome unknown; clients must not automatically retry a non-idempotent operation.

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

## Lifecycle

Workload exit or external termination first closes socket and owner admission, then drains only already admitted work
inside the existing finalization/shutdown budget. One logical winner freezes the registry; all later publishers are
rejected as `late`. Exactly one final generation is materialized and offered to Core. Natural completion delegates to
the unchanged immediate, duration or scrape-count final wait; external termination uses the existing immediate bounded
shutdown path. A restart begins a new empty epoch.

## Observability and security

Managed operation, bounded rejection class, protocol, queue/resource, generation, materialization, freeze and final
install outcomes use the closed registries in the self-metrics and structured-logging specifications. Application
metric names, labels, payloads, socket paths and client identities never become self-metric labels or diagnostic data.
The local filesystem permission boundary is the authentication boundary; protocol input is always untrusted.

## Compatibility

The shell command and PHP 5.4 client are stateless protocol adapters. Accepted means committed, not necessarily already
installed in Core. Rejected operations are safe with respect to committed state; an unknown result is not safe to
retry automatically. Snapshot mode behavior and the Core application-snapshot contract remain unchanged.
