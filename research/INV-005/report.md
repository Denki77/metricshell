# INV-005 Report — Ingestion Transport Comparison

Status: completed
Run date: 2026-07-23
Docker Servers: 29.4.3, 27.4.0
Docker platforms: LinuxKit aarch64, LinuxKit x86_64
Reference runs: `results/20260723T152957Z`, `results/20260723T153335Z`
Summaries: `results/20260723T152957Z/summary.tsv`, `results/20260723T153335Z/summary.tsv`

## Goal

Compare all INV-005 candidates without mixing publication, consumer observation and acknowledgement semantics, and
narrow the first-release candidates before the focused INV-006–009 protocol investigations.

## Prototype

`prototype/cmd/bench` implements all seven transports, three explicit delivery profiles, raw latency recording,
wall-clock scenario timing, executable boundary probes and one-shot servers for PHP integration. `run-bench.sh` builds
the image, runs every profile, verifies PHP file/Unix stream/HTTP delivery and retains only current evidence. Artifacts
are written to a temporary Docker named volume and exported with `docker cp`; the host repository path is never mounted
into a container.

## Run Commands

```bash
./research/INV-005/run-bench.sh
INV005_COUNT=5000 ./research/INV-005/run-bench.sh
```

## Run Environments and Fingerprint

| Environment              | Docker | Architecture | Container kernel | Scope clean | Untracked | Result set         |
|--------------------------|-------:|--------------|------------------|-------------|-----------|--------------------|
| Docker Desktop on macOS  | 29.4.3 | aarch64      | LinuxKit 6.12.76 | false       | 0         | `20260723T152957Z` |
| Docker Desktop on Ubuntu | 27.4.0 | x86_64       | LinuxKit 6.10.14 | true        | 0         | `20260723T153335Z` |

Both runs used 2 CPU / 512 MiB and the identical benchmark fingerprint:

```text
71eb92f8d9eb1fd400f040706197d2d8edd7f84c9580bf61cbdf66621d0002b1
```

The fingerprint includes `prototype/` and `run-bench.sh`. Repository HEAD differs and is context only. Fingerprint
equality establishes benchmark identity.

## Benchmark Contracts

| Profile           | Completion point                                 | Applicable candidates                       |
|-------------------|--------------------------------------------------|---------------------------------------------|
| publish-only      | producer API returns after publication           | file, stream, datagram, shared memory, mmap |
| consumer-observed | independent consumer observes the exact sequence | file, stream, datagram, shared memory, mmap |
| acknowledged      | application response received                    | stream, HTTP, gRPC                          |

Datagram is never presented as acknowledged. File readback is performed by the consumer loop, not the producer.
Shared-memory/mmap observation is also performed by the consumer loop. Results are compared only inside a profile.

`observation-contracts.tsv` makes these rules executable. Unix stream and single-producer snapshot rows must be
complete or the runner exits nonzero. Datagram always records sent, observed, lost and loss ratio. Multi-producer
snapshot loss is allowed only under `allow_superseded_record_loss` and is never hidden.

## Throughput Method

Each scenario records a wall-clock start immediately before releasing all producers and an end after all producers
finish:

`aggregate_ops_s = total operations / wall-clock elapsed`

No sum of individual latencies and no producer multiplier enters aggregate throughput. Individual samples are used only
for p50/p95/p99.

## Results

In both environments all 52 cells were emitted, all 15/15 assertions and 20/20 observation contracts passed, and
publish-only and acknowledged operations completed 100%. Exact consumer observation was 17,482/17,500 in each
environment. The four-producer file,
shared-memory and mmap snapshot cells each superseded 6/2,000 intermediate publications in both runs; this is evidence
of state-replacement semantics, not a runner failure.

The enforced observation assertions passed:

| Contract assertion                            | Actual          |
|-----------------------------------------------|-----------------|
| Unix stream observation complete              | true            |
| Reliable single-producer snapshots complete   | true            |
| Datagram sent/observed/lost recorded          | 4/4 scenarios   |
| Multi-producer snapshot supersession recorded | 3 rows, 18 lost |

All five executable failure probes and all three PHP delivery assertions also passed in both environments.

### Publish-only, 64 B, one producer

| Transport     |     ops/s | p50 µs | p95 µs |  p99 µs |
|---------------|----------:|-------:|-------:|--------:|
| File          |    22,177 | 39.458 | 77.875 | 128.041 |
| Unix stream   |   153,927 |  4.125 |  9.625 |  14.000 |
| Unix datagram |   136,900 |  4.209 |  9.458 |  14.208 |
| Shared memory | 9,002,197 |  0.042 |  0.042 |   0.042 |
| mmap          | 5,943,536 |  0.042 |  0.084 |   0.084 |

### Consumer-observed, 64 B, one producer

| Transport     |     ops/s | p50 µs | p95 µs |  p99 µs |
|---------------|----------:|-------:|-------:|--------:|
| File          |    19,214 | 48.958 | 83.375 | 102.083 |
| Unix stream   |    58,390 |  9.625 | 31.917 |  52.917 |
| Unix datagram |    98,071 |  6.125 | 26.334 |  35.208 |
| Shared memory | 1,544,602 |  0.500 |  0.792 |   1.209 |
| mmap          | 1,030,486 |  0.500 |  1.417 |   7.458 |

### Acknowledged, 64 B, one producer

| Transport   |  ops/s | p50 µs | p95 µs | p99 µs |
|-------------|-------:|-------:|-------:|-------:|
| Unix stream | 62,279 | 12.042 | 34.917 | 38.041 |
| HTTP        | 36,562 | 15.584 | 50.167 | 82.375 |
| gRPC        | 42,881 | 15.458 | 31.084 | 88.792 |

