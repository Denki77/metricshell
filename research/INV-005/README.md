# INV-005 — Ingestion Transport Comparison

Status: completed
Reference runs: `results/20260723T152957Z`, `results/20260723T153335Z`
Report: [report.md](report.md)

## Question

Which ingestion transports should be supported in the first stable release, and which should remain optional adapters?

## Candidates

File snapshot, Unix stream, Unix datagram, loopback TCP HTTP, local gRPC over Unix socket, POSIX shared-memory-backed
mapping and regular memory-mapped file.

## Initial Hypotheses

- file snapshot is the simplest PHP state-publication baseline;
- Unix stream is the strongest event-oriented first-release candidate;
- datagrams trade acknowledgement for lower publication cost;
- HTTP is the simplest request/response integration;
- gRPC, shared memory and mmap require a material benefit to offset protocol and client complexity.

## Evidence Required

Containerized prototypes, identical-code macOS/Ubuntu runs, equivalent delivery profiles, wall-clock throughput,
per-operation latency, payload/concurrency profiles, executable PHP integrations and real failure probes.

## Experiments

The runner separates three contracts:

- `publish-only`: producer API publication completed;
- `consumer-observed`: an independent consumer goroutine observed that exact sequence number;
- `acknowledged`: the transport returned an explicit application response.

Only rows with the same profile are comparable. Each supported transport/profile pair runs 64 B, 1 KiB, 16 KiB and
four-producer 64 B cases, with 500 operations per producer. Aggregate throughput is total completed operations divided
by scenario wall time; percentiles remain per-operation latency.

## Results

Both matching-fingerprint Docker Desktop/LinuxKit runs emitted all 52 cells and passed all 15/15 assertions and 20/20
observation contracts. All publish-only and acknowledged API operations completed. Consumer-observed delivered
17,482/17,500 exact publications in each environment. The 18 superseded publications were six each in the
four-producer file, shared-memory and mmap snapshot cells; Unix stream and datagram observed every publication.

| Environment              | Architecture | Docker | Kernel           | Result set         | Fingerprint     |
|--------------------------|--------------|--------|------------------|--------------------|-----------------|
| Docker Desktop on macOS  | aarch64      | 29.4.3 | LinuxKit 6.12.76 | `20260723T152957Z` | `71eb92f8…02b1` |
| Docker Desktop on Ubuntu | x86_64       | 27.4.0 | LinuxKit 6.10.14 | `20260723T153335Z` | `71eb92f8…02b1` |

64 B, single-producer macOS results:

| Transport     | Profile           |     ops/s | p50 µs | p95 µs |  p99 µs |
|---------------|-------------------|----------:|-------:|-------:|--------:|
| File          | publish-only      |    22,177 | 39.458 | 77.875 | 128.041 |
| File          | consumer-observed |    19,214 | 48.958 | 83.375 | 102.083 |
| Unix stream   | publish-only      |   153,927 |  4.125 |  9.625 |  14.000 |
| Unix stream   | consumer-observed |    58,390 |  9.625 | 31.917 |  52.917 |
| Unix stream   | acknowledged      |    62,279 | 12.042 | 34.917 |  38.041 |
| Unix datagram | publish-only      |   136,900 |  4.209 |  9.458 |  14.208 |
| Unix datagram | consumer-observed |    98,071 |  6.125 | 26.334 |  35.208 |
| HTTP          | acknowledged      |    36,562 | 15.584 | 50.167 |  82.375 |
| gRPC          | acknowledged      |    42,881 | 15.458 | 31.084 |  88.792 |
| Shared memory | publish-only      | 9,002,197 |  0.042 |  0.042 |   0.042 |
| Shared memory | consumer-observed | 1,544,602 |  0.500 |  0.792 |   1.209 |
| mmap          | publish-only      | 5,943,536 |  0.042 |  0.084 |   0.084 |
| mmap          | consumer-observed | 1,030,486 |  0.500 |  1.417 |   7.458 |

Ubuntu 64 B single-producer figures and every payload/multi-producer cell are recorded in
`results/20260723T153335Z/summary.tsv`.

## Conclusion

Support file snapshot, Unix stream and loopback HTTP in the first stable release. Treat Unix datagram as an optional,
explicitly unacknowledged adapter. Do not include gRPC, shared memory or mmap in the first stable transport set.
Snapshot transports use latest-state semantics and may supersede intermediate concurrent publications.

The conclusion is confirmed across matching-fingerprint LinuxKit aarch64 and x86_64 environments.

## Decision Output

- Prototype: `prototype/`
- Runner: `run-bench.sh`
- Raw evidence: `results/20260723T152957Z/`, `results/20260723T153335Z/`
- Report: [report.md](report.md)
- ADR: [ADR-005](../../docs/06-architecture/adr/ADR-005.md)

## Running the Prototype

The command is identical on macOS and Ubuntu:

```bash
./research/INV-005/run-bench.sh
```

Higher sample count:

```bash
INV005_COUNT=5000 ./research/INV-005/run-bench.sh
```

The runner removes older INV-005 results, builds the image, applies 2 CPU / 512 MiB limits, executes every enabled
profile, writes `observation-contracts.tsv`, runs failure probes and PHP delivery checks, then writes
`latest-results.txt`. Benchmark artifacts use a temporary Docker named volume and are exported with `docker cp`; no
host path is bind-mounted. It exits nonzero if Unix stream observation or a reliable single-producer snapshot is
incomplete.

## Prototype Limits

- Both tested container environments use LinuxKit. Native non-LinuxKit Linux remains unverified.
- The independent consumer is a separate goroutine, not a separate OS process.
- Consumer-observed uses a benchmark control-plane tracker; it is not a transport acknowledgement.
- File polling is a tight research loop, not the INV-006 inotify/reconciliation design.
- HTTP is loopback TCP with port allocation and HTTP parsing; HTTP over Unix socket was not tested.
- gRPC uses a raw codec to isolate gRPC transport overhead rather than generated protobuf messages.
- Timing values are comparative observations, not production SLOs.

## Additional Benchmarks

Covered:

- 52 transport/profile/scenario cells and raw samples;
- wall-clock aggregate throughput and p50/p95/p99 latency;
- 64 B, 1 KiB, 16 KiB and four producers;
- executable missing Unix socket, refused HTTP, cross-filesystem rename and mmap resize probes;
- measured AF_UNIX datagram accepted boundary: 212,960 bytes in this environment;
- executable PHP delivery for file, Unix stream and HTTP;
- enforced observation contracts with sent, observed, lost and loss ratio per transport/scenario;
- benchmark fingerprint including `prototype/` and `run-bench.sh`;
- dirty/untracked benchmark-scope metadata.

Future follow-up:

- separate-process consumer confirmation;
- producer/consumer crash, restart, slow consumer, backlog saturation and sustained datagram-loss injection;
- CPU/RSS/cgroup sampling and repeated cold/warm runs;
- file reconciliation in INV-006 and production socket protocol in INV-007.
