# INV-009 Report — Shared Memory and mmap Adapter

**Status:** completed  
**Run date:** 2026-08-02  
**Docker servers:** 29.6.2 (macOS/LinuxKit), 27.4.0 (Ubuntu/LinuxKit)  
**Docker platforms:** linux/aarch64, linux/x86_64  
**Reference runs:** `results/20260802T162350Z`, `results/20260802T163406Z`  
**Fingerprint:** `41c97e834fba84c771980e2563a9817509add8fd419cde367b34cde5f2e77d07`

## Goal

Determine whether shared memory provides a sufficiently broad transport-performance advantage to justify a native
binary ABI with synchronization and memory-safety risks instead of the framed Unix-socket adapter, while preserving
ADR-004 complete-snapshot semantics exactly.

## Scope Correction from ADR-004

MetricShell accepts one complete, conflict-free application snapshot per publication. It structurally validates the
whole candidate and atomically replaces the last valid snapshot exposed through the Prometheus endpoint. It does not
sum snapshots, merge series, apply `increment`/`set`/`observe`, retain per-producer contributions, reconcile sequences,
or aggregate independently owned registries.

The benchmark therefore treats every payload as a complete Prometheus-compatible candidate snapshot. Its concurrent
publishers model concurrent submissions of complete workload-owned candidates. Successful candidates have one linear
replacement order. Publisher identity exists only to make candidates distinguishable in the research workload; it is
not application protocol state and no publisher values are combined.

The ring sequence and commit markers are transport implementation mechanics used to detect fully published bytes and
overwrites. They are not metric-state semantics and do not authorize partial snapshots or operation replay.

## Prototype

- `prototype/cmd/inv009-bench` — complete-snapshot correctness, failure and performance matrix.
- `prototype/Dockerfile` — reproducible Linux image.
- `run-bench.sh` — identical macOS/Ubuntu runner, evidence extraction, fingerprint and cgroup memory test.
- `results/<timestamp>` — assertions, performance observations, environment metadata and raw logs.

Each candidate body is a syntactically Prometheus-compatible complete snapshot padded to the selected byte size. The
mmap candidates use a versioned mapping header, fixed slots, atomic allocation and per-slot commit sequence. The socket
baseline uses length-delimited frames and exact reads. All transports carry the same complete-candidate bytes.

## Run Commands

```bash
./research/INV-009/run-bench.sh
latest="$(cat research/INV-009/latest-results.txt)"
cat "$latest/publisher-performance.tsv"
cat "$latest/acceptance-performance.tsv"
cat "$latest/resources.tsv"
cat "$latest/assertions.tsv"
cat "$latest/external-assertions.tsv"
cat "$latest/environment.tsv"
```

The same command and benchmark fingerprint are used on macOS and Ubuntu.

## Run Environments

| Environment                       | Date       | Docker | Architecture | Result                     | Status                |
|-----------------------------------|------------|-------:|--------------|----------------------------|-----------------------|
| Docker Desktop on macOS/LinuxKit  | 2026-08-02 | 29.6.2 | aarch64      | `results/20260802T162350Z` | 80/80 assertions pass |
| Docker Desktop on Ubuntu/LinuxKit | 2026-08-02 | 27.4.0 | x86_64       | `results/20260802T163406Z` | 80/80 assertions pass |

Both runs used benchmark fingerprint
`41c97e834fba84c771980e2563a9817509add8fd419cde367b34cde5f2e77d07`. The repository SHA and image ID are retained as
context only.

## Results

### Producer publication cost

`publisher-performance.tsv` measures only reserve/copy/commit for mmap and write/enqueue for the socket. It does not
measure acceptance and is not used as adapter throughput. For example, mmap publisher p50 values of `125–166 ns` at
1 publisher/128 B are local commit cost, while end-to-end p50 is `542–1,083 ns`.
`publisher_publications_per_second` is the inverse mean local commit/write duration, not wall-clock accepted throughput.

### End-to-end acceptance

Every transport uses a live consumer that reads the complete bytes, calls the same validation function, atomically
replaces an immutable active-state reference, acknowledges the exact publication sequence and increments accepted count.
Elapsed time ends only after all acknowledgements.

| Publishers | Snapshot | mmap file accepted/s | mmap tmpfs accepted/s | socket accepted/s | Fastest     |
|-----------:|---------:|---------------------:|----------------------:|------------------:|-------------|
|          1 |    128 B |              293,535 |               388,400 |            68,321 | mmap tmpfs  |
|          8 |    128 B |              302,658 |               474,496 |           164,690 | mmap tmpfs  |
|          1 |    4 KiB |              121,243 |               188,694 |            39,838 | mmap tmpfs  |
|          8 |    4 KiB |              173,940 |               174,395 |           124,292 | mmap tmpfs  |
|          1 |   64 KiB |               15,528 |                19,222 |            14,536 | mmap tmpfs  |
|          8 |   64 KiB |               21,240 |                24,282 |            28,513 | Unix socket |

