# INV-005 Report — Ingestion Transport Comparison

Status: in progress  
Run date: 2026-07-23  
Docker Server: 29.4.3  
Docker platform: LinuxKit aarch64  
Reference run: `results/20260723T152957Z`
Summary: `results/20260723T152957Z/summary.tsv`

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

## Run Environment and Fingerprint

| Key                             | Value                                                              |
|---------------------------------|--------------------------------------------------------------------|
| Docker                          | Desktop 29.4.3                                                     |
| Container kernel                | LinuxKit 6.12.76                                                   |
| Architecture                    | aarch64                                                            |
| Limits                          | 2 CPU, 512 MiB                                                     |
| Fingerprint                     | `71eb92f8d9eb1fd400f040706197d2d8edd7f84c9580bf61cbdf66621d0002b1` |
| Benchmark scope diff clean      | false                                                              |
| Benchmark scope untracked count | 0                                                                  |

The fingerprint includes `prototype/` and `run-bench.sh`. The dirty flag is expected for the current uncommitted
research changes and prevents claiming a clean repository state. Ubuntu comparison requires fingerprint equality.

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

All 52 cells were emitted. Publish-only and acknowledged operations completed 100%. Exact consumer observation was
17,482/17,500. The four-producer file, shared-memory and mmap snapshot cells each superseded 6/2,000 intermediate
publications; this is evidence of state-replacement semantics, not a runner failure.

The enforced observation assertions passed:

| Contract assertion                            | Actual          |
|-----------------------------------------------|-----------------|
| Unix stream observation complete              | true            |
| Reliable single-producer snapshots complete   | true            |
| Datagram sent/observed/lost recorded          | 4/4 scenarios   |
| Multi-producer snapshot supersession recorded | 3 rows, 18 lost |

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

Full payload and four-producer results are in `summary.tsv`; `samples.tsv` contains every operation.

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

| Criterion                        | File                | Stream    | Datagram          | HTTP     | gRPC        | Shared memory       | mmap                |
|----------------------------------|---------------------|-----------|-------------------|----------|-------------|---------------------|---------------------|
| PHP path                         | executed            | executed  | example only      | executed | high effort | high effort         | high effort         |
| Acknowledgement                  | no                  | yes       | no                | yes      | yes         | no                  | no                  |
| Exact multi-producer observation | snapshot supersedes | 100%      | 100% in run       | 100% ack | 100% ack    | snapshot supersedes | snapshot supersedes |
| Recovery model                   | persistent snapshot | reconnect | loss/retry policy | retry    | retry       | protocol required   | remap/reconcile     |
| Provisional role                 | serious             | serious   | optional/lossy    | serious  | reject v1   | reject v1           | reject v1           |

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

## Additional Benchmarking

Every implemented profile runs by default; no benchmark is hidden behind an opt-in agreement. Remaining work needs new
fault-capable implementations: separate-process consumers, crash/restart, slow consumers, backlog saturation, sustained
datagram loss, CPU/RSS/cgroup sampling, repeated runs and unchanged-fingerprint Ubuntu confirmation.

## Conclusion

The corrected macOS evidence supports continuing with file snapshots, Unix stream and loopback HTTP. Datagram remains
optional and unacknowledged. Snapshot transports demonstrably supersede some concurrent intermediate updates, so they
must represent replaceable state rather than an event log. INV-005 remains in progress until Ubuntu and deeper fault
evidence are recorded; no ADR is produced yet.
