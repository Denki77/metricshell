# INV-004 — Metric-state Ownership and Semantics

Status: validation in progress

Reference run: `results/20260723T073114Z`

Report: [report.md](report.md)

## Question

Does the workload send complete registry snapshots, absolute series values, update operations, or a combination?

## Context

The representation determines who owns metric truth, whether loss and restart are recoverable, and whether several
producers can update the same exported family without ambiguity.

## Candidates

- complete per-producer registry snapshots;
- absolute values for individual series;
- operations (`increment`, `set`, `observe`);
- hybrid: operations as an optional fast path plus authoritative periodic/final snapshots.

## Initial Hypothesis

File ingestion naturally favors complete snapshots. Socket and local push may favor operations or absolute updates.
Equivalent client semantics do not require identical transport semantics.

## Experiments

The Docker prototype evaluates 33 deterministic scenarios covering counters, gauges, histograms, duplicate series, type
conflicts,
multiple producers, ordering, dropped updates, producer/receiver restarts, stale data and final reconciliation. It then
runs all candidate combinations at 1/4/16 producers and 1/100/1,000/10,000 series, plus 30 representative repetitions.

## Results

Reference run `results/20260723T073114Z` recorded 33 scenarios, 29 confirmed invariants and four expected
counterexamples, with 0 failed per-scenario assertions and 129 benchmark rows. The benchmark-code fingerprint is
`e52784470ff33e35fb58ab142be26a345bb6a373bd2eac58666b269f56875fd6`.

- Complete per-producer snapshots recovered from a dropped intermediate update and receiver/producer epoch changes,
  removed stale series, rejected duplicate sequence numbers and aggregated two owners deterministically.
- Per-series absolute counter values allowed a decrease (`10 -> 7`) and last-writer-wins could not represent two
  producers' intended aggregate (`3` expected, `2` retained).
- Operations were idempotent when owner sequence numbers were present and counter increments from different owners
  commuted, but one dropped increment produced `2` instead of `3`; receiver restart produced `0` instead of `5`.
- A rejected type conflict does not consume its sequence. Gaps explicitly mark an owner incomplete; duplicate and late
  operations neither create nor repair a gap, and an authoritative snapshot clears the incomplete flag.
- Operations carry `(producer_id, producer_epoch, sequence)`. Old epochs are rejected; a new epoch is incomplete and
  cannot submit operations until its initial authoritative snapshot is accepted.
- Snapshots reject counter decreases, type changes and histogram schema changes within an epoch. A new epoch may begin
  with a lower counter or reset histogram. Histograms model bounds, cumulative buckets, count and sum; compatible
  states aggregate component-wise, while bucket mismatches and cumulative bucket decreases are rejected.
- Duplicate gauges are rejected without a policy; an explicit `sum` policy is verified separately.
- Hybrid reconciliation repaired both loss and receiver restart and removed stale series through a complete owner
  snapshot.
- Representative p50 throughput was 28,108 full snapshots/s for 100-series snapshots, 4.48M operation-fast-path
  updates/s and 4.31M amortized hybrid updates/s with reconciliation every 1,000 operations.
- At 4 producers / 1,000 series, reconciliation every 100/1,000/10,000 operations yielded 0.48M/2.30M/2.05M updates/s
  and spent 85.6%/35.7%/5.9% of measured time reconciling. These single sensitivity samples are observations, not a
  monotonic performance claim; the 10,000 result shows host noise can dominate one run.
- At 16 producers / 10,000 series the snapshot case allocated about 3.82 MB per complete update, operation fast path
  34 B/update, and amortized hybrid at interval 1,000 about 1,788 B/update.

## Conclusion

The macOS/LinuxKit evidence partially confirms and refines the initial hypothesis: transport representation may differ,
but application-level truth must not. Provisionally select the hybrid model with **complete, versioned, per-producer
snapshots as the authoritative state**, pending the matching-fingerprint Ubuntu validation run.
Operations may be supported only as a performance adapter between snapshots; they cannot be the sole durable truth.

Reject unowned absolute counter values and operation-only recovery. Gauges may use absolute `set`, but still require an
owner identity and reconciliation. Histogram observations need the same sequencing/deduplication guarantees as counter
operations; authoritative snapshots carry cumulative bucket/count/sum state.

