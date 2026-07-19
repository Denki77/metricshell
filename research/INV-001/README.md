# INV-001 — Process and PID 1 Model

Status: completed  
Reference runs: `results/20260717T192610Z`, `results/20260718T085124Z`  
Report: [report.md](report.md)

## Question

Should MetricShell run directly as PID 1, run under an init process such as Tini, or delegate process management to
another component?

## Context

MetricShell must start the workload, receive and forward signals, observe workload exit, preserve exit semantics, manage
responsible descendants and remain alive after workload completion when configured.

## Candidates

### A. MetricShell as PID 1

MetricShell implements required init and supervisor behavior directly.

### B. Tini as PID 1

Tini runs MetricShell, and MetricShell runs the workload.

### C. Another process supervisor

Examples: dumb-init, s6 or supervisord.

## Initial Hypotheses

- MetricShell must own workload lifecycle even when Tini is present.
- Tini may reduce PID 1 edge cases but adds another binary and signal layer.
- A correct single-binary implementation may be simpler operationally.
- Process-group and orphaned-descendant behavior are the main correctness risks.

## Evidence Required

- Linux process and signal documentation;
- Docker init/process documentation;
- Tini behavior and source review;
- prototype for each viable process tree;
- tests for signals, descendants, zombies and exit codes.

## Experiments

### E-001.1 — Signal forwarding

Implemented in `run-bench.sh` with direct child, process group, shell script, `/bin/sh -c` wrapper and Docker init/Tini
variants.

### E-001.2 — Child reaping

Implemented with short-lived child spawning and double-fork daemonization scenarios. The prototype records
`descendant_reaped` events when MetricShell owns the orphaned descendant.

### E-001.3 — Exit status

Implemented for exit `0`, exit `17`, TERM, KILL, workload start failure and simulated MetricShell internal failure.

### E-001.4 — Post-workload survival

Implemented with `--post-exit=3s`; `/metrics` remains available and exposes the preserved workload exit code.

## Evaluation Criteria

- correctness;
- process-tree control;
- exit-code integrity;
- implementation complexity;
- external dependency count;
- image size;
- portability;
- operational clarity.

## Open Questions

- Should MetricShell use `PR_SET_CHILD_SUBREAPER`?
  - Yes when MetricShell is not PID 1 and still needs ownership/visibility of daemonized descendants.
- Is one process group created for every workload?
  - Recommended when group-wide signal delivery is required; this needs explicit shell-form exit semantics.
- Are daemonized descendants supported?
  - Supported when MetricShell is PID 1 or when it is configured as a subreaper under another init process.
- What behavior is expected for shell-form commands?
  - Shell-form commands may return `143` after TERM even when descendants receive the signal. This should be
    documented and tested.
- Does Tini add meaningful correctness after MetricShell implements lifecycle ownership?
  - It can reduce generic PID 1 duties above MetricShell, but does not replace MetricShell lifecycle ownership.

## Results

The final macOS and Ubuntu/LinuxKit Docker runs passed all defined expectations:

| Environment                                | Date       | Result set                 | Summary                                             | Benchmark fingerprint                                              |
|--------------------------------------------|------------|----------------------------|-----------------------------------------------------|--------------------------------------------------------------------|
| Docker Desktop on macOS / LinuxKit aarch64 | 2026-07-17 | `results/20260717T192610Z` | [summary.tsv](results/20260717T192610Z/summary.tsv) | `35dc9c63a0a9f6dedf56a1c6c80b582919d5961b8f233c49ef1aed55652b71fb` |
| Docker Desktop on Ubuntu / LinuxKit x86_64 | 2026-07-18 | `results/20260718T085124Z` | [summary.tsv](results/20260718T085124Z/summary.tsv) | `35dc9c63a0a9f6dedf56a1c6c80b582919d5961b8f233c49ef1aed55652b71fb` |

Key findings:

- MetricShell as PID 1 correctly forwarded signals, preserved exit codes, reaped an orphaned double-fork descendant and
  served post-exit metrics.
- All case-level assertions passed in both environments. The assertion set covers exit codes, startup readiness,
  signal receipt, descendant reaping expectations and post-exit HTTP/metrics availability.
