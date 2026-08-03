# INV-008 Report — Local Push Ingestion

Status: completed  
Run date: 2026-07-27  
Docker server: 29.4.3  
Docker platform: LinuxKit linux/aarch64  
Reference runs: `results/20260727T185055Z`, `results/20260728T114459Z`  
Benchmark fingerprint: `34bee766d38ee43421cd100d3b23a387b7736c660d13bd6e28b591505bd101d4`

## Goal

Test whether a container-local HTTP or gRPC ingestion API adds enough integration or performance value over the
Unix-socket adapter to justify another production protocol, dependency stack and exposed endpoint.

## Prototype

The prototype is located in `research/INV-008`.

- `prototype/cmd/inv008-bench` hosts all three adapters and executes correctness/resource/performance cases.
- `prototype/api/ingest.proto` defines the versionable unary gRPC contract.
- `prototype/api/*.pb.go` is generated client/server and protobuf code.
- `prototype/Dockerfile` builds the single cross-architecture Linux benchmark image.
- `run-bench.sh` runs the same image/matrix on macOS and Ubuntu and captures its fingerprint.
- `results/<timestamp>` contains TSV evidence, build log and environment metadata.

The adapters share one atomic accepted-record store so protocol overhead is compared against the same minimal state
operation. Every adapter performs one request containing `N` records and increments the store by the actual `N`.
Unix framing is a record count followed by repeated record-length/record pairs. HTTP accepts `POST /v1/metrics` JSON.
gRPC uses unary protobuf `ingest.Ingest/Push`.

## Run Commands

```bash
./research/INV-008/run-bench.sh
latest="$(cat research/INV-008/latest-results.txt)"
cat "$latest/summary.tsv"
cat "$latest/correctness.tsv"
cat "$latest/performance.tsv"
cat "$latest/resources.tsv"
cat "$latest/environment.tsv"
```

No host path is mounted into the container. Evidence is copied out after completion, avoiding Docker Desktop file-share
effects and Ubuntu daemon mount-policy differences.

## Run Environments

| Environment             | Date       | Docker | Kernel           | Architecture | Result set                 | Fingerprint                                                        |
|-------------------------|------------|--------|------------------|--------------|----------------------------|--------------------------------------------------------------------|
| Docker Desktop on macOS | 2026-07-27 | 29.4.3 | LinuxKit 6.12.76 | aarch64      | `results/20260727T185055Z` | `34bee766d38ee43421cd100d3b23a387b7736c660d13bd6e28b591505bd101d4` |
| Ubuntu Docker Desktop   | 2026-07-28 | 27.4.0 | LinuxKit 6.10.14 | x86_64       | `results/20260728T114459Z` | `34bee766d38ee43421cd100d3b23a387b7736c660d13bd6e28b591505bd101d4` |

The image ID is architecture-specific and is not the portable identity. The fingerprint hashes the prototype, module
lock and runner contents with relative names. Both runs recorded a clean benchmark scope and zero untracked benchmark
files. The fingerprints are identical.

## Results

### Correctness

All fourteen assertions passed in both environments:

| Case                                                  | Result |
|-------------------------------------------------------|--------|
| malformed HTTP rejected with 400                      | pass   |
| empty HTTP record set rejected with 422               | pass   |
| HTTP encoded-body guard rejected with 413             | pass   |
| Unix decoded `1 MiB` accepted                         | pass   |
| Unix decoded `1 MiB + 1 byte` rejected                | pass   |
| HTTP decoded `1 MiB` accepted                         | pass   |
| HTTP decoded `1 MiB + 1 byte` rejected with 413       | pass   |
| gRPC decoded `1 MiB` accepted                         | pass   |
| gRPC decoded `1 MiB + 1 byte` was `ResourceExhausted` | pass   |
| all 36 matrix rows completed without request errors   | pass   |
| HTTP loopback-only bind                               | pass   |
| gRPC loopback-only bind                               | pass   |
| adapter shutdown retained accepted state              | pass   |
| HTTP adapter restart accepted new requests            | pass   |

The `empty HTTP record set` assertion used a syntactically valid request that decoded to zero records. It did not send
an empty HTTP body and did not model the production zero-series snapshot contract. ADR-004 supersedes rejection of this
decoded representation; the assertion remains accurate historical prototype evidence, not a production requirement.

The application limit is one decoded MiB for every transport. HTTP’s encoded-body guard is deliberately larger than
the base64/JSON representation of a valid one-MiB decoded request; the shared store applies the authoritative decoded
limit. All transports accept exactly one decoded MiB and reject one MiB plus one byte without incrementing state.

### Performance matrix

