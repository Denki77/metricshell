# INV-007 — Socket-Based Ingestion

[Русская версия](README_ru.md)

**Status:** completed  
Reference runs: `results/20260731T085625Z`, `results/20260731T125558Z`  
**Report:** [report.md](report.md)  
**Decision:** [ADR-007](../../docs/06-architecture/adr/ADR-007.md)

## Question

Which local socket type, framing and acknowledgement model should carry the complete application snapshots defined by
ADR-004?

## Scope Alignment

MetricShell accepts one complete, conflict-free application snapshot per publication. It does not accept
instrumentation operations, aggregate per-producer registries, or own producer identity, sequencing and reconciliation.

Multipart framing assembles one complete candidate at the transport layer. Parts are never installed or exposed
independently.

## Revised ACK Contract

- `FRAME_ACCEPTED <frame-id>` confirms only that `snapshot_begin` or `snapshot_part` was accepted into bounded temporary
  transaction state.
- `FRAME_ACCEPTED` does not confirm publication acceptance and does not permit the client to discard authoritative
  publication state.
- `ACK <publication-id>` is emitted only after `snapshot_commit`, complete structural validation and atomic
  installation.
- `NACK <publication-id> <reason>` rejects the publication and preserves the last valid snapshot.
- A successful socket `Write` is not application acceptance.

The current prototype recorded `FRAME_ACCEPTED part-0`, final `ACK snap-1`, and `NACK snap-2 invalid` for an incomplete
publication.

## Selected Direction

- Primary transport: Unix stream.
- Native framing: bounded versioned newline-delimited text.
- Candidate larger than one frame: `snapshot_begin`, bounded indexed `snapshot_part`, `snapshot_commit`.
- Initial configurable frame default: 8 KiB; this is not a benchmark-derived architecture limit.
- Maximum individual payload exercised: 65,536 B; this is not a built-in hard ceiling.
- Application connection limit below `RLIMIT_NOFILE`.
- Finite read, write, idle, transaction and shutdown deadlines.
- Unix datagram is not an application-snapshot ingestion transport.

## Current Environment

| Environment           | Docker | Kernel           | Architecture | Result set                 | Fingerprint                                                        |
|-----------------------|-------:|------------------|--------------|----------------------------|--------------------------------------------------------------------|
| macOS Docker Desktop  | 29.6.2 | LinuxKit 6.12.76 | aarch64      | `results/20260731T085625Z` | `298879f5849c6bb14e4ff7bbd8849987a4583c6141a791c0b94b19332056f391` |
| Ubuntu Docker Desktop | 27.4.0 | LinuxKit 6.10.14 | x86_64       | `results/20260731T125558Z` | `298879f5849c6bb14e4ff7bbd8849987a4583c6141a791c0b94b19332056f391` |

The fingerprints are identical and all assertions passed in both environments. Both container environments use
LinuxKit; native non-LinuxKit Linux remains unverified.

## Current Assertions

| Group                | macOS | Ubuntu |
|----------------------|------:|-------:|
| Correctness          | 45/45 |  45/45 |
| Performance delivery | 81/81 |  81/81 |
| Pressure/resource    |   6/6 |    6/6 |
| Bounded memory       |   1/1 |    1/1 |
| Snapshot publication |   2/2 |    2/2 |

### Bounded parser

Sixteen 131,075-byte lines used at most 65,537 B of parser buffer in both environments. RSS changed from 9,004 to
6,152 KiB on macOS and from 8,620 to 6,136 KiB on Ubuntu, remaining inside the 16 MiB test allowance.

The parser uses bounded `ReadSlice` chunks and drains oversized lines without accumulating the complete input.
`performance.tsv` separates `go_runtime_sys_kib` from actual Linux `/proc/self/statm` `rss_kib`.

### Shutdown and reconnect

Active idle/unterminated stream connections are tracked and closed during bounded shutdown. The old connection receives
an error, new connections are refused after shutdown, and bounded reconnect delivers into a new server epoch.

### Snapshot publication

A 12,000-byte candidate was assembled from three bounded parts. Intermediate frames received `FRAME_ACCEPTED`; only
commit received `ACK snap-1`. A later incomplete candidate received `NACK snap-2 invalid` and retained committed
snapshot `snap-1`.

Publication ID is transport correlation, not producer ownership or an aggregation key.

### Pressure and resource behavior

| Case                                |       macOS |      Ubuntu |
|-------------------------------------|------------:|------------:|
| stream-line slow reader             | 2,000/2,000 | 2,000/2,000 |
| stream-framed slow reader           | 2,000/2,000 | 2,000/2,000 |
| datagram slow reader                | 1,696/2,000 | 2,000/2,000 |
| application-limit excess rejections |          22 |          13 |
| system FD established/rejected      |      62/194 |      65/191 |
| datagram FD model                   |     256/256 |     256/256 |

The datagram experiment remains comparative evidence only. It does not establish one cause for loss and does not make
datagram an application ingestion transport.

## Running

Run the same command on macOS and Ubuntu:

```bash
./research/INV-007/run-bench.sh
```

Inspect:

```bash
cat "$(cat research/INV-007/latest-results.txt)/summary.tsv"
cat "$(cat research/INV-007/latest-results.txt)/correctness.tsv"
cat "$(cat research/INV-007/latest-results.txt)/memory.tsv"
cat "$(cat research/INV-007/latest-results.txt)/performance.tsv"
cat "$(cat research/INV-007/latest-results.txt)/pressure.tsv"
cat "$(cat research/INV-007/latest-results.txt)/snapshot.tsv"
cat "$(cat research/INV-007/latest-results.txt)/environment.tsv"
```

Both reference runs have fingerprint
`298879f5849c6bb14e4ff7bbd8849987a4583c6141a791c0b94b19332056f391`.

## Limits and Remaining Work

- Both reference environments use LinuxKit; native Linux remains a separate gap.
- `FRAME_ACCEPTED`, ACK/NACK and multipart grammar are research forms, not the final wire specification.
- The 8 KiB configurable default requires realistic complete-snapshot cardinality benchmarks.
- Memory evidence is scoped to the defined workload.
- Production implementation requires independent unit, integration, fuzz, race, security and end-to-end coverage.