- Docker init/Tini without `PR_SET_CHILD_SUBREAPER` reaped daemonized descendants outside MetricShell visibility.
- Docker init/Tini with `PR_SET_CHILD_SUBREAPER` allowed MetricShell to reap daemonized descendants.
- Process-group signaling reached shell descendants, but shell wrappers can still resolve to exit `143`.
- The 30-run `repeat_signal_direct_pg` signal-to-exit latency benchmark measured p50/p95/p99 `0.434/0.581/0.625 ms`
  on macOS/LinuxKit aarch64 and `0.468/1.894/2.155 ms` on Ubuntu/LinuxKit x86_64.
- The extended benchmark run measured 30 signal-to-exit repetitions for `repeat_signal_direct_pg`, single-run
  signal-to-exit smoke samples for other signal cases, explicit signal delivery events, CPU/RSS samples, 1k/10k child
  churn, zombie scan samples, forced shutdown grace, external post-exit scrapes and environment metadata.

## Conclusion

Accept the direction "MetricShell as PID 1 by default" for the next design step. The assumption is confirmed within the
tested Docker Desktop/LinuxKit container environments on macOS aarch64 and Ubuntu x86_64.

Retain Tini/Docker init as a compatibility mode, not as a replacement for MetricShell lifecycle ownership. If
MetricShell runs below an init process and must manage daemonized descendants, it should enable
`PR_SET_CHILD_SUBREAPER`.

Process groups should be considered for signal coverage, but shell-form command behavior must be documented because
clean exit-code preservation depends on workload wrapper behavior.

## Decision output

- Prototype: `prototype/`
- Runner: `run-bench.sh`
- Raw evidence: `results/20260717T192610Z/`, `results/20260718T085124Z/`
- Report: [report.md](report.md)
- Recommended ADR input: MetricShell runs as PID 1 by default; optional init-process compatibility requires subreaper
  mode for descendant ownership.

## Running the Prototype

Full run:

```bash
./research/INV-001/run-bench.sh
```

Inspect latest results:

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

When comparing environments, use `benchmark_code_fingerprint_sha256` from `environment.tsv` as the benchmark code
identity. `repository_head_sha` is recorded only as context and may change after documentation-only commits.
`summary.tsv` reports `pass` only when all case-level assertions in `assertions.tsv` pass.

Manual PID 1 run:

```bash
docker build -t metricshell-inv001:prototype research/INV-001/prototype
docker run --rm metricshell-inv001:prototype --http=:9090 -- /usr/local/bin/workload-trap
```

Manual Docker init/Tini run:

```bash
docker run --rm --init metricshell-inv001:prototype --http=:9090 -- /usr/local/bin/workload-trap
```

Manual subreaper run:

```bash
docker run --rm --init metricshell-inv001:prototype --http=:9090 --subreaper -- /prototype/workloads/double-fork.sh
```

## Prototype Limits

- The prototype is research code, not production MetricShell.
- Results were collected in Docker Desktop container environments that both use LinuxKit container kernels. This covers
  macOS/LinuxKit aarch64 and Ubuntu/LinuxKit x86_64, but not native non-LinuxKit Linux or Kubernetes runtime behavior.
- Docker `--init` was used as the Tini-compatible model; standalone init binaries were not compared.
- Timing values are suitable for architectural evidence, not performance promises.
- Shell behavior depends on the actual shell and workload wrapper.

## Additional Benchmarks

Covered by the current prototype:

- 30 repetitions with p50/p95/p99 signal-to-exit latency for `repeat_signal_direct_pg`.
- Single-run signal-to-exit smoke samples for other signal cases.
- Explicit signal delivery checks for direct workload, shell parent and grandchild cases.
- Case-level assertions for exit code, readiness, signal delivery, double-fork reaping and post-exit metrics.
- Raw structured event aggregation in `events.jsonl`.
- Automatic environment metadata in `environment.tsv`.
- CPU/RSS measurement during idle, signal handling, child storm and post-exit windows.
- High child churn tests with 1k and 10k short-lived children plus zombie sampling.
- Shutdown budget tests with TERM grace and forced KILL.
- Comparison of PID 1, Docker `--init`, no-subreaper and subreaper modes.
- External Prometheus-like scraping during post-exit.

Still open:

- 100k child churn as a heavy opt-in run: `INV001_RUN_HEAVY=1 ./research/INV-001/run-bench.sh`.
- Standalone Tini, dumb-init and s6 comparison.
- Native non-LinuxKit Linux and Kubernetes Job/CronJob repeats.
