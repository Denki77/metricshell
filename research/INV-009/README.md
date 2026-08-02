# INV-009 — Shared Memory and mmap Adapter

Status: completed

macOS reference run: `results/20260802T162350Z`

Ubuntu/LinuxKit reference run: `results/20260802T163406Z`

Report: [report.md](report.md)

## Question

Can shared memory or mmap provide a useful high-performance adapter without making clients unsafe or platform-specific?

## Context and Hypothesis

ADR-004 fixes the application contract: every transport carries one complete, conflict-free Prometheus-compatible
application snapshot. MetricShell validates the whole candidate and atomically replaces the last valid snapshot. It does
not sum snapshots, merge series, accept instrumentation operations, retain per-producer contributions or aggregate
independent registries.

INV-009 therefore compares only transport mechanics for the same complete snapshot. “Concurrent publishers” in the
benchmark are concurrent submissions of complete workload-owned candidate snapshots. MetricShell assigns their accepted
candidates one linear replacement order; they are not independent metric owners and their values are never combined.

The hypothesis is that shared memory can improve publication speed for small snapshots, but the gain will narrow as
snapshot size grows and will not justify the binary ABI, synchronization, recovery, portability and PHP-client cost.

## Evidence Required

- identical complete-snapshot semantics for file mmap, `/dev/shm` mmap and framed Unix socket;
- 1 and 8 concurrent complete-snapshot publishers with 128 B, 4 KiB and 64 KiB candidates;
- separate publisher commit cost and end-to-end acceptance metrics;
- identical live consumer path: read, validate, atomically install, acknowledge and count;
- accepted-snapshot throughput, end-to-end p50/p95/p99, CPU per acceptance and RSS;
- atomic replacement without cross-snapshot merge;
- overflow, process crash during an uncommitted candidate, schema mismatch, reopen and memory limits;
- private permissions, client complexity, portability and a matching-fingerprint Ubuntu repeat.

## Current Result

Both macOS/LinuxKit aarch64 and Ubuntu/LinuxKit x86_64 passed 78/78 in-container assertions and both external OOM
assertions. In each environment, all 16,740 complete candidates were read, validated, atomically installed and
acknowledged by live consumers. Parallel active-state
readers performed 158,526 and 84,677 reads respectively and observed zero malformed/torn or unknown states.

Producer publication cost and end-to-end acceptance are reported separately. The former measures only local commit or
socket write/enqueue and is not adapter throughput. The comparable end-to-end results have no universal winner: tmpfs
mmap led 1 publisher/128 B (`388,400 accepted/s`) and 8 publishers/128 B (`474,496/s`), file and tmpfs mmap were
nearly tied at 8 publishers/4 KiB (`173,940/s` and `174,395/s`), and Unix socket led 8 publishers/64 KiB (`28,513/s`).
The hypothesis is only
partially supported; the measured gain is shape-dependent and does not justify a default native binary ABI.

Both runs recorded fingerprint `41c97e834fba84c771980e2563a9817509add8fd419cde367b34cde5f2e77d07`.
The investigation is completed and [ADR-009](../../docs/06-architecture/adr/ADR-009.md) records the decision.

## Admissible Values

- application operation: complete candidate validation followed by atomic replacement of the active snapshot;
- no summing, merging, operation replay, producer identity or cross-producer aggregation;
- snapshot sizes tested: `128 B`, `4 KiB`, `64 KiB`;
- concurrent candidate publishers tested: `1–8`, with one linear acceptance order;
- mapping file mode: `0600`;
- shared-memory transport metadata may use a versioned header and per-slot commit marker, but these are transport
  mechanics and must not become application metric semantics;
- overwrite is permitted only if explicitly observable; an overwritten candidate is not an accepted application state;
- unknown mapping versions are rejected; crash recovery exposes only fully committed candidate bytes;
- mapped bytes must be bounded below both container memory and `/dev/shm` backing limits;
- shared memory is not selected as the default ingestion adapter.

## Running the Prototype

From the repository root on macOS or Ubuntu:

```bash
./research/INV-009/run-bench.sh
```

Inspect the latest result:

```bash
latest="$(cat research/INV-009/latest-results.txt)"
cat "$latest/assertions.tsv"
cat "$latest/external-assertions.tsv"
cat "$latest/publisher-performance.tsv"
cat "$latest/acceptance-performance.tsv"
cat "$latest/resources.tsv"
cat "$latest/environment.tsv"
```

