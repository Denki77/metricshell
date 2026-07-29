# INV-007 Report — Socket-Based Ingestion

[Русская версия](report_ru.md)

Status: in progress

Run date: 2026-07-29

Reference run: `results/20260729T072602Z`

Fingerprint: `cb1dd3d415e35ee1f58fa3de0de6e42a2583eecb24771c915402a64776eabfa7`

## Goal

Re-evaluate Unix socket ingestion after closing evidence gaps in bounded parsing, shutdown, reconnect, acknowledgement,
authoritative snapshots, memory naming and application connection limits.

## Environment

| Environment                   |                             Docker | Kernel           | Architecture | Result set         |
|-------------------------------|-----------------------------------:|------------------|--------------|--------------------|
| macOS Docker Desktop/LinuxKit |                             29.6.2 | LinuxKit 6.12.76 | aarch64      | `20260729T072602Z` |
| Ubuntu/LinuxKit               | pending matching-fingerprint rerun | —                | x86_64       | —                  |

The current evidence environment uses LinuxKit. Native non-LinuxKit Linux is unverified.

## Results

All current macOS portable assertions passed:

- correctness: 45/45;
- performance delivery: 81/81;
- pressure/resource: 6/6;
- bounded memory: 1/1;
- snapshot transaction: 2/2.

### Bounded parser and memory

`ReadBytes` was replaced with a bounded `ReadSlice` parser. `bufio.ErrBufferFull` marks a message oversized and drains
it in bounded chunks.

Sixteen 131,075-byte lines produced a maximum parser chunk of 65,537 B. RSS changed from 9,028 KiB to 6,144 KiB
(-2,884 KiB), demonstrating no growth in this run and remaining below the 16 MiB test allowance.

An exactly-limit line without newline remained connected until shutdown. Shutdown closed the connection and returned
inside the one-second assertion bound.

`performance.tsv` distinguishes Go runtime reserved memory (`go_runtime_sys_kib`) from actual process RSS
(`rss_kib`, read from `/proc/self/statm`).

### Shutdown and reconnect

The server tracks accepted stream connections and closes them during shutdown. Tests confirm bounded shutdown with an
active client, an error on the old connection, refusal of new connections after shutdown, and successful bounded
reconnect into a new metric-state epoch.

### ACK/NACK contract

Confirmed mode uses a correlation/message ID:

- `ACK <id>` is emitted only after validation and the atomic-application callback;
- `NACK <id> <reason>` reports rejection;
- disconnect before ACK is ambiguous and may require retry;
- retries requiring duplicate suppression use producer identity plus message/snapshot sequence.

The prototype verified `ACK 42` for valid input and `NACK 43 invalid` for invalid input for both stream framings.

### Authoritative snapshots

The selected model transmits large authoritative snapshots as a stream transaction:

1. `snapshot_begin`;
2. one or more independently bounded `snapshot_part` frames;
3. `snapshot_commit`.

Only commit atomically replaces the producer snapshot. A committed 12,000-byte three-part snapshot passed; an incomplete
following snapshot retained the previous committed version.

This aligns socket ingestion with ADR-004 without requiring one snapshot to fit one 8 KiB frame.

### Resource limits

An application limit of eight concurrent stream connections rejected excess clients and accepted a new client after
connections were released. A separate `RLIMIT_NOFILE=128` case proves bounded OS-level errors but is not evidence for
the application limit.

### Datagram

The current slow-reader case delivered 1,875/2,000 messages and reported 125 failures. Earlier results were removed
because their fingerprint is obsolete. The experiment does not isolate kernel, socket-buffer, runtime scheduling or
host-load causes. The supported conclusion is only that datagram has not demonstrated a portable reliable-delivery
contract.

## Decision State

The technical direction remains Unix stream plus a versioned line protocol with acknowledged mode and multipart
authoritative snapshots. ADR-007 is Proposed until a matching-fingerprint Ubuntu run passes.

The initial configurable frame-size default is 8 KiB. It is a conservative operational starting point, not a limit
derived from the benchmark. `65,536 B` is the maximum payload exercised in this reference run, not a built-in hard
ceiling.

## Signal-to-exit

Not applicable. INV-007 neither supervises nor signals a workload. Its latency metric is producer timestamp to protocol
acceptance.

## Remaining Work

- repeat the complete runner on Ubuntu with fingerprint
  `cb1dd3d415e35ee1f58fa3de0de6e42a2583eecb24771c915402a64776eabfa7`;
- compare all assertions and revised TSV schemas;
- replace Proposed/in-progress status only after matching evidence;
- benchmark realistic snapshot format/cardinality before freezing frame and snapshot limits;
- specify the final ACK/NACK and multipart snapshot wire grammar;
- repeat on native non-LinuxKit Linux separately.
