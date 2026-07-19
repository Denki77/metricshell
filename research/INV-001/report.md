# INV-001 Report — Process and PID 1 Model

Status: completed  
Run dates: 2026-07-17, 2026-07-18  
Docker Servers: 29.4.3, 27.4.0  
Docker platforms: linux/aarch64, linux/x86_64  
Reference runs: `results/20260717T192610Z`, `results/20260718T085124Z`  
Summaries: `results/20260717T192610Z/summary.tsv`, `results/20260718T085124Z/summary.tsv`

## Goal

Validate the INV-001 assumption: MetricShell can correctly own the workload lifecycle when running as PID 1 or under
Docker init/Tini if it explicitly implements workload startup, signal forwarding, exit status observation, descendant
reaping and post-workload survival.

## Prototype

The prototype is located in `research/INV-001`.

- `prototype/cmd/metricshell` — Go supervisor prototype.
- `prototype/cmd/workload-trap` — Go workload that records TERM/INT/HUP without an intermediate shell.
- `prototype/workloads/*.sh` — shell workload scenarios for wrappers, child spawning, double fork and exit status
  checks.
- `prototype/Dockerfile` — container image definition for `metricshell-inv001:prototype`.
- `run-bench.sh` — image build and scenario runner.
- `results/<timestamp>` — Local result directory contains raw logs, inspect JSON and exit files.
  The committed evidence set contains aggregated structured events and TSV summaries.

The prototype supports:

- starting the workload as a child process;
- optionally creating a dedicated process group for the workload;
- forwarding TERM/INT/HUP/QUIT to the child PID or process group;
- optionally enabling `PR_SET_CHILD_SUBREAPER`;
- force-killing a workload after a configurable shutdown grace period;
- waiting for the workload and preserving its exit code;
- reaping adopted descendants after workload exit;
- keeping the `/metrics` HTTP endpoint available during a bounded post-exit window.

## Run Commands

Full run:

```bash
./research/INV-001/run-bench.sh
```

Latest result:

```bash
cat research/INV-001/latest-results.txt
cat "$(cat research/INV-001/latest-results.txt)/summary.tsv"
cat "$(cat research/INV-001/latest-results.txt)/assertions.tsv"
cat "$(cat research/INV-001/latest-results.txt)/environment.tsv"
cat "$(cat research/INV-001/latest-results.txt)/signal-delivery.tsv"
cat "$(cat research/INV-001/latest-results.txt)/signal-to-exit-latency-stats.tsv"
cat "$(cat research/INV-001/latest-results.txt)/resources.tsv"
cat "$(cat research/INV-001/latest-results.txt)/scrapes.tsv"
cat "$(cat research/INV-001/latest-results.txt)/zombies.tsv"
```

Manual PID 1 run without Docker init:

```bash
docker build -t metricshell-inv001:prototype research/INV-001/prototype
docker run --rm metricshell-inv001:prototype --http=:9090 -- /usr/local/bin/workload-trap
```

Manual run under Docker init/Tini:

```bash
docker run --rm --init metricshell-inv001:prototype --http=:9090 -- /usr/local/bin/workload-trap
```

Manual run with a process group:

```bash
docker run --rm metricshell-inv001:prototype --http=:9090 --process-group -- /usr/local/bin/workload-trap
```

Manual run under Docker init/Tini with `PR_SET_CHILD_SUBREAPER`:

```bash
docker run --rm --init metricshell-inv001:prototype --http=:9090 --subreaper -- /prototype/workloads/double-fork.sh
```

## Run Environments

