# INV-017 — Concurrent Publishers and Ordering

**Status:** completed

**Run:** `results/20260924T194912Z`

**Reference evidence:** `results/20260924T194912Z/reference`

**Extended evidence:** `results/20260924T194912Z/extended`

**Ubuntu confirmation:** `results/20260924T195921Z` — confirmed

**Report:** [report.md](report.md)

**Decision:** [ADR-017](../../docs/06-architecture/adr/ADR-017.md)

## Question

How should one managed registry accept concurrent operations with an observable order, bounded memory and explicit
duplicate/unknown-outcome behavior?

## Current Result

The macOS Docker Desktop/LinuxKit ARM64 and Ubuntu Docker Desktop/LinuxKit x86_64 runs each passed 136/136 reference
assertions, 311/311 extended assertions and the race detector with zero reported races. Both used fingerprint
`22bc1820e394d9e7331be2857ca2298376b0d410ce52728833f1ca2f04792697`. Linked cross-family mutations showed
that the tested sharded and atomic-family candidates can return mixed-generation complete snapshots; serialized,
global-lock and copy-on-write candidates returned one committed registry generation. ADR-017 selects a bounded
single-owner serialized mutation loop for correctness and proof simplicity, not because it won the throughput benchmark.

Requirements mandate bounded, observable overload but do not mandate equal admission share or starvation protection.
The stand therefore retains the negative fairness evidence and documents an explicit no-fairness guarantee rather than
adding an unrequested scheduler.

## Decision Summary

- Accepted mutations have one registry-wide commit order; per-connection receive order is preserved into the owner.
- A complete managed snapshot corresponds to one registry-wide committed state.
- Success ACK means committed, not merely received, parsed or queued.
- Admission is bounded and overload rejection is observable; no production queue capacity is selected.
- Equal admission share and starvation protection under overload are not guaranteed.
- Lost ACK after possible commit is an unknown outcome without stable publisher/session plus operation identity.
- Histogram observation updates count, sum and applicable cumulative buckets as one mutation.
- First accepted descriptor wins; compatible repeats succeed and conflicts reject without mutation.

## Run the Prototype

```bash
./research/INV-017/run-research.sh
reference="research/INV-017/$(cat research/INV-017/latest-reference-results.txt)"
extended="research/INV-017/$(cat research/INV-017/latest-extended-results.txt)"
cat "$reference/summary.tsv"
cat "$extended/summary.tsv"
```

The command is identical on macOS and Ubuntu; only Docker is required. It creates one timestamped run directory with
`reference/`, `extended/` and `race/`, verifies the fingerprint and executes all three phases. A failure retains
`failure.tsv`, all logs produced so far and prints the failing phase plus log tails to stderr. It never deletes result
sets; evidence cleanup is an explicit
maintainer action, so an Ubuntu confirmation cannot remove macOS evidence. Defaults are 1,000 operations
per publisher with three repetitions for reference and 10,000 with ten repetitions for extended. Override them with
`INV017_REFERENCE_OPS`, `INV017_REFERENCE_REPETITIONS`, `INV017_EXTENDED_OPS` and
`INV017_EXTENDED_REPETITIONS`.

To carry the exact stand scope to Ubuntu:

```bash
./research/INV-017/export-stand.sh /tmp/inv017-stand.tar.gz
tar -xzf /tmp/inv017-stand.tar.gz
cd research/INV-017
./verify-fingerprint.sh
./run-research.sh
```

The portable source/runner fingerprint is
`22bc1820e394d9e7331be2857ca2298376b0d410ce52728833f1ca2f04792697`. Image IDs are architecture-specific and are
recorded for provenance, but are not used to claim identical ARM64/AMD64 binaries.

## Evidence Files

Run-level files are `summary.tsv`, `fingerprint.tsv`, `run-environment.tsv` and `run-set.tsv`. Reference and extended
subdirectories contain:

- `assertions.tsv` — portable semantic and safety checks;
- `throughput.tsv`, `latency.tsv` — observational candidate/publisher matrix;
- `snapshot-concurrency.tsv` — snapshots sampled while histogram writes are active;
- `snapshot-linearizability.tsv` — registry-wide generation checks during linked mutations;
- `backpressure.tsv` — capacities 1/16/64/1024 under non-blocking overload;
- `fairness.tsv` — order baseline (limited; see report);
- `overload-fairness.tsv` — per-publisher admission distribution and Jain index;
- `mixed-workload.tsv` — counter/gauge/histogram interaction under concurrency;
- `environment.tsv` — repository, benchmark, Docker, kernel and image fingerprints.

The `race/` subdirectory contains `race-detector.log` and `race-detector.tsv`.

The runner uses `docker create`, `docker start` and `docker cp`; it does not bind-mount host result or source paths into
containers. This keeps the command valid when Docker Desktop or a remote-context daemon cannot see the client path.

## Prototype Limits

- The framing harness is an internal test adapter, not the INV-018 protocol choice.
- The registry state is deliberately minimal and reuses the semantic decisions of ADR-016; it is not production code.
- Linked counter/gauge markers are a synthetic witness for mixed registry generations, not a proposed client operation.
- Timings include goroutine scheduling inside one container and are not portable limits.
- Queue capacity 64 is a reference setting, not a production default. Production sizing needs INV-019 workload/SLA data.
- The run does not replace sanitizer runs, native non-LinuxKit kernel coverage or end-to-end legacy clients. Those gaps
  are explicit in the report and belong to later investigations where applicable.
