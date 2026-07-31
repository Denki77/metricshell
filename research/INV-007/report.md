# INV-007 Report — Socket-Based Ingestion

[Русская версия](report_ru.md)

**Status:** completed  
**Run date:** 2026-07-29  
**Reference runs:** `results/20260729T072602Z`, `results/20260729T164723Z`  
**Fingerprint:** `cb1dd3d415e35ee1f58fa3de0de6e42a2583eecb24771c915402a64776eabfa7`

## Goal

Select a bounded, acknowledged local socket protocol for the complete application snapshot contract in revised ADR-004.

## Scope Correction

MetricShell receives one complete application snapshot per publication. It does not aggregate independent producer
registries, accept instrumentation operations, or own producer sequencing/reconciliation.

The prototype's multipart case is interpreted only as transport framing of one complete candidate. No part is visible
as metric state. ACK confirms complete validation and atomic installation.

## Environments

| Environment           | Docker | Kernel           | Architecture | Result set         |
|-----------------------|-------:|------------------|--------------|--------------------|
| macOS Docker Desktop  | 29.6.2 | LinuxKit 6.12.76 | aarch64      | `20260729T072602Z` |
| Ubuntu Docker Desktop | 27.4.0 | LinuxKit 6.10.14 | x86_64       | `20260729T164723Z` |

Both runs use the same fingerprint. Both container environments use LinuxKit; native non-LinuxKit Linux is not covered.

## Cross-Environment Confirmation

| Evidence                       |       macOS |      Ubuntu |
|--------------------------------|------------:|------------:|
| Correctness                    |       45/45 |       45/45 |
| Performance delivery           |       81/81 |       81/81 |
| Pressure/resource              |         6/6 |         6/6 |
| Memory                         |         1/1 |         1/1 |
| Snapshot                       |         2/2 |         2/2 |
| Maximum parser buffer          |    65,537 B |    65,537 B |
| RSS delta in memory case       |  -2,884 KiB |  -2,200 KiB |
| Stream-line slow reader        | 2,000/2,000 | 2,000/2,000 |
| Stream-framed slow reader      | 2,000/2,000 | 2,000/2,000 |
| Datagram slow reader           | 1,875/2,000 | 2,000/2,000 |
| App-limit excess rejections    |          23 |           5 |
| System-FD established/rejected |      71/185 |      61/195 |

All portable assertions passed in both environments. Exact timing and resource counts are observations, not portable
equality requirements.

## Findings

### Bounded framing and memory

The line reader uses bounded `ReadSlice` chunks. Oversized input is drained without buffering the complete line.
Sixteen 131,075-byte lines stayed at a maximum 65,537-byte parser chunk in both environments and showed no RSS growth
in the measured case.

`go_runtime_sys_kib` and actual `/proc/self/statm` `rss_kib` are separate columns.

### Shutdown and restart

The server tracks and closes active stream connections. Bounded shutdown, error on the old connection, refusal after
shutdown and reconnect into a new epoch passed in both environments.

### Application acknowledgement

`ACK <id>` distinguishes application acceptance from a successful socket write. The prototype verified ACK for valid
input and NACK for invalid input for both stream framings.

Under revised ADR-004, success means the one complete candidate has been validated and atomically installed. A
disconnect before ACK is ambiguous; repeating the same complete snapshot may cause another linear acceptance, but does
not require MetricShell to own per-producer operation sequences.

### Complete candidate framing

A complete 12,000-byte application candidate was assembled from three bounded frames and committed. An incomplete
following transaction retained the previous committed snapshot. Production grammar must also bound total candidate
bytes, part count, transaction lifetime and concurrent candidates.

### Datagram

Datagram outcomes differed: the macOS pressure run reported 125 failed/timed-out messages, while Ubuntu delivered all.
The cause was not isolated. The justified conclusion is only that datagram did not demonstrate a portable reliable
contract; it is rejected as the primary transport.

### Connection limits

The application limit rejected excess connections and recovered after release in both environments. A separate
`RLIMIT_NOFILE` test demonstrated bounded system errors and is not treated as application-limit evidence.

## Accepted Direction

- Unix stream primary transport.
- Versioned newline-delimited framing.
- Confirmed publication mode with correlation ID and ACK/NACK.
- ACK only after complete-candidate validation and atomic installation.
- Multipart transport framing when one candidate exceeds one frame.
- Initial configurable 8 KiB frame default; final value deferred to realistic complete-snapshot benchmarks.
- 65,536 B described only as maximum individual payload exercised.
- Datagram rejected as reliable primary.
- Configurable application connection limit below `RLIMIT_NOFILE`.
- Bounded read/write/idle/transaction/shutdown deadlines.

## Signal-to-Exit

Not applicable. The relevant measurement is producer timestamp to protocol acceptance.

## Limits and Follow-Ups

- Both reference environments use LinuxKit.
- Native Linux, Kubernetes and multi-user permission behavior remain unverified.
- The final snapshot wire grammar and structural parser are not implemented.
- Synthetic parsing does not represent real cardinality and exposition validation.
- The memory assertion is workload-specific.
- Production code needs independent fuzz, race, security and end-to-end tests.

## Conclusion

Matching-fingerprint LinuxKit aarch64/x86_64 evidence confirms Unix stream with bounded versioned line framing and
application ACK/NACK for complete application snapshots. All assertions passed in both environments.

The decision is recorded in [ADR-007](../../docs/06-architecture/adr/ADR-007.md).