| Environment                                | Date       | Docker server | Platform | Architecture | Result set                 | Summary                                             | Notes                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           |
|--------------------------------------------|------------|---------------|----------|--------------|----------------------------|-----------------------------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Docker Desktop on macOS                    | 2026-07-17 | 29.4.3        | linux    | aarch64      | `results/20260717T192610Z` | [summary.tsv](results/20260717T192610Z/summary.tsv) | Current reference run; benchmark fingerprint `35dc9c63a0a9f6dedf56a1c6c80b582919d5961b8f233c49ef1aed55652b71fb`; see also [assertions.tsv](results/20260717T192610Z/assertions.tsv), [environment.tsv](results/20260717T192610Z/environment.tsv), [signal-delivery.tsv](results/20260717T192610Z/signal-delivery.tsv), [events.jsonl](results/20260717T192610Z/events.jsonl), [signal-to-exit-latency-stats.tsv](results/20260717T192610Z/signal-to-exit-latency-stats.tsv), [resources.tsv](results/20260717T192610Z/resources.tsv), [scrapes.tsv](results/20260717T192610Z/scrapes.tsv), [zombies.tsv](results/20260717T192610Z/zombies.tsv). |
| Docker Desktop on Ubuntu / LinuxKit x86_64 | 2026-07-18 | 27.4.0        | linux    | x86_64       | `results/20260718T085124Z` | [summary.tsv](results/20260718T085124Z/summary.tsv) | Current reference run; benchmark fingerprint `35dc9c63a0a9f6dedf56a1c6c80b582919d5961b8f233c49ef1aed55652b71fb`; see also [assertions.tsv](results/20260718T085124Z/assertions.tsv), [environment.tsv](results/20260718T085124Z/environment.tsv), [signal-delivery.tsv](results/20260718T085124Z/signal-delivery.tsv), [events.jsonl](results/20260718T085124Z/events.jsonl), [signal-to-exit-latency-stats.tsv](results/20260718T085124Z/signal-to-exit-latency-stats.tsv), [resources.tsv](results/20260718T085124Z/resources.tsv), [scrapes.tsv](results/20260718T085124Z/scrapes.tsv), [zombies.tsv](results/20260718T085124Z/zombies.tsv). |

Both reference runs used the same benchmark fingerprint:
`35dc9c63a0a9f6dedf56a1c6c80b582919d5961b8f233c49ef1aed55652b71fb`. This fingerprint is calculated only from
`research/INV-001/prototype` and `research/INV-001/run-bench.sh`; documentation-only commits do not change the
benchmark identity.

## Cross-Environment Confirmation

| Metric                                    |   macOS/LinuxKit aarch64 |   Ubuntu/LinuxKit x86_64 |
|-------------------------------------------|-------------------------:|-------------------------:|
| Passed summary cases                      |                       52 |                       52 |
| Passed assertions                         |                      141 |                      141 |
| Explicit signal-delivery observations     |                       42 |                       42 |
| Aggregated structured events              |                      597 |                      598 |
| Successful post-exit HTTP scrapes         |                        5 |                        5 |
| Zero-zombie samples before container exit |                       25 |                       27 |
| `repeat_signal_direct_pg` p50 / p95 / p99 | 0.434 / 0.581 / 0.625 ms | 0.468 / 1.894 / 2.155 ms |
| Forced shutdown signal-to-exit latency    |               506.627 ms |               502.317 ms |

## Results

| Case                              | Model                                                     | Expected | Actual | Result |
|-----------------------------------|-----------------------------------------------------------|---------:|-------:|--------|
| `signal_direct_pid1`              | MetricShell PID 1, direct child                           |        0 |      0 | pass   |
| `signal_direct_pg`                | MetricShell PID 1, direct child, process group            |        0 |      0 | pass   |
| `signal_direct_pg_init`           | Docker init/Tini, direct child, process group             |        0 |      0 | pass   |
| `signal_direct_pg_init_subreaper` | Docker init/Tini + subreaper, direct child, process group |        0 |      0 | pass   |
| `signal_shell_script_pg`          | shell script workload, process group                      |      143 |    143 | pass   |
| `signal_shell_no_pg`              | `/bin/sh -c`, no process group                            |      143 |    143 | pass   |
| `signal_shell_pg`                 | `/bin/sh -c`, process group                               |      143 |    143 | pass   |
| `signal_shell_pg_init`            | Docker init/Tini + `/bin/sh -c`, process group            |      143 |    143 | pass   |
| `signal_bash_pg`                  | `/bin/bash -c`, process group                             |      143 |    143 | pass   |
| `reap_short_children_200`         | 200 short-lived children                                  |        0 |      0 | pass   |
| `child_churn_1000`                | 1k short-lived children                                   |        0 |      0 | pass   |
| `child_churn_10000`               | 10k short-lived children                                  |        0 |      0 | pass   |
| `double_fork_pid1_no_subreaper`   | MetricShell PID 1, daemonized grandchild                  |        0 |      0 | pass   |
| `double_fork_no_subreaper_init`   | Docker init/Tini, no subreaper                            |        0 |      0 | pass   |
| `double_fork_subreaper_init`      | Docker init/Tini + subreaper                              |        0 |      0 | pass   |
| `exit_zero`                       | workload exits 0                                          |        0 |      0 | pass   |
| `exit_17`                         | workload exits 17                                         |       17 |     17 | pass   |
| `sigkill`                         | workload receives KILL                                    |      137 |    137 | pass   |
| `shutdown_grace_forced_kill`      | TERM ignored, forced KILL after 500 ms                    |      137 |    137 | pass   |
| `start_failure`                   | workload start failure                                    |      127 |    127 | pass   |
| `internal_failure`                | MetricShell internal failure                              |       70 |     70 | pass   |
| `post_exit_survival`              | post-exit metrics with original exit 17                   |       17 |     17 | pass   |

