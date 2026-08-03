# INV-015 Report — Benchmarks and Final Comparison

**Status:** completed
**Run dates:** 2026-08-02–2026-08-03
**Reference runs:** `results/20260802T180839Z`, `results/20260803T071831Z`
**Fingerprint:** `ffd49f1d5a23d5484a270329c6610d99196480228f4b84ad6b1aac10bbd9a807`
**Decision:** [ADR-015](../../docs/06-architecture/adr/ADR-015.md)

## Goal and Semantic Boundary

Measure repeated costs and execute the correctness/failure matrix for the selected immutable complete-snapshot model.
Publication means validating or generating one complete candidate and atomically replacing active state. Benchmark
operation counts are not application-metric increments, and snapshots are never summed.

## Method

The container performs warm-up plus ten iterations for ingestion, cardinality, concurrent scrape, file detection and
initialization. The runner aggregates p50/p95/p99 and then executes real HTTP/container startup, idle, replacement,
cardinality, concurrency, queue, drain, final-scrape, abort, timeout, bind and OOM scenarios. Raw and summarized data
are
retained independently from pass/fail assertions; timing observations are not assertion thresholds.

## Run Environments

| Environment                       | Date       | Docker | Architecture | Result                     | Status                |
|-----------------------------------|------------|-------:|--------------|----------------------------|-----------------------|
| Docker Desktop on macOS/LinuxKit  | 2026-08-02 | 29.6.2 | aarch64      | `results/20260802T180839Z` | 23/23 assertions pass |
| Docker Desktop on Ubuntu/LinuxKit | 2026-08-03 | 27.4.0 | x86_64       | `results/20260803T071831Z` | 23/23 assertions pass |

Both runs have the same fingerprint and all 23 assertions passed in each environment. Both container environments use
LinuxKit. This is a cross-architecture LinuxKit comparison, not native non-LinuxKit Linux performance certification.

## Results

### Cardinality and exposition

|  Series | Encoded bytes | macOS encode p50/p95 | Ubuntu encode p50/p95 | macOS HTTP scrape | Ubuntu HTTP scrape |
|--------:|--------------:|---------------------:|----------------------:|------------------:|-------------------:|
|     100 |         5,080 |     27.166/39.458 µs |      45.236/67.709 µs |         20.927 ms |          12.348 ms |
|   1,000 |        52,780 |   297.750/315.000 µs |    536.000/631.374 µs |         22.658 ms |          12.901 ms |
|  10,000 |       547,780 |       3.426/4.229 ms |        4.390/6.081 ms |         25.251 ms |          18.131 ms |
| 100,000 |     5,677,780 |     42.053/43.892 ms |      47.359/57.563 ms |         73.183 ms |          67.106 ms |

The HTTP response adds a small prototype self-metric to the encoded application body. Encoding cost scales primarily
with cardinality. Architecture and host scheduling change timings but not response sizes or correctness.

### Complete-snapshot ingestion

| Target rate | Accepted per 100 ms | macOS achieved p50 | Ubuntu achieved p50 | macOS latency p50/p95 | Ubuntu latency p50/p95 |
|------------:|--------------------:|-------------------:|--------------------:|----------------------:|-----------------------:|
|       100/s |                  10 |          111.070/s |           111.070/s |      26.750/32.875 µs |       28.522/59.661 µs |
|     1,000/s |                 100 |        1,009.859/s |         1,009.812/s |      23.375/30.708 µs |       26.535/62.012 µs |
|    10,000/s |               1,000 |       10,007.639/s |        10,007.259/s |      23.084/30.541 µs |       26.431/63.583 µs |

These synthetic 100-series replacements characterize the in-process selected architecture; they do not measure a
production transport or authorize cumulative metric semantics.

### Concurrent scrape

| Concurrent readers | macOS wall p50 | Ubuntu wall p50 |
|-------------------:|---------------:|----------------:|
|                  1 |       0.916 µs |        1.032 µs |
|                  2 |       0.625 µs |        1.761 µs |
|                  5 |       1.375 µs |        2.391 µs |
|                 10 |       1.875 µs |        4.977 µs |

The in-process shape isolates immutable pointer/body access. The external ten-client HTTP wall observation was
321.664 ms on macOS and 503.644 ms on Ubuntu. Concurrent publication still exposed one exact generation.

### File detection

| Mode    |    macOS p50/p95 |     Ubuntu p50/p95 |
|---------|-----------------:|-------------------:|
| polling |   1.311/1.363 ms | 237.989/299.719 µs |
| inotify | 54.541/90.750 µs | 277.413/479.544 µs |
| hybrid  | 36.167/52.958 µs | 275.151/487.869 µs |

