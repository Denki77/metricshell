# INV-015 — Benchmarks and Final Comparison

**Status:** completed
**macOS reference run:** `results/20260802T180839Z`
**Ubuntu/LinuxKit reference run:** `results/20260803T071831Z`
**Report:** [report.md](report.md)
**Decision:** [ADR-015](../../docs/06-architecture/adr/ADR-015.md)

## Question

What are the measured costs, saturation behavior and failure characteristics of the selected complete-snapshot
architecture?

## Context and Rules

ADR-004 remains the benchmark semantic boundary: every publication is a complete application snapshot and successful
publication atomically replaces active state. Benchmark counts are not increments and no results are summed across
snapshots. Warm-up, repeated iterations, raw data, environment metadata and p50/p95/p99 are retained. Timing values are
observations, not pass thresholds.

## Evidence Required

- idle CPU/memory/descriptors and startup distribution;
- complete-snapshot ingestion at 100/1k/10k publications/s;
- 100/1k/10k/100k cardinality;
- 1/2/5/10 concurrent scrapes;
- polling/inotify/hybrid file detection;
- graceful drain and forced resource exhaustion;
- final scrape, aborted response and timeout;
- malformed input, bind failure and queue overflow;
- matching Ubuntu fingerprint.

## Confirmed Result

Both matching-fingerprint Docker/LinuxKit runs passed 23/23 assertions. Ten repeated iterations were recorded per
in-container shape.
Complete-snapshot replacement under 100 concurrent publications exposed exactly 100 series from one generation. The
HTTP cardinality response grew from 5,120 bytes at 100 series to 5,677,820 bytes at 100k; observed host scrape time grew
from 20.927 ms to 73.183 ms.

In-container 100k encoding p50/p95 was 42.053/43.892 ms. File-detection p50 was 1.311 ms polling, 54.541 µs inotify and
36.167 µs hybrid in this run. Ten concurrent external scrapes completed in 321.664 ms. Graceful stop waited for an
in-flight 800 ms response and completed in 1.364 s. Final-scrape, aborted-large-response, timeout, malformed, queue,
bind-failure and OOM profiles all passed.

## Final Interpretation

- immutable pointer replacement keeps concurrent scrape correctness simple;
- complete-snapshot cost scales approximately with cardinality and must be bounded;
- inotify/hybrid has materially lower detection latency than polling in LinuxKit, but polling remains portable fallback;
- final-scrape completion must be counted after successful full response write;
- queue, parser, bind and cgroup failures need explicit bounded policies;
- these cross-environment research numbers are not production SLOs or saturation limits.

## Running the Prototype

```bash
./research/INV-015/run-bench.sh
latest="$(cat research/INV-015/latest-results.txt)"
cat "$latest/assertions.tsv"
cat "$latest/benchmark-raw.tsv"
cat "$latest/benchmark-summary.tsv"
cat "$latest/startup.tsv"
cat "$latest/idle.tsv"
cat "$latest/cardinality-http.tsv"
cat "$latest/environment.tsv"
```

The runner executes every listed workload locally; final-scrape and failure cases are not delegated to other
investigations. macOS and Ubuntu use the same fingerprint.

## Prototype Limits

- Synthetic selected-architecture model, not the final production binary.
- Ingestion rate benchmark uses 100-series complete snapshots for a 100 ms target window; it characterizes replacement
  cost, not network transport saturation.
- Ten iterations satisfy repeated-run comparison but 30+ are recommended for release SLO work.
- Concurrent in-process scrape work isolates snapshot read/body access; the external 10-client case includes HTTP.
- Docker Desktop host timing includes scheduler and curl overhead.
- Only Linux inotify is compared; kqueue and other platforms are outside Linux deployment scope.
- Both container environments use LinuxKit: macOS/LinuxKit aarch64 and Ubuntu/LinuxKit x86_64. Native non-LinuxKit
  Linux is not covered.

## Additional Benchmarks

| Benchmark                                 | Status                              |
|-------------------------------------------|-------------------------------------|
| warm-up and 10 repeated iterations        | covered                             |
| ingestion 100/1k/10k complete snapshots/s | covered                             |
| p50/p95/p99 ingestion latency             | covered                             |
| cardinality 100/1k/10k/100k               | covered in-process and HTTP         |
| concurrent scrape 1/2/5/10                | covered plus external 10-client run |
| polling/inotify/hybrid                    | covered                             |
| startup distribution                      | covered: 10 containers              |
| idle CPU/memory/FD                        | covered: 10 samples                 |
| graceful HTTP drain                       | covered                             |
| final one/abort/timeout                   | covered locally                     |
| malformed/bind/queue/OOM                  | covered locally                     |
| Ubuntu matching fingerprint               | covered: 23/23 assertions           |
| 30+ iterations, pinned CPU and perf/eBPF  | recommended on native Ubuntu        |
| production transports side-by-side        | repeat after production integration |

## Better Follow-up Benchmarking

Repeat 30–100 times with pinned CPUs, cgroup CPU quotas and controlled
host load. Record allocations, cycles, cache misses and context switches. Run the same semantic complete snapshots
through production file/socket/HTTP adapters; never compare partial updates with complete snapshots.

## Decision Output

- Prototype: `prototype/`
- macOS evidence: `results/20260802T180839Z/`
- Ubuntu evidence: `results/20260803T071831Z/`
- Report: [report.md](report.md)
- ADR: [ADR-015](../../docs/06-architecture/adr/ADR-015.md)