Key observations:

- `summary.tsv` now reports a case as `pass` only when all case assertions pass. Assertions include exit code,
  startup readiness, signal receipt, process-tree behavior and post-exit HTTP/metric checks where applicable.
- All case-level assertions passed in both reference environments.
- Direct child signal forwarding works both by child PID and by process group.
- The 30-run signal-to-exit latency benchmark for `repeat_signal_direct_pg` measured p50 `0.434 ms`, p95 `0.581 ms`,
  p99 `0.625 ms`, min `0.326 ms`, max `0.943 ms` on macOS/LinuxKit aarch64.
- The same 30-run benchmark on Ubuntu/LinuxKit x86_64 measured p50 `0.468 ms`, p95 `1.894 ms`, p99 `2.155 ms`,
  min `0.373 ms`, max `7.326 ms`.
- Signal receipt is explicitly recorded in `signal-delivery.tsv`, not inferred from exit code alone. Direct workload,
  shell parent, grandchild and stubborn-workload signal observations were all present in the expected cases.
- `assertions.tsv` explicitly checks descendant signal receipt, double-fork reaping expectations and post-exit
  `/metrics` availability.
- `events.jsonl` contains 597 aggregated structured events in the macOS/LinuxKit run and 598 in the Ubuntu/LinuxKit
  run, keyed by benchmark case.
- `environment.tsv` records `benchmark_code_fingerprint_sha256` for the benchmark scope
  (`research/INV-001/prototype` and `research/INV-001/run-bench.sh`). The `repository_head_sha` is retained as context
  only; documentation-only commits should be compared through the benchmark fingerprint instead.
- Process-group signaling reaches shell descendants, but shell-form commands can still resolve to signal exit `143`.
  This was observed for `/bin/sh -c`, `/bin/bash -c` and a shell-script workload. This is acceptable only if documented;
  it is not equivalent to a clean workload-controlled exit.
- When MetricShell is PID 1, a daemonized double-fork descendant is reparented to MetricShell and was reaped with
  original exit `23`.
- When Docker init/Tini is PID 1 and MetricShell is not a subreaper, the daemonized descendant is not visible to
  MetricShell after workload exit; Docker init/Tini owns the reaping.
- When Docker init/Tini is PID 1 and MetricShell enables `PR_SET_CHILD_SUBREAPER`, MetricShell reaps the daemonized
  descendant and observes exit `23`.
- The shutdown grace scenario force-killed a TERM-ignoring workload after `506.627 ms` on macOS/LinuxKit and
  `502.317 ms` on Ubuntu/LinuxKit, preserving signal exit `137` in both environments.
- Child churn completed for 200, 1k and 10k short-lived children. The 10k zombie scan observed `0` zombies in all
  successful samples before the container exited.
- MetricShell RSS/HWM samples stayed in the `8296-8380 KiB` range on macOS/LinuxKit and `8728-10740 KiB` on
  Ubuntu/LinuxKit across idle post-exit, CPU workload and child churn scenarios.
- Post-workload survival preserved the workload exit code. External host scrapes during the configured post-exit window
  returned HTTP `200` and `metricshell_workload_exit_code 17` in all five samples in both environments.

