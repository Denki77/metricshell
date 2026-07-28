# INV-007 — Socket-Based Ingestion

[Русская версия](README_ru.md)

**Status:** completed  
**Reference runs:** `results/20260723T190106Z`, `results/20260728T114224Z`  
**Report:** [report.md](report.md)  
**Decision:** [ADR-007](../../docs/06-architecture/adr/ADR-007.md)

## Question

Which local socket model and protocol best fit MetricShell?

## Candidates

- Unix stream with newline-delimited versioned text.
- Unix stream with a four-byte big-endian length prefix.
- Unix datagram with newline-delimited versioned text and StatsD-like transport semantics.
- Unversioned StatsD was rejected as the primary protocol because its operations and error model do not implement the
  required versioned registry/update contract.

## Hypothesis

Unix stream with an explicit versioned protocol provides reliable local delivery and backpressure. Datagram reduces
connection and server file-descriptor cost, but cannot provide a portable reliable-delivery contract under pressure.
Length framing is justified only if the payload must contain arbitrary binary data or raw newlines.

## Experiments

`run-bench.sh` builds and runs one Linux container without host bind mounts. It performs, in each environment:

- 32 correctness assertions;
- 81 performance rows: three protocols × 1/8/32 producers × 64/1,024/8,192-byte payloads × three repetitions;
- five pressure and resource assertions;
- malformed, partial, exact-maximum and oversized message cases;
- startup retry, restart/reconnect, shutdown and socket-permission cases;
- slow-reader and file-descriptor-pressure cases.

## Environments and fingerprint

| Environment           | Docker | Kernel           | Architecture | Result set                 | Fingerprint                                                        |
|-----------------------|-------:|------------------|--------------|----------------------------|--------------------------------------------------------------------|
| macOS Docker Desktop  | 29.4.3 | LinuxKit 6.12.76 | aarch64      | `results/20260723T190106Z` | `585b91f1a73f1359953cc313af2e1f3f7ff1f9757ee00086056c812357a78bca` |
| Ubuntu Docker Desktop | 27.4.0 | LinuxKit 6.10.14 | x86_64       | `results/20260728T114224Z` | `585b91f1a73f1359953cc313af2e1f3f7ff1f9757ee00086056c812357a78bca` |

The fingerprints are identical. Repository HEAD, image ID and architecture differ as expected; the content fingerprint
proves that the prototype and runner were identical.

Both container environments use LinuxKit. The evidence confirms cross-architecture behavior inside LinuxKit
aarch64/x86_64, but does not verify native non-LinuxKit Linux.

## Results

All portable assertions passed in both environments:

| Assertion group           | macOS | Ubuntu |
|---------------------------|------:|-------:|
| Correctness               | 32/32 |  32/32 |
| Performance delivery rows | 81/81 |  81/81 |
| Pressure/resource         |   5/5 |    5/5 |

The tested maximum payload was 65,536 bytes.

### Slow reader and backpressure

| Environment | Protocol      | Input | Delivered | Failed/blocked |     Duration |
|-------------|---------------|------:|----------:|---------------:|-------------:|
| macOS       | stream-line   | 2,000 |     2,000 |              0 |   256.502 ms |
| macOS       | stream-framed | 2,000 |     2,000 |              0 |   254.300 ms |
| macOS       | datagram-line | 2,000 |     1,258 |            742 | 2,023.599 ms |
| Ubuntu      | stream-line   | 2,000 |     2,000 |              0 |   203.927 ms |
| Ubuntu      | stream-framed | 2,000 |     2,000 |              0 |   132.688 ms |
| Ubuntu      | datagram-line | 2,000 |     2,000 |              0 | 1,744.505 ms |

Both stream candidates applied backpressure and delivered every message in both environments. Datagram delivered 62.9%
on macOS and 100% on Ubuntu. The assertion requires bounded accounting, so both cases pass. The different outcomes
prove that datagram delivery under pressure depends on scheduling headroom and is not a portable reliable-delivery
contract.

### Ubuntu latency statistics

INV-007 has no signal-to-exit statistic because the prototype neither supervises nor signals a workload. The relevant
latency is producer timestamp to protocol acceptance.

