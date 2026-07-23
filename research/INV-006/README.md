# INV-006 — File-Based Ingestion

Status: completed
Reference runs: `results/20260723T160155Z`, `results/20260723T161216Z`
Report: [report.md](report.md)

## Question

How should MetricShell detect and read file updates safely inside a Linux container?

## Context

The metrics file is container-local. The primary filesystems are the writable container layer and container-local
tmpfs; a Docker named volume is measured for information. Host bind mounts are deliberately outside the supported
contract.

## Candidates

- Polling: stat/read on a fixed interval.
- Directory-level inotify.
- Directory-level inotify plus periodic reconciliation.
- Producer-triggered reload: not selected because it adds another transport and does not remove reconciliation needs.

## Initial Hypothesis

Directory-level inotify plus low-frequency reconciliation provides low idle overhead, event-speed detection and
recovery from missed events, queue overflow and replaced watches.

## Evidence Required

- Existing, initially absent, atomically replaced, invalid and deleted files.
- Writer crash before rename, repeated replacement, directory recreation and process restart.
- Real `IN_Q_OVERFLOW` plus recovery to the latest complete state.
- Update latency, idle CPU, high-frequency updates, file-size scaling and missed final states.
- Writable layer, tmpfs and named-volume comparison.
- A path-independent fingerprint for an identical macOS and Ubuntu invocation.

## Experiments

`run-bench.sh` runs 162 correctness assertions, six controlled lost-event A/B cases, 135 paced performance rows,
12 bursts of 10,000 updates, six overflow-pressure/recovery cases of 20,000 updates and 18 idle measurements. The
paced matrix covers 128 B, 4 KiB and 1 MiB files and three repetitions.

## Results

| Environment           | Docker | Kernel           | Architecture | Result set                 | Fingerprint                                                        |
|-----------------------|-------:|------------------|--------------|----------------------------|--------------------------------------------------------------------|
| macOS Docker Desktop  | 29.4.3 | LinuxKit 6.12.76 | aarch64      | `results/20260723T160155Z` | `4238d6e1be961e2d864ccfc95c763c1c9a03365c74d9c868e38f1b0d96eb1580` |
| Ubuntu Docker Desktop | 27.4.0 | LinuxKit 6.10.14 | x86_64       | `results/20260723T161216Z` | `4238d6e1be961e2d864ccfc95c763c1c9a03365c74d9c868e38f1b0d96eb1580` |

The macOS/LinuxKit aarch64 and Ubuntu/LinuxKit x86_64 reference runs used the same fingerprint and passed every
portable assertion:

- correctness: 162/162 in each environment;
- controlled lost-event A/B: 6/6 in each environment;
- 10,000-update burst convergence: 12/12 in each environment;
- overflow-pressure final-state recovery: 6/6 in each environment.

Each environment had one inotify-only paced row with one missed intermediate version; hybrid paced tests had no misses.
The kernel reported a real queue overflow in four of six pressure cases in each environment. Intermediate-version and
overflow counts are observations, not portable pass criteria.

For 4 KiB files, mean-of-three p95 detection latency was:

| Environment     | Filesystem | Poll 10 ms | Poll 100 ms |  inotify | Hybrid 100 ms | Hybrid 1 s |
|-----------------|------------|-----------:|------------:|---------:|--------------:|-----------:|
| macOS/LinuxKit  | Layer      |  11.670 ms |  103.560 ms | 0.879 ms |      0.825 ms |   1.001 ms |
| macOS/LinuxKit  | tmpfs      |  11.570 ms |  103.353 ms | 0.604 ms |      0.489 ms |   0.707 ms |
| macOS/LinuxKit  | Volume     |  11.786 ms |  103.679 ms | 0.608 ms |      0.755 ms |   0.646 ms |
| Ubuntu/LinuxKit | Layer      |  10.810 ms |  100.778 ms | 1.202 ms |      0.988 ms |   0.785 ms |
| Ubuntu/LinuxKit | tmpfs      |  10.709 ms |  100.914 ms | 0.896 ms |      0.835 ms |   1.083 ms |
| Ubuntu/LinuxKit | Volume     |  10.793 ms |  100.889 ms | 0.927 ms |      0.848 ms |   0.843 ms |

Five-second hybrid/1s idle CPU was 0.272–0.418% on macOS/LinuxKit and 0.402–0.438% on Ubuntu/LinuxKit. Polling every
10 ms measured 2.593–3.014% and 4.496–5.085%, respectively. These are short container-process measurements, not
production resource promises.

INV-006 has no signal-to-exit metric because the file-ingestion prototype does not supervise or signal a workload.
Its latency statistic is producer timestamp to observed file update, reported above as p50/p95/p99 detection latency.

## Hypothesis Evaluation

