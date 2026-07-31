# INV-004 Report — Metric-state Ownership and Semantics

Status: completed

Run dates: 2026-07-23

Docker servers: 29.4.3, 27.4.0

Docker platforms: linux/aarch64, linux/x86_64

Reference runs: `results/20260723T073114Z`, `results/20260723T150118Z`

Summaries: `results/20260723T073114Z/summary.tsv`, `results/20260723T150118Z/summary.tsv`

## Goal

Determine the minimum sufficient metric-state contract for MetricShell after separating transport/runtime
responsibilities from out-of-scope instrumentation and cross-producer aggregation.

## Scope correction

The experiment intentionally compared a broader set of semantic models than the product ultimately requires. Its
original interpretation assumed MetricShell might accept independent operations, own per-producer contributions, and
aggregate them for exposition.

Project scope excludes local or distributed aggregation of metric values across producers and business metric design.
Functional requirements require consistent acceptance and exposition of counters, gauges, and histograms, but do not
require `increment`, `set`, `observe`, producer epochs, sequence recovery, or completeness classification. The workload
and its libraries own those concerns and must publish one complete, conflict-free application snapshot.

The prototype and result files remain unchanged. Multi-owner and operation scenarios are retained as evidence of the
complexity avoided by the selected scope.

## Prototype

The prototype is located in `research/INV-004`.

- `prototype/cmd/inv004` — executable semantic model and allocation/throughput benchmark.
- `prototype/Dockerfile` — reproducible Linux build/runtime image; uses `COPY ["cmd", "./cmd/"]`.
- `run-bench.sh` — full semantic matrix, scale matrix, repetitions, assertions and environment fingerprint.
- `results/<timestamp>` — TSV evidence and Docker build log.

## Run Commands

```bash
./research/INV-004/run-bench.sh
latest="$(cat research/INV-004/latest-results.txt)"
cat "$latest/summary.tsv" "$latest/assertions.tsv" "$latest/benchmark-stats.tsv"
```

The same command is used on macOS and Ubuntu. Increase repetitions with `INV004_REPEAT_COUNT=100`.

## Run Environment

| Environment             |       Date | Docker | Platform         | Result set                 | Fingerprint                                                        |                Result |
|-------------------------|-----------:|-------:|------------------|----------------------------|--------------------------------------------------------------------|----------------------:|
| Docker Desktop on macOS | 2026-07-23 | 29.4.3 | LinuxKit/aarch64 | `results/20260723T073114Z` | `e52784470ff33e35fb58ab142be26a345bb6a373bd2eac58666b269f56875fd6` | 34/34 assertions pass |
| Ubuntu / LinuxKit       | 2026-07-23 | 27.4.0 | LinuxKit/x86_64  | `results/20260723T150118Z` | `e52784470ff33e35fb58ab142be26a345bb6a373bd2eac58666b269f56875fd6` | 34/34 assertions pass |

The fingerprint covers only benchmark source and runner content with relative names, so it is invariant to checkout
path and repository HEAD. Both environments recorded the same fingerprint. Environment and image identifiers are
separately recorded in each `environment.tsv`.

Both runs produced identical `semantics.tsv` and `assertions.tsv`: all 33 named semantic scenarios had their expected
result and the scenario-set cardinality assertion passed, for 34/34 assertions in each environment.

## Results

### Semantic scenarios

| Candidate           |          Recover dropped update |       Receiver restart |                       Multi-producer |        Stale removal | Scope interpretation                  |
|---------------------|--------------------------------:|-----------------------:|-------------------------------------:|---------------------:|---------------------------------------|
| Complete snapshot   |           yes, at next snapshot |              republish | workload resolves before publication |                  yes | selected minimum sufficient model     |
| Per-series absolute | only when same series is resent |   only after republish |           ambiguous last-writer-wins | no registry boundary | insufficient partial-state contract   |
| Operations only     |                              no |                     no |           counter increments commute |                   no | out of scope and insufficient truth   |
| Hybrid              |          yes, at reconciliation | yes, at reconciliation |                         owner-scoped |                  yes | valid aggregator design, out of scope |

Four rows in `semantics.tsv` intentionally have `result=fail`: they are falsifying counterexamples, not runner
failures. They demonstrate absolute counter decrease, absolute multi-producer collision, lost operation and state loss
after receiver restart. `assertions.tsv` checks every named scenario independently plus exact scenario-set cardinality;
it no longer relies on aggregate pass/fail counts.

