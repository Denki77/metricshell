# Numeric Semantics

[Russian version](../../docs-ru/04-specification/numeric-semantics.md)

> Status: Accepted normative specification
> Decisions: ADR-004 and amended ADR-016

Prometheus exposition and data-model semantics are authoritative. Core absolute snapshots and Managed materialization
use the same representation; Managed operations add only instrumentation invariants such as non-negative counter
deltas and atomic histogram observations.

## Audit table

| Metric/type                       | Value                                           | Prometheus semantics                                                                                                       | Previous MetricShell             | Required/current action                                                                 |
|-----------------------------------|-------------------------------------------------|----------------------------------------------------------------------------------------------------------------------------|----------------------------------|-----------------------------------------------------------------------------------------|
| Counter total                     | `0`, positive finite                            | Valid, monotonically non-decreasing (`OM`)                                                                                 | Accepted                         | Accept                                                                                  |
| Counter total                     | `-0`                                            | Wire-representable (`P`); equal to zero. `Counter.Add(-0)` accepts it but performs an integer-zero no-op (`CG-C`)          | Rejected                         | Core preserves direct snapshot spelling; Managed initialize/add normalize state to `+0` |
| Counter total                     | `+Inf`                                          | Wire-representable (`P`) and valid non-NaN terminal total (`OM`). `Counter.Add(+Inf)` accepts it (`CG-C`)                  | Rejected                         | Accept in Core, initialize, and add; finite floating overflow reaches `+Inf` (`CG-C`)   |
| Counter total                     | `NaN`, negative, `-Inf`                         | Wire-representable (`P`), but invalid typed counter state under the non-NaN/monotonic rule (`OM`)                          | Rejected                         | Reject atomically in Core and Managed operations                                        |
| Gauge                             | finite signed, `0`, `-0`, `NaN`, `+Inf`, `-Inf` | Valid wire sample values (`P`, `OM`)                                                                                       | Accepted                         | Accept and preserve                                                                     |
| Histogram observation             | finite signed, `0`, `-0`                        | `Histogram.Observe` accepts them (`CG-H`); negative values may make `_sum` decrease (`PH`)                                 | Negative rejected                | Accept                                                                                  |
| Histogram observation             | `NaN`                                           | `Observe(NaN)` increments count, makes sum `NaN`, and increments no configured bucket (`CG-H`, `NH`)                       | Rejected                         | Accept; stored terminal `+Inf` count still tracks total exposition count                |
| Histogram observation             | `+Inf`, `-Inf`                                  | Accepted and accumulated with IEEE-754 arithmetic (`CG-H`, `NH`)                                                           | Only `+Inf` accepted             | Accept                                                                                  |
| Histogram boundary                | finite signed, `0`, `-0`, `-Inf`                | Wire-representable ordered classic boundary (`P`); negative observations/bounds are supported by the client model (`CG-H`) | Negative/sign-bit rejected       | Accept and preserve                                                                     |
| Histogram boundary                | `+Inf`                                          | Required terminal classic bucket (`P`)                                                                                     | Accepted                         | Require exactly as terminal boundary                                                    |
| Histogram boundary                | `NaN`                                           | Has no meaningful ordering and is forbidden (`OM`)                                                                         | Rejected                         | Reject atomically                                                                       |
| Histogram generated count/buckets | unsigned cumulative integers                    | Non-negative, cumulative; terminal `+Inf` equals count (`P`, `OM`)                                                         | Accepted                         | Preserve                                                                                |
| Histogram generated sum           | signed finite, `NaN`, `+Inf`, `-Inf`            | Observation sum uses IEEE-754 (`CG-H`, `NH`); negative classic sums are documented (`PH`)                                  | Negative, `NaN`, `-Inf` rejected | Accept and preserve                                                                     |