Confirmed in matching-fingerprint macOS/LinuxKit aarch64 and Ubuntu/LinuxKit x86_64 runs. Directory-level inotify gives
event-speed detection. In controlled lost-event tests, inotify-only failed to recover without a subsequent event on all
three filesystems in both environments, while hybrid recovered on its reconciliation interval.

## Acceptable Values

- Watch the directory, not the file.
- Treat the file as complete registry state; intermediate versions may be coalesced.
- Require producer writes to a same-directory temporary file followed by atomic rename.
- Parse and validate before swapping state; invalid, partial and deleted inputs retain the last valid state.
- Reconcile every `1s` by default; tested recovery range is `100ms–1s`.
- Supported tested file range is `128 B–1 MiB`; this does not establish a production maximum.
- A successful reload must be observable by content/version change, not only mtime.
- On `IN_Q_OVERFLOW`, watch invalidation or directory recreation, reinstall watches and immediately reconcile.

## Decision Output

Accepted ADR input: use directory-level inotify plus 1 s reconciliation for container-local files; require
same-directory atomic rename and last-valid-state retention. See
[ADR-006](../../docs/06-architecture/adr/ADR-006.md).

## Running the Prototype

Run the complete matrix on either macOS or Ubuntu:

```bash
./research/INV-006/run-bench.sh
```

Inspect the latest evidence:

```bash
cat research/INV-006/latest-results.txt
cat "$(cat research/INV-006/latest-results.txt)/summary.tsv"
cat "$(cat research/INV-006/latest-results.txt)/environment.tsv"
cat "$(cat research/INV-006/latest-results.txt)/correctness.tsv"
cat "$(cat research/INV-006/latest-results.txt)/performance.tsv"
cat "$(cat research/INV-006/latest-results.txt)/burst.tsv"
cat "$(cat research/INV-006/latest-results.txt)/overflow.tsv"
cat "$(cat research/INV-006/latest-results.txt)/lost-event.tsv"
cat "$(cat research/INV-006/latest-results.txt)/idle.tsv"
```

The same command is the Ubuntu handoff. Compare `benchmark_code_fingerprint_sha256`, not repository HEAD or image ID:
the fingerprint is architecture-independent, while a locally built image ID normally is not.

Manual hybrid run:

```bash
docker build -t metricshell-inv006:prototype research/INV-006/prototype
docker run --name inv006-manual --tmpfs /data:rw,size=256m,mode=0755 \
  metricshell-inv006:prototype --mode=performance --strategy=hybrid \
  --interval=1s --updates=30 --file-bytes=4096 --output=/results/inv006-manual.tsv
docker cp inv006-manual:/results/inv006-manual.tsv .
docker rm inv006-manual
```

## Prototype Limits

- Research code, not production MetricShell.
- Evidence covers Docker Desktop/LinuxKit on aarch64 and x86_64. Both container environments use LinuxKit; native
  non-LinuxKit Linux remains unverified.
- Bind mounts and their host filesystem semantics are intentionally excluded.
- The runner does not require host bind mounts: evidence is written inside each container and extracted with
  `docker cp`.
- The file format is a synthetic complete-state record; parser and series-cardinality costs are not represented.
- CPU percentages include the synthetic producer and hashing during active tests.
- Queue overflow is kernel scheduling dependent. Every pressure case must recover the final state, but real overflow
  occurrence is recorded only as an observation by default.
- Set `INV006_REQUIRE_REAL_OVERFLOW=1` for a local stress run that requires at least one real overflow per inotify
  strategy.
- No crash-consistency claim is made for storage hardware; the producer does not fsync the file and directory.

## Additional Benchmarks

Covered:

- all required correctness cases on layer, tmpfs and named volume;
- polling at 10 ms, 100 ms and 1 s;
- hybrid reconciliation at 100 ms and 1 s;
- 128 B, 4 KiB and 1 MiB files;
- three paced repetitions with p50/p95/p99/max, CPU and read counts;
- 10,000-update bursts for final-state convergence;
- real `IN_Q_OVERFLOW` injection with 20,000 updates and final-state recovery;
- controlled event loss proving that inotify-only has no bounded recovery and hybrid does;
- five-second idle CPU measurements;
- benchmark fingerprint, Docker/kernel/platform metadata and image identity.

Recommended follow-ups:

- use 30–100 repetitions on a dedicated idle runner for confidence intervals;
- add realistic parser/cardinality payloads after the file format is selected;
- use cgroup CPU and RSS sampling for long 5–15 minute steady-state runs;
- test larger files and series counts to choose production limits;
- repeat on native Linux and under Kubernetes emptyDir (disk and Memory);
- test fsync/fdatasync durability separately if crash persistence becomes a requirement.