Ubuntu mean-of-three:

| Protocol      | Producers | Payload |       p50 |       p95 |        p99 |
|---------------|----------:|--------:|----------:|----------:|-----------:|
| stream-line   |         1 |   1 KiB |      5 µs |     70 µs |     163 µs |
| stream-line   |        32 |   8 KiB |    631 µs | 32.682 ms |  56.788 ms |
| stream-framed |         1 |   1 KiB |    280 µs |    692 µs |     944 µs |
| stream-framed |        32 |   8 KiB | 16.281 ms | 80.298 ms | 118.910 ms |
| datagram-line |         1 |   1 KiB |     10 µs |     59 µs |     129 µs |
| datagram-line |        32 |   8 KiB |    743 µs |  4.779 ms |   8.784 ms |

### File descriptors

At `RLIMIT_NOFILE=128`, the stream test established 63/256 connections on macOS and 60/256 on Ubuntu, rejecting the
rest in a bounded way. The datagram server processed 256/256 messages with no accepted per-producer server FD in both
environments.

## Conclusion

The hypothesis is confirmed:

- select Unix stream as the primary reliable socket transport;
- select a custom versioned newline-delimited text protocol;
- retain length framing only if binary or newline-containing payloads become a requirement;
- reject Unix datagram as the primary reliable transport;
- require bounded startup retry, reconnect, write deadlines and explicit failure reporting;
- do not promise exactly-once delivery across ambiguous disconnects.

## Accepted Values

- socket type: Unix stream;
- socket mode: `0660`, with explicit container user/group ownership;
- initial normal payload limit: 8 KiB;
- tested hard ceiling: 65,536 bytes;
- malformed, partial or oversized input: reject without replacing last-valid metric state;
- connection limit: configurable and below `RLIMIT_NOFILE`, with descriptors reserved for runtime duties;
- retry/reconnect: bounded with backoff;
- producer write deadline: finite;
- duplicate handling after ambiguous retry: explicit producer identity and sequence number where required;
- datagram: best-effort compatibility adapter only, with drop/error counters.

## Running the Prototype

The command is identical on macOS and Ubuntu:

```bash
./research/INV-007/run-bench.sh
```

```bash
cat research/INV-007/latest-results.txt
cat "$(cat research/INV-007/latest-results.txt)/summary.tsv"
cat "$(cat research/INV-007/latest-results.txt)/environment.tsv"
cat "$(cat research/INV-007/latest-results.txt)/correctness.tsv"
cat "$(cat research/INV-007/latest-results.txt)/performance.tsv"
cat "$(cat research/INV-007/latest-results.txt)/pressure.tsv"
```

Higher-confidence run:

```bash
INV007_REPETITIONS=30 INV007_MAX_PAYLOAD=65536 ./research/INV-007/run-bench.sh
```

Compare `benchmark_code_fingerprint_sha256`, not repository HEAD or image ID.

## Prototype Limits

- Research harness, not production MetricShell or a production parser.
- Both evidence environments use LinuxKit; native non-LinuxKit Linux remains unverified.
- Producers and server share one Go process, so CPU/RSS are combined.
- Producer-to-accept latency is not end-to-end metric exposition latency.
- Three repetitions establish feasibility and ranking, not stable capacity.
- Synthetic parsing does not model production cardinality or registry contention.
- Multi-user UID/GID access, parser fuzzing, race detection, Kubernetes and connection-churn sweeps remain follow-ups.

## Additional Benchmarks

Covered: one/8/32 producers, 64 B/1 KiB/8 KiB payloads, three repetitions, p50/p95/p99, throughput, CPU/runtime memory,
slow reader, partial messages, malformed/max/oversized input, startup, reconnect, shutdown, mode `0660`, stream FD
exhaustion, datagram FD model, environment metadata and identical cross-environment fingerprint.

Recommended follow-ups: 30–100 repetitions on dedicated hosts, separate producer/server processes, cgroup CPU/RSS,
realistic payload/cardinality parsing, connection churn and backlog sweeps, stalled frames, multi-user permission tests,
parser fuzzing, native Linux and Kubernetes.
