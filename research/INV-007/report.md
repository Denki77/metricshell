# INV-007 Report — Socket-Based Ingestion

[Русская версия](report_ru.md)

Status: completed
Run dates: 2026-07-23, 2026-07-28
Docker servers: 29.4.3, 27.4.0
Docker platforms: LinuxKit 6.12.76 `linux/aarch64`, LinuxKit 6.10.14 `linux/x86_64`  
Reference runs: `results/20260723T190106Z`, `results/20260728T114224Z`
Summaries: `results/20260723T190106Z/summary.tsv`, `results/20260728T114224Z/summary.tsv`

## Goal

Select the local socket type, framing, resource boundaries and failure policies for MetricShell ingestion.

## Prototype

- `prototype/cmd/inv007-bench`: Unix-socket server, producers, assertions and measurements.
- `prototype/Dockerfile`: multi-stage Linux image.
- `run-bench.sh`: identical macOS/Ubuntu runner, evidence extraction and fingerprint capture.
- `results/<timestamp>`: correctness, performance, pressure and environment TSV evidence.

The candidates are newline-delimited Unix stream, four-byte length-framed Unix stream and newline-delimited Unix
datagram. Every application message starts with protocol version `v1`.

## Run Environments

| Environment           | Date       | Docker | Kernel           | Architecture | Result set                 | Fingerprint                                                        |
|-----------------------|------------|-------:|------------------|--------------|----------------------------|--------------------------------------------------------------------|
| macOS Docker Desktop  | 2026-07-23 | 29.4.3 | LinuxKit 6.12.76 | aarch64      | `results/20260723T190106Z` | `585b91f1a73f1359953cc313af2e1f3f7ff1f9757ee00086056c812357a78bca` |
| Ubuntu Docker Desktop | 2026-07-28 | 27.4.0 | LinuxKit 6.10.14 | x86_64       | `results/20260728T114224Z` | `585b91f1a73f1359953cc313af2e1f3f7ff1f9757ee00086056c812357a78bca` |

The fingerprints are identical. Repository HEAD and architecture-specific image IDs differ as expected.

Both container environments use LinuxKit. This confirms cross-architecture behavior inside LinuxKit aarch64/x86_64,
not native non-LinuxKit Linux.

## Assertions

All portable assertions passed in both environments:

| Group                | macOS | Ubuntu |
|----------------------|------:|-------:|
| Correctness          | 32/32 |  32/32 |
| Performance delivery | 81/81 |  81/81 |
| Pressure/resource    |   5/5 |    5/5 |

Correctness covers permission, one producer, valid/malformed input, exact 65,536-byte maximum, oversized rejection,
partial stream message, shutdown refusal, bounded startup retry and restart reconnect.

## Performance

All 81 performance rows per environment delivered every submitted message without a write deadline.

Representative mean-of-three values:

| Environment | Protocol      | Producers | Payload | Messages/s |       p50 |       p95 |        p99 |
|-------------|---------------|----------:|--------:|-----------:|----------:|----------:|-----------:|
| macOS       | stream-line   |         1 |   1 KiB |    312,652 |     22 µs |    180 µs |     261 µs |
| Ubuntu      | stream-line   |         1 |   1 KiB |    334,953 |      5 µs |     70 µs |     163 µs |
| macOS       | stream-line   |        32 |   8 KiB |     58,645 |    495 µs | 32.262 ms |  54.921 ms |
| Ubuntu      | stream-line   |        32 |   8 KiB |     48,666 |    631 µs | 32.682 ms |  56.788 ms |
| macOS       | stream-framed |        32 |   8 KiB |     27,814 |  1.863 ms | 52.898 ms |  89.613 ms |
| Ubuntu      | stream-framed |        32 |   8 KiB |     15,584 | 16.281 ms | 80.298 ms | 118.910 ms |
| macOS       | datagram-line |        32 |   8 KiB |     50,653 |    250 µs |  3.187 ms |   5.439 ms |
| Ubuntu      | datagram-line |        32 |   8 KiB |     30,995 |    743 µs |  4.779 ms |   8.784 ms |

Timing distributions differ, but correctness and candidate selection are consistent. Stream-line was competitive or
faster than stream-framed in the tested implementation; length framing has no demonstrated requirement for text.

### Signal-to-exit applicability

INV-007 has no signal-to-exit statistic because the prototype neither supervises nor signals a workload. Reporting one
would mislabel the measurement. The relevant latency is producer timestamp to protocol acceptance; Ubuntu p50/p95/p99
statistics are included above and in `performance.tsv`.