The experimental operation path enforces the full `(producer_id, producer_epoch, sequence)` boundary. An old epoch is
rejected. A
new epoch observed through an operation becomes incomplete/non-authoritative and cannot mutate values; its initial
complete snapshot authorizes subsequent operations starting at the new sequence space.

This boundary is an observed prerequisite of the counterfactual operation/hybrid design, not an admissible production
protocol field set for MetricShell.

Histogram evidence uses complete `bounds`, cumulative `buckets`, `count` and `sum`, not a scalar proxy. The suite
checks component-wise aggregation, boundary compatibility, `count == final cumulative bucket`, non-decreasing buckets
within an epoch, and a permitted reset in a new epoch.

### Representative performance

| Candidate                          | Workload                                |                                        Throughput | Allocation/update |
|------------------------------------|-----------------------------------------|--------------------------------------------------:|------------------:|
| Snapshot                           | 1 producer, 100 series, 100 snapshots   | 15,500 snapshots/s single scale point; p50 28,108 |      25,890 B p50 |
| Operation fast path used by hybrid | 1 producer, 100 series, 100k operations |                3.91M ops/s scale point; p50 4.48M |           6 B p50 |
| Hybrid amortized, interval 1,000   | 1 producer, 100 series, 100k operations |            4.14M updates/s scale point; p50 4.31M |          19 B p50 |

At the largest point (16 producers, 10,000 series), snapshots achieved 249 complete updates/s and allocated about
3.82 MB/update. The operation fast path achieved 4.16M updates/s at 34 B/update. Actual hybrid reconciliation every
1,000 operations achieved 0.33M updates/s at 1,788 B/update and spent 83.1% of measured time reconciling.

At 4 producers / 1,000 series, intervals 100/1,000/10,000 produced 0.48M/2.30M/2.05M updates/s, with reconciliation
shares 85.6%/35.7%/5.9%. These are single sensitivity observations; the non-monotonic 10,000 result demonstrates host
noise and must not be treated as a stable throughput ordering. Hybrid is measured as an amortized model.

### Cross-environment benchmark statistics

| Candidate                        | Environment            | Repetitions | p50 updates/s | p95 updates/s | p99 updates/s | p50 bytes/update |
|----------------------------------|------------------------|------------:|--------------:|--------------:|--------------:|-----------------:|
| Snapshot                         | macOS/LinuxKit aarch64 |          30 |        28,108 |        36,507 |        37,232 |           25,890 |
| Snapshot                         | Ubuntu/LinuxKit x86_64 |          30 |         6,402 |        16,391 |        20,877 |           25,892 |
| Operation fast path              | macOS/LinuxKit aarch64 |          30 |     4,482,102 |     5,325,475 |     5,397,698 |                6 |
| Operation fast path              | Ubuntu/LinuxKit x86_64 |          30 |     2,208,350 |     2,987,775 |     3,274,232 |                6 |
| Hybrid amortized, interval 1,000 | macOS/LinuxKit aarch64 |          30 |     4,314,971 |     4,991,317 |     5,044,581 |               19 |
| Hybrid amortized, interval 1,000 | Ubuntu/LinuxKit x86_64 |          30 |     1,970,287 |     2,391,584 |     2,954,448 |               19 |

| Scale/sensitivity case                           |                         macOS/LinuxKit |                        Ubuntu/LinuxKit |
|--------------------------------------------------|---------------------------------------:|---------------------------------------:|
| Snapshot, 16 producers / 10k series              |      249 updates/s; 3,822,541 B/update |      216 updates/s; 3,822,694 B/update |
| Operation fast path, 16 producers / 10k series   |           4.16M updates/s; 34 B/update |           1.81M updates/s; 34 B/update |
| Hybrid interval 1,000, 16 producers / 10k series | 0.335M updates/s; 83.1% reconciliation | 0.331M updates/s; 85.3% reconciliation |
| Hybrid interval 100, 4 producers / 1k series     | 0.476M updates/s; 85.6% reconciliation | 0.331M updates/s; 84.7% reconciliation |
| Hybrid interval 1,000, 4 producers / 1k series   |  2.30M updates/s; 35.7% reconciliation |  1.15M updates/s; 34.5% reconciliation |
| Hybrid interval 10,000, 4 producers / 1k series  |   2.05M updates/s; 5.9% reconciliation |   2.10M updates/s; 7.2% reconciliation |

Signal-to-exit latency is not an INV-004 metric: this prototype does not launch or signal a workload and generates no
signal-to-exit evidence. That lifecycle measurement belongs to INV-001/INV-002. The Ubuntu statistics relevant to
INV-004 are the throughput, allocation and reconciliation measurements above.

