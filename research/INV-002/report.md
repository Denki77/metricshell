# INV-002 Report — Workload Lifecycle and Exit Semantics

Status: validation in progress

Run date: 2026-07-21

Docker server: 29.4.3

Docker platform: linux/aarch64

Reference runs: `results/20260721T200345Z`, `results/20260721T200406Z-extended`

Summaries: `results/20260721T200345Z/summary.tsv`, `results/20260721T200406Z-extended/latency-stats.tsv`

## Goal

Validate the INV-002 assumption: one MetricShell process should execute exactly one workload, keep one unambiguous
metric-state epoch, optionally expose the final state for a bounded period and leave retry policy to Docker, Compose or
Kubernetes.

## Prototype

The prototype is located in `research/INV-002`.

- `prototype/cmd/metricshell` — Go supervisor with single-run and deliberately comparative internal-restart modes.
- `prototype/cmd/workload` — configurable workload for exit codes, counter increments, duration and memory pressure.
- `prototype/Dockerfile` — image definition for `metricshell-inv002:prototype`.
- `compose.yml` — Compose-owned `on-failure:2` restart scenario.
- `run-bench.sh` — core lifecycle and metric-state runner.
- `run-extended-bench.sh` — repetitions, Compose, scrapes, faults, post-exit grid and restart storms.
- `results/<timestamp>` — raw logs and TSV evidence.

The prototype starts and waits for a workload, preserves its outcome, compares counter reset/preservation across
internal retries, exposes metrics during a bounded post-exit interval and supports workload memory pressure for OOM
fault injection.

## Run Commands

Core and extended runs:

```bash
./research/INV-002/run-bench.sh
./research/INV-002/run-extended-bench.sh
```

Latest results:

```bash
cat research/INV-002/latest-results.txt
cat research/INV-002/latest-extended-results.txt
cat "$(cat research/INV-002/latest-results.txt)/summary.tsv"
cat "$(cat research/INV-002/latest-extended-results.txt)/latency-stats.tsv"
cat "$(cat research/INV-002/latest-extended-results.txt)/faults.tsv"
cat "$(cat research/INV-002/latest-extended-results.txt)/post-exit-grid.tsv"
cat "$(cat research/INV-002/latest-extended-results.txt)/restart-storm.tsv"
cat "$(cat research/INV-002/latest-extended-results.txt)/coverage.tsv"
```

Manual single execution:

```bash
docker build -t metricshell-inv002:prototype research/INV-002/prototype
docker run --rm metricshell-inv002:prototype \
  --policy=single --post-exit=2s -- \
  /usr/local/bin/workload --state=/tmp/attempt --exits=17 --increments=5
```

Manual Compose-owned restart:

```bash
docker compose -p inv002 -f research/INV-002/compose.yml up --abort-on-container-exit
docker compose -p inv002 -f research/INV-002/compose.yml down -v
```

Set `INV002_REPEAT_COUNT=100` to increase the default 30 repetitions.

## Run Environments

| Environment              | Date       | Docker server | Platform         | Architecture | Result set                          | Evidence                                                                 | Notes                                               |
|--------------------------|------------|--------------:|------------------|--------------|-------------------------------------|--------------------------------------------------------------------------|-----------------------------------------------------|
| Docker Desktop on macOS  | 2026-07-21 |        29.4.3 | LinuxKit 6.12.76 | aarch64      | `results/20260721T200345Z`          | [summary.tsv](results/20260721T200345Z/summary.tsv)                      | Path-independent core run, 8/8 pass.                |
| Docker Desktop on macOS  | 2026-07-21 |        29.4.3 | LinuxKit 6.12.76 | aarch64      | `results/20260721T200406Z-extended` | [latency-stats.tsv](results/20260721T200406Z-extended/latency-stats.tsv) | Path-independent extended run; all assertions pass. |
| Docker Desktop on Ubuntu | -          |        27.4.0 | LinuxKit 6.10.14 | x86_64       | -                                   | -                                                                        | -                                                   |

