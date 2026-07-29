# INV-004 — Metric-state Ownership and Semantics

**Status:** completed; conclusion revised after scope review

**Reference runs:** `results/20260723T073114Z`, `results/20260723T150118Z`

**Report:** [report.md](report.md)

**Decision:** [ADR-004](../../docs/06-architecture/adr/ADR-004.md)

## Question

Which metric-state representation is the minimum sufficient contract for a runtime wrapper that transports, validates,
and exposes metrics but does not aggregate metric values across producers?

## Scope correction

The original investigation treated independent operations and multiple producer-owned registries as possible product
requirements. That expanded MetricShell into a local metrics aggregator with producer identity, epoch, sequence,
reconciliation, completeness, and type-specific aggregation policy.

Project scope explicitly excludes local or distributed aggregation of metric values across producers and business
metric design. Functional requirements require accepted metric types and consistent state, but do not require
MetricShell to implement `increment`, `set`, or `observe`. The workload and its libraries are responsible for producing
one complete, conflict-free application snapshot.

The prototype and raw evidence are retained. They show two separate costs. Reconciliation is required if MetricShell
accepts state-changing operations. Producer ownership and aggregation policies are required if MetricShell combines
independently owned registries. Both capabilities are outside MetricShell's scope.

## Candidates evaluated

- complete snapshots;
- absolute values for individual series;
- operations (`increment`, `set`, `observe`);
- hybrid operations plus authoritative snapshot reconciliation.

## Experiments

The Docker prototype evaluates 33 deterministic scenarios covering counters, gauges, histograms, duplicate series, type
conflicts, multiple producers, ordering, dropped updates, producer/receiver restarts, stale data, and reconciliation. It
also benchmarks candidate combinations at 1/4/16 producers and 1/100/1,000/10,000 series, plus 30 representative
repetitions.

These scenarios deliberately explore a superset of product scope. Multi-producer aggregation and operation ownership
are counterfactual complexity evidence, not production protocol requirements.

## Results

Both reference runs recorded 33 scenarios, 29 confirmed invariants and four expected counterexamples, with 0 failed
per-scenario assertions and 129 benchmark rows each. The shared benchmark-code fingerprint is
`e52784470ff33e35fb58ab142be26a345bb6a373bd2eac58666b269f56875fd6`.

| Environment       | Result set                 | Docker platform | Assertions | Snapshot p50 | Operation p50 | Hybrid p50 |
|-------------------|----------------------------|-----------------|-----------:|-------------:|--------------:|-----------:|
| macOS / LinuxKit  | `results/20260723T073114Z` | `linux/aarch64` | 34/34 pass |     28,108/s |       4.48M/s |    4.31M/s |
| Ubuntu / LinuxKit | `results/20260723T150118Z` | `linux/x86_64`  | 34/34 pass |      6,402/s |       2.21M/s |    1.97M/s |

The evidence relevant to the corrected scope is:

- complete snapshots restore full state after a dropped or superseded intermediate publication;
- omission from a newer complete snapshot provides deterministic stale-series removal;
- operations alone cannot reconstruct a dropped increment or lost in-memory receiver state;
- hybrid recovery works, but requires sequencing, gap detection, authoritative snapshots, and reconciliation;
- automatic multi-owner handling requires counter/histogram aggregation and a gauge collision policy;
- snapshot cost depends on registry cardinality: at 16 producers / 10,000 series the synthetic snapshot case allocated
  about 3.82 MB per complete update;
- at 4 producers / 1,000 series, measured hybrid reconciliation consumed 85.6%, 35.7%, and 5.9% of time at intervals
  100, 1,000, and 10,000 respectively.

Performance numbers compare the research models; they are not production SLOs. The operation path is faster because it
does less work per update, while hybrid retains the snapshot cost needed to repair operation loss.

## Conclusion

Reconciliation is required if MetricShell accepts state-changing operations. Producer ownership and aggregation policies
are required if MetricShell combines independently owned registries. Both capabilities are outside MetricShell's scope.

Therefore the selected production model is:

```text
workload/library
    -> one complete, conflict-free application snapshot
    -> file | Unix stream | local HTTP
    -> structural validation
    -> atomic last-valid replacement
    -> Prometheus exposition
    -> frozen final snapshot
```

All transports carry equivalent complete snapshots. MetricShell preserves counter, gauge, and histogram representation,
but does not apply `increment`, `set`, or `observe`, enforce business monotonicity across snapshots, track producer
epochs, recover missing operations, or aggregate colliding series.

## Admissible semantic values

- authoritative unit: one complete application snapshot;
- producer coordination: owned by the workload or its libraries, outside MetricShell;
- initial state: a zero-series application snapshot;
- valid zero-series snapshot: clear all active application series;
- absent or empty transport payload: malformed input, not a zero-series snapshot;
- missing series in a newer accepted snapshot: absent from the new active state;
- counter: structurally valid Prometheus counter representation in one snapshot;
- gauge: absolute value in one snapshot;
- histogram: ordered boundaries, cumulative buckets, required `+Inf` equal to `count`, and numeric `sum` in one
  snapshot;
- duplicate series, type conflict, or candidate-internal metadata conflict: reject the complete candidate atomically;
- established family name-to-type binding: retained through omission until the next workload execution;
- other metadata: validated for internal consistency within each candidate, not lifetime-bound;
- invalid candidate: retain the previous valid snapshot;
- socket/HTTP success acknowledgement: candidate was atomically installed in one linear acceptance order;
- producer timestamp: not an ordering or freshness authority;
- cross-snapshot counter or histogram monotonicity: not validated by MetricShell;
- cross-producer aggregation: not performed;
- final state: last valid accepted complete application snapshot, or the zero-series initial snapshot if none was
  accepted.

## Running the prototype

From the repository root, on macOS or Ubuntu with Docker:

```bash
./research/INV-004/run-bench.sh
```

Inspect evidence:

```bash
latest="$(cat research/INV-004/latest-results.txt)"
cat "$latest/summary.tsv"
cat "$latest/assertions.tsv"
cat "$latest/semantics.tsv"
cat "$latest/benchmark-stats.tsv"
cat "$latest/environment.tsv"
cat "$latest/coverage.tsv"
```

The prototype still exposes operation and hybrid benchmark modes because those raw experiments are retained as
scope-boundary evidence.

## Cross-environment fingerprint

The runner hashes normalized relative names and contents of `prototype/` plus `run-bench.sh`; it does not include the
host path, repository HEAD, timestamps, or results. Both completed runs produced fingerprint
`e52784470ff33e35fb58ab142be26a345bb6a373bd2eac58666b269f56875fd6`.

## Prototype limits

- This is an in-memory semantic model, not production parsing, persistence, or a transport implementation.
- Measurements include Go map/string allocation and Docker startup per sample; they are comparative evidence.
- Both measured container environments use LinuxKit. Native non-LinuxKit Linux, containerd/CRI-O, and Kubernetes remain
  unverified.
- The recorded `container_go_version` field contains the prototype help banner; it is excluded from evidence comparison.
- The prototype's producer ownership and aggregation branches exceed current product scope.

## Decision output

- Prototype and runner: retained unchanged as research evidence.
- Raw evidence: `results/20260723T073114Z/`, `results/20260723T150118Z/`.
- Detailed report: [report.md](report.md).
- Decision: [ADR-004](../../docs/06-architecture/adr/ADR-004.md) — complete application snapshots with atomic last-valid
  replacement and no cross-producer aggregation.