## Admissible Semantic Values

- state owner: explicit stable producer ID;
- producer epoch: monotonically changed on producer restart;
- update sequence: strictly increasing within `(producer, epoch)`; duplicate/older values are ignored;
- initial snapshot gate: a producer epoch is not authoritative and cannot apply operations until its first complete
  snapshot; observing a newer operation epoch marks it incomplete but does not mutate metric values;
- authoritative unit: complete snapshot of one producer's registry, not a global snapshot from an arbitrary producer;
- missing series in a newer complete owner snapshot: delete that owner's series contribution;
- counter: non-negative cumulative value in snapshots; no decrease within an epoch;
- gauge: absolute set is allowed;
- histogram: cumulative buckets, count and sum in snapshots; bucket schema/type is immutable within an epoch;
- multi-producer export: aggregate only compatible types/schema; counters and histogram components sum, gauge
  aggregation must be explicitly selected (otherwise reject duplicates);
- type/schema conflict: reject the conflicting update and expose an ingestion error;
- operations: optional, owner-sequenced and deduplicated; reconcile after detected gaps, receiver restart and at final
  application state;
- snapshot interval/operation batch size: not fixed by INV-004; choose from transport benchmarks in INV-005–008.

## Running the Prototype

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

Manual scenario run:

```bash
docker build -t metricshell-inv004:prototype research/INV-004/prototype
docker run --rm metricshell-inv004:prototype --mode=scenarios
```

Manual scale point:

```bash
docker run --rm metricshell-inv004:prototype \
  --mode=benchmark --candidate=hybrid_amortized --producers=16 --series=10000 --updates=100000 \
  --reconciliation-interval=1000
```

Set `INV004_REPEAT_COUNT=100` for a longer distribution run. The runner only removes containers named `inv004-*` and
writes a fresh UTC result directory.

## Ubuntu Fingerprint

The runner hashes normalized relative names and contents of `prototype/` plus `run-bench.sh`; it does not include the
host path, repository HEAD, timestamps or results. Therefore the same unmodified checkout produces one
`benchmark_code_fingerprint_sha256` on macOS and Ubuntu. Compare that field in `environment.tsv` before comparing
measurements. `container_kernel`, Docker platform, image ID and repository HEAD remain provenance, not code identity.

## Prototype Limits

- This is an in-memory semantic model, not production parsing, persistence or a transport implementation.
- Measurements include Go map/string allocation and Docker startup per sample; they are comparative research evidence,
  not service SLOs.
- Crash-safe disk persistence, wire encoding, authentication and hostile cardinality are deferred to transport/security
  investigations.
- The current measured host is Docker Desktop/LinuxKit aarch64. An Ubuntu run is prepared with the same fingerprint but
  must be executed on that environment before claiming cross-host timing parity.
- Gauge aggregation is intentionally rejected unless configured because sum/last/max have different meanings.

## Additional Benchmarks

All in-scope additions were executed: candidate comparison; 1/4/16 producers; 1/100/1,000/10,000 series; dropped,
reordered and duplicate updates; producer and receiver restarts; stale deletion; type conflict; duplicate ownership;
counter/gauge/histogram behavior; explicit gap state; transactional conflict rejection; hybrid intervals
100/1,000/10,000; 30-run p50/p95/p99 throughput; allocation per update; and the
macOS/Ubuntu path-independent fingerprint.

For stronger follow-up evidence, run `INV004_REPEAT_COUNT=100` on a quiet native Ubuntu host, pin CPU/memory, record
`docker stats`, and add real encodings/transports. Histogram bucket-count scaling, fsync/WAL recovery, cardinality
limits, payload compression and concurrent ingestion belong in INV-005–009 because this prototype deliberately
isolates semantics from transport.

## Decision Output

- Prototype: `prototype/`
- Runner: `run-bench.sh`
- Raw evidence: `results/20260723T073114Z/`
- Detailed report: [report.md](report.md)
- Provisional ADR input: authoritative versioned per-producer snapshots with optional sequenced operations and
  mandatory reconciliation; finalize after Ubuntu validation.