## Hypothesis Evaluation

### MetricShell must own workload lifecycle even when Tini is present

Supported by matching benchmark-fingerprint runs on Docker Desktop macOS/LinuxKit aarch64 and Docker Desktop
Ubuntu/LinuxKit x86_64.

Tini/Docker init can sit above MetricShell and reap processes that MetricShell does not adopt, but it does not preserve
MetricShell-level knowledge of daemonized descendants. MetricShell still needs to start workload, forward signals, wait
for workload exit, preserve exit status and own post-exit behavior.

### Tini may reduce PID 1 edge cases but adds another binary and signal layer

Supported by matching benchmark-fingerprint runs on Docker Desktop macOS/LinuxKit aarch64 and Docker Desktop
Ubuntu/LinuxKit x86_64.

With Docker init/Tini, MetricShell runs as PID 7 instead of PID 1 in this environment. Tini can act as the namespace
init, but MetricShell loses orphan descendant ownership unless it uses `PR_SET_CHILD_SUBREAPER`. This adds an
operational mode and another process layer without removing supervisor responsibilities from MetricShell.

### A correct single-binary implementation may be simpler operationally

Supported by matching benchmark-fingerprint runs on Docker Desktop macOS/LinuxKit aarch64 and Docker Desktop
Ubuntu/LinuxKit x86_64.

MetricShell as PID 1 successfully handled direct signals, exit codes, KILL semantics, start failure, internal failure,
post-exit metrics and daemonized descendant reaping in this prototype. The implementation still needs production-grade
shutdown budgets and tests, but no hard blocker appeared for a single-binary PID 1 model.

### Process-group and orphaned-descendant behavior are the main correctness risks

Supported by matching benchmark-fingerprint runs on Docker Desktop macOS/LinuxKit aarch64 and Docker Desktop
Ubuntu/LinuxKit x86_64.

The most important negative result is shell behavior: process-group TERM reaches children but may produce exit `143`
from `/bin/sh -c` or shell scripts with foreground children. The most important Tini result is orphan adoption: under
Docker init/Tini, MetricShell must use subreaper mode if it needs visibility or cleanup responsibility for daemonized
descendants.

## Acceptable Values and Policies

Recommended baseline for MetricShell:

- Default runtime model: MetricShell runs as PID 1.
- Workload process model: start exactly one workload as direct child.
- Process group: create a dedicated process group for workload when group-wide signal delivery is required.
- Signal forwarding: forward TERM/INT/HUP/QUIT to workload process group when enabled, otherwise to direct child PID.
- Shutdown grace: after a forwarded termination signal, a bounded force-kill fallback is required for non-cooperative
  workloads. The prototype validated a `500 ms` grace value as a test setting, not as a production default.
- Exit status:
  - workload exit `0` -> MetricShell exit `0`;
  - workload exit `N` -> MetricShell exit `N`;
  - workload killed by signal `S` -> MetricShell exit `128 + S`;
  - workload start failure -> MetricShell exit `127`;
  - MetricShell internal failure before workload result is resolved -> MetricShell-owned non-workload code, prototype
    used `70`.
- Post-workload survival: allowed only as bounded duration; prototype validated 3 seconds. The configured value should
  be less than the external shutdown grace budget from INV-003.
- Docker init/Tini support: allowed, but if MetricShell is not PID 1 and must manage daemonized descendants, enable
  `PR_SET_CHILD_SUBREAPER`.
- Shell-form commands: supported with documented caveat that shell wrappers may return `143` on TERM even when
  descendants received the signal.

## Prototype Limits

- The prototype is not a production MetricShell implementation.
- Both reference runs were performed in Docker Desktop container environments that use LinuxKit container kernels. This
  covers macOS/LinuxKit aarch64 and Ubuntu/LinuxKit x86_64, but it is not evidence for native non-LinuxKit Linux or
  Kubernetes runtime behavior.
- Docker `--init` is used as the Tini-compatible layer; standalone Tini, dumb-init and s6 versions were not compared.
- Timing is measured from JSONL events and runner wall-clock duration; it is useful for architecture evidence but is
  still not a full performance benchmark.
