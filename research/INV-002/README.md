# INV-002 — Workload Lifecycle and Exit Semantics

Status: validation in progress

Reference run: `results/20260721T200345Z`

Extended reference run: `results/20260721T200406Z-extended`

Report: [report.md](report.md)

## Question

What is the exact lifecycle of one MetricShell execution?

## Context

MetricShell must keep workload outcome, application metric state and container restart behavior unambiguous. An internal
retry turns one container execution into several application executions and forces an additional decision about whether
counters reset or are merged.

## Candidates

1. Execute exactly one workload and return its outcome.
2. Retry the workload inside MetricShell.
3. Execute exactly once and delegate retry to Docker, Compose or Kubernetes.

## Initial Hypothesis

MetricShell should run exactly one workload execution. Restart policy should remain outside MetricShell.

## Experiments

The Docker prototype compares:

- successful and failed single executions;
- workload start failure;
- bounded post-exit metrics availability;
- internal restart with counter reset and counter preservation;
- an internal restart limit;
- Docker `--restart=on-failure:2` around a single-execution MetricShell.

Each case records resolved exit code, execution count and final `app_events_total`. See
[`summary.tsv`](results/20260721T200345Z/summary.tsv) and
[`observations.tsv`](results/20260721T200345Z/observations.tsv).

## Results

All 8 case-level expectations passed on Docker 29.4.3, LinuxKit 6.12.76, aarch64:

| Case                        |  Executions | Exit | Counter sequence      | Result |
|-----------------------------|------------:|-----:|-----------------------|--------|
| Single success              |           1 |    0 | 5                     | pass   |
| Single failure              |           1 |   17 | 5                     | pass   |
| Internal retry, reset       |           2 |    0 | 5 → 2                 | pass   |
| Internal retry, preserve    |           2 |    0 | 5 → 7                 | pass   |
| Internal retry limit        |           3 |   17 | 1 → 2 → 3             | pass   |
| Start failure               |           0 |  127 | 0                     | pass   |
| Docker external restart     | 2 processes |    0 | 5 → 2                 | pass   |
| Two-second post-exit window |           1 |   17 | 5, endpoint available | pass   |

Internal retry makes both plausible counter policies problematic: reset causes a counter decrease within one
MetricShell lifetime, while preservation merges distinct workload executions. Docker restart produced two clean
single-execution lifecycles and retained the runtime's visible restart count.

## Conclusion

The hypothesis is confirmed in the corrected macOS/LinuxKit aarch64 run. Status remains `validation in progress`
until Ubuntu/LinuxKit x86_64 repeats fingerprint
`a8042a6b12b8d659f701125584223373a934186d45fdeae2e64f89f6362c05f2`. The submitted Ubuntu run used the preceding
fingerprint and exposed locale/host-timing defects in the runner, so it is diagnostic rather than confirming evidence.
MetricShell should execute the workload exactly
once;
retry count and backoff are outside its configuration surface. Runtime restart creates a new metric-state epoch.

Kubernetes behavior was evaluated structurally, not executed: a Pod `restartPolicy: OnFailure` restarts the container,
while a Job controller applies its own retry/backoff policy. Both preserve the same ownership boundary, but require a
separate Kubernetes integration run before claiming runtime parity.

## Admissible Lifecycle Values

- `workload executions per MetricShell process`: exactly `1`;
- `internal retry count`: `0` (no retry option);
- `post-exit duration`: `0` by default or an explicit finite duration; `2s` is functionally verified here, not a
  production sizing recommendation;
- workload outcome: preserve normal exit code or signal-derived outcome; start failure must be distinguishable
  (`127` in this prototype); MetricShell's own failure must take precedence and use a distinct documented code;
- application counters: one state epoch per MetricShell/container execution; never silently merge retries;
- external retry: Docker/Compose restart policy or Kubernetes Pod/Job policy, with operator-owned limits and backoff.

## Running the Prototype

From the repository root:

```bash
./research/INV-002/run-bench.sh
./research/INV-002/run-extended-bench.sh
```

Inspect the latest evidence:

```bash
latest="$(cat research/INV-002/latest-results.txt)"
cat "$latest/summary.tsv"
cat "$latest/observations.tsv"
cat "$latest/environment.tsv"
cat "$latest/post-exit.metrics"

extended="$(cat research/INV-002/latest-extended-results.txt)"
cat "$extended/latency-stats.tsv"
cat "$extended/faults.tsv"
cat "$extended/post-exit-grid.tsv"
cat "$extended/restart-storm.tsv"
cat "$extended/coverage.tsv"
```

Manual single-execution run:

```bash
docker build -t metricshell-inv002:prototype research/INV-002/prototype
docker run --rm metricshell-inv002:prototype \
  --policy=single --post-exit=2s -- \
  /usr/local/bin/workload --state=/tmp/attempt --exits=17 --increments=5
```

The benchmark creates only `inv002-*` containers and one temporary named volume and removes them on exit. Results are
written to a new UTC timestamp directory; `latest-results.txt` points to it.

## Prototype Limits

- Research code, not a production supervisor; it intentionally omits ingestion protocols and production hardening.
- One Docker Desktop/LinuxKit aarch64 environment was measured. Native Linux, x86_64, containerd/CRI-O and Kubernetes
  were not executed.
- The synthetic counter is line-based and tests lifecycle semantics, not ingestion throughput.
- The tested `0–30s` range validates bounded post-exit behavior only. INV-003 must determine the production budget.
- TERM, KILL, container-OOM, restart storms and concurrent restart scrapes are covered. Daemon restart, host reboot and
  disk-full
  remain untested.
- Docker's restart policy reuses a container; persistent application data can survive independently of MetricShell
  metric memory. The prototype uses a volume only to make the synthetic workload fail once.

## Additional Benchmarks

The extended run covers 30 repetitions with p50/p95/p99, Compose restart, 150 restart-boundary scrape samples,
TERM/KILL/container-OOM, post-exit values `0, 1, 2, 5, 10, 30s`, and 10/100/1000-attempt internal restart storms.

Native Linux/x86_64 was unavailable. Kubernetes could not run because the configured cluster OAuth refresh token is
invalid. Docker daemon restart and disk-full injection were not performed because they could disrupt unrelated
workloads. Exact statuses are recorded in
[`coverage.tsv`](results/20260721T200406Z-extended/coverage.tsv).

Both currently available container environments use LinuxKit. Native non-LinuxKit Linux remains unverified.

## Decision Output

- Prototype: `prototype/`
- Runners: `run-bench.sh`, `run-extended-bench.sh`
- Raw evidence: `results/20260721T200345Z/`
- Extended evidence: `results/20260721T200406Z-extended/`
- Detailed analysis: [report.md](report.md)
- ADR input: exactly one workload execution; external restart ownership; one metric-state epoch per execution.
