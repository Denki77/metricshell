# INV-007 Report — Socket-Based Ingestion

Status: in progress  
Run date: 2026-07-23  
Docker server: 29.4.3  
Docker platform: LinuxKit 6.12.76 linux/aarch64  
Reference run: `results/20260723T190106Z`  
Summary: `results/20260723T190106Z/summary.tsv`

## Goal

Determine whether a local Unix socket is a viable MetricShell ingestion transport and select a provisional socket
model, framing, resource boundaries and failure policies.

## Prototype

The prototype is located in `research/INV-007`.

- `prototype/cmd/inv007-bench` — Linux Unix-socket server, producers, assertions and measurements.
- `prototype/Dockerfile` — multi-stage Linux image.
- `run-bench.sh` — identical macOS/Ubuntu runner, evidence extraction and fingerprint capture.
- `results/<timestamp>` — raw correctness, performance, pressure and environment TSV evidence.

The server compares newline-delimited Unix stream, four-byte length-framed Unix stream and newline-delimited Unix
datagram protocols. All application messages begin with a version token (`v1`).

## Run Commands

```bash
./research/INV-007/run-bench.sh
cat "$(cat research/INV-007/latest-results.txt)/summary.tsv"
cat "$(cat research/INV-007/latest-results.txt)/environment.tsv"
```

Ubuntu uses the same command. Cross-environment evidence is comparable only when
`benchmark_code_fingerprint_sha256` is equal.

## Run Environments

| Environment          | Date       | Docker | Kernel           | Architecture | Result set                 | Fingerprint                                                        |
|----------------------|------------|--------|------------------|--------------|----------------------------|--------------------------------------------------------------------|
| macOS Docker Desktop | 2026-07-23 | 29.4.3 | LinuxKit 6.12.76 | aarch64      | `results/20260723T190106Z` | `585b91f1a73f1359953cc313af2e1f3f7ff1f9757ee00086056c812357a78bca` |
| Ubuntu               | pending    | —      | —                | —            | —                          | must match                                                         |

The reference run recorded `benchmark_scope_diff_clean=true` and `benchmark_scope_untracked_count=4`; the latter is
expected because the new prototype and runner were not tracked at run time. The fingerprint directly hashes their
contents and is the handoff identity.

## Results

### Correctness

All 32 correctness assertions passed. Every protocol covered socket permission, single producer, valid message,
malformed input, exact 65,536-byte payload, oversized rejection, shutdown refusal, bounded startup retry and reconnect
after restart. Stream candidates additionally rejected disconnects in the middle of a line or frame.

Malformed, partial and oversized input did not become a valid update. A stopped server refused new producers. A client
starting before the server connected within the bounded retry window, and a new connection delivered after server
restart.

### Performance matrix

All 81 rows delivered every submitted message without a write deadline: three protocols × three producer counts ×
three payload sizes × three repetitions.

Mean-of-three selected cells:

| Protocol      | Producers | Payload | Messages/s |      p50 |       p95 |       p99 |
|---------------|----------:|--------:|-----------:|---------:|----------:|----------:|
| stream-line   |         1 |    64 B |    565,585 |    47 µs |    279 µs |    347 µs |
| stream-framed |         1 |    64 B |    388,776 |   202 µs |    304 µs |    336 µs |
| datagram-line |         1 |    64 B |    303,019 |    11 µs |     22 µs |     53 µs |
| stream-line   |         8 |   1 KiB |    294,751 |   145 µs |  5.569 ms | 12.032 ms |
| stream-framed |         8 |   1 KiB |    280,834 |   301 µs |  4.480 ms |  7.004 ms |
| datagram-line |         8 |   1 KiB |    286,046 |    25 µs |    191 µs |    619 µs |
| stream-line   |        32 |   8 KiB |     58,645 |   495 µs | 32.262 ms | 54.921 ms |
| stream-framed |        32 |   8 KiB |     27,814 | 1.863 ms | 52.898 ms | 89.613 ms |
| datagram-line |        32 |   8 KiB |     50,653 |   250 µs |  3.187 ms |  5.439 ms |

Datagram accept latency was low while the reader kept up. Stream-line had the highest throughput in most comparable
cells. Stream-framed did not establish a consistent advantage in this implementation.

### Slow reader and backpressure

| Protocol      | Input | Delivered | Failed/blocked |     Duration | Result |
|---------------|------:|----------:|---------------:|-------------:|--------|
| stream-line   | 2,000 |     2,000 |              0 |   256.502 ms | pass   |
| stream-framed | 2,000 |     2,000 |              0 |   254.300 ms | pass   |
| datagram-line | 2,000 |     1,258 |            742 | 2,023.599 ms | pass   |

The server slept 200 µs per accepted message and each producer had a two-second write deadline. Both stream candidates
backpressured writers and delivered all messages. Datagram delivered 62.9% and rejected or timed out 37.1%. The case
passes because bounded failure is the expected datagram behavior; it is evidence against selecting datagram as a
reliable primary channel.

### File descriptors

With the process soft `RLIMIT_NOFILE` reduced to 128, a request for 256 simultaneous stream connections established 63
and rejected 193 in 4.008 ms. The process failed in a bounded way. The exact established count is environment-dependent
because the process already owns descriptors.

