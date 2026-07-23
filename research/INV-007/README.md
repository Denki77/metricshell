# INV-007 — Socket-Based Ingestion

Status: in progress  
Reference run: `results/20260723T190106Z`  
Report: [report.md](report.md)

## Question

Which local socket model and protocol best fit MetricShell?

## Context

MetricShell and its producers run in one Linux container. The transport must support one or many producers, preserve
message boundaries, bound resource use, expose backpressure and recover explicitly from startup and restart races.

## Candidates

- Unix stream socket with newline-delimited, versioned text messages.
- Unix stream socket with a four-byte big-endian length prefix and versioned payload.
- Unix datagram socket with newline-delimited, versioned text messages (StatsD-like transport semantics).
- Unversioned StatsD was not retained as a primary protocol because its metric operations and error model do not cover
  the required versioned registry/update contract.

## Initial Hypothesis

A Unix stream socket with an explicit versioned protocol will provide reliable local delivery and backpressure.
Length framing may be safer than line framing for future binary payloads. Unix datagrams may reduce connection and
file-descriptor cost, but can lose messages under a slow reader and therefore are unlikely to be acceptable as the
primary ingestion path.

## Evidence Required

- Single and many producers.
- Burst throughput, latency, CPU and memory observations.
- Slow reader and backpressure behavior.
- Disconnect in the middle of a message.
- MetricShell startup and restart/reconnect.
- Malformed and oversized messages.
- Maximum accepted payload.
- Socket permissions and file-descriptor exhaustion.
- Identical macOS and Ubuntu command with a path-independent benchmark fingerprint.

## Experiments

`run-bench.sh` builds and runs one Linux container. The prototype performs 32 correctness assertions, 81 performance
rows (three protocols, 1/8/32 producers, 64/1024/8192-byte payloads, three repetitions) and five pressure/resource
rows.

Each performance message contains its producer timestamp. The server records producer-to-accept p50/p95/p99, delivery,
wall throughput, process CPU and Go runtime memory. Correctness and pressure results use portable invariants; timing
values are observations.

## Results

The macOS Docker Desktop/LinuxKit aarch64 reference run passed every portable assertion:

- correctness: 32/32;
- performance delivery: 81/81 rows delivered all messages;
- pressure/resource cases: 5/5;
- tested maximum payload: 65,536 bytes;
- fingerprint: `585b91f1a73f1359953cc313af2e1f3f7ff1f9757ee00086056c812357a78bca`.

Key pressure result:

| Protocol      | Case                      | Input | Delivered | Failed/blocked |     Duration |
|---------------|---------------------------|------:|----------:|---------------:|-------------:|
| stream-line   | slow reader               | 2,000 |     2,000 |              0 |   256.502 ms |
| stream-framed | slow reader               | 2,000 |     2,000 |              0 |   254.300 ms |
| datagram-line | slow reader               | 2,000 |     1,258 |            742 | 2,023.599 ms |
| stream-line   | FD exhaustion (limit 128) |   256 |        63 |            193 |     4.008 ms |
| datagram-line | no accepted FD per sender |   256 |       256 |              0 |     1.900 ms |

Stream transports applied backpressure and preserved all messages. Datagram preserved message boundaries and avoided
per-connection server FDs, but lost/rejected 37.1% of the slow-reader workload after the bounded producer deadline.

Selected mean-of-three throughput observations:

| Protocol      | Producers | Payload | Messages/s |       p95 |
|---------------|----------:|--------:|-----------:|----------:|
| stream-line   |         1 |    64 B |    565,585 |    279 µs |
| stream-framed |         1 |    64 B |    388,776 |    304 µs |
| datagram-line |         1 |    64 B |    303,019 |     22 µs |
| stream-line   |        32 |    64 B |  1,497,470 | 15.009 ms |
| stream-framed |        32 |    64 B |  1,239,313 |  9.376 ms |
| datagram-line |        32 |    64 B |    383,943 |  0.429 ms |
| stream-line   |        32 | 8,192 B |     58,645 | 32.262 ms |
| stream-framed |        32 | 8,192 B |     27,814 | 52.898 ms |
| datagram-line |        32 | 8,192 B |     50,653 |  3.187 ms |

These measurements rank this prototype implementation only. Datagram's low accept latency does not compensate for its
loss behavior under pressure.

## Hypothesis Evaluation

Partially confirmed on macOS/LinuxKit aarch64:

- Unix stream provides reliable delivery and observable backpressure in the tested range.
- Datagram reduces server-side FD use and can be fast, but does not provide the required delivery behavior under a
  bounded slow-reader workload.
- The prototype does not show a performance reason to require binary length framing. Line framing was simpler and
  faster in most tested stream cells. Length framing remains useful only if newline-containing or binary payloads
  become a requirement.