### Ubuntu/LinuxKit x86_64, 64 B, one producer

| Transport     | Profile           |     ops/s |  p50 µs |    p95 µs |    p99 µs |
|---------------|-------------------|----------:|--------:|----------:|----------:|
| File          | publish-only      |     3,328 | 111.453 | 1,058.916 | 4,162.222 |
| File          | consumer-observed |     8,916 | 110.210 |   151.584 |   212.376 |
| Unix stream   | publish-only      |    18,585 |  21.032 |    29.055 |    35.842 |
| Unix stream   | consumer-observed |     8,217 |  69.317 |   347.134 |   422.273 |
| Unix stream   | acknowledged      |    20,084 |  41.323 |    74.891 |    88.654 |
| Unix datagram | publish-only      |    63,352 |  14.194 |    18.068 |    27.765 |
| Unix datagram | consumer-observed |    45,238 |  16.985 |    40.548 |    62.738 |
| HTTP          | acknowledged      |    14,434 |  51.697 |   153.210 |   231.362 |
| gRPC          | acknowledged      |     9,574 |  68.163 |   324.494 |   555.286 |
| Shared memory | publish-only      | 1,430,059 |   0.254 |     0.422 |     0.435 |
| Shared memory | consumer-observed |   333,398 |   2.335 |     3.294 |    17.304 |
| mmap          | publish-only      | 6,909,513 |   0.045 |     0.046 |     0.047 |
| mmap          | consumer-observed |   802,955 |   0.891 |     1.400 |     1.682 |

Full payload and four-producer metrics are in each run's `summary.tsv`; `samples.tsv` contains every operation.

### Signal-to-exit

Signal-to-exit is not an INV-005 transport metric: the prototype neither supervises a workload nor sends termination
signals. No signal-to-exit samples exist in either result set. The comparable timing evidence for this investigation is
transport operation latency and scenario throughput above. Signal-to-exit remains lifecycle evidence in INV-001/002.

## PHP Integration Evidence

The image executes, rather than syntax-checks, these producer paths:

| Path               | Consumer evidence                          | Result |
|--------------------|--------------------------------------------|--------|
| file atomic rename | resulting payload read from mounted path   | pass   |
| Unix stream        | Go consumer persisted received bytes       | pass   |
| loopback HTTP      | Go handler persisted body and returned 204 | pass   |

PHP effort remains a qualitative rating, but the three serious candidate paths are executable.

## Failure-Mode Evidence

`probes.tsv` is generated by executed operations:

| Probe                                   | Observation                                                  |
|-----------------------------------------|--------------------------------------------------------------|
| missing Unix socket connect             | rejected                                                     |
| HTTP connection to closed loopback port | rejected                                                     |
| `/tmp` to `/dev/shm` rename             | rejected as cross-filesystem                                 |
| mmap file growth                        | existing mapping length remained 4,096 bytes; remap required |
| AF_UNIX datagram boundary               | largest accepted payload measured as 212,960 bytes           |

The datagram number is environment evidence, not a portable protocol maximum.

## Evaluation Matrix

| Criterion                        | File                | Stream    | Datagram          | HTTP      | gRPC        | Shared memory       | mmap                |
|----------------------------------|---------------------|-----------|-------------------|-----------|-------------|---------------------|---------------------|
| PHP path                         | executed            | executed  | example only      | executed  | high effort | high effort         | high effort         |
| Acknowledgement                  | no                  | yes       | no                | yes       | yes         | no                  | no                  |
| Exact multi-producer observation | snapshot supersedes | 100%      | 100% in run       | 100% ack  | 100% ack    | snapshot supersedes | snapshot supersedes |
| Recovery model                   | persistent snapshot | reconnect | loss/retry policy | retry     | retry       | protocol required   | remap/reconcile     |
| Final role                       | stable v1           | stable v1 | optional/lossy    | stable v1 | reject v1   | reject v1           | reject v1           |

## Acceptable Values and Policies

- payload range carried forward: 64 B–16 KiB;
- producers carried forward: 1 and 4;
- serious first-release candidates: file snapshot, Unix stream, loopback HTTP;
- datagram: optional, explicitly unacknowledged, configured below a measured environment-specific limit;
- snapshot transports: latest-state semantics; intermediate publication observation is not guaranteed;
- gRPC/shared memory/mmap: no v1 inclusion without a requirement that offsets client/protocol complexity;
- confirmation minimum: 500 samples/cell; 5,000 recommended.

## Prototype Limits

The consumer is independent benchmark logic but remains a goroutine in the same process. Consumer-observed timing uses a
control-plane tracker, not a producer-visible transport ack. HTTP uses loopback TCP, not HTTP over Unix socket. File
polling is not the future inotify implementation. The gRPC raw codec excludes generated-protobuf serialization cost.
Both tested container environments use LinuxKit; native non-LinuxKit Linux remains unverified.

## Additional Benchmarking

Every implemented profile runs by default; no benchmark is hidden behind an opt-in agreement. Future focused transport
work may add separate-process consumers, crash/restart, slow consumers, backlog saturation, sustained datagram loss,
CPU/RSS/cgroup sampling and native non-LinuxKit Linux confirmation.

## Conclusion

Matching-fingerprint macOS/LinuxKit aarch64 and Ubuntu/LinuxKit x86_64 evidence confirms the decision:

- support file snapshot, Unix stream and loopback HTTP in the first stable release;
- retain Unix datagram only as an optional, explicitly unacknowledged adapter;
- exclude gRPC, shared memory and mmap from the first stable transport set;
- define snapshot transports as replaceable latest state, not an event log.

INV-005 is completed. Detailed file/socket protocol mechanics and additional fault behavior remain assigned to
INV-006–009 and do not block this transport-set decision.