Inotify/hybrid materially outperformed the one-millisecond polling configuration on macOS/LinuxKit. Ubuntu/LinuxKit
showed lower polling latency for this short sample. The portable conclusion is event-driven notification with periodic
reconciliation, not a universal latency ranking.

### Startup and idle

Ten startup and ten idle samples were retained in each environment. macOS readiness ranged from 194.877 to 514.710 ms;
Ubuntu readiness ranged from 2,345.439 to 3,610.678 ms. Idle CPU was reported as 0.00% in all samples. Memory stayed
near
2.93–3.23 MiB on macOS and 2.93–2.97 MiB on Ubuntu; open descriptors remained 6.

### Drain, final scrape and failures

Docker stop waited for an in-flight 800 ms response. Observed drain/stop time was 1.364 s on macOS and 1.464 s on
Ubuntu. Queue saturation produced explicit 429 responses. Malformed candidates returned 400 with last-valid retention.
The final-scrape cases confirmed that health requests do not count, one complete response releases the wait, an aborted
large response does not count, a later complete response does count, and timeout exits with zero fabricated scrapes.
Invalid bind returned internal exit 70. A 128 MiB allocation under 32 MiB was OOMKilled with exit 137.

## Evaluation

- Immutable pointer replacement preserved one-generation scrape consistency in both environments.
- Complete-snapshot encoding and exposition cost scale with cardinality and require explicit bounds.
- File transport should use event notification plus reconciliation; measured latency ordering is environment-specific.
- Backpressure, timeout, bind and cgroup failures are observable and bounded.
- Final response counting belongs after a complete successful write and still does not prove TSDB persistence.
- Timing differences are operational observations, not correctness differences.

## Accepted Architecture Values

- Preserve atomic immutable complete-snapshot replacement.
- Retain explicit payload, cardinality, concurrency, timeout and memory bounds from INV-014.
- Use inotify with periodic polling/reconciliation fallback for Linux file transport.
- Pre-encode and bound large exposition responses before committing success.
- Use one eligible complete-response final scrape plus a finite timeout as the default from INV-011.
- Emit explicit self-metrics for malformed input, queue rejection, bind failure and resource exhaustion.
- Treat 100k series as the tested upper research shape, not a default or an SLO.
- Require controlled 30+ run release benchmarks before publishing performance targets.

## Limitations

- Both container environments use LinuxKit; native non-LinuxKit Linux was not tested.
- The selected architecture is synthetic and not yet the final production binary with every transport integrated.
- Ten iterations are architecture evidence, not final statistical certification.
- Host timing includes Docker Desktop scheduling and command-line client overhead.
- The ingestion benchmark uses generated 100-series snapshots and does not measure network transport saturation.
- Only Linux inotify is compared; kqueue and non-Linux notification mechanisms are outside scope.
- In-process concurrent scrape isolates pointer/body access from HTTP and network work.

## Additional Benchmarking

| Benchmark                                 | Status    | Evidence/Boundary                        |
|-------------------------------------------|-----------|------------------------------------------|
| warm-up and ten repeated iterations       | covered   | raw and summary TSV                      |
| ingestion 100/1k/10k complete snapshots/s | covered   | both environments                        |
| cardinality 100/1k/10k/100k               | covered   | in-process and HTTP                      |
| concurrent scrape 1/2/5/10                | covered   | in-process plus external ten-client case |
| polling/inotify/hybrid                    | covered   | environment-specific observations        |
| startup/idle CPU/memory/FD                | covered   | ten samples in each environment          |
| graceful drain                            | covered   | in-flight 800 ms response                |
| final scrape/abort/timeout                | covered   | executed locally in both runs            |
| malformed/bind/queue/OOM                  | covered   | executed locally in both runs            |
| matching Ubuntu fingerprint               | covered   | 23/23 assertions, identical fingerprint  |
| 30–100 pinned CPU runs and perf/eBPF      | follow-up | release performance certification        |
| production adapters side-by-side          | follow-up | repeat with identical complete snapshots |

## Conclusion

INV-015 confirms that the selected complete-snapshot architecture is viable within the tested envelope in both
LinuxKit environments. Cardinality is the dominant encoding/exposition dimension; event-driven file notification with
reconciliation is preferred; explicit backpressure, timeouts and cgroups are mandatory. The measurements are research
baselines rather than production SLOs. The decision is recorded in
[ADR-015](../../docs/06-architecture/adr/ADR-015.md).

## Decision Output

- macOS evidence: `results/20260802T180839Z/`
- Ubuntu evidence: `results/20260803T071831Z/`
- ADR: [ADR-015](../../docs/06-architecture/adr/ADR-015.md)
