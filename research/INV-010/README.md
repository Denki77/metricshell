# INV-010 — Prometheus Exposition

**Status:** completed
**macOS reference run:** `results/20260803T182806Z`
**Ubuntu/LinuxKit reference run:** `results/20260803T184050Z`
**Report:** [report.md](report.md)
**Decision:** [ADR-010](../../docs/06-architecture/adr/ADR-010.md)

## Question

Which exposition formats and consistency guarantees should MetricShell provide?

## Context and Hypotheses

ADR-004 requires one complete, conflict-free application snapshot at a time. MetricShell validates a complete
candidate and atomically replaces the previous accepted snapshot; it never sums snapshots, merges series, applies
instrumentation operations or aggregates producer registries. A scrape must therefore load one immutable application
snapshot and expose it in full, together with separately owned MetricShell self-metrics.

The initial hypothesis is that an existing Prometheus parser/encoder should define canonical text output and validation
instead of a handwritten formatter. Prometheus text 0.0.4 should be the compatibility baseline; OpenMetrics 1.0 may be
offered by content negotiation. Response limits must fail before a partial successful response is committed.

## Evidence Required

- Prometheus text and OpenMetrics negotiation, metadata, classic histograms and optional timestamps;
- external `promtool check metrics` validation;
- atomic complete-snapshot replacement during concurrent scrape;
- last-valid-snapshot retention after malformed input;
- 0, 1k, 10k and 100k application-series observations;
- multiple concurrent, slow and disconnected scrapers;
- gzip negotiation and response-size enforcement;
- one matching-fingerprint Ubuntu/LinuxKit repeat.

## Confirmed Result

The matching-fingerprint macOS/LinuxKit aarch64 and Ubuntu/LinuxKit x86_64 runs each passed all 18 portable assertions.
The official Prometheus `promtool` image accepted the generated text in both environments. In each run, all 120 scrapes
concurrent with 120 alternating complete A/B replacements contained exactly 250 series from one generation and no
empty, partial or mixed snapshot. A malformed candidate returned `400` and retained the previous complete snapshot.
The 100k-series response was 6,378,736 bytes in both environments.

Both runs also passed 32 concurrent scrapers, gzip, slow-client and disconnected-client survival, and returned a
preflight `503` under a 1,024-byte response limit. The identical fingerprint is
`edfea8c5efb2528bb1a131b8e1125c4a00aa354a011a5015272bb83086969456`.

## Accepted Values

- application state: one immutable, complete accepted snapshot loaded once per scrape;
- snapshot update: whole-candidate validation followed by atomic replacement, never summation or merge;
- baseline format: Prometheus text `0.0.4`;
- negotiated format: OpenMetrics text `1.0.0` with `# EOF`;
- metadata: preserve valid HELP, TYPE, classic histogram and optional timestamp representation;
- malformed candidate: reject atomically and retain the last valid snapshot;
- response limit: evaluate the complete uncompressed response before committing a success status;
- compression: gzip may be negotiated independently of application-snapshot identity;
- self-metrics: appended from MetricShell-owned state and never folded into application state;
- measured research envelope: 0–100,000 synthetic application gauge series and 1–32 concurrent scrapers.

These values are adopted by [ADR-010](../../docs/06-architecture/adr/ADR-010.md).

## Running the Prototype

Run from the repository root on macOS or Ubuntu:

```bash
./research/INV-010/run-bench.sh
```

Inspect the latest evidence:

```bash
latest="$(cat research/INV-010/latest-results.txt)"
cat "$latest/assertions.tsv"
cat "$latest/summary.tsv"
cat "$latest/cardinality.tsv"
cat "$latest/concurrent-scrapes.tsv"
cat "$latest/observations.tsv"
cat "$latest/environment.tsv"
cat "$latest/promtool.log"
```

The runner builds one Linux image, uses ephemeral loopback host ports, runs the complete matrix and stores raw evidence
under `results/<UTC timestamp>/`. Ubuntu uses exactly the same command. Compare
`benchmark_code_fingerprint_sha256`; repository HEAD and image/container IDs are context only. The fingerprint includes
only `prototype/` and `run-bench.sh`, so documentation and result changes do not change benchmark identity.

Manual server run:

```bash
docker build -t metricshell-inv010:prototype research/INV-010/prototype
docker run --rm -p 127.0.0.1:19100:19100 metricshell-inv010:prototype
curl -H 'Accept: application/openmetrics-text; version=1.0.0' http://127.0.0.1:19100/metrics
```

## Prototype Limits

- Research server, not the production MetricShell ingestion or exposition implementation.
- The Prometheus common parser/encoder is exercised, but the prototype does not implement every ADR-004 structural
  rule, especially lifetime name-to-type binding and the product's explicit zero-family snapshot encoding.
- OpenMetrics output uses canonical Prometheus-family text plus the required EOF marker; exemplars, native histograms
  and protobuf exposition are not evaluated.
- The cardinality numbers are single-run Docker Desktop observations, not SLOs or accepted defaults.
- A successful HTTP write proves only completion of the server-side write operation, not Prometheus TSDB persistence.
- Both container environments use LinuxKit: macOS/LinuxKit aarch64 and Ubuntu/LinuxKit x86_64. Native non-LinuxKit
  Linux, other HTTP stacks and Kubernetes are outside this investigation.

## Additional Benchmarks

| Benchmark                                                     | Status                                                                     |
|---------------------------------------------------------------|----------------------------------------------------------------------------|
| Prometheus text 0.0.4 and OpenMetrics 1.0 negotiation         | covered                                                                    |
| HELP, TYPE, classic histogram and optional timestamp          | covered                                                                    |
| official `promtool check metrics`                             | covered with pinned image digest                                           |
| malformed candidate and last-valid retention                  | covered                                                                    |
| concurrent complete-snapshot replacement and scrape           | covered: 120 replacements, 120 scrapes, zero mixed bodies                  |
| cardinality 0, 1k, 10k and 100k                               | covered                                                                    |
| 32 concurrent scrapers                                        | covered                                                                    |
| gzip negotiation                                              | covered                                                                    |
| slow and disconnected scrapers                                | covered                                                                    |
| response-size preflight failure                               | covered at 1,024 bytes                                                     |
| runtime self-metrics beside application snapshot              | covered                                                                    |
| matching-fingerprint Ubuntu/LinuxKit repeat                   | covered: 18/18 assertions with the same fingerprint                        |
| repeated latency distribution, CPU/RSS and allocation profile | recommended on Ubuntu with 30+ repetitions                                 |
| exemplars and native histograms                               | not covered; outside the accepted text/OpenMetrics snapshot scope          |
| HTTP/2, TLS and reverse proxy behavior                        | outside local exposition decision; test after deployment model is selected |

## Better Follow-up Benchmarking

For additional performance characterization, repeat each cardinality/concurrency shape
at least 30 times, pin CPUs where possible, record cgroup CPU/RSS and allocations, and report median plus p95/p99 rather
than promoting this macOS timing to a limit. Any extension must continue to publish complete snapshots and assert that
each scrape sees one old or new snapshot, never a sum or partial merge.

## Decision Output

- Prototype: `prototype/`
- Runner: `run-bench.sh`
- macOS raw evidence: `results/20260803T182806Z/`
- Ubuntu raw evidence: `results/20260803T184050Z/`
- Detailed analysis: [report.md](report.md)
- ADR: [ADR-010](../../docs/06-architecture/adr/ADR-010.md)