- Zombie state is sampled through `/proc`; this improves evidence for child churn, but does not prove the absence of
  very short-lived zombie windows between samples.
- The shell workload uses `/bin/sh`; bash, busybox ash and app-specific wrapper behavior should be checked separately.
- The HTTP endpoint was checked only with a simple scrape during the post-exit window.

## Additional Benchmarking

The current prototype now covers most local Docker benchmarks that were listed as follow-up work:

| Benchmark item                                                              | Status                                                                            | Evidence                                                                            |
|-----------------------------------------------------------------------------|-----------------------------------------------------------------------------------|-------------------------------------------------------------------------------------|
| 30 repetitions with p50/p95/p99 for `signal_forwarded -> workload_exited`   | Covered for `repeat_signal_direct_pg` only                                        | `signal-to-exit-latency.tsv`, `signal-to-exit-latency-stats.tsv`                    |
| Single-run signal-to-exit samples for the other signal cases                | Covered as smoke evidence only                                                    | `signal-to-exit-latency.tsv`, `signal-to-exit-latency-stats.tsv`                    |
| Explicit signal receipt by descendants                                      | Covered for shell parent and grandchild signal cases                              | `signal-delivery.tsv`, `events.jsonl`                                               |
| Case-level assertions beyond exit code                                      | Covered; `summary.tsv` is derived from assertion results                          | `assertions.tsv`, `summary.tsv`                                                     |
| Raw structured event aggregation                                            | Covered                                                                           | `events.jsonl`                                                                      |
| Automatic environment metadata                                              | Covered                                                                           | `environment.tsv`                                                                   |
| CPU/RSS during idle, active workload, child churn and post-exit phases      | Covered with `/proc` samples for the MetricShell process; CLK_TCK is in-container | `resources.tsv`, `environment.tsv`                                                  |
| High-churn workloads with 1k, 10k and zombie-window sampling                | Covered for 1k and 10k; 100k remains opt-in via `INV001_RUN_HEAVY=1`              | `summary.tsv`, `zombies.tsv`                                                        |
| Shutdown budget benchmark with TERM, grace, forced KILL and final exit code | Covered with `--shutdown-grace=500ms`                                             | `summary.tsv`, `signal-to-exit-latency-stats.tsv`, `shutdown_grace_forced_kill.log` |
| PID 1, Docker `--init`, no-subreaper and subreaper comparison               | Covered for Docker `--init`; standalone init binaries were not downloaded         | `summary.tsv`                                                                       |
| Shell variants                                                              | Covered for exec-form, `/bin/sh -c`, `/bin/bash -c` and shell-script wrapper      | `summary.tsv`, `signal-delivery.tsv`                                                |
| External Prometheus-like scrape during post-exit                            | Covered with host-side HTTP scrapes                                               | `scrapes.tsv`                                                                       |
| Ubuntu/LinuxKit x86_64 repeat                                               | Covered with the same benchmark fingerprint as the macOS reference run            | `results/20260718T085124Z/summary.tsv`, `results/20260718T085124Z/assertions.tsv`   |
| Native non-LinuxKit Linux and Kubernetes Job/CronJob repeats                | Still open                                                                        | Requires another runtime environment                                                |
| Standalone Tini, dumb-init and s6 comparison                                | Still open                                                                        | Requires adding those binaries/images intentionally                                 |

To run the heavy 100k child churn case:

```bash
INV001_RUN_HEAVY=1 ./research/INV-001/run-bench.sh
```

## Conclusion

The INV-001 assumption is confirmed within the tested Docker Desktop/LinuxKit container environments:

- MetricShell can run as PID 1 and implement supervisor/init responsibilities itself.
- Tini/Docker init does not replace MetricShell lifecycle ownership.
- If MetricShell runs under an init process and must be responsible for daemonized descendants, `PR_SET_CHILD_SUBREAPER`
  is required.
- A process group is useful for reaching descendants with signals, but shell-form commands require an explicit exit
  semantics policy.

Recommended direction for the next architecture iteration: single-binary MetricShell as PID 1 by default, with an
optional compatibility mode under Tini plus subreaper when descendant ownership is required.