The datagram server accepted 256 sequential producer messages with zero per-producer accepted server FDs. This is a
real resource advantage, but not enough to offset loss under reader pressure.

### Protocol boundaries

- Stream requires explicit framing; EOF before newline or full frame is malformed.
- Length framing supports arbitrary bytes but adds parser/state complexity and showed no measured requirement here.
- Line framing is easy to inspect and was competitive or faster, but payload values must escape or forbid newline.
- Datagram preserves one-send/one-receive boundaries, but reliable overload behavior requires acknowledgements,
  retransmission and deduplication, which would recreate a stream-like protocol.
- Protocol version is mandatory independently of transport.

## Hypothesis Evaluation

### Unix stream provides suitable delivery and backpressure

Supported in the tested macOS/LinuxKit environment. Both stream candidates delivered every normal and slow-reader
message, rejected partial messages and exposed connection exhaustion as bounded producer failure.

### Length framing is required

Not supported by current evidence. It is only required if the selected payload is binary or permits raw newline.
Versioned line framing is the provisional simpler choice.

### Unix datagram is a viable primary transport

Rejected for reliable ingestion. It avoids accepted connection FDs and has low latency while uncongested, but lost or
rejected 742 of 2,000 messages under the defined slow-reader deadline.

### Startup and restart are transparent

Rejected. Producers need bounded startup retry and reconnect after a MetricShell epoch change. Delivery across a
broken connection requires application-level retry/idempotency if at-least-once semantics are required.

## Acceptable Values and Policies

Provisional pending Ubuntu:

- Unix stream is the primary socket type.
- Use versioned newline-delimited text unless the payload specification requires binary/newline-containing values.
- Socket permission is `0660`; configure owner/group explicitly.
- 8 KiB is a provisional operational payload default; 65,536 bytes is the tested hard ceiling, not a recommended
  routine message size.
- Reject invalid, partial and oversized messages without replacing last-valid metric state.
- Require bounded connect retry and reconnect with backoff.
- Require a finite producer write deadline and surface failures to application instrumentation.
- Limit concurrent stream connections below `RLIMIT_NOFILE`; reserve descriptors for exposition and runtime duties.
- Do not promise exactly-once delivery. If clients retry after ambiguous disconnect, use producer identity and sequence
  numbers to make duplicate handling explicit.
- Do not select Unix datagram as primary reliable ingestion. Any compatibility adapter must publish drop/error counters.

## Prototype Limits

- Only macOS Docker Desktop/LinuxKit aarch64 is measured; Ubuntu confirmation is pending.
- Server and clients share one Go process.
- Synthetic parsing does not represent a production registry or cardinality contention.
- Latency ends at protocol acceptance, not metric exposition.
- Three repetitions characterize feasibility, not stable capacity.
- Memory is Go runtime process memory, not isolated server RSS.
- No multi-user UID/GID attack test, parser fuzzing, race test, Kubernetes or native Linux run.
- Connection backlog, churn, half-close and long-stalled frames are not swept parametrically.

## Additional Benchmarking

| Benchmark item                    | Status                                         | Evidence                             |
|-----------------------------------|------------------------------------------------|--------------------------------------|
| Single producer                   | Covered                                        | `correctness.tsv`, `performance.tsv` |
| 8 and 32 producers                | Covered                                        | `performance.tsv`                    |
| 64 B, 1 KiB, 8 KiB                | Covered                                        | `performance.tsv`                    |
| Three repetitions, p50/p95/p99    | Covered                                        | `performance.tsv`                    |
| Throughput, CPU, runtime memory   | Covered                                        | `performance.tsv`                    |
| Slow reader/backpressure          | Covered                                        | `pressure.tsv`                       |
| Disconnect during message         | Covered for both stream framings               | `correctness.tsv`                    |
| Startup race                      | Covered with bounded retry                     | `correctness.tsv`                    |
| Restart/reconnect                 | Covered                                        | `correctness.tsv`                    |
| Malformed, max, oversized         | Covered                                        | `correctness.tsv`                    |
| Socket permission                 | Covered for mode `0660`                        | `correctness.tsv`                    |
| FD exhaustion                     | Covered for stream; datagram FD model recorded | `pressure.tsv`                       |
| Environment/fingerprint           | Covered                                        | `environment.tsv`                    |
| Ubuntu matching fingerprint       | Pending                                        | same command required                |
| Native Linux / Kubernetes         | Not run                                        | follow-up                            |
| Separate processes and cgroup RSS | Not run                                        | follow-up                            |
| Parser fuzz/race test             | Not run                                        | follow-up                            |
| Real StatsD compatibility grammar | Not run                                        | only if adapter retained             |

## Conclusion

The macOS/LinuxKit evidence provisionally selects a Unix stream socket with a custom versioned line protocol.
Stream backpressure preserved all messages in the tested pressure case; Unix datagram did not and is rejected as the
primary reliable transport. Four-byte length framing remains a fallback for binary payloads, not the default.

The result is not final until Ubuntu runs the same fingerprint. Status remains `in progress`; no ADR is produced.
