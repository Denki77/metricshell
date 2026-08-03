# INV-010 Report — Prometheus Exposition

**Status:** completed
**Run dates:** 2026-08-03
**Reference runs:** `results/20260803T182806Z`, `results/20260803T184050Z`
**Fingerprint:** `edfea8c5efb2528bb1a131b8e1125c4a00aa354a011a5015272bb83086969456`

## Goal

Determine the minimum correct Prometheus-compatible exposition contract, prove scrape consistency under complete
snapshot replacement, and select format, concurrency and resource-limit policies without violating
ADR-004.

## ADR-004 Boundary

The benchmark never sums snapshots or combines independently owned registries. Each generated A/B/cardinality body is
one complete workload-owned candidate. The prototype parses the whole candidate, canonically encodes it and swaps an
immutable pointer only after success. Every scrape loads that pointer once. MetricShell self-metrics are appended from a
separate state domain. A malformed candidate cannot mutate active application state.

## Prototype and Commands

- `prototype/cmd/inv010` — parser/encoder, atomic snapshot holder and HTTP exposition server;
- `prototype/Dockerfile` — reproducible Linux build and runtime;
- `run-bench.sh` — complete correctness and observation matrix;
- `results/<timestamp>` — assertions, observations, raw bodies, headers and logs.

```bash
./research/INV-010/run-bench.sh
latest="$(cat research/INV-010/latest-results.txt)"
cat "$latest/assertions.tsv"
cat "$latest/cardinality.tsv"
cat "$latest/concurrent-scrapes.tsv"
cat "$latest/environment.tsv"
```

The pinned Prometheus image digest runs `promtool check metrics`. Both environments used the same runner and
fingerprint;
there is no separate Ubuntu script or code copy.

## Run Environment

| Environment                       |       Date | Docker | Architecture | Result                     | Status                |
|-----------------------------------|-----------:|-------:|--------------|----------------------------|-----------------------|
| Docker Desktop on macOS/LinuxKit  | 2026-08-03 | 29.6.2 | aarch64      | `results/20260803T182806Z` | 18/18 assertions pass |
| Docker Desktop on Ubuntu/LinuxKit | 2026-08-03 | 27.4.0 | x86_64       | `results/20260803T184050Z` | 18/18 assertions pass |

Both reference runs have benchmark fingerprint
`edfea8c5efb2528bb1a131b8e1125c4a00aa354a011a5015272bb83086969456`. All 18 assertions passed in both environments.

## Results

### Format and validation

Prometheus text responses negotiated `text/plain; version=0.0.4`; OpenMetrics requests negotiated
`application/openmetrics-text; version=1.0.0` and ended with `# EOF`. HELP/TYPE metadata and the classic histogram
(`+Inf == count == 5`) were present. A snapshot with timestamp `1700000000000` was accepted and preserved. The pinned
official `promtool` returned exit 0 for the baseline response.

### Atomic replacement and partial failure

The runner alternated 120 complete A/B installations while issuing 120 scrapes at concurrency 16. Every response
header and all 250 application-series labels identified the same generation; zero response contained a mixed A/B
state. A syntactically malformed candidate returned HTTP 400, and the previously accepted timestamped snapshot remained
visible. These results support whole-candidate validation and atomic replacement, not merge semantics.

### Cardinality observations

| Application series | Response bytes | macOS install/scrape | Ubuntu install/scrape |
|-------------------:|---------------:|---------------------:|----------------------:|
|                  0 |            874 |   22.947 / 21.258 ms |    21.884 / 21.182 ms |
|              1,000 |         58,734 |   29.487 / 21.737 ms |    24.957 / 22.295 ms |
|             10,000 |        608,735 |   60.980 / 48.559 ms |    80.620 / 30.728 ms |
|            100,000 |      6,378,736 |  305.846 / 64.312 ms |  466.965 / 134.274 ms |

These host-side single-run values include curl and Docker Desktop scheduling. They describe scale, not an SLO or an
accepted production limit.

### Client and resource behavior

All 32 parallel scrape commands completed; their aggregate host-side wall time was recorded in `observations.tsv`.
Gzip negotiation produced a valid decompressed body. A 1 KiB/s client with a one-second client timeout and an explicitly
disconnected raw TCP client did not make the server unhealthy. With a 1,024-byte response bound and 10k application
series, the server returned HTTP 503 with a small error body before committing exposition headers.

## Hypothesis Evaluation

### Use an existing Prometheus parser/encoder

Supported. The existing common library parsed candidates, rejected malformed syntax and generated
canonical family text accepted by official `promtool`. Handwritten code remains necessary for HTTP negotiation,
snapshot selection, response preflight and lifecycle policy, but not for metric grammar.

