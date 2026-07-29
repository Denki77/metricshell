# INV-008 — Local Push Ingestion

Status: completed  
macOS reference run: `results/20260727T185055Z`  
Ubuntu reference run: `results/20260728T114459Z`  
Report: [report.md](report.md)  
Decision: [ADR-008](../../docs/06-architecture/adr/ADR-008.md)

## Question

Does a local HTTP or gRPC ingestion API provide enough value beyond the socket adapter to justify implementation and
maintenance?

## Context

This research concerns only producer-to-MetricShell ingestion inside one container/network namespace. MetricShell does
not push to Prometheus, Pushgateway or a central collector. The Unix-socket framed adapter represents the “no local push
API” candidate and is the baseline selected by the preceding transport research.

## Candidates

- no additional push API: framed Unix domain socket;
- local HTTP/1.1 JSON API;
- local unary gRPC/protobuf API.

## Initial Hypothesis

Local HTTP may simplify integration for languages with mature HTTP clients, but may duplicate socket capabilities and
increase attack surface.

## Evidence Required

- identical-container comparison of Unix socket, HTTP and gRPC;
- payload, batch and producer-concurrency scaling;
- latency, throughput, idle/active CPU and RSS;
- malformed and oversized requests, bounded payload and bind scope;
- shutdown, retained state and restart recovery;
- protocol/client complexity and debugging trade-offs;
- architecture-independent benchmark fingerprint for an Ubuntu repeat.

## Experiments

The full runner executes 36 performance combinations: three transports, payloads `64 B`, `1 KiB`, `16 KiB`, batches
`1` and `16`, producers `1` and `8`, with 100–300 requests per producer. Every transport performs one request containing
the same `N` individual records and increments the shared store by the real accepted record count. The runner also
checks malformed/empty/encoded-oversized HTTP requests, an identical decoded `1 MiB`/`1 MiB + 1 byte` boundary on all
three transports, loopback-only HTTP/gRPC binding, shutdown state retention, restart recovery and resources.

## Results

The matching-fingerprint macOS/LinuxKit aarch64 and Ubuntu/LinuxKit x86_64 runs passed all portable checks:

- performance matrix: 36 rows and zero request errors in each environment;
- correctness: 14/14 pass in each environment;
- accepted records including boundary and active resource samples: 385,313 in each environment;
- benchmark scope: clean and tracked, with zero untracked files in each environment;
- identical benchmark fingerprint:
  `34bee766d38ee43421cd100d3b23a387b7736c660d13bd6e28b591505bd101d4`.

Representative `1 KiB` results:

| Environment | Shape                            | Unix socket | HTTP JSON | gRPC protobuf |
|-------------|----------------------------------|------------:|----------:|--------------:|
| macOS       | 1 record, 1 producer, records/s  |     240,827 |    26,786 |        24,679 |
| Ubuntu      | 1 record, 1 producer, records/s  |      53,048 |     8,334 |         7,391 |
| macOS       | batch 16, 8 producers, records/s |   1,630,940 |    89,072 |       634,355 |
| Ubuntu      | batch 16, 8 producers, records/s |     928,096 |    21,172 |       187,377 |
| macOS       | batch 16, 8 producers, p95       |    0.245 ms |  3.246 ms |      1.046 ms |
| Ubuntu      | batch 16, 8 producers, p95       |    0.400 ms | 11.301 ms |      2.297 ms |

The Unix-socket baseline led every representative shape. HTTP was simpler and slightly faster than gRPC for the
single-producer/single-record case. gRPC amortized framing and strongly outperformed JSON HTTP for concurrent batches,
but remained below Unix-socket throughput in both environments.

Timing distributions differ by environment and are observations rather than portable pass criteria. Candidate ordering
and all correctness conclusions are consistent.

## Conclusion

The hypothesis is confirmed by matching-fingerprint macOS/LinuxKit aarch64 and Ubuntu/LinuxKit x86_64 runs.

- Do not require an additional local push API: the Unix-socket candidate already covers ingestion with lower latency,
  higher throughput, fewer endpoint-exposure modes and no HTTP/2/protobuf client stack.
- Retain HTTP JSON as an optional compatibility adapter if concrete users need standard-library HTTP integration or
  curl-level debugging.
- Do not add gRPC by default. Its concurrent batched performance is materially better than JSON HTTP, but it duplicates
  socket semantics and introduces generated clients, protobuf schema evolution and HTTP/2 dependencies.

## Admissible Values