Ubuntu/LinuxKit x86_64 is available, but it must be rerun with the corrected benchmark. A Kubernetes context exists,
but its cluster rejected authentication because the OAuth refresh token is invalid. Gaps are recorded in
[`coverage.tsv`](results/20260721T200406Z-extended/coverage.tsv).

Both corrected macOS reference runs used benchmark fingerprint
`a8042a6b12b8d659f701125584223373a934186d45fdeae2e64f89f6362c05f2`. The fingerprint covers `prototype/`,
`run-bench.sh`, `run-extended-bench.sh` and `compose.yml`; it is the required identity for the pending Ubuntu run.
`benchmark_scope_diff_clean=false` records that the benchmark changes were not committed at collection time, while
`benchmark_scope_untracked_count=0` confirms that no benchmark-scope file was omitted as untracked. The submitted
Ubuntu run used the previous fingerprint `7515b5d373b0228cdd58860c64498a16f96b24c98e7e0d8319e3477c05792075`
and therefore cannot be compared to the corrected reference run.

## Results

### Core lifecycle cases

| Case                      | Expected exit | Actual exit |  Executions | Final counter | Result |
|---------------------------|--------------:|------------:|------------:|--------------:|--------|
| `single_success`          |             0 |           0 |           1 |             5 | pass   |
| `single_failure`          |            17 |          17 |           1 |             5 | pass   |
| `internal_reset`          |             0 |           0 |           2 |             2 | pass   |
| `internal_preserve`       |             0 |           0 |           2 |             7 | pass   |
| `restart_limit`           |            17 |          17 |           3 |             3 | pass   |
| `start_failure`           |           127 |         127 |           0 |             0 | pass   |
| `external_docker_restart` |             0 |           0 | 2 processes |             2 | pass   |
| `post_exit_endpoint`      |            17 |          17 |           1 |             5 | pass   |

`post_exit_endpoint` is derived from six assertions rather than hardcoded summary values: `docker wait` returned `17`,
the log contained exactly one `attempt_started` and one `lifecycle_finalized`, the finalized counter was `5`, `/metrics`
served the final state during the window, and MetricShell measured `2005.003 ms` internally for the configured `2s`
window. See `assertions.tsv`, `post-exit.log` and `post-exit-duration.tsv` in the core result set.

### Repetitions and latency

| Count | Passed |        p50 |        p95 |        p99 |        Min |        Max |
|------:|-------:|-----------:|-----------:|-----------:|-----------:|-----------:|
|    30 |     30 | 285.287 ms | 392.271 ms | 410.242 ms | 222.859 ms | 660.293 ms |

This is end-to-end `docker run` latency, including container startup and a 1 ms workload, not only supervisor overhead.

### Signal-to-exit latency

The corrected runner measures `signal_forwarded -> workload exit observed` inside MetricShell, excluding Docker CLI
transport latency:

| Environment            | Count | Passed |      p50 |      p95 |      p99 |      Min |      Max |
|------------------------|------:|-------:|---------:|---------:|---------:|---------:|---------:|
| macOS/LinuxKit aarch64 |    30 |     30 | 0.594 ms | 1.194 ms | 2.010 ms | 0.330 ms | 2.194 ms |
| Ubuntu/LinuxKit x86_64 |     — |      — |        — |        — |        — |        — |        — |

Ubuntu signal-to-exit statistics are not present in the submitted result set because that run predates this measurement.
They must be collected by rerunning `run-extended-bench.sh` with fingerprint
`a8042a6b12b8d659f701125584223373a934186d45fdeae2e64f89f6362c05f2`.

### Runtime restart behavior

Compose `on-failure:2` performed one restart: the first MetricShell lifecycle exited `17`, the second exited `0`, and
the container finished with restart count `1`. Both MetricShell processes executed their workload once.