The three counter layers are deliberately distinct. Core snapshot representation preserves an input `-0` token and
accepts `+Inf`. Managed `counter_initialize` accepts `-0` but stores canonical `+0`, and accepts `+Inf` as an absolute
non-NaN monotonic total. Managed `counter_add` accepts `-0` as a no-op and accepts `+Inf`, matching `client_golang`;
ordinary IEEE-754 addition makes finite overflow `+Inf`. Negative values, `-Inf`, and `NaN` are rejected as typed
counter/operation violations, even though the generic text grammar can carry them.

For histogram observation, count and every applicable cumulative bucket are checked before mutation and then updated
with the sum as one atomic operation. `-Inf` belongs to every bucket and `+Inf` only to the terminal bucket. For
`client_golang` classic histograms, `NaN` increments no configured bucket; its text encoder derives the required
terminal `+Inf` bucket from total count. MetricShell stores that terminal bucket explicitly, so it alone increments.
These representations are exposition-equivalent. Sum follows IEEE-754 addition, including `+Inf + -Inf = NaN`.

## Format-specific exposition

Core and Managed Registry retain the full Prometheus-compatible classic-histogram state. Exposition then applies the
contract of the negotiated format:

- Prometheus text 0.0.4 always emits `_sum`, including negative, `NaN`, `+Inf`, and `-Inf` values.
- OpenMetrics text 1.0 emits `_sum` only when every threshold is non-negative and sum is neither negative nor `NaN`.
  `+Inf` sum is permitted. If any threshold is negative, or sum is negative, `NaN`, or `-Inf`, `_sum` is omitted for
  that MetricPoint. `_count` and all cumulative buckets, including the single terminal `+Inf` bucket, remain present.

OpenMetrics 1.0 says Histogram Sum is optional (`SHOULD`), but if present it must not be negative or `NaN`, and it must
be absent when negative thresholds exist. Omission therefore represents every supported Core histogram validly without
changing stored state or falling back to another format. Encoding and response-size checks complete before HTTP success
headers, so an encoding failure cannot produce a partial response labeled OpenMetrics 1.0.

## Evidence and discrepancy

- **[P]** [Prometheus exposition format](https://prometheus.io/docs/instrumenting/exposition_formats/) defines sample values as
  floats and explicitly supports `NaN`, `+Inf`, and `-Inf`; it requires a terminal classic `+Inf` bucket equal to count.
- **[PH]** [Prometheus histogram guidance](https://prometheus.io/docs/practices/histograms/#count-and-sum-of-observations)
  explicitly discusses negative observations and the resulting decrease of classic `_sum`.
- **[CG-C]** [`client_golang` `counter.Add`](https://github.com/prometheus/client_golang/blob/main/prometheus/counter.go)
  rejects only `v < 0`, routes `-0` through an integer-zero no-op, and otherwise atomically adds binary64 values; this
  directly establishes `+Inf` acceptance and natural finite overflow.
- **[CG-H]** [`client_golang` `histogram.Observe`](https://github.com/prometheus/client_golang/blob/main/prometheus/histogram.go)
  calls `findBucket` then `observe`; comparisons with `NaN` select no configured bucket, while `observe` still adds
  `NaN` to sum and increments count last. The client text encoder derives the terminal `+Inf` bucket from count.
- **[NH]** [Prometheus native histogram special-value specification](https://prometheus.io/docs/specs/native_histograms/#special-cases-of-observed-values)
  states the same intentional rule explicitly: `NaN` increments count, sets the sum through normal floating-point
  arithmetic, and enters no bucket.
- **[OM]** [OpenMetrics 1.0 specification](https://github.com/prometheus/OpenMetrics/blob/v1.0.0/specification/OpenMetrics.md)
  requires support for non-real float values and defines counters as non-NaN and monotonic. For Histogram MetricPoints,
  Sum is optional, must not be negative or `NaN` when present, and must be absent when a negative threshold exists.

Malformed tokens, `NaN` boundaries, unordered boundaries, decreasing bucket counts, count overflow, and a terminal
bucket unequal to count are rejected as a whole candidate/operation. Rejection never changes the Managed generation or
the last valid Core snapshot.