- bind HTTP/gRPC to `127.0.0.1` by default; wildcard/external exposure requires an explicit, separately secured mode;
- maximum decoded request payload: `1 MiB` in this prototype;
- batch size: `1–16` records tested; prefer batching for HTTP/gRPC;
- concurrent producers: `1–8` tested;
- malformed JSON: HTTP `400`; empty records: HTTP `422`; decoded/encoded oversize: HTTP `413`;
- reject empty, malformed and oversized requests without changing accepted state;
- retain accepted in-memory state when an adapter stops; clients must reconnect after restart;
- use versioned paths for HTTP (`/v1/metrics`) and versioned protobuf packages/services for gRPC;
- apply bounded request/body reads and deadlines; production queue/cardinality limits remain owned by INV-014.

## Running the Prototype

From the repository root on either macOS or Ubuntu:

```bash
./research/INV-008/run-bench.sh
```

Inspect the result:

```bash
latest="$(cat research/INV-008/latest-results.txt)"
cat "$latest/summary.tsv"
cat "$latest/correctness.tsv"
cat "$latest/performance.tsv"
cat "$latest/resources.tsv"
cat "$latest/environment.tsv"
```

The runner builds one Linux image, creates a container without host bind mounts, runs the matrix inside it and copies
evidence out with `docker cp`. Ubuntu uses exactly the same command. Compare
`benchmark_code_fingerprint_sha256`; repository HEAD and architecture-specific image IDs are context only.

Manual run:

```bash
docker build --pull=false -t metricshell-inv008:prototype research/INV-008/prototype
docker run --rm metricshell-inv008:prototype /out
```

## Prototype Limits

- Research code, not a production parser, registry or admission-control implementation.
- Both tested container environments use LinuxKit. The results confirm cross-architecture behavior inside
  macOS/LinuxKit aarch64 and Ubuntu/LinuxKit x86_64 containers, not native non-LinuxKit Linux behavior.
- All transports carry one request containing a record array. Unix framing is `record count` followed by repeated
  `record length + record`, so wire and store semantics match HTTP and gRPC.
- HTTP uses JSON byte strings (base64 encoding), intentionally exposing serialization overhead.
- The gRPC test is unary; streaming RPC would be a distinct protocol with more lifecycle/backpressure complexity.
- CPU/RSS describe one combined benchmark process, not isolated per-adapter production footprints.
- The in-memory store counts accepted records and does not model cardinality, label validation or metric conflicts.
- Loopback binding reduces exposure but is not authentication against other processes in the same network namespace.
- Timing values on Docker Desktop are architectural comparisons, not service-level objectives.

## Additional Benchmarks

All safe, locally executable variants identified for this research were run:

| Benchmark                                   | Status                                                                                       |
|---------------------------------------------|----------------------------------------------------------------------------------------------|
| 64 B / 1 KiB / 16 KiB payloads              | covered                                                                                      |
| batches 1 and 16                            | covered                                                                                      |
| 1 and 8 concurrent producers                | covered                                                                                      |
| Unix socket / HTTP / gRPC                   | covered                                                                                      |
| 100–300 requests per producer               | covered                                                                                      |
| p50/p95/p99 and throughput                  | covered                                                                                      |
| malformed, empty and encoded-oversized HTTP | covered                                                                                      |
| identical decoded 1 MiB boundary            | covered for Unix, HTTP and gRPC                                                              |
| loopback bind assertions                    | covered                                                                                      |
| shutdown, retained state and restart        | covered                                                                                      |
| idle CPU, active CPU and peak RSS           | covered                                                                                      |
| matching-fingerprint Ubuntu run             | covered: `results/20260728T114459Z`                                                          |
| native non-LinuxKit Linux                   | pending; no such environment available                                                       |
| PHP HTTP client                             | covered by INV-005; protocol compatibility is unchanged                                      |
| PHP gRPC client                             | not locally executable; requires extension/toolchain and is itself adoption-cost evidence    |
| TLS/mTLS and remote authentication          | not applicable to the selected local-only default; required if exposure is added             |
| streaming gRPC                              | rejected from this focused candidate because it changes lifecycle and backpressure semantics |
| production parser/cardinality pressure      | deferred to INV-014/INV-015 after schema and limits are selected                             |

## Decision Output

- Prototype: `prototype/`
- Runner: `run-bench.sh`
- Raw evidence: `results/20260727T185055Z/`, `results/20260728T114459Z/`
- Detailed analysis: [report.md](report.md)
- Accepted decision: [ADR-008](../../docs/06-architecture/adr/ADR-008.md)
