# INV-016 — Managed Registry Semantics

**Status:** completed

**Reference run:** `results/20260924T114723Z`

**Ubuntu confirmation run:** `results/20260924T123514Z`

**Report:** [report.md](report.md)

**Decision:** [ADR-016](../../docs/06-architecture/adr/ADR-016.md)

## Question

Which registry, descriptor, operation and snapshot semantics are viable before concurrency, transport, limits and final
lifecycle integration are selected?

## Evidence Rule

Prototype assertions demonstrate that candidate semantics are implemented consistently and preserve the tested
invariants. They do not alone select a product decision. INV-016 combines those assertions, actual Core-parser outcomes
and matching-fingerprint portable confirmation; later concerns remain deferred to INV-017–INV-020.

## Evidence and Portable Confirmation

The runner covers E-016.1–E-016.7. The macOS/LinuxKit aarch64 reference run and Ubuntu/LinuxKit x86_64 confirmation run
both passed 64/64 semantic assertions and matched all 7/7 expected Core compatibility outcomes. No architecturally
significant difference was found.

Both runs recorded:

- benchmark fingerprint: `1f4be9f67c79cb5710dc24bcc6401c1f3d2112f95ee66933e4a1dfd3eb435f1e`;
- Core source fingerprint: `6cb7e1021fb8806af25a8d3470cf2b789d4958089777a837c800d94c6dc26f9d`.

Fingerprints identify the benchmark sources, Dockerfiles, runner behavior, expected Core outcomes and exact Core
snapshot package. Repository HEAD/dirtiness and timings are recorded separately. Timing differences between ARM64 and
x86_64 are observations only, not portable acceptance criteria; INV-019 owns performance conclusions.

## Final Conclusions

- Managed Registry uses explicit descriptor semantics at its internal boundary. A descriptor supplies metric-family
  identity, metric type, HELP, label schema and histogram bucket schema where applicable. Convenience clients may hide
  declaration; the external API remains INV-018 scope.
- Counter baseline semantics are explicit initialization and increment/add with a non-negative finite delta. A new
  epoch starts empty and a counter cannot silently decrease within an epoch.
- Gauge SET is the required semantic baseline.
- Initial histogram semantics use non-negative observations and non-negative fixed bucket boundaries so every state is
  representable by the existing Core snapshot contract.
- Metric family plus canonical label set determines series identity. Label input order does not change identity;
  missing or extra labels violate the descriptor schema.
- Descriptor conflicts and rejected mutations fail atomically without corrupting prior valid state.
- Managed Registry materializes one complete candidate application snapshot and passes it through the existing Core
  validation and installation boundary. Managed Aggregation is not an alternative Core; ADR-001–ADR-015 are unchanged.

## Demonstrated Feasibility, Not Selected API

- Monotonic absolute counter update and gauge ADD/SUB work in one ordered mutation stream.
- All-or-nothing batching works through candidate/clone state.
- Signed histogram candidates can update count, sum and cumulative buckets atomically.
- A negative observation followed by a positive observation can yield a Core-accepted final snapshot when final sum and
  boundaries are non-negative. A complete snapshot therefore does not encode observation history.

Prometheus instrumentation permits negative observations, but current Core requires a present non-negative sum and
non-negative boundaries. Managed Registry must not admit an intermediate state that cannot safely cross that boundary.
INV-016 does not change Core; signed-histogram support is separate future scope.

## Deferred

- Multiple publishers, ordering, retry, duplicate delivery, lost ACK, absolute counter updates and same-epoch reset:
  INV-017. Same-epoch reset is neither accepted nor prohibited by INV-016.
- Gauge ADD/SUB concurrency: INV-017; external API value: INV-018.
- External protocol, wire duplicate-key rules and optional batch API: INV-018.
- Publisher ownership/races for deletion: INV-017; staleness, freeze and epoch cleanup: INV-020.
- Snapshot materialization strategy, resource limits and cadence: INV-019.

## Follow-up Status

[INV-017](../INV-017/README.md) is completed. Matching-fingerprint macOS/LinuxKit and Ubuntu/LinuxKit runs cover all five
concurrency candidates and E-017.1–E-017.9, including registry-wide snapshot linearizability and the explicit overload
fairness contract. Its decision is recorded in [ADR-017](../../docs/06-architecture/adr/ADR-017.md). This does not change
the completed INV-016 semantic decision.

## Running the Prototype

```bash
./research/INV-016/run-bench.sh
latest="$(cat research/INV-016/latest-results.txt)"
cat "$latest/summary.tsv"
cat "$latest/assertions.tsv"
cat "$latest/core-validation.tsv"
cat "$latest/materialization.tsv"
cat "$latest/scaling.tsv"
cat "$latest/environment.tsv"
```

The same command runs on macOS and Ubuntu. Increase observation-only repetitions with `INV016_REPETITIONS=100`.

## Prototype Limits and Further Benchmarking

- The prototype is single-threaded and has no external protocol or legacy-client implementation.
- Signed histogram mode is a comparison candidate, not production behavior.
- Deletion, persistence, summaries, native histograms, exemplars and timestamps are not implemented.
- Microbenchmarks do not select production limits.

The default runner covers repeated materialization, 0/1/10/100/1,000/10,000-series scaling, byte size, seven Core
compatibility cases and environment/code fingerprints. INV-019 should add allocation/RSS, mutation throughput,
label-width, bucket-count, mixed-family, cadence, cgroup and percentile matrices under pinned, controlled resources.

## Decision Output

- Prototype: `prototype/`
- Core compatibility checker: `corecheck/`
- Runner: `run-bench.sh`
- Reference evidence: `results/20260924T114723Z/`
- Ubuntu evidence: `results/20260924T123514Z/`
- Detailed analysis: [report.md](report.md)
- ADR: [ADR-016](../../docs/06-architecture/adr/ADR-016.md)
