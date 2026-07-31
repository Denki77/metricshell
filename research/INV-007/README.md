# INV-007 — Socket-Based Ingestion

[Русская версия](README_ru.md)

**Status:** completed  
**Reference runs:** `results/20260729T072602Z`, `results/20260729T164723Z`  
**Report:** [report.md](report.md)  
**Decision:** [ADR-007](../../docs/06-architecture/adr/ADR-007.md)

## Question

Which local socket type, framing and acknowledgement model should carry the complete application snapshots defined by
ADR-004?

## Scope Alignment

ADR-004 was narrowed while INV-007 was running. MetricShell now accepts one complete, conflict-free application
snapshot per publication. It does not accept instrumentation operations, aggregate per-producer registries, or own
producer identity, sequencing and reconciliation.

Socket multipart framing is therefore transport-level assembly of one complete candidate snapshot. Parts are never
installed or exposed independently. A valid commit triggers complete-candidate validation and atomic replacement.

## Selected Direction

- Unix stream is the primary reliable socket transport.
- Native framing is versioned newline-delimited text.
- Confirmed mode correlates a publication ID with `ACK <id>` or `NACK <id> <reason>`.
- ACK is emitted only after the complete candidate has been structurally validated and atomically installed.
- A candidate larger than one frame uses `snapshot_begin`, bounded `snapshot_part` frames and `snapshot_commit`.
- Initial configurable frame-size default is 8 KiB. It is a conservative starting value, not a benchmark-derived
  architecture limit.
- 65,536 B is the maximum individual payload exercised in the reference runs, not a built-in hard ceiling.
- Datagram is rejected as the reliable primary transport.

## Environments and Fingerprint

| Environment           | Docker | Kernel           | Architecture | Result set                 | Fingerprint                                                        |
|-----------------------|-------:|------------------|--------------|----------------------------|--------------------------------------------------------------------|
| macOS Docker Desktop  | 29.6.2 | LinuxKit 6.12.76 | aarch64      | `results/20260729T072602Z` | `cb1dd3d415e35ee1f58fa3de0de6e42a2583eecb24771c915402a64776eabfa7` |
| Ubuntu Docker Desktop | 27.4.0 | LinuxKit 6.10.14 | x86_64       | `results/20260729T164723Z` | `cb1dd3d415e35ee1f58fa3de0de6e42a2583eecb24771c915402a64776eabfa7` |

Fingerprints are identical. Repository HEAD, image ID and architecture differ as expected; the content fingerprint
proves prototype and runner identity.

Both container environments use LinuxKit. The results confirm cross-architecture behavior inside LinuxKit
aarch64/x86_64, not native non-LinuxKit Linux.

## Assertions

All portable assertions passed in both environments:

| Group                | macOS | Ubuntu |
|----------------------|------:|-------:|
| Correctness          | 45/45 |  45/45 |
| Performance delivery | 81/81 |  81/81 |
| Pressure/resource    |   6/6 |    6/6 |
| Bounded memory       |   1/1 |    1/1 |
| Snapshot transaction |   2/2 |    2/2 |

Correctness includes bounded oversized parsing, unterminated active client shutdown, old-connection failure, reconnect,
ACK/NACK, exact maximum payload, malformed input, socket mode and startup retry.

## Key Evidence

### Bounded line parsing

| Environment | Message bytes | Messages | Max parser buffer | RSS before | RSS after |  RSS delta | Allowed |
|-------------|--------------:|---------:|------------------:|-----------:|----------:|-----------:|--------:|
| macOS       |       131,075 |       16 |          65,537 B |  9,028 KiB | 6,144 KiB | -2,884 KiB |  16 MiB |
| Ubuntu      |       131,075 |       16 |          65,537 B |  8,632 KiB | 6,432 KiB | -2,200 KiB |  16 MiB |

The parser uses bounded `ReadSlice` chunks and drains oversized lines without accumulating the complete message.
`performance.tsv` reports Go runtime reserved memory as `go_runtime_sys_kib` and actual Linux process resident pages as
`rss_kib`.

### Shutdown, reconnect and acknowledgement

In both environments:

- shutdown closed active idle/unterminated stream connections inside the bounded assertion window;
- the old connection observed an error;
- bounded reconnect delivered to a new server epoch;
- valid `id=42` received `ACK 42`;
- invalid `id=43` received `NACK 43 invalid`.

For the production complete-snapshot contract, the ACK point is after atomic installation, as required by ADR-004.

### Complete candidate assembly

A synthetic 12,000-byte candidate was assembled from three bounded parts and committed atomically. A later incomplete
candidate did not replace committed version `snap-1` in either environment.

The snapshot ID is transport correlation for one application publication; it is not producer ownership or a
MetricShell aggregation key.

### Pressure and resource behavior

| Environment | Stream line slow reader | Stream framed | Datagram    | App-limit rejected | FD established/rejected |
|-------------|-------------------------|---------------|-------------|-------------------:|------------------------:|
| macOS       | 2,000/2,000             | 2,000/2,000   | 1,875/2,000 |                 23 |                71 / 185 |
| Ubuntu      | 2,000/2,000             | 2,000/2,000   | 2,000/2,000 |                  5 |                61 / 195 |

The application connection-limit test also confirmed recovery after connections were released. The separate
`RLIMIT_NOFILE` case proves bounded OS errors only.

The experiment does not isolate why datagram results differ. It establishes that datagram did not demonstrate a
portable reliable-delivery contract under the bounded-pressure scenario.

### Representative producer-to-accept latency

| Environment | Protocol    | Producers | Payload | Messages/s |    p50 |       p95 |       p99 |
|-------------|-------------|----------:|--------:|-----------:|-------:|----------:|----------:|
| macOS       | stream-line |         1 |   1 KiB |    347,520 |  11 µs |    173 µs |    354 µs |
| Ubuntu      | stream-line |         1 |   1 KiB |    257,842 |   7 µs |     64 µs |    403 µs |
| macOS       | stream-line |        32 |   8 KiB |    118,566 | 351 µs | 13.799 ms | 22.930 ms |
| Ubuntu      | stream-line |        32 |   8 KiB |     64,249 | 268 µs | 26.546 ms | 44.630 ms |

INV-007 has no signal-to-exit metric because it neither supervises nor signals a workload.

## Running

```bash
./research/INV-007/run-bench.sh
```

```bash
cat "$(cat research/INV-007/latest-results.txt)/summary.tsv"
cat "$(cat research/INV-007/latest-results.txt)/correctness.tsv"
cat "$(cat research/INV-007/latest-results.txt)/memory.tsv"
cat "$(cat research/INV-007/latest-results.txt)/performance.tsv"
cat "$(cat research/INV-007/latest-results.txt)/pressure.tsv"
cat "$(cat research/INV-007/latest-results.txt)/snapshot.tsv"
cat "$(cat research/INV-007/latest-results.txt)/environment.tsv"
```

## Limits

- Both reference environments use LinuxKit; native Linux remains unverified.
- ACK/NACK and multipart grammar are research forms, not the final wire specification.
- The 8 KiB frame default still requires realistic complete-snapshot cardinality benchmarks before production freeze.
- Memory evidence is scoped to the defined workload, not a universal process-memory guarantee.
- Clients and server share one Go process in the prototype.
- Production implementation requires independent unit, integration, fuzz, race and security coverage.
