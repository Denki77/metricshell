# INV-007 Report — Socket-Based Ingestion

[Русская версия](report_ru.md)

**Status:** completed  
**Run date:** 2026-07-31  
**Reference runs:** `results/20260731T085625Z`, `results/20260731T125558Z`  
**Fingerprint:** `298879f5849c6bb14e4ff7bbd8849987a4583c6141a791c0b94b19332056f391`

## Goal

Select a bounded local socket protocol for publishing the complete application snapshot defined by ADR-004.

## Scope Correction

MetricShell receives one complete, conflict-free application snapshot per publication. It does not aggregate
independent producer registries, accept instrumentation operations, or own producer sequencing and reconciliation.
A socket publication may span several bounded transport frames. No intermediate frame is metric state.

## Environment

| Environment           | Docker | Kernel           | Architecture | Result set         |
|-----------------------|-------:|------------------|--------------|--------------------|
| macOS Docker Desktop  | 29.6.2 | LinuxKit 6.12.76 | aarch64      | `20260731T085625Z` |
| Ubuntu Docker Desktop | 27.4.0 | LinuxKit 6.10.14 | x86_64       | `20260731T125558Z` |

Both runs have the same fingerprint. Both container environments use LinuxKit; native non-LinuxKit Linux is not
covered.

## Assertions

| Evidence             | macOS | Ubuntu |
|----------------------|------:|-------:|
| Correctness          | 45/45 |  45/45 |
| Performance delivery | 81/81 |  81/81 |
| Pressure/resource    |   6/6 |    6/6 |
| Memory               |   1/1 |    1/1 |
| Snapshot publication |   2/2 |    2/2 |

All assertions passed in both environments.

## Findings

### Bounded framing and memory

The line reader uses bounded `ReadSlice` chunks. Oversized input is drained without buffering the complete line.
Sixteen 131,075-byte lines produced a maximum 65,537-byte parser buffer in both environments. RSS changed from 9,004
to 6,152 KiB (`-2,852 KiB`) on macOS and from 8,620 to 6,136 KiB (`-2,484 KiB`) on Ubuntu. Both workload-specific
non-growth assertions passed.

### Shutdown and reconnect

The server tracks and closes active stream connections. Bounded shutdown, error on the old connection, refusal after
shutdown, and reconnect into a new epoch passed.

### Publication acknowledgement

Transport-frame acceptance and publication acceptance have distinct responses:

- `FRAME_ACCEPTED <frame-id>` means that `snapshot_begin` or `snapshot_part` was accepted into a bounded temporary
  transaction; it does not acknowledge or expose the publication;
- `ACK <publication-id>` is emitted only after `snapshot_commit`, validation of the complete candidate, and atomic
  installation;
- `NACK <publication-id> <reason>` rejects the publication and preserves the last valid snapshot.

The multipart test returned `FRAME_ACCEPTED` for the parts and `ACK snap-1` only at commit. The incomplete transaction
returned `NACK snap-2 invalid` and retained `snap-1`. This aligns the prototype with FR-016, AC-ING-012, ADR-004, and
ADR-007.

A successful socket write is not application acceptance. Disconnect before the final ACK remains ambiguous.

### Pressure and resource limits

| Case                            |       macOS |      Ubuntu |
|---------------------------------|------------:|------------:|
| Stream-line slow reader         | 2,000/2,000 | 2,000/2,000 |
| Stream-framed slow reader       | 2,000/2,000 | 2,000/2,000 |
| Datagram slow reader            | 1,696/2,000 | 2,000/2,000 |
| Application-limit rejections    |          22 |          13 |
| System-FD established/rejected  |      62/194 |      65/191 |
| Datagram FD exhaustion/recovery |     256/256 |     256/256 |

The application limit rejected excess connections and recovered after release. The separate `RLIMIT_NOFILE` case
demonstrated bounded system errors and is not application-limit evidence.

Datagram delivered 1,696/2,000 on macOS and 2,000/2,000 on Ubuntu. The difference is evidence that this test does not
establish a portable acknowledged-delivery contract. Per ADR-005, datagram is not an application-snapshot ingestion
transport; it may only be considered for explicitly best-effort diagnostics.

## Selected Direction

- Unix stream primary ingestion transport.
- One complete application snapshot per accepted publication.
- Versioned newline-delimited bounded framing.
- `FRAME_ACCEPTED` for accepted intermediate transport frames.
- `ACK <publication-id>` only after complete validation and atomic installation; structured `NACK` on rejection.
- Multipart framing when one publication exceeds one frame.
- Initial configurable 8 KiB frame default; the final value remains subject to realistic snapshot benchmarks.
- 65,536 B is only the maximum individual payload exercised.
- Datagram is not an authoritative application-snapshot ingestion transport.
- Configurable application connection limit below `RLIMIT_NOFILE`.
- Finite read, write, idle, transaction, and shutdown deadlines.

## Signal-to-Exit

Not applicable. INV-007 does not supervise or signal a workload. The relevant measured latency is producer timestamp
to protocol acceptance. Final ACK round-trip latency was not measured.

## Limitations and Follow-Ups

- Both reference container environments use LinuxKit; native non-LinuxKit Linux remains unverified.
- Kubernetes and multi-user permission behavior remain unverified.
- The final snapshot wire grammar and structural parser are not implemented.
- Synthetic parsing does not represent real cardinality and exposition validation.
- The memory assertion is workload-specific.
- Production code needs independent fuzz, race, security, and end-to-end tests.
- Realistic complete snapshots should benchmark frame and candidate limits, cardinality, validation cost, concurrent
  transactions, slow clients, and ACK latency.

## Conclusion

Matching-fingerprint macOS/LinuxKit aarch64 and Ubuntu/LinuxKit x86_64 evidence confirms Unix stream with bounded
versioned framing and distinct frame/publication responses. All assertions passed in both environments, including
atomic multipart publication.

The accepted decision is recorded in [ADR-007](../../docs/06-architecture/adr/ADR-007.md).
