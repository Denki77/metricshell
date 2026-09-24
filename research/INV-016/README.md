# INV-016 — Managed Registry Semantics

**Status:** in progress

**Reference run:** `results/20260924T114723Z`

**Report:** [report.md](report.md)

**Decision:** deferred until matching-fingerprint Ubuntu evidence and later ADR review

## Question

Which registry, descriptor, operation and snapshot semantics are viable before concurrency, transport, limits and final lifecycle integration are selected?

## Evidence Interpretation

This investigation separates external constraints, candidate semantics, prototype assertions, evidence-backed results,
provisional recommendations and questions deferred to INV-017–INV-020.

Passing prototype assertions demonstrates that the tested candidate semantics are implemented consistently and preserve
the tested invariants. It does not by itself prove that those semantics are the correct product or architectural choice.

## Candidates

- explicit declaration, implicit first-use declaration, or explicit internal declaration hidden by a convenience client;
- counter increment/add, initialization, absolute update, same-epoch reset and new-epoch reset;
- gauge SET and optional ordered ADD/SUB;
- non-negative-only or signed classic-histogram observations/boundaries;
- individual operations or all-or-nothing batches;
- epoch lifetime with deletion deferred, or explicit deletion/staleness.

## Experiments and Results

The runner covers E-016.1–E-016.7. The reference run passed 64/64 candidate-invariant assertions. It generated seven
representative snapshots and checked every one with the actual `implementation/internal/snapshot` parser.

The signed-histogram candidate accepted negative observations, negative/mixed boundaries, a negative sum, a sum crossing
zero, repeated negative/positive observations and `-Inf`. Core supplied independent compatibility evidence:

- negative sum: rejected `histogram_invalid`;
- negative bucket boundary: rejected `histogram_invalid`;
- balanced signed histogram with negative boundary: rejected `histogram_invalid`;
- `-Inf` sum: rejected `histogram_invalid`;
- negative then positive observations whose final sum is `0.5`, with non-negative boundaries: accepted.

Thus negative observations are not intrinsically impossible, but not every intermediate or final signed state can cross
the existing complete-snapshot boundary. See [report.md](report.md) for the model review and analysis.

## Demonstrated

- Typed operations can preserve deterministic state in one ordered mutation stream.
- Rejected operations and batches can leave prior state byte-identical.
- Increment/add, initialization and monotonic absolute counter update are implementable.
- Gauge SET and finite ADD/SUB are implementable under ordered mutation semantics.
- Restrictive and signed histogram candidates can update count/sum/cumulative buckets atomically.
- All-or-nothing batching is feasible with candidate/clone state.
- Explicit descriptors and canonical labels can generate complete Core snapshots.
- A new epoch starts empty; disconnect need not mutate state.

These are feasibility and consistency results, not accepted architecture.

## Provisional Recommendations

- Keep explicit descriptor semantics at the registry boundary because type, HELP, labels and histogram buckets must exist
  before an operation can be interpreted. A convenience client may hide declaration.
- Keep increment/add as the counter baseline. Initialization and monotonic absolute update remain provisionally viable.
- Require gauge SET; consider ADD/SUB convenience only after ordering and client-use evidence.
- For the initial Core-compatible candidate, use the current non-negative histogram subset. This is a compatibility
  recommendation, not a claim that Prometheus histograms cannot observe negative values.
- Keep atomic batch as a feasible option rather than an initial-protocol requirement.

## Deferred

- Counter absolute update, same-epoch reset and duplicate/retry behavior: INV-017.
- Gauge ADD/SUB concurrency: INV-017; external API value: INV-018.
- Any Core revision for signed histograms: outside INV-016; current Core remains unchanged.
- Batch in the external protocol: INV-018.
- Deletion/staleness: publisher ownership in INV-017 and lifecycle in INV-020.
- Resource limits and snapshot cadence: INV-019.
- Final acceptance and ADR: matching Ubuntu evidence first.

## Running the Prototype

```bash
./research/INV-016/run-bench.sh
```

```bash
latest="$(cat research/INV-016/latest-results.txt)"
cat "$latest/summary.tsv"
cat "$latest/assertions.tsv"
cat "$latest/core-validation.tsv"
cat "$latest/materialization.tsv"
cat "$latest/scaling.tsv"
cat "$latest/environment.tsv"
```

The same command runs on macOS and Ubuntu. Increase observation-only repetitions with `INV016_REPETITIONS=100`.

## Ubuntu Fingerprint Check

Compare `benchmark_code_fingerprint_sha256` and `core_snapshot_source_sha256` in both `environment.tsv` files. Both must
match. Repository HEAD and timings are context, not identity or portable pass criteria.

- benchmark: `1f4be9f67c79cb5710dc24bcc6401c1f3d2112f95ee66933e4a1dfd3eb435f1e`;
- Core source: `6cb7e1021fb8806af25a8d3470cf2b789d4958089777a837c800d94c6dc26f9d`.

## Prototype Limits

- Single-threaded; no concurrency, delivery or retry conclusions.
- No external protocol or legacy-client implementation.
- Signed histogram mode is a comparison candidate, not production behavior.
- Current Core requires a sum and rejects negative sums/bounds; the prototype does not change Core.
- Microbenchmarks are observations only and do not select production limits.
- Deletion, persistence, summaries, native histograms, exemplars and timestamps are not implemented.
- Ubuntu confirmation remains pending.

## Additional Benchmarks

The default runner executes repeated materialization, 0/1/10/100/1,000/10,000-series scaling, byte-size measurement,
seven Core compatibility cases and environment/code fingerprints. INV-019 should add allocation/RSS, mutation
throughput, label-width, bucket-count, mixed-family, cadence, cgroup and percentile matrices.

## Decision Output

- Prototype: `prototype/`
- Core compatibility checker: `corecheck/`
- Runner: `run-bench.sh`
- Evidence: `results/20260924T114723Z/`
- Detailed analysis: [report.md](report.md)
- ADR: not created.