## Slow Reader and Backpressure

| Environment | Protocol      | Input | Delivered | Failed/blocked |     Duration |
|-------------|---------------|------:|----------:|---------------:|-------------:|
| macOS       | stream-line   | 2,000 |     2,000 |              0 |   256.502 ms |
| macOS       | stream-framed | 2,000 |     2,000 |              0 |   254.300 ms |
| macOS       | datagram-line | 2,000 |     1,258 |            742 | 2,023.599 ms |
| Ubuntu      | stream-line   | 2,000 |     2,000 |              0 |   203.927 ms |
| Ubuntu      | stream-framed | 2,000 |     2,000 |              0 |   132.688 ms |
| Ubuntu      | datagram-line | 2,000 |     2,000 |              0 | 1,744.505 ms |

Both stream candidates preserved every message through backpressure. Datagram behavior differed by environment:
37.1% failed or timed out on macOS, while Ubuntu completed before the deadline. This scheduling-dependent result
rejects datagram as a portable reliable primary transport.

## File Descriptors

At `RLIMIT_NOFILE=128`, stream established 63/256 connections on macOS and 60/256 on Ubuntu and rejected the rest in a
bounded way. Exact counts depend on descriptors already owned by the process.

Datagram processed 256/256 messages with no accepted per-producer server FD in both environments. This resource
advantage does not compensate for the non-portable pressure behavior.

## Hypothesis Evaluation

- Unix stream delivery and backpressure: confirmed in both environments.
- Versioned protocol: required and confirmed.
- Length framing required for text: rejected; retain only for binary/raw-newline payloads.
- Datagram as reliable primary transport: rejected.
- Transparent startup/restart: rejected; bounded retry and reconnect are required.
- Exactly-once across ambiguous disconnect: not provided; identity/sequence semantics are required if retries occur.

## Accepted Values and Policies

- Primary transport: Unix stream.
- Protocol: custom versioned newline-delimited text.
- Socket mode: `0660`, with explicit UID/GID ownership.
- Initial normal payload limit: 8 KiB.
- Tested hard ceiling: 65,536 bytes.
- Invalid/partial/oversized message: reject without replacing last-valid state.
- Concurrent connection limit: configurable below `RLIMIT_NOFILE`.
- Startup/reconnect: bounded retry with backoff.
- Producer writes: finite deadline and observable error.
- Datagram: best-effort compatibility adapter only, with drop/error counters.

## Prototype Limits

- Both evidence environments use LinuxKit; native non-LinuxKit Linux remains unverified.
- Producers and server run in one Go process.
- Latency stops at protocol acceptance, not exposition.
- CPU and runtime memory are combined process observations.
- Synthetic parsing does not model production cardinality or registry contention.
- Three repetitions are insufficient for capacity guarantees.
- Multi-user permissions, fuzzing, race detection, Kubernetes and connection churn remain untested.

## Additional Benchmarking

| Item                                   | Status          | Evidence          |
|----------------------------------------|-----------------|-------------------|
| One/8/32 producers                     | Covered in both | `performance.tsv` |
| 64 B/1 KiB/8 KiB                       | Covered in both | `performance.tsv` |
| Three repetitions, p50/p95/p99         | Covered in both | `performance.tsv` |
| Slow reader/backpressure               | Covered in both | `pressure.tsv`    |
| Partial/malformed/max/oversized        | Covered in both | `correctness.tsv` |
| Startup/reconnect/shutdown             | Covered in both | `correctness.tsv` |
| Mode `0660`                            | Covered in both | `correctness.tsv` |
| Stream FD exhaustion/datagram FD model | Covered in both | `pressure.tsv`    |
| Identical fingerprint                  | Confirmed       | `environment.tsv` |
| Native non-LinuxKit Linux/Kubernetes   | Not run         | follow-up         |
| Separate processes/cgroup RSS          | Not run         | follow-up         |
| Parser fuzz/race test                  | Not run         | follow-up         |

## Conclusion

Matching-fingerprint macOS/LinuxKit aarch64 and Ubuntu/LinuxKit x86_64 evidence confirms Unix stream with a custom
versioned line protocol. All assertions passed in both environments. Datagram is rejected as the reliable primary
transport; length framing remains a conditional binary-payload option.

The decision is recorded in [ADR-007](../../docs/06-architecture/adr/ADR-007.md).
