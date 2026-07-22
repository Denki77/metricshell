# INV-004 Report — Metric-state Ownership and Semantics

Status: validation in progress

Run date: 2026-07-22

Docker server: 29.4.3

Docker platform: linux/aarch64, LinuxKit 6.12.76

Reference run: `results/20260723T073114Z`

Summary: `results/20260723T073114Z/summary.tsv`

## Goal

Determine which component owns metric truth and which update semantics remain correct under loss, ordering changes,
duplicates, restarts, stale data, type conflicts and multiple producers.

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

| Environment             |              Date | Docker | Platform         | Result set                 | Fingerprint                                                        |                Result |
|-------------------------|------------------:|-------:|------------------|----------------------------|--------------------------------------------------------------------|----------------------:|
| Docker Desktop on macOS |        2026-07-23 | 29.4.3 | LinuxKit/aarch64 | `results/20260723T073114Z` | `e52784470ff33e35fb58ab142be26a345bb6a373bd2eac58666b269f56875fd6` | 34/34 assertions pass |
| Ubuntu                  | pending execution |      — | —                | —                          | must equal reference                                               |              prepared |

The fingerprint covers only benchmark source and runner content with relative names, so it is invariant to checkout
path and repository HEAD. Environment and image identifiers are separately recorded in `environment.tsv`.

## Results

### Semantic scenarios

| Candidate                   |          Recover dropped update |       Receiver restart |                       Multi-producer |        Stale removal | Verdict                 |
|-----------------------------|--------------------------------:|-----------------------:|-------------------------------------:|---------------------:|-------------------------|
| Complete per-owner snapshot |           yes, at next snapshot |   yes, after republish | deterministic compatible aggregation |                  yes | authoritative model     |
| Per-series absolute         | only when same series is resent |   only after republish |           ambiguous last-writer-wins | no registry boundary | gauges only, with owner |
| Operations only             |                              no |                     no |           counter increments commute |                   no | reject as sole truth    |
| Hybrid                      |          yes, at reconciliation | yes, at reconciliation |                         owner-scoped |                  yes | accept                  |

Four rows in `semantics.tsv` intentionally have `result=fail`: they are falsifying counterexamples, not runner
failures. They demonstrate absolute counter decrease, absolute multi-producer collision, lost operation and state loss
after receiver restart. `assertions.tsv` checks every named scenario independently plus exact scenario-set cardinality;
it no longer relies on aggregate pass/fail counts.

The operation path enforces the full `(producer_id, producer_epoch, sequence)` boundary. An old epoch is rejected. A
new epoch observed through an operation becomes incomplete/non-authoritative and cannot mutate values; its initial
complete snapshot authorizes subsequent operations starting at the new sequence space.

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

## Admissible Values and Protocol Constraints

The accepted boundary is `(producer_id, producer_epoch, sequence)`. A complete owner snapshot replaces only that
owner's registry contribution. Older epochs and older/duplicate sequences are ignored. A newly observed epoch cannot
apply operations until its initial authoritative snapshot. Missing series become stale and are removed. Types and
histogram boundaries cannot change silently. Counter values and cumulative histogram buckets/count cannot decrease
within an epoch; a new epoch may reset them.

Operations are optional hints/fast-path updates. A sequence gap makes the owner incomplete until a snapshot repairs it;
receiver restart requires republish; final application state requires a final snapshot. No fixed reconciliation period
is selected here because it depends on transport throughput and acceptable loss window.

## Additional Benchmarks and Coverage

The runner executes every in-scope semantic and synthetic benchmark identified for INV-004: all candidates, scale and
producer grids, 30-run distributions, allocation, loss/reorder/duplicate faults, both restart directions, explicit gap
state, transactional conflict rejection, operation epoch transitions, snapshot monotonicity, full histogram component
aggregation, gauge/type/histogram ownership conflicts, and hybrid reconciliation intervals 100/1,000/10,000.
`semantics.tsv` and `assertions.tsv` record the individual contracts; `coverage.tsv` records the high-level groups.

The next higher-confidence run should use a quiet native Ubuntu host with 100 repetitions and fixed CPU/memory. Real
transport encodings, histogram bucket scaling, crash-safe persistence, disk-full behavior and hostile cardinality are
not hidden agreements: they are explicit follow-up measurements for INV-005–009 because they cannot be measured
honestly by a transport-free semantic model.

## Limitations

- Research-only in-memory Go model; no production protocol or durable store.
- Current timing evidence is one macOS-hosted LinuxKit/aarch64 Docker environment.
- Docker image tags may resolve to different base image digests later; compare benchmark fingerprint and recorded image
  provenance, or archive/promote the built image for byte-identical reruns.
- Synthetic throughput isolates ownership data structures and excludes serialization, syscalls and network/filesystem
  transport.

## Conclusion

The macOS/LinuxKit evidence partially confirms the original hypothesis. Different transports may use different
representations, but they must converge on one authoritative semantic model. Provisionally select hybrid semantics:
versioned complete per-producer snapshots own truth; sequenced/deduplicated operations are optional acceleration;
reconciliation is mandatory after gaps and restart and for final state. Operations-only ownership and unowned absolute
counters are rejected by the current evidence. Completion and final ADR input remain gated on the matching-fingerprint
Ubuntu validation run.