## Evaluation Against Criteria

| Criterion                    | Snapshot                | Absolute           | Operations                          | Hybrid                         |
|------------------------------|-------------------------|--------------------|-------------------------------------|--------------------------------|
| Correct after drops          | yes after next snapshot | only resent series | no                                  | yes after reconciliation       |
| Receiver restart recovery    | republish               | partial republish  | no                                  | republish                      |
| Client complexity            | medium                  | low                | medium                              | highest                        |
| Protocol complexity          | medium                  | low                | medium                              | highest                        |
| Throughput                   | registry-size dependent | expected high      | high                                | high fast path + snapshot cost |
| Memory/network amplification | highest                 | low                | low                                 | configurable                   |
| Multiple producers           | owner-scoped            | ambiguous          | safe for compatible commutative ops | owner-scoped                   |

## Admissible values and protocol constraints

The accepted production boundary is one complete application snapshot.

- MetricShell structurally validates the whole candidate before state mutation.
- A valid candidate atomically replaces the previous valid snapshot.
- A correctly encoded zero-series snapshot clears active application series; an absent or empty payload is malformed.
- Missing series are removed by omission from the newly accepted snapshot.
- Duplicate series, type conflicts, and candidate-internal metadata conflicts reject the complete candidate.
- An established family name-to-type binding survives omission until the next workload execution.
- Other metadata is validated for internal consistency within each candidate and is not lifetime-bound.
- Classic histograms require ordered cumulative buckets, `+Inf` equal to `count`, and numeric `sum`.
- Counters and histograms are not validated for business monotonicity across snapshots.
- MetricShell does not accept instrumentation operations or aggregate values across producers.
- Socket/HTTP success acknowledges atomic installation in one linear acceptance order; producer timestamps do not
  reorder
  candidates.
- The final application state is the last valid accepted complete snapshot, or the zero-series initial snapshot.

`producer_id`, `producer_epoch`, `sequence`, gaps, completeness state, and reconciliation intervals belong only to the
rejected operation/hybrid aggregator model.

## Additional Benchmarks and Coverage

The runner executes the complete original semantic and synthetic benchmark scope: all candidates, scale and
producer grids, 30-run distributions, allocation, loss/reorder/duplicate faults, both restart directions, explicit gap
state, transactional conflict rejection, operation epoch transitions, snapshot monotonicity, full histogram component
aggregation, gauge/type/histogram ownership conflicts, and hybrid reconciliation intervals 100/1,000/10,000.
`semantics.tsv` and `assertions.tsv` record the individual contracts; `coverage.tsv` records the high-level groups.

This is deliberately broader than current product scope. Keeping the broader runner preserves falsifying and complexity
evidence without converting its explored mechanisms into implementation requirements.

For higher-confidence performance sizing, use a quiet native Linux host with 100 repetitions and fixed CPU/memory. Real
transport encodings, histogram bucket scaling, crash-safe persistence, disk-full behavior and hostile cardinality are
not hidden agreements: they are explicit follow-up measurements for INV-005–009 because they cannot be measured
honestly by a transport-free semantic model.

## Limitations

- Research-only in-memory Go model; no production protocol or durable store.
- Both measured container environments use LinuxKit: macOS/LinuxKit `linux/aarch64` and Ubuntu/LinuxKit
  `linux/x86_64`. Native non-LinuxKit Linux, containerd/CRI-O and Kubernetes remain unverified.
- The runner recorded the prototype help banner in `container_go_version` rather than the build-stage Go version in
  both environments. This malformed informational field is excluded from all evidence comparisons; source identity is
  established by the matching benchmark fingerprint.
- Docker image tags may resolve to different base image digests later; compare benchmark fingerprint and recorded image
  provenance, or archive/promote the built image for byte-identical reruns.
- Synthetic throughput isolates ownership data structures and excludes serialization, syscalls and network/filesystem
  transport.

## Conclusion

The matching-fingerprint evidence supports a narrower conclusion than the original report. Reconciliation is required
if MetricShell accepts state-changing operations. Producer ownership and aggregation policies are required if
MetricShell combines independently owned registries. Both capabilities are outside MetricShell's scope.

Select one complete, conflict-free application snapshot for file, Unix stream, and local HTTP. Every transport performs
the same application operation: structural validation followed by atomic replacement of the last valid snapshot.
MetricShell preserves Prometheus types but does not apply instrumentation operations, enforce business monotonicity
between snapshots, or aggregate colliding producer values. The decision is recorded in
[ADR-004](../../docs/06-architecture/adr/ADR-004.md).
