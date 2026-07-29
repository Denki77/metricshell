# INV-007 — Socket-Based Ingestion

[Русская версия](README_ru.md)

Status: in progress

Current reference run: `results/20260729T072602Z`

Report: [report.md](report.md)

Proposed decision: [ADR-007](../../docs/06-architecture/adr/ADR-007.md)

## Question and Candidates

Select the local socket model and protocol for MetricShell. The tested candidates are versioned newline-framed Unix
stream, length-framed Unix stream and versioned Unix datagram.

## Revised Protocol Direction

- Primary transport: Unix stream.
- Native protocol: versioned newline-framed text.
- Confirmed acceptance mode: request message ID plus `ACK <id>` after syntactic/semantic validation and atomic
  application, or `NACK <id> <reason>`.
- A disconnect before ACK is ambiguous; a retry may duplicate delivery and therefore requires producer identity,
  snapshot ID and sequence/message ID.
- Authoritative producer snapshots use `snapshot_begin`, bounded `snapshot_part` messages and `snapshot_commit`.
  MetricShell applies the new snapshot atomically only after a valid commit. Operations remain an optional acceleration
  path, consistent with ADR-004.
- Initial configurable default per-frame limit: 8 KiB. This is a conservative starting value, not an architecture
  limit derived from the benchmark.
- Maximum payload exercised in the current reference run: 65,536 B.

## Current Environment

| Environment          |        Docker | Kernel           | Architecture | Result set                 | Fingerprint                                                        |
|----------------------|--------------:|------------------|--------------|----------------------------|--------------------------------------------------------------------|
| macOS Docker Desktop |        29.6.2 | LinuxKit 6.12.76 | aarch64      | `results/20260729T072602Z` | `cb1dd3d415e35ee1f58fa3de0de6e42a2583eecb24771c915402a64776eabfa7` |
| Ubuntu               | pending rerun | —                | x86_64       | —                          | must match                                                         |

The current container environment uses LinuxKit. Native non-LinuxKit Linux remains unverified.

## Current Results

| Assertion group      | Result |
|----------------------|-------:|
| Correctness          |  45/45 |
| Performance delivery |  81/81 |
| Pressure/resource    |    6/6 |
| Bounded-memory       |    1/1 |
| Snapshot transaction |    2/2 |

### Bounded line parsing

The line reader uses bounded `ReadSlice` chunks and drains oversized lines without accumulating the complete message.

| Message bytes | Messages | Maximum parser buffer | RSS before | RSS after |  RSS delta | Allowed delta |
|--------------:|---------:|----------------------:|-----------:|----------:|-----------:|--------------:|
|       131,075 |       16 |              65,537 B |  9,028 KiB | 6,144 KiB | -2,884 KiB |    16,384 KiB |

The correctness suite also holds an exactly-limit unterminated line open while shutdown begins. Server shutdown closes
the active connection and completes within one second.

### Shutdown and restart

- active idle/partial stream clients are closed by server shutdown;
- shutdown completes within the bounded assertion window;
- the old connection observes an error;
- a bounded reconnect delivers only to the new server epoch;
- new connections are refused after shutdown.

### ACK/NACK

Both stream framings passed:

- valid `id=42` → `ACK 42`;
- invalid `id=43` → `NACK 43 invalid`.

ACK is allowed only after the callback representing validation and atomic application returns successfully. A
disconnect before ACK has an unknown result and must follow the protocol retry/deduplication policy.

### Authoritative snapshots

A synthetic 12,000-byte snapshot was split into three bounded parts and committed as `snap-1`. A later incomplete
snapshot retained committed version `snap-1`. The final grammar and cardinality limits remain future protocol work.

### Pressure and resource limits

| Case                                  | Input |       Delivered | Failed/rejected | Result |
|---------------------------------------|------:|----------------:|----------------:|--------|
| stream-line slow reader               | 2,000 |           2,000 |               0 | pass   |
| stream-framed slow reader             | 2,000 |           2,000 |               0 | pass   |
| datagram slow reader                  | 2,000 |           1,875 |             125 | pass   |
| application connection limit/recovery |    32 | 1 after release |              22 | pass   |
| system FD exhaustion                  |   256 |              71 |             185 | pass   |
| datagram no accepted per-producer FD  |   256 |             256 |               0 | pass   |

The experiment does not isolate why datagram outcomes differ between runs or hosts. It establishes only that datagram
did not demonstrate a portable reliable-delivery contract in the bounded-pressure scenario.

### Memory columns

`performance.tsv` now reports both:

- `go_runtime_sys_kib`: Go runtime `MemStats.Sys`;
- `rss_kib`: actual process resident pages from Linux `/proc/self/statm`.

These remain combined producer/server process observations, not isolated server peak RSS.

## Running

The same command must be used on macOS and Ubuntu:

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

Ubuntu evidence is comparable only if `benchmark_code_fingerprint_sha256` equals
`cb1dd3d415e35ee1f58fa3de0de6e42a2583eecb24771c915402a64776eabfa7`.

## Limits and Remaining Work

- Ubuntu matching-fingerprint rerun is required before completion and ADR acceptance.
- Both intended reference environments use LinuxKit; native Linux remains a separate gap.
- ACK/NACK and snapshot grammar are synthetic research forms, not a final wire specification.
- The 8 KiB configurable default requires realistic parser/cardinality benchmarks before production freeze.
- The memory test proves a bounded parser buffer and bounded RSS change for its workload, not a universal memory bound.
- Connection limit recovery is covered; backlog, churn and multi-user security require further testing.
- Race-enabled compilation passes; production code still requires dedicated unit and integration tests under the race
  detector.
