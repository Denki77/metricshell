# INV-020 — Managed Aggregation Lifecycle and Core Integration

**Status:** in progress

**macOS reference run:** `results/20260925T122604Z`

**Ubuntu confirmation:** `results/20260925T121532Z` — confirmed

**Report:** [report.md](report.md)

## Question

What exact lifecycle connects managed ingestion, workload termination, registry freeze, final snapshot installation and
the existing Core final-scrape lifecycle?

## Current Result

The matching-fingerprint macOS-host Docker Desktop/LinuxKit ARM64 and Ubuntu-host Docker Desktop/LinuxKit x86_64 runs
each passed 54/54 portable assertions, all E-020.1–E-020.7 summaries and three process-level lifecycle checks. All four
freeze candidates and all six receive-to-ACK shutdown stages were executed. The evidence provisionally selects a
bounded hybrid: close admission first, allow only already admitted work to reach commit within the remaining
finalization budget, then freeze and install exactly one complete generation.

Ubuntu confirmation is complete. The research remains **in progress** until ADR-020 is prepared and accepted; no ADR
is created by this package yet.

## Prototype and Evidence

- `prototype/cmd/inv020` — deterministic lifecycle model plus concurrency and fault matrices.
- `prototype/Dockerfile` — Linux stand used identically on macOS and Ubuntu.
- `run-bench.sh` — builds the image, runs all matrices and process-level exit/signal/post-exit checks, and captures the
  environment.
- `freeze-candidates.tsv` — immediate, unbounded drain, explicit handshake and bounded-hybrid comparison.
- `shutdown-stages.tsv` — partial, received, queued, committing, committed-before-ACK and acknowledged outcomes.
- `drain-deadlines.tsv` — 30 budget/cost combinations.
- `final-scrape-matrix.tsv` — three wait modes crossed with seven request classes.
- `failure-injection.tsv` — conversion, validation and installation failures.
- `restart-epochs.tsv` — 30 fresh-process epochs.
- `freeze-concurrency.tsv` — 30 rounds with 128 concurrent freeze contenders.
- `process-lifecycle.tsv` — real container exits for natural completion, TERM and Job-shaped post-exit behavior.

## Run the Stand

The command is identical on macOS and Ubuntu and requires Docker, `tar`, Perl and standard POSIX shell tools:

```bash
./research/INV-020/run-bench.sh
latest="research/INV-020/$(cat research/INV-020/latest-results.txt)"
cat "$latest/summary.tsv"
cat "$latest/assertions.tsv"
cat "$latest/environment.tsv"
```

The runner uses a 2-CPU, 256-MiB container for the main matrix. It does not bind-mount host source or result paths into
the container; evidence is copied with `docker cp`.

## Ubuntu Stand Fingerprint

Export and run the exact same stand:

```bash
./research/INV-020/export-stand.sh /tmp/inv020-stand.tar.gz
mkdir /tmp/inv020-stand
tar -xzf /tmp/inv020-stand.tar.gz -C /tmp/inv020-stand
cd /tmp/inv020-stand
./verify-fingerprint.sh
./run-bench.sh
```

Expected portable source/runner fingerprint:

```text
bbdde683d7c54d83ed82181634b639297d68e41ba2fb15ca4f350b5eaf36264f
```

The fingerprint covers the Dockerfile, Go source, module file, runner, exporter and verifier. Image IDs legitimately
differ by architecture and are provenance, not portable identity.

## Prototype Limits

- This is research code and a deterministic lifecycle model, not the production MetricShell implementation.
- Core validation/install and the managed Unix transport are represented at their contract boundaries; they are not
  imported from production code.
- The Job/CronJob matrix verifies API-independent container lifecycle semantics, not a live Kubernetes control plane.
- Both environments use Docker Desktop/LinuxKit; a native Ubuntu kernel remains unverified.
- Microsecond sleep timings expose scheduler granularity and must not be used as deadline or latency guarantees.
- The 250 ms post-exit case proves bounded behavior, not a production default.

## Better Follow-up Benchmarking

Stronger capacity evidence should use a native Ubuntu kernel with pinned CPUs, fixed governor and at least 30 full
process repetitions. Integrate the production Unix parser, bounded owner queue,
Core validator/atomic installer and HTTP server; timestamp close-admission, last commit, freeze, install and final-wait
entry. Add CPU throttling, maximum-cardinality snapshots, slow/aborting scrapers, socket backlog saturation and cgroup
memory pressure. Run race detection and multi-hour randomized exit/signal/ACK-loss schedules. These improve sizing and
implementation confidence; they do not replace the complete candidate and stage matrices already run here.
