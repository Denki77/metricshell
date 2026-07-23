# INV-006 Report — File-Based Ingestion

Status: in progress  
Run date: 2026-07-23  
Docker server: 29.4.3  
Docker platform: LinuxKit 6.12.76, linux/aarch64  
Reference run: `results/20260723T160155Z`
Summary: `results/20260723T160155Z/summary.tsv`

## Goal

Validate whether directory-level inotify plus low-frequency reconciliation safely detects a container-local complete-
state metrics file with lower latency and acceptable idle overhead than polling alone.

## Prototype

The prototype is located in `research/INV-006`.

- `prototype/cmd/inv006-bench` — Linux watcher, producer, correctness cases and measurements.
- `prototype/Dockerfile` — multi-stage Linux image.
- `run-bench.sh` — identical macOS/Ubuntu matrix runner and fingerprint capture.
- `results/<timestamp>` — raw per-case TSV files plus aggregate evidence.

The implementation uses directory watches, reads complete file snapshots, validates before replacing the last state,
reinstalls invalidated watches and reconciles on a fixed interval.

The runner uses no host bind mount. Each case writes evidence into its container, and the host extracts it with
`docker cp`; this avoids Docker Desktop file-sharing and Ubuntu daemon mount-policy differences.

## Run Commands

```bash
./research/INV-006/run-bench.sh
cat "$(cat research/INV-006/latest-results.txt)/summary.tsv"
cat "$(cat research/INV-006/latest-results.txt)/environment.tsv"
```

Ubuntu uses the same command. A valid cross-environment comparison requires equal
`benchmark_code_fingerprint_sha256`.

## Run Environments

| Environment             | Date       | Docker | Kernel           | Architecture | Result set                 | Fingerprint                                                        |
|-------------------------|------------|--------|------------------|--------------|----------------------------|--------------------------------------------------------------------|
| Docker Desktop on macOS | 2026-07-23 | 29.4.3 | LinuxKit 6.12.76 | aarch64      | `results/20260723T160155Z` | `4238d6e1be961e2d864ccfc95c763c1c9a03365c74d9c868e38f1b0d96eb1580` |
| Ubuntu                  | pending    | —      | —                | —            | —                          | must match                                                         |

The fingerprint directly hashes the prototype and runner content and is the portable identity used for the Ubuntu
comparison.

## Results

### Correctness

All 162 assertions passed across layer, tmpfs and named volume for polling, inotify and hybrid candidates. Covered
cases: initial file, initially absent file, atomic rename, 100 replacements, crashed writer temporary file, invalid
input, deletion, directory recreation and MetricShell restart.

Invalid, partial and deleted inputs retained the last valid state. Directory recreation and restart converged to the
current valid file.

### Detection latency

Mean of each repetition's percentile for 4 KiB files:

| Filesystem | Strategy    |       p50 |        p95 |        p99 | Missed |
|------------|-------------|----------:|-----------:|-----------:|-------:|
| Layer      | poll 10 ms  |  9.974 ms |  11.670 ms |  11.926 ms |      0 |
| Layer      | poll 100 ms | 99.915 ms | 103.560 ms | 103.859 ms |      0 |
| Layer      | inotify     |  0.231 ms |   0.879 ms |   1.016 ms |      0 |
| Layer      | hybrid 1 s  |  0.265 ms |   1.001 ms |   2.614 ms |      0 |
| tmpfs      | poll 10 ms  | 10.006 ms |  11.570 ms |  11.936 ms |      0 |
| tmpfs      | poll 100 ms | 99.931 ms | 103.353 ms | 104.155 ms |      0 |
| tmpfs      | inotify     |  0.109 ms |   0.604 ms |   0.758 ms |      0 |
| tmpfs      | hybrid 1 s  |  0.109 ms |   0.707 ms |   0.775 ms |      0 |
| Volume     | poll 10 ms  |  9.980 ms |  11.786 ms |  11.958 ms |      0 |
| Volume     | poll 100 ms | 99.745 ms | 103.679 ms | 104.830 ms |      0 |
| Volume     | inotify     |  0.169 ms |   0.608 ms |   0.747 ms |      0 |
| Volume     | hybrid 1 s  |  0.171 ms |   0.646 ms |   0.846 ms |      0 |

The event path determines normal latency; changing hybrid reconciliation from 100 ms to 1 s did not materially slow
normal event detection.

Across the full paced matrix, one volume/inotify-only 1 MiB repetition observed 4/5 updates and timed out on one
intermediate version. Every hybrid paced row had zero misses. This is consistent with the complete-state contract:
intermediate versions may be coalesced, while reconciliation must converge to the latest version.

### Idle overhead

| Filesystem | Poll 10 ms | Poll 100 ms | Poll 1 s | inotify | Hybrid 100 ms | Hybrid 1 s |
|------------|-----------:|------------:|---------:|--------:|--------------:|-----------:|
| Layer      |     2.969% |      0.480% |   0.060% |  0.189% |        0.427% |     0.272% |
| tmpfs      |     3.014% |      0.481% |   0.358% |  0.231% |        0.433% |     0.418% |
| Volume     |     2.593% |      0.361% |   0.087% |  0.245% |        0.553% |     0.286% |

These five-second process CPU samples show the expected tradeoff: slow polling is cheapest but slow to detect, while
hybrid 1 s retains event latency with less than 0.42% measured idle CPU in this run.

### Burst and overflow recovery