| Scrape samples | HTTP 200 | Gaps | Counter 5 | Counter 2 | Completed lifecycles |
|---------------:|---------:|-----:|----------:|----------:|---------------------:|
|            150 |        5 |  145 |         5 |         0 |                    2 |

Both externally restarted processes completed, but the second metric epoch was not observed and the endpoint had a long
gap. Runtime restart provides lifecycle ownership, not final-scrape or endpoint-continuity guarantees.

The scenario passed because Docker completed two distinct MetricShell lifecycles with outcomes `17` and `0`, each
MetricShell process executed the workload exactly once, and Docker reported restart count `1`.

Scrape visibility is recorded as an observation rather than a portable pass criterion. In this reference run, a restart
gap was observed, the first epoch was scraped and the second epoch was not. See `restart-scrape-assertions.tsv` for
portable invariants and `restart-scrape-observations.tsv` for timing-dependent evidence.

### Fault injection

| Case                    | Expected exit | Actual exit | Result |
|-------------------------|--------------:|------------:|--------|
| TERM                    |           143 |         143 | pass   |
| KILL                    |           137 |         137 | pass   |
| Container OOM at 32 MiB |           137 |         137 | pass   |

Docker marked the container `OOMKilled=true` and it exited `137`. MetricShell emitted `attempt_exited` and
`lifecycle_finalized` with `137` before termination in this run; the evidence does not identify the exact kernel OOM
victim independently of Docker's container-level state.

### Post-exit grid

| Configured | MetricShell internal elapsed | Host lifecycle | Exit | Result |
|-----------:|-----------------------------:|---------------:|-----:|--------|
|        0 s |                     0.001 ms |     508.319 ms |   17 | pass   |
|        1 s |                  1007.201 ms |    1517.810 ms |   17 | pass   |
|        2 s |                  2004.732 ms |    2444.365 ms |   17 | pass   |
|        5 s |                  5008.071 ms |    5511.979 ms |   17 | pass   |
|       10 s |                 10004.263 ms |   10544.773 ms |   17 | pass   |
|       30 s |                 30023.144 ms |   30540.120 ms |   17 | pass   |

Every duration preserved exit `17`. Pass/fail uses the locale-independent in-process timer; host Docker lifecycle time
is retained only as an operational observation.

### Restart storms

| Internal attempts |    Duration | Exit |  Log size | Result |
|------------------:|------------:|-----:|----------:|--------|
|                10 |  267.706 ms |   17 |   1,783 B | pass   |
|               100 |  295.248 ms |   17 |  17,914 B | pass   |
|              1000 | 1137.820 ms |   17 | 175,585 B | pass   |

The rejected internal-retry model scales supervisor work and log volume linearly. A 1000-attempt storm generated about
175.6 KB in 1.138 seconds without backoff. External Docker storms were not extended to 100/1000 because the runtime
applies
intentional restart delay/backoff.

## Hypothesis Evaluation

### MetricShell should run exactly one workload execution

Supported in the current environment. Single execution preserved success, failure, TERM, KILL, container-OOM and
start-failure outcomes. All 30 repeated
single-execution cases passed.

### Restart policy should remain outside MetricShell

Supported. Docker and Compose performed externally visible restarts while every MetricShell process retained one
workload execution. Internal retries add eligibility, limits, backoff, cancellation and outcome aggregation.

### One MetricShell lifetime should equal one metric-state epoch

Supported. Internal reset made a counter decrease `5 -> 2`; internal preservation merged executions `5 -> 7`.
External restart produced separate epochs `5 -> 2`, matching normal process-counter reset semantics.

### Bounded post-exit availability is feasible

Supported for `0, 1, 2, 5, 10, 30s`. This does not select a default. The restart scrape result proves that bounded
availability alone does not guarantee collection of final state.

## Acceptable Values and Policies

Recommended baseline:

