# INV-016 Report — Managed Registry Semantics

Status: completed

Run date: 2026-09-24

Reference run: `results/20260924T114723Z`

Ubuntu confirmation run: `results/20260924T123514Z`

Environment: Docker Desktop 29.8.0, LinuxKit 7.0.12, linux/aarch64

Result: 64/64 candidate-invariant assertions passed; 7/7 expected Core compatibility outcomes matched

Decision: [ADR-016](../../docs/06-architecture/adr/ADR-016.md)

## Goal and Evidence Rule

INV-016 compares registry semantics before concurrency, transport, limits and lifecycle integration are selected.
Accepted Core contracts and ADR-001–ADR-015 remain unchanged.

Passing prototype assertions demonstrates that the tested candidate semantics are implemented consistently and preserve
the tested invariants. It does not by itself prove that those semantics are the correct product or architectural choice.
Each area therefore separates external constraints, candidates, prototype evidence, result, recommendation and deferred
questions.

## External Model and Core Constraints

- [Prometheus histogram guidance](https://prometheus.io/docs/practices/histograms/) discusses negative observations and
  warns that `rate()` cannot be applied directly to a `_sum` that can decrease. That query limitation is not equivalent
  to an invalid histogram model.
- OpenMetrics permits negative histogram thresholds, but a counter histogram must then omit its sum; sum and cumulative
  bucket populations otherwise have counter semantics. See the
  [OpenMetrics model](https://github.com/prometheus/OpenMetrics/blob/main/specification/OpenMetrics.txt#L2811-L2849).
- The accepted Application Snapshot Protocol requires a histogram sum, non-negative boundaries and non-negative sum.
  The current Core validator implements that contract and has no omitted-sum representation.
- FR-MA-007/008/016 require complete snapshots and Core reuse without weakening Core.

The general metric model, current Core compatibility and Managed Aggregation recommendation are therefore distinct.

## Counter Operations

### External constraints and candidates

Counters are cumulative within an epoch and cannot silently decrease. Candidates were increment, non-negative add,
initialization, absolute update, same-epoch reset and new-epoch reset.

### Prototype evidence

Increment/add produced an exact value. Initialization worked for a new series. Reinitialization, negative/special deltas
and overflow rejected atomically. Repeated/increasing absolute values worked in one ordered stream. A decreasing set and
the prototype's unsupported reset left state unchanged. A new registry started empty.

### Final investigation conclusion

The accepted baseline is explicit initialization plus increment/add with a non-negative finite delta. A new epoch starts
empty and a counter cannot silently decrease within one epoch. Monotonic absolute update is feasible in one ordered
mutation stream but is not accepted as an external operation. The unsupported-reset PASS only describes this candidate;
same-epoch reset is neither accepted nor prohibited here.

### Deferred

Absolute updates, reset, retries, lost ACKs, duplicates, multiple publishers and reordering require INV-017.

## Gauge Operations

### External constraints and candidates

Core accepts finite values and `NaN`/`±Inf`. Candidates were SET alone or SET plus ADD/SUB.

### Prototype evidence and result

SET preserved every Core numeric class. Finite ADD/SUB worked in an ordered stream; arithmetic with special values
rejected atomically. This demonstrates feasibility, not API necessity.

### Final investigation conclusion and deferred

SET is the required baseline. ADD/SUB is feasible in an ordered stream; INV-017 must define its concurrency semantics
and INV-018 must establish whether it belongs in the external API.

## Histogram Semantics

### External constraints

Prometheus instrumentation can observe negative values, although `_sum` can then decrease and counter-style `rate()` is
unsafe. OpenMetrics permits negative thresholds only when sum is omitted. Current Core instead requires non-negative sum
and boundaries, so it represents a stricter subset.

### Candidates

1. Core-compatible non-negative observations/bounds.
2. Signed observations with non-negative bounds.
3. Signed observations with negative and positive bounds.
4. NaN and `±Inf` observations.

### Prototype evidence

Both implementations update count, sum and cumulative buckets atomically. The signed candidate covered negative sum,
crossing zero, mixed observations, mixed boundaries and `-Inf`. NaN was rejected by the candidate; `+Inf` was accepted
by the restrictive candidate.

| Snapshot | Prototype | Core | Evidence |
| --- | --- | --- | --- |
| regular non-negative plus `+Inf` | accepted | accepted, 5 series / 646 bytes | compatible |
| empty registry | accepted | accepted, 0 series / 35 bytes | compatible |
| negative observation, sum `-0.5` | accepted | `histogram_invalid` | negative sum incompatible |
| `-0.5`, then `1`, sum `0.5`, non-negative bounds | accepted | accepted, 1 series / 261 bytes | history absent from final snapshot |
| negative and positive boundaries | accepted | `histogram_invalid` | negative bounds incompatible |
| signed observations returning sum to zero, negative bound | accepted | `histogram_invalid` | bound incompatible |
| `-Inf` observation/sum | accepted | `histogram_invalid` | incompatible |

### Result

Negative observations are valid in the wider Prometheus instrumentation model. Core does not categorically detect or
reject their history: a final non-negative sum with non-negative buckets passes. A negative intermediate/final sum cannot
be published, and OpenMetrics negative-bound histograms require an omitted sum that Core cannot express.

### Final investigation conclusion and deferred

Use non-negative observations and non-negative fixed bucket boundaries for initial Managed Aggregation. This follows
from the existing complete-snapshot boundary and predictable `_sum` semantics, not from a claim that negative
observations are universally invalid. Signed support would require explicit future Core scope review; INV-016 does not
authorize it. A negative observation followed by a positive observation can yield a Core-accepted final snapshot when
its final sum and boundaries are non-negative, demonstrating that a complete snapshot does not encode observation
history. The registry must nevertheless reject state that cannot safely cross the Core boundary when materialized.

## Descriptor Declaration

### Candidates

| Candidate | Metadata | Conflicts | Legacy simplicity |
| --- | --- | --- | --- |
| explicit declaration | complete before mutation | deterministic | extra step |
| implicit first use | defaults/inference required | later declarations can conflict | simplest first call |
| explicit internal + convenience call | complete internally | deterministic | declaration hidden |

### Prototype evidence and result

The prototype implemented explicit declaration and rejected undeclared operations/conflicts atomically. That does not
disprove implicit declaration. The independent reason to prefer explicit internal semantics is that buckets, HELP and
exact label schema cannot be inferred from generic increment/set/observe without protocol defaults.

### Final investigation conclusion and deferred

Descriptors are explicit at the registry semantic boundary and define metric-family identity, type, HELP, label schema
and histogram bucket schema where applicable. A convenience client may hide declaration. INV-018 decides the external
client and wire shape.

## Identity and Conflicts

Metric family plus canonical label set determines series identity. Reordered labels coalesced; missing/extra labels
violated the descriptor schema. Type, HELP, labels, buckets, derived names and reserved names conflicted atomically, and
rejected mutations preserved prior valid state. Wire-format duplicate-key parsing belongs to INV-018.

## Batch Semantics

All-or-nothing batching is feasible through candidate/clone state. This does not establish that batching belongs in the
initial protocol. INV-018 must supply client/use-case evidence; independent operations can remain unbatched.

## Deletion, Staleness and Lifetime

The prototype candidate keeps state for the epoch, treats disconnect as no-op and does not implement deletion. This is a
coherent baseline, not proof that deletion is wrong. Correct deletion depends on publisher ownership, races,
stale-marker/exposition policy, freeze and epoch cleanup, deferred to INV-017 and INV-020.

## Snapshot Atomicity and Core Compatibility

Rejected mutations remained byte-identical to the prior complete snapshot. Populated, empty and signed candidate
snapshots were checked by the actual Core parser. Managed Registry must materialize one complete candidate application
snapshot and pass it through the existing Core validation/installation boundary. Managed Aggregation is not an
alternative Core, and ADR-001–ADR-015 remain unchanged.

## Observational Benchmarks

Thirty five-series materializations took 144,125 ns total (4,804 ns each).

| Series | Bytes | ns/snapshot |
| ---: | ---: | ---: |
| 0 | 35 | 245 |
| 1 | 129 | 869 |
| 10 | 435 | 5,594 |
| 100 | 3,585 | 42,223 |
| 1,000 | 35,985 | 497,380 |
| 10,000 | 368,985 | 5,945,861 |

These are observations, not limits or performance decisions. INV-019 owns those conclusions.

## Portable Confirmation

| Role | Environment | Docker | Result |
| --- | --- | --- | --- |
| Reference | macOS/LinuxKit, linux/aarch64 | 29.8.0 | 64/64 assertions; 7/7 Core outcomes |
| Confirmation | Ubuntu/LinuxKit, linux/x86_64 | 27.4.0 | 64/64 assertions; 7/7 Core outcomes |

Both runs have benchmark fingerprint
`1f4be9f67c79cb5710dc24bcc6401c1f3d2112f95ee66933e4a1dfd3eb435f1e` and Core source fingerprint
`6cb7e1021fb8806af25a8d3470cf2b789d4958089777a837c800d94c6dc26f9d`. Semantic assertions and the complete Core
compatibility matrix match; no architecturally significant difference was found. Timing differences across ARM64 and
x86_64 are observations only and are not a portable acceptance criterion. INV-019 owns performance conclusions.

## Conclusion

INV-016 establishes the descriptor-driven semantic boundary, counter and gauge baselines, initial Core-compatible
histogram subset, canonical identity/conflict rules, epoch reset and complete-snapshot handoff captured by ADR-016.
Counter absolute update, same-epoch reset, gauge arithmetic concurrency, external batching, transport, deletion,
materialization limits and lifecycle remain deferred to INV-017–INV-020.