All 12 10,000-update bursts converged to the final complete-state version. During forced reader pauses and 20,000
atomic replacements, all six cases recovered the final version. The kernel emitted `IN_Q_OVERFLOW` in four cases;
the other two completed without a reported overflow.

An overflow event is not itself a failure. It is a signal that event history is incomplete and an immediate full
reconciliation is required. Conversely, absence of a real overflow is not a failure: occurrence depends on kernel
scheduling and filesystem speed. `INV006_REQUIRE_REAL_OVERFLOW=1` enables a stricter optional local stress criterion.
The portable assertion is final-state convergence in every pressure case.

### Controlled lost-event boundary

All six A/B assertions passed:

| Filesystem | inotify-only after dropped event | Hybrid after dropped event |
|------------|----------------------------------|----------------------------|
| Layer      | did not recover                  | recovered                  |
| tmpfs      | did not recover                  | recovered                  |
| Volume     | did not recover                  | recovered                  |

The watcher deliberately consumed one update event without reconciling, then received no further event. Inotify-only
had no mechanism to discover the current snapshot. Hybrid discovered it on its 250 ms reconciliation timer. This is
direct experimental evidence for periodic reconciliation, not only failure-model analysis.

### File size

The matrix covered 128 B, 4 KiB and 1 MiB. Event detection stayed fast, but 1 MiB active tests became hashing/I/O bound
and routinely consumed roughly one core while producing and validating updates. This establishes feasibility, not a
production size or series limit.

## Hypothesis Evaluation

### Directory-level inotify provides fast normal detection

Supported. The 4 KiB hybrid/inotify p95 range was 0.604–1.001 ms across tested filesystems, versus roughly 11.6–11.8 ms
for 10 ms polling and 103.4–103.7 ms for 100 ms polling.

### Reconciliation is required for correctness

Supported by the controlled A/B test, failure-model analysis and recovery tests. Events can be coalesced or lost,
watches can be invalidated, and queue overflow was observed. Inotify-only did not recover after controlled loss;
hybrid recovered on its interval.

### A 1 s reconciliation interval is acceptable

Provisionally supported. It bounds recovery after a missed event to approximately one interval while normal updates
still use the event path. A 100 ms interval increased idle reads/CPU without improving normal latency materially.

### Container-local filesystems behave consistently

Supported for writable layer, tmpfs and named volume in the current LinuxKit run. Host bind mounts remain excluded.

## Acceptable Values and Policies

- directory watch with immediate reconcile after relevant event;
- periodic reconciliation: default `1s`, tested acceptable range `100ms–1s`;
- complete-state file, not an append/event log;
- same-directory temporary file plus atomic rename;
- last-valid-state retention on invalid content, deletion or partial temporary file;
- immediate watch reinstall and reconcile on invalidation or `IN_Q_OVERFLOW`;
- tested file range `128 B–1 MiB`, with production maximum deferred;
- intermediate versions may be coalesced; final-state convergence is required;
- primary storage: writable layer or tmpfs; named volume informational; bind mounts unsupported.

## Prototype Limits

- Only macOS/LinuxKit aarch64 is recorded; Ubuntu remains pending.
- Synthetic format and SHA-256 validation do not model the production parser or series registry.
- Active CPU includes writer, filesystem and hashing work in one process.
- Percentiles use 30 updates per repetition for small files and five per repetition for 1 MiB files.
- Five-second idle samples are sensitive to scheduler noise.
- Overflow occurrence varies with filesystem speed and scheduling.
- The test uses atomic rename but not fsync; it does not prove power-loss durability.
- No host bind mount, native Linux, Kubernetes emptyDir or network filesystem was tested.

## Additional Benchmarking

| Benchmark item                         | Status                         | Evidence                                         |
|----------------------------------------|--------------------------------|--------------------------------------------------|
| Required correctness cases             | Covered, 162/162 pass          | `correctness.tsv`                                |
| Layer, tmpfs, named volume             | Covered                        | all aggregate TSV files                          |
| Poll 10/100/1000 ms                    | Covered                        | `correctness.tsv`, `performance.tsv`, `idle.tsv` |
| Hybrid reconcile 100/1000 ms           | Covered                        | same                                             |
| 128 B, 4 KiB, 1 MiB                    | Covered                        | `performance.tsv`                                |
| Three repetitions and percentiles      | Covered                        | `performance.tsv`                                |
| 10,000-update bursts                   | Covered, 12/12 converge        | `burst.tsv`                                      |
| Controlled lost event A/B              | Covered, 6/6 pass              | `lost-event.tsv`                                 |
| Real queue overflow                    | Covered, observed in 4/6 cases | `overflow.tsv`                                   |
| Final recovery after overflow pressure | Covered, 6/6 converge          | `overflow.tsv`                                   |
| Idle CPU                               | Covered                        | `idle.tsv`                                       |
| Environment and fingerprint            | Covered                        | `environment.tsv`                                |
| Ubuntu matching-fingerprint run        | Pending                        | run unchanged command                            |
| Native Linux / Kubernetes emptyDir     | Not run                        | follow-up                                        |
| Real parser and cardinality limits     | Not run                        | format not selected                              |
| fsync crash durability                 | Not run                        | separate persistence requirement                 |

## Conclusion

The assumption is provisionally confirmed on Docker Desktop/LinuxKit aarch64: directory-level inotify plus 1 s
reconciliation is the best tested balance of normal detection latency, idle overhead and recoverability.

The investigation remains in progress. Completion requires running the unchanged fingerprint on Ubuntu, comparing
portable correctness and recovery outcomes, recording environment-specific timing separately and then preparing the
ADR.