The runner builds one Linux image, runs the full matrix without host bind mounts, copies `/out` with `docker cp`, and
runs the cgroup memory-limit case separately. Ubuntu uses exactly the same command. Compare
`benchmark_code_fingerprint_sha256`; image IDs and repository HEAD are context, not benchmark identity.

Manual run:

```bash
docker build --pull=false -t metricshell-inv009:prototype research/INV-009/prototype
docker run --rm metricshell-inv009:prototype /out
```

## Prototype Limits

- Research code, not a production parser, registry or formally verified multi-process lock-free queue.
- Candidate bodies are syntactically Prometheus-compatible complete snapshots of fixed byte sizes. The benchmark tests
  transport copy/publication mechanics, not full production structural validation or scrape concurrency.
- Concurrent publishers submit complete candidates. They do not model independent producer registries and MetricShell
  never aggregates their values.
- The ring uses Go atomics. A multi-language production ABI requires specified alignment and memory ordering plus
  independent-process conformance tests.
- `publisher_publications_per_second` is the inverse mean local commit/write duration; it is publisher cost, not wall
  throughput and not accepted-snapshot throughput.
- The mmap consumer uses an in-process commit notification as a stand-in for a production doorbell. It still reads and
  validates mmap bytes and acknowledges through the mapped accepted-sequence marker; eventfd/futex/polling choice is
  unmeasured transport work.
- PHP has no portable built-in mmap/atomic-ring API. FFI or an extension would be required, increasing adoption cost and
  memory-safety risk without changing the complete-snapshot contract.
- Both evidence environments use LinuxKit. Cross-architecture behavior is confirmed inside LinuxKit, not on native
  non-LinuxKit Linux, containerd, CRI-O or Kubernetes.
- CPU/RSS and Docker Desktop timings are architectural observations, not SLOs.

## Additional Benchmarks

| Benchmark                                                   | Status                                                                    |
|-------------------------------------------------------------|---------------------------------------------------------------------------|
| complete Prometheus-compatible snapshots only               | covered                                                                   |
| atomic replacement; no cross-snapshot merge                 | covered                                                                   |
| file mmap, `/dev/shm` mmap and framed Unix socket           | covered                                                                   |
| 128 B, 4 KiB and 64 KiB snapshots                           | covered                                                                   |
| 1 and 8 concurrent complete-candidate publishers            | covered                                                                   |
| publisher commit p50/p95/p99 and publication cost           | covered separately; not called accepted throughput                        |
| live consumer read, validation, install and sequence ack    | covered for every transport and shape                                     |
| accepted throughput and end-to-end p50/p95/p99              | covered in `acceptance-performance.tsv`                                   |
| CPU per accepted snapshot and peak RSS                      | covered in `resources.tsv`                                                |
| exact acknowledged/accepted snapshot counts                 | covered                                                                   |
| concurrent active-state reader; no torn/mixed state         | covered: zero bad reads                                                   |
| ring overwrite accounting                                   | covered: 936 overwritten candidates from 1,000 publications into 64 slots |
| real writer crash before candidate commit                   | covered: exit 99, candidate remained uncommitted                          |
| mapping schema mismatch and reopen                          | covered                                                                   |
| private mapping permissions                                 | covered: `0600`                                                           |
| cgroup memory-limit enforcement                             | covered: exit 137 and Docker `OOMKilled=true`                             |
| matching-fingerprint Ubuntu run                             | covered; identical fingerprint and 80/80 assertions pass                  |
| PHP FFI/extension client                                    | not portable; requirement itself is adoption-cost evidence                |
| full Prometheus structural validation and concurrent scrape | owned by INV-010/implementation tests, not duplicated here                |
| soak/wraparound under a production ABI                      | recommended only after transport selection                                |
| perf/eBPF contention profiling and 30+ repetitions          | recommended in the Ubuntu environment with pinned CPUs                    |

## Better Follow-up Benchmarking

For deeper transport work on native non-LinuxKit Linux, run at least 30
repetitions per shape, pin candidate publishers and consumer to CPUs, report median and dispersion, sample cgroup v2
CPU/memory, and use `perf stat` for cycles, instructions, cache misses and context switches. Any long wraparound soak
must
continue to publish complete snapshots and verify atomic replacement; it must not introduce summation or partial-update
semantics.

## Decision Output

- Prototype: `prototype/`
- Runner: `run-bench.sh`
- Raw evidence: `results/20260802T162350Z/`, `results/20260802T163406Z/`
- Detailed analysis: [report.md](report.md)
- ADR: [ADR-009](../../docs/06-architecture/adr/ADR-009.md)