The matrix comprises three payload sizes × two batch sizes × two producer counts × three transports. All 36 rows had
zero request errors in both environments, and both accepted exactly 385,313 records across the full run.

Representative `1 KiB` rows:

| Environment | Transport | Batch | Producers | Records/s |      p50 |       p95 |       p99 |
|-------------|-----------|------:|----------:|----------:|---------:|----------:|----------:|
| macOS       | Unix      |     1 |         1 |   240,827 | 0.002 ms |  0.005 ms |  0.028 ms |
| macOS       | HTTP      |     1 |         1 |    26,786 | 0.025 ms |  0.057 ms |  0.106 ms |
| macOS       | gRPC      |     1 |         1 |    24,679 | 0.022 ms |  0.077 ms |  0.157 ms |
| Ubuntu      | Unix      |     1 |         1 |    53,048 | 0.011 ms |  0.055 ms |  0.067 ms |
| Ubuntu      | HTTP      |     1 |         1 |     8,334 | 0.088 ms |  0.225 ms |  0.438 ms |
| Ubuntu      | gRPC      |     1 |         1 |     7,391 | 0.092 ms |  0.243 ms |  0.484 ms |
| macOS       | Unix      |    16 |         8 | 1,630,940 | 0.029 ms |  0.245 ms |  0.959 ms |
| macOS       | HTTP      |    16 |         8 |    89,072 | 1.357 ms |  3.246 ms |  4.153 ms |
| macOS       | gRPC      |    16 |         8 |   634,355 | 0.080 ms |  1.046 ms |  1.998 ms |
| Ubuntu      | Unix      |    16 |         8 |   928,096 | 0.074 ms |  0.400 ms |  0.869 ms |
| Ubuntu      | HTTP      |    16 |         8 |    21,172 | 5.727 ms | 11.301 ms | 14.151 ms |
| Ubuntu      | gRPC      |    16 |         8 |   187,377 | 0.295 ms |  2.297 ms |  5.639 ms |

For low concurrency, HTTP’s standard persistent connection is competitive with unary gRPC, though both are far behind
the Unix frame. With batching and concurrency, protobuf/gRPC avoids JSON/base64 overhead and substantially exceeds
HTTP throughput. It still does not catch the simpler Unix socket. Timing distributions differ, but candidate ordering is
consistent in both environments.

Batching improves records per request for every transport, but increases the damage radius of a rejected request and
requires a batch-level acceptance policy. A tested batch of 16 is an admissible research value, not a final production
maximum.

### Resources

| Environment | Phase                         |    Elapsed |    CPU, one-core equivalent | Peak RSS/HWM |
|-------------|-------------------------------|-----------:|----------------------------:|-------------:|
| macOS       | all adapters idle             |   2,000 ms |                      0.500% |   54,260 KiB |
| macOS       | gRPC batch/concurrency active | 114.716 ms |                    331.251% |   54,260 KiB |
| Ubuntu      | all adapters idle             |   2,000 ms | below 10 ms tick resolution |   49,264 KiB |
| Ubuntu      | gRPC batch/concurrency active | 260.805 ms |                    578.974% |   49,264 KiB |

The active sample used eight producers and therefore legitimately used multiple cores. RSS includes all servers,
clients, Go runtime, protobuf and HTTP stacks. It is evidence of feasibility, not isolated adapter cost.

### Client and operational complexity

HTTP requires only a standard HTTP client, a documented JSON schema and a versioned path. It is directly observable
with curl and common proxies/debuggers.

gRPC requires a `.proto`, generated client code, compatible protobuf/gRPC runtime versions and HTTP/2-aware debugging.
Its stronger typed contract and efficient batched encoding are real advantages, but MetricShell would maintain another
schema/toolchain in addition to the socket protocol. PHP commonly needs an extension and generated stubs, making its
adoption effort materially higher than HTTP.

Both TCP APIs create bind/address configuration that a Unix pathname does not. Loopback prevents accidental host/LAN
exposure in the normal container configuration, but any process sharing the network namespace can connect. Wildcard
binding must never be the implicit default.

## Hypothesis Evaluation

### HTTP simplifies language integration

Supported. Standard HTTP clients and curl-level inspection reduce integration and debugging effort. The
benefit is compatibility, not measured performance.

### HTTP duplicates socket capability

Supported. Every tested acceptance and lifecycle behavior was already expressible through the framed Unix socket, which
was faster in all representative comparisons.

### HTTP increases attack surface

Supported structurally and by the bind checks. HTTP introduces a TCP listener, method/path/parser behavior and an
address-exposure mode. Loopback-only bind is required for the local contract; authentication/TLS would become necessary
if that contract changes.