Ubuntu/LinuxKit x86_64 end-to-end results:

| Publishers | Snapshot | mmap file accepted/s | mmap tmpfs accepted/s | socket accepted/s | Fastest     |
|-----------:|---------:|---------------------:|----------------------:|------------------:|-------------|
|          1 |    128 B |              118,574 |               111,120 |            20,864 | mmap file   |
|          8 |    128 B |              352,135 |               340,843 |           126,318 | mmap file   |
|          1 |    4 KiB |               27,917 |                30,792 |             6,506 | mmap tmpfs  |
|          8 |    4 KiB |               24,576 |                28,138 |            23,513 | mmap tmpfs  |
|          1 |   64 KiB |                3,903 |                 3,642 |             2,503 | mmap file   |
|          8 |   64 KiB |                2,289 |                 2,868 |             3,985 | Unix socket |

All 16,740 candidates in each environment were validated, installed and acknowledged with zero errors. No transport
wins consistently. Parallel readers performed 158,526 active-state reads on aarch64 and 84,677 on x86_64 and observed
only exact known old/new snapshots. All 78 in-container and both external assertions passed in both environments.

### Correctness and failure behavior

- Active state is an immutable snapshot behind an atomic pointer. Concurrent readers observed only complete old or new
  snapshots and zero torn/mixed states in every transport/shape.
- A 64-slot ring reported 936 overwritten candidates after 1,000 publications. Overwrite accounting is transport
  evidence, not acceptance of lost candidates.
- A real child writer exited 99 after modifying payload bytes without publishing the slot commit marker. Reopen treated
  that candidate as uncommitted.
- Mapping version 99 was rejected.
- Reopen recovered transport committed sequence 100. This tests mapping recovery only; ADR-004 does not require a new
  MetricShell execution to reconcile independently surviving producers.
- Mapping permissions were `0600`.
- A 128 MiB allocation under a 32 MiB cgroup limit exited 137 and Docker reported `OOMKilled=true`.

### Resources

`resources.tsv` reports CPU per accepted snapshot. Representative 8-publisher/4 KiB values on aarch64 were `17,267 ns`
for file mmap, `16,138 ns` for tmpfs mmap and `26,787 ns` for the socket; on x86_64 they were `133,576 ns`, `122,461
ns` and `232,871 ns`. Peak RSS was `21,576 KiB` and `19,828 KiB`, respectively. These are environment-specific
observations, not acceptance limits.

## Hypothesis Evaluation

### Shared memory offers the best raw transport performance

Not supported as a general adapter claim. Once identical consumer work and acknowledgement are included, the winner
depends on snapshot size and concurrency. mmap wins the tested 128 B and 4 KiB shapes, while the socket is fastest at
8 publishers / 64 KiB.

### The gain does not justify general client complexity

Supported. Correctness requires a binary mapping schema, cross-process atomics, alignment and memory-order
rules, per-slot commit visibility, overwrite behavior, capacity enforcement and crash handling. None of these replace
ADR-004 validation or atomic active-snapshot replacement; they are additional transport complexity.

### PHP adoption is materially worse than socket adoption

Supported by interface analysis, not by changing the metric contract. PHP has portable stream/socket APIs but no
portable built-in mmap plus atomic-ring API. FFI or an extension is required and must reproduce the exact binary ABI and
memory-order contract. Benchmarking one extension would not remove that deployment and memory-safety cost.

### Portability is weaker than the socket adapter

Supported within the tested container scope. File mmap exists broadly, but atomics, endianness, alignment, mapping
lifetime, tmpfs sizing, descriptor inheritance and SIGBUS behavior form a platform-sensitive ABI.

## Evaluation Against Criteria

| Criterion                      | Shared memory/mmap                            | Framed Unix socket                    |
|--------------------------------|-----------------------------------------------|---------------------------------------|
| complete snapshot contract     | possible, explicitly tested                   | natural framed message                |
| accepted snapshot throughput   | wins some shapes; no consistent lead          | wins some shapes; simpler ABI         |
| atomic application replacement | still required after candidate commit         | still required after frame validation |
| cross-snapshot aggregation     | forbidden                                     | forbidden                             |
| crash/torn candidate detection | explicit commit protocol required             | truncated frame rejected              |
| overflow/backpressure          | shared capacity and overwrite policy required | socket buffering/backpressure         |
| schema/versioning              | binary ABI plus application format            | framing plus application format       |
| PHP client                     | FFI/extension                                 | built-in stream APIs                  |
| portability/debugging          | lower                                         | higher                                |

