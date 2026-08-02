# INV-015 Report — Benchmarks and Final Comparison

**Status:** in progress
**Run date:** 2026-08-02
**Reference run:** `results/20260802T180839Z`
**Platform:** Docker 29.6.2, LinuxKit aarch64, 12 host CPUs, 16 GiB host RAM
**Fingerprint:** `ffd49f1d5a23d5484a270329c6610d99196480228f4b84ad6b1aac10bbd9a807`

## Goal and Semantic Boundary

Measure repeated costs and execute the full correctness/failure matrix for the selected immutable complete-snapshot
model. Publication means validate/generate one complete candidate and atomically replace active state. No operation
counts are added to application metrics.

## Method

The container performs warm-up and ten iterations for ingestion, cardinality, concurrency, file detection and
initialization. The runner aggregates p50/p95/p99, then performs real HTTP/container startup, idle, replacement,
cardinality, concurrency, queue, drain, final-scrape, abort, timeout, bind and OOM scenarios. Raw and summarized data
are
both retained.

## Results

All 23 portable assertions passed.

### Cardinality

|  Series | Encoded bytes | Encode p50 | Encode p95 | HTTP bytes | Host scrape observation |
|--------:|--------------:|-----------:|-----------:|-----------:|------------------------:|
|     100 |         5,080 |  27.166 µs |  39.458 µs |      5,120 |               20.927 ms |
|   1,000 |        52,780 | 297.750 µs | 315.000 µs |     52,820 |               22.658 ms |
|  10,000 |       547,780 |   3.426 ms |   4.229 ms |    547,820 |               25.251 ms |
| 100,000 |     5,677,780 |  42.053 ms |  43.892 ms |  5,677,820 |               73.183 ms |

The difference between encoded and HTTP bytes is the prototype self-metric. Host observations include Docker and curl.

### File detection

| Mode    |       p50 |       p95 |
|---------|----------:|----------:|
| polling |  1.311 ms |  1.363 ms |
| inotify | 54.541 µs | 90.750 µs |
| hybrid  | 36.167 µs | 52.958 µs |

Hybrid races inotify with a polling fallback. These LinuxKit observations support event-driven detection with bounded
fallback, not a universal timing guarantee.

### Runtime and failures

Ten startup and ten idle samples were recorded. One hundred concurrent complete publications ended with one exact
100-series generation. The external ten-scraper wall observation was 321.664 ms. Queue saturation produced explicit
429 responses. A malformed candidate returned 400 with last-valid retention. Docker stop waited for an 800 ms in-flight
response; observed stop time was 1.364 s.

The local final-scrape cases proved that health requests do not count, one complete response exits, an aborted large
chunked response does not count, a later complete response exits, and timeout exits with zero scrapes. Invalid bind
returned 70. A 128 MiB allocation under 32 MiB was OOMKilled with exit 137.

## Evaluation

Complete-snapshot encoding scales with series count and needs configured bounds. Atomic immutable replacement produced
single-generation scrapes under concurrent publication. Event-driven file detection outperformed polling locally.
Backpressure and cgroup failure were observable and bounded. Final response counting must remain after full write.

The benchmark does not choose transport using unequal semantics: all measured publications are full snapshots. The
results must not be used as production SLOs until reproduced on Ubuntu/native Linux with controlled resources.

## Provisional Architecture Values

- preserve atomic immutable complete-snapshot replacement;
- retain explicit payload/cardinality/concurrency/time/memory bounds from INV-014;
- prefer inotify with polling/reconciliation fallback for Linux file transport;
- pre-encode/bound large exposition responses;
- retain one eligible full-response final-scrape default plus timeout;
- retain explicit error counters for malformed, queue, bind and resource failures;
- use 100k series only as a tested upper research shape, not a default.

## Limitations and Additional Benchmarking

Ten iterations and Docker Desktop are architecture evidence, not final performance certification. The in-process
concurrency microbenchmark isolates pointer/body access. Network transport candidates are not compared here because the
production integration is not yet present. Ubuntu, 30+ pinned runs, perf/eBPF, allocation profiles and production
adapter
comparison remain. Every listed local failure/final scenario was executed rather than referenced.

## Conclusion

The selected model is viable within the tested envelope and all local correctness/failure cases passed. Cardinality is
the dominant exposition/encoding dimension; event-driven file detection is preferable on Linux; explicit backpressure,
timeouts and cgroups are mandatory. The investigation remains in progress until matching Ubuntu evidence and final
production integration.

## Decision Output

- Evidence: `results/20260802T180839Z/`
- Ubuntu evidence: pending
- Final ADR/completion: pending