- Startup and restart require bounded client retry/reconnect; an existing connection is not an epoch-independent
  channel.

Ubuntu confirmation is intentionally pending. Therefore the investigation status remains `in progress` and no ADR is
created yet.

## Acceptable Values

Provisional values pending the matching-fingerprint Ubuntu run:

- primary transport: Unix stream socket;
- protocol: custom versioned text, newline-delimited unless binary payloads are required;
- socket mode: `0660`, with container user/group ownership configured explicitly;
- maximum payload: 65,536 bytes tested; provisional production default should be lower (8 KiB) until realistic metric
  cardinality payloads are measured;
- producer write deadline: required and finite; exact default deferred to workload behavior;
- client startup/reconnect: bounded retry with backoff; never assume the socket exists before workload start;
- malformed/partial/oversized message: reject only that message or connection, retain last valid metric state;
- stream connection budget: enforce a configurable limit below `RLIMIT_NOFILE` and reserve descriptors for HTTP,
  logging and runtime needs;
- datagram: not acceptable as the reliable primary channel; possible best-effort compatibility adapter only with
  explicit loss accounting.

## Decision Output

Provisional direction only: Unix stream plus a versioned line protocol. No ADR until the same fingerprint is run on
Ubuntu and results are compared.

## Running the Prototype

Run the complete matrix on macOS or Ubuntu:

```bash
./research/INV-007/run-bench.sh
```

Inspect the latest evidence:

```bash
cat research/INV-007/latest-results.txt
cat "$(cat research/INV-007/latest-results.txt)/summary.tsv"
cat "$(cat research/INV-007/latest-results.txt)/environment.tsv"
cat "$(cat research/INV-007/latest-results.txt)/correctness.tsv"
cat "$(cat research/INV-007/latest-results.txt)/performance.tsv"
cat "$(cat research/INV-007/latest-results.txt)/pressure.tsv"
```

Increase repetitions or change the tested payload ceiling:

```bash
INV007_REPETITIONS=30 INV007_MAX_PAYLOAD=65536 ./research/INV-007/run-bench.sh
```

The Ubuntu handoff uses exactly the same command. Compare `benchmark_code_fingerprint_sha256`, not repository HEAD,
image ID or architecture-specific digest. Results are created inside the container and extracted with `docker cp`; no
host bind mount is used.

Manual execution:

```bash
docker build -t metricshell-inv007:prototype research/INV-007/prototype
docker create --name inv007-manual metricshell-inv007:prototype \
  --output-dir=/results --max-payload=65536 --repetitions=3
docker start -a inv007-manual
docker cp inv007-manual:/results/. ./inv007-manual-results
docker rm inv007-manual
```

## Prototype Limits

- Research harness, not production MetricShell or a production metric parser.
- Current evidence is macOS Docker Desktop/LinuxKit aarch64 only; Ubuntu and native non-LinuxKit Linux are unverified.
- Client and server run in one process, so process CPU/RSS are combined and scheduler effects differ from separate
  application processes.
- Producer-to-accept latency is not end-to-end metric exposition latency.
- The benchmark uses blocking Unix datagrams; nonblocking sends would expose loss earlier, not remove it.
- FD exhaustion is induced with `RLIMIT_NOFILE=128` in one process. It validates bounded failure, not a production
  connection limit.
- The synthetic payload has minimal parsing and does not model cardinality, registry contention or Prometheus encoding.
- Socket owner/group transitions and adversarial cross-user access require a multi-user container test.

## Additional Benchmarks

Covered now:

- all protocols with one, 8 and 32 producers;
- 64 B, 1 KiB and 8 KiB messages;
- three repetitions and p50/p95/p99;
- burst throughput, CPU and runtime memory observations;
- slow reader/backpressure with a bounded producer deadline;
- disconnect mid-line and mid-frame;
- malformed, exact-maximum and oversized messages;
- startup retry and restart reconnect;
- socket mode `0660`;
- stream FD exhaustion and datagram no-per-producer-accepted-FD behavior;
- environment metadata, image identity and path-independent benchmark fingerprint.

Recommended higher-confidence follow-ups:

- run 30–100 repetitions on an idle dedicated host and report confidence intervals;
- separate producer and server processes and sample cgroup CPU/RSS for 5–15 minutes;
- test 64/128/256 KiB ceilings and realistic metric/cardinality payloads;
- sweep slow-reader delay, producer write deadline, connection count and accept backlog;
- test connection churn, half-close, stalled partial frames and shutdown grace with active clients;
- fuzz the selected parser and run race detection outside the minimal image;
- repeat on native Linux, Ubuntu matching fingerprint and Kubernetes `emptyDir`;
- add multi-user UID/GID permission and unauthorized-client tests;
- compare a real StatsD client/grammar only if a compatibility adapter remains desirable.