## Acceptable Values and Policies

- Preserve ADR-004 exactly: one complete candidate, whole-candidate validation, atomic replacement, no summation or
  merge, and no per-producer state in MetricShell.
- Do not select shared memory as the default adapter from current evidence.
- If retained as an expert opt-in transport: use a versioned little-endian header, aligned atomics, per-slot commit
  marker, observable overwrite/backpressure policy, `0600` permissions and a hard mapping-size limit.
- A successful transport commit is not sufficient application acceptance; the complete candidate must still pass
  structural validation and atomically replace the last valid snapshot.
- Unknown mapping versions are rejected. A crash exposes only committed complete candidate bytes.
- Tested envelope: complete snapshots of 128 B–64 KiB and 1–8 concurrent candidate publishers.
- Anonymous inherited mappings are restricted to processes receiving an inherited descriptor and are not a general
  endpoint for unrelated post-exec clients.
- `/dev/shm` size must be configured explicitly; touching pages beyond backing capacity may produce SIGBUS.

## Prototype Limits

- Both evidence environments use LinuxKit. The matching aarch64/x86_64 runs do not verify native non-LinuxKit Linux,
  containerd, CRI-O or Kubernetes behavior.
- The prototype tests complete-candidate transport publication, not production Prometheus parsing, all ADR-004
  structural conflicts, active-state scrape concurrency or final-state freezing. Those remain required implementation
  tests and INV-010/INV-011 concerns.
- Concurrent publishers are distinguishable complete candidates, not independent metric owners.
- The Go ring is not a formally verified multi-process queue. Independent-process and multi-language conformance remain
  untested.
- An in-process commit notification wakes the mmap consumer as a stand-in for a doorbell. Candidate bytes and
  acknowledgements remain in the mapping, but production eventfd/futex/polling behavior is not benchmarked.
- The default Docker `/dev/shm` capacity bounds the matrix. An exploratory oversized mapping produced SIGBUS and was
  discarded rather than reported as a performance result.
- Timing, CPU and RSS are architectural comparisons, not SLOs.

## Additional Benchmarking

| Item                                               | Status           | Evidence/Reason                                         |
|----------------------------------------------------|------------------|---------------------------------------------------------|
| complete snapshots; no summation/merge             | covered          | replacement assertions and fixed complete candidates    |
| backing stores, sizes and concurrency              | covered          | 18 comparable rows                                      |
| isolated publisher commit cost                     | covered          | `publisher-performance.tsv`; not acceptance             |
| live consumer and exact sequence acknowledgement   | covered          | `acceptance-performance.tsv`                            |
| end-to-end p50/p95/p99 and accepted throughput     | covered          | `acceptance-performance.tsv`                            |
| CPU per accepted snapshot and RSS                  | covered          | `resources.tsv`                                         |
| atomic active-state reader                         | covered          | 158,526 reads, zero bad states                          |
| acknowledged/accepted count correctness            | covered          | `assertions.tsv`                                        |
| overwrite, crash/torn candidate, schema and reopen | covered          | `assertions.tsv`                                        |
| permissions and cgroup OOM                         | covered          | exit 137 and `OOMKilled=true`                           |
| matching-fingerprint Ubuntu repeat                 | covered          | identical fingerprint; 80/80 assertions pass            |
| PHP FFI/extension                                  | adoption blocker | no portable extension-free atomic mmap API              |
| complete Prometheus structural validation          | not duplicated   | ADR-004 implementation requirement; INV-010 integration |
| concurrent scrape during replacement               | not duplicated   | ADR-004/INV-010 implementation requirement              |
| native perf/eBPF and 30+ repetitions               | recommended      | native non-LinuxKit Linux with pinned CPUs              |
| production wraparound soak                         | deferred         | requires selected ABI and backpressure policy           |

## Conclusion

The initial hypothesis is partially confirmed and the investigation is completed. Shared memory reduces
publisher commit cost, but comparable end-to-end acceptance has no consistent winner. It adds a native binary ABI with
synchronization and memory-safety risks without reducing any ADR-004 responsibility.

The final decision is no default shared-memory adapter. Retain it only as a possible expert opt-in if production
profiling finds a small-snapshot transport bottleneck that batching or the socket path cannot solve. ADR-009 records
this decision.

## Decision Output

- Prototype: `prototype/`
- Runner: `run-bench.sh`
- Raw evidence: `results/20260802T162350Z/`, `results/20260802T163406Z/`
- Summary: complete snapshots only; no aggregation; end-to-end mmap advantage is inconsistent.
- ADR: [ADR-009](../../docs/06-architecture/adr/ADR-009.md)