### gRPC provides sufficient incremental value

Not supported as a default adapter. gRPC showed a clear advantage over JSON HTTP for concurrent batching but no
advantage over Unix socket. The performance gain does not remove generated clients, protobuf versioning or HTTP/2
dependencies.

## Acceptable Values and Policies

- default candidate: use the Unix-socket adapter;
- stable request/response transport selected by ADR-005: loopback HTTP JSON;
- gRPC: not part of the default implementation;
- bind: `127.0.0.1`; external bind requires explicit security design;
- HTTP endpoint: versioned `/v1/metrics`;
- maximum decoded request: `1 MiB` tested;
- batch: `1–16` records tested;
- producers: `1–8` concurrent tested;
- deadlines and bounded reads required;
- malformed JSON: HTTP `400`; prototype empty decoded record set: `422`; oversize HTTP request: `413`;
- production contract: an empty transport payload is malformed, while a syntactically valid snapshot containing zero
  metric families or series is a valid zero-series snapshot and must be accepted;
- malformed/oversized request: reject atomically and retain prior state;
- adapter shutdown/restart: retain MetricShell state, return connection failure during downtime, require reconnect;
- schema compatibility: additive changes within a version; breaking changes require a new HTTP path/protobuf package.

## Prototype Limits

- Both evidence environments use LinuxKit. The evidence confirms cross-architecture behavior in LinuxKit
  aarch64/x86_64 containers, not native non-LinuxKit Linux.
- Both client and server run in the same benchmark process and network namespace.
- The Unix baseline avoids JSON/protobuf serialization, but uses the same request-with-`N`-records semantics and
  performs the same store operation.
- No production metric parsing, conflict handling, cardinality enforcement, queue or disk persistence is modeled.
- HTTP JSON bytes are base64-encoded by Go’s JSON package.
- Only unary gRPC was evaluated.
- CPU tick resolution is 10 ms and the active window is short.
- TLS, mTLS and authentication were not exercised because external exposure is outside the local-only candidate.
- Native Linux, containerd/CRI-O and Kubernetes were not available.

## Additional Benchmarking

| Benchmark item                           | Status                                 | Evidence / rationale                            |
|------------------------------------------|----------------------------------------|-------------------------------------------------|
| all three candidates                     | covered                                | `performance.tsv`                               |
| payload scaling                          | covered: 64 B, 1 KiB, 16 KiB           | `performance.tsv`                               |
| batching                                 | covered: 1, 16                         | `performance.tsv`                               |
| producer concurrency                     | covered: 1, 8                          | `performance.tsv`                               |
| repetitions/request populations          | covered: 100–300 requests per producer | `performance.tsv`                               |
| latency percentiles and throughput       | covered                                | `performance.tsv`                               |
| malformed/empty/encoded oversize HTTP    | covered                                | `correctness.tsv`                               |
| decoded 1 MiB boundary on all transports | covered                                | `correctness.tsv`                               |
| bind exposure default                    | covered                                | `correctness.tsv`                               |
| shutdown, retained state, restart        | covered                                | `correctness.tsv`                               |
| idle/active resources                    | covered                                | `resources.tsv`                                 |
| environment and code fingerprint         | covered                                | `environment.tsv`                               |
| clean tracked benchmark scope            | covered: `true`, zero untracked files  | `environment.tsv`                               |
| Ubuntu same-fingerprint repeat           | covered                                | `results/20260728T114459Z`                      |
| native non-LinuxKit Linux                | not covered                            | environment unavailable                         |
| PHP HTTP interoperability                | already covered by INV-005             | no new wire requirement                         |
| PHP gRPC                                 | unavailable locally                    | extension/generated-stub cost is material       |
| streaming gRPC                           | not selected                           | would add a fourth protocol/lifecycle candidate |
| TLS/auth                                 | not applicable to local-only default   | mandatory follow-up for external bind           |
| cardinality/backpressure limits          | deferred                               | INV-014/INV-015 after production schema         |

## Conclusion

INV-008’s initial assumption is confirmed by identical-fingerprint macOS/LinuxKit aarch64 and Ubuntu/LinuxKit x86_64
runs. HTTP can improve client convenience, but it duplicates the socket adapter and adds a TCP/HTTP attack surface.
gRPC improves batched HTTP performance but does not beat the socket baseline and has the highest client/toolchain cost.

Decision: keep Unix socket as the default, retain loopback HTTP as the stable request/response transport selected by
ADR-005, and reject gRPC from the default surface. See
[ADR-008](../../docs/06-architecture/adr/ADR-008.md).