- workload executions per MetricShell process: exactly `1`;
- internal retry count: `0`; no production retry flag;
- metric state: one epoch per MetricShell/container process;
- post-exit duration: `0` or explicit finite value; tested functional range `0–30s`;
- post-exit default: defer to INV-003 and scrape-probability measurements;
- normal workload exit `N`: preserve `N`;
- signal exit `S`: `128 + S`; TERM `143` and KILL `137` verified; the container-OOM scenario exited `137`;
- workload start failure: distinct code, prototype uses `127`;
- MetricShell internal failure: distinct MetricShell-owned code;
- retry limit/backoff: Docker/Compose/Kubernetes deployment policy;
- monitoring contract: do not promise endpoint continuity or a final scrape across restart.

## Prototype Limits

- Research code, not production MetricShell.
- Results cover Docker Desktop/LinuxKit aarch64 only; native Linux and x86_64 remain unverified.
- Kubernetes could not run because the configured context has an invalid OAuth refresh token.
- Docker daemon restart was not attempted because it could disrupt unrelated containers.
- Disk-full injection was excluded as unsafe and outside focused INV-002 scope.
- Network partition was inapplicable because the prototype has no remote dependency.
- The synthetic line-based counter does not benchmark production ingestion throughput.
- Restart storms finished too quickly for reliable point-in-time CPU/RSS sampling; continuous cgroup sampling is future
  work if resource limits enter this decision.
- The nominal 50 ms scrape loop had an actual roughly 90–100 ms cadence due to launching curl each time; raw timestamps
  are retained.

## Additional Benchmarking

| Benchmark item                          | Status                                                | Evidence                                                           |
|-----------------------------------------|-------------------------------------------------------|--------------------------------------------------------------------|
| 30 repetitions with p50/p95/p99         | Covered                                               | `repetitions.tsv`, `latency-stats.tsv`                             |
| Docker external restart                 | Covered                                               | core `external_docker_restart.log`                                 |
| Compose external restart                | Covered                                               | `runtime-restarts.tsv`, `compose-restart.log`                      |
| Scrapes across restart boundary         | Covered; timing observations separate from invariants | `restart-scrape-assertions.tsv`, `restart-scrape-observations.tsv` |
| TERM, KILL and container-OOM injection  | Covered                                               | `faults.tsv`, `container-oom-assertions.tsv`                       |
| Post-exit 0, 1, 2, 5, 10 and 30 seconds | Covered                                               | `post-exit-grid.tsv`                                               |
| Storms at 10, 100 and 1000 attempts     | Covered for rejected internal model                   | `restart-storm.tsv`, `storm-*.log`                                 |
| Environment metadata                    | Covered                                               | `environment.tsv`                                                  |
| Native Linux arm64/x86_64               | Not run                                               | Current environment is LinuxKit/aarch64                            |
| Kubernetes Job/OnFailure                | Not run                                               | Cluster OAuth token invalid                                        |
| Docker daemon restart                   | Not run                                               | Unsafe without separate authorization                              |
| Disk-full injection                     | Not run                                               | Unsafe and outside INV-002 scope                                   |
| Network partition                       | Not applicable                                        | No remote dependency                                               |

Machine-readable statuses and blockers are in
[`coverage.tsv`](results/20260721T200406Z-extended/coverage.tsv).

## Conclusion

The INV-002 assumption is confirmed in Docker Desktop/LinuxKit aarch64, but investigation status remains
`validation in progress` until the corrected fingerprint is confirmed on Ubuntu/LinuxKit x86_64:

- MetricShell executes exactly one workload per process.
- One MetricShell process owns one application metric-state epoch.
- Internal retries are rejected because reset/merge semantics are ambiguous and supervisor scope expands.
- Docker and Compose can own restarts without changing single-run semantics.
- A bounded post-exit interval from 0 to 30 seconds works functionally; production sizing belongs to INV-003.
- Runtime restart does not guarantee scrape continuity or collection of the next/final epoch.

Recommended direction: keep retry and backoff in deployment configuration and design final-state delivery without
assuming Prometheus will successfully scrape during a restart boundary.