### Prometheus text should be the baseline and OpenMetrics negotiated

Supported within the tested metric types. Both formats carried the same selected application snapshot. OpenMetrics adds
the EOF framing requirement. Exemplars and native histograms remain outside this evidence.

### Concurrent scrape can remain consistent without registry locking

Supported for immutable pointer replacement. Loading one snapshot pointer per request was sufficient for zero mixed
bodies during the tested replacement/scrape race.

### Partial failures must not emit partial success

Supported. Malformed candidates did not replace active state, and oversized responses returned a preflight 503. A
production implementation must preserve this ordering when adding all structural rules and streaming/compression.

## Evaluation Against Criteria

| Criterion                     | Confirmed result                                 |
|-------------------------------|--------------------------------------------------|
| Prometheus compatibility      | text 0.0.4 accepted by pinned `promtool`         |
| OpenMetrics                   | 1.0 negotiation and EOF covered                  |
| complete-snapshot consistency | 120/120 concurrent bodies were single-generation |
| partial candidate failure     | atomic rejection with last-valid retention       |
| large registry                | 0–100k synthetic series observed                 |
| concurrent clients            | 32 complete clients covered                      |
| slow/disconnected clients     | server survival covered                          |
| response limit                | preflight 503 covered                            |
| Ubuntu reproducibility        | 18/18 with matching fingerprint                  |

## Accepted Values and Policies

- Prometheus text 0.0.4 is the default compatibility format.
- OpenMetrics text 1.0 is opt-in through Accept negotiation and ends with EOF.
- A scrape selects one immutable complete application snapshot once and appends separate self-metrics.
- A candidate becomes active only after complete parsing and validation; rejection preserves the last valid snapshot.
- Application snapshots are replaced, never summed, merged or reconciled.
- Response limits apply to the fully encoded uncompressed body before a success response is committed.
- Compression is a transfer concern and must not change snapshot identity.
- The tested 100k series and 32 clients are research envelope values, not proposed defaults.

## Prototype Limits

- Both container environments use LinuxKit: macOS/LinuxKit aarch64 and Ubuntu/LinuxKit x86_64. Native non-LinuxKit
  Linux is not covered.
- No complete implementation of every ADR-004 structural rule or explicit zero-family product encoding.
- No exemplars, native histograms, protobuf, TLS, HTTP/2 or reverse proxy.
- Single-run wall times include host tools and Docker Desktop.
- Disconnect handling proves server-side write failure/survival behavior, never TSDB persistence.
- No 30-run latency distribution or CPU/RSS/allocation profile yet.

## Additional Benchmarking

| Item                                | Status      | Evidence/Reason                         |
|-------------------------------------|-------------|-----------------------------------------|
| formats, negotiation and metadata   | covered     | response bodies and headers             |
| classic histogram and timestamp     | covered     | `prometheus.txt`, `timestamp.metrics`   |
| official promtool validation        | covered     | `promtool.log`, exit assertion          |
| malformed candidate retention       | covered     | HTTP 400 and retained timestamp         |
| concurrent replace/scrape           | covered     | `concurrent-scrapes.tsv`                |
| 0/1k/10k/100k cardinality           | covered     | `cardinality.tsv`                       |
| 32 concurrent clients               | covered     | assertion and wall observation          |
| gzip, slow and disconnected clients | covered     | headers/logs and health assertions      |
| response-size preflight             | covered     | 1,024-byte bound, HTTP 503              |
| Ubuntu matching fingerprint         | covered     | 18/18 assertions, identical fingerprint |
| repeated CPU/RSS/latency profile    | recommended | 30+ Ubuntu repetitions, pinned CPUs     |
| exemplars/native histograms         | not covered | outside the accepted snapshot scope     |
| proxy/TLS/HTTP2                     | not covered | deployment-layer scope                  |

## Conclusion

The matching macOS and Ubuntu results confirm the library-based exposition direction. Prometheus text 0.0.4 plus
negotiated
OpenMetrics 1.0 is sufficient for the tested types, and immutable complete-snapshot replacement provides consistent
concurrent scrapes. Pre-encoding enables a truthful response-size failure before partial success.

INV-010 is completed. The selected contract is recorded in [ADR-010](../../docs/06-architecture/adr/ADR-010.md).

## Decision Output

- Prototype: `prototype/`
- Runner: `run-bench.sh`
- macOS raw evidence: `results/20260803T182806Z/`
- Ubuntu raw evidence: `results/20260803T184050Z/`
- Direction: Prometheus text default, negotiated OpenMetrics, immutable snapshot per scrape, preflight bound
- ADR: [ADR-010](../../docs/06-architecture/adr/ADR-010.md)
