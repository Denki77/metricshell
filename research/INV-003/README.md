# INV-003 — Shutdown Time Budgeting

Status: completed for Docker Desktop/LinuxKit; native Ubuntu confirmation prepared

Reference run: `results/20260723T150539Z`

Report: [report.md](report.md)

## Question

How much of the external shutdown grace period may be given to the workload, and how much must MetricShell reserve for
itself?

## Context

MetricShell must forward termination, bound workload shutdown, force-kill when required, collect exit status, finalize
metrics and diagnostics, drain HTTP requests, and exit before the runtime sends SIGKILL.

## Candidates

1. Fixed reserve: `workload_grace = total_grace - fixed_reserve`.
2. Percentage reserve: `workload_grace = total_grace × configured_ratio`.
3. Explicit workload timeout plus explicit MetricShell reserve.
4. Absolute external deadline with a dynamically computed remaining budget.

## Initial Hypothesis

Explicit workload timeout plus runtime reserve is easier to reason about than automatic inference.

## Experiments

The Docker prototype covers total windows `1, 5, 10, 30, 60s` and workloads that stop immediately, 100 ms before
their budget, after their budget, or never. It compares fixed, percentage and explicit arithmetic, separately exercises
a real externally supplied absolute deadline, rejects an overcommitted explicit budget, repeats shutdown 30 times,
tests HTTP admission plus drain, verifies Docker's independent stop deadline, and checks that post-exit waiting is
skipped after external termination begins.

## Results

All 20 mandatory grid cases and every additional assertion passed on Docker Desktop/LinuxKit aarch64. Workloads after
the deadline and workloads ignoring TERM were killed with exit `137`; earlier workloads exited `0`. MetricShell always
emitted `shutdown_complete` before the configured deadline.

| Window | Workload budget | Reserve | Just-before total | Forced total | Result |
|-------:|----------------:|--------:|------------------:|-------------:|--------|
|    1 s |          750 ms |  250 ms |        674.507 ms |   773.309 ms | pass   |
|    5 s |             4 s |     1 s |       3930.539 ms |  4023.113 ms | pass   |
|   10 s |             9 s |     1 s |       8935.097 ms |  9039.308 ms | pass   |
|   30 s |            28 s |     2 s |      27935.941 ms | 28062.445 ms | pass   |
|   60 s |            58 s |     2 s |      57941.380 ms | 58078.702 ms | pass   |

All explicit assertions in `shutdown-grid-assertions.tsv` passed: exit semantics, forced state, lifecycle marker counts,
budget-expiry count, total deadline, finalization cap, HTTP cap and no early forced kill. The 30-run p50/p95/p99 was
`6.825/7.710/7.986 ms`.

The absolute-deadline cases measured `4955 ms` fully available, `3930 ms` after deliberate grace consumption,
`194 ms` nearly expired, `0 ms` expired and `421 ms` when reserve exceeded remaining time. Workload budget was computed
from observed remaining time and became zero when reserve could not fit.

A 100 ms active scrape drained, a 1500 ms scrape was capped at about 700 ms, and a second request after
`shutdown_started` was not admitted normally (`HTTP 000`, listener closed). Docker external windows of 1 s and 5 s
used smaller internal totals of 750 ms and 4 s, leaving safety margins of `472.177 ms` and `1470.075 ms`.

## Conclusion

The hypothesis is supported in the measured environment. Use explicit independent values as the configuration model,
but validate them against one known total grace/deadline. Fixed reserve is a useful deployment-derived default;
percentage-only policy is rejected because its absolute safety margin becomes too small for short windows and wasteful
for long ones. Absolute deadline is best when the runtime can supply it reliably.

## Acceptable Values and Policies

- Reject configuration when `workload_timeout + reserve > total_grace`; never silently overcommit.
- For a 1 s external window, the tested lower bound is a 250 ms reserve and 750 ms workload budget.
- For 5–10 s windows, use at least a 1 s reserve.
- For 30–60 s windows, use a 2 s reserve; the extra time is operational margin, not measured CPU need.
- Bound HTTP drain separately and cap every phase by the absolute remaining deadline.
- Do not perform configurable post-exit scrape waiting after TERM/INT; only drain requests already active.
- Force-killed workload outcome remains `137`; MetricShell must still finalize and exit within its reserve.
- Treat these as researched starting values, not cross-platform guarantees, until the matching-fingerprint Ubuntu run
  is collected.

## Running the Prototype

From the repository root:

```bash
./research/INV-003/run-bench.sh
```

Inspect the latest evidence:

```bash
latest="$(cat research/INV-003/latest-results.txt)"
cat "$latest/shutdown-grid.tsv"
cat "$latest/shutdown-grid-assertions.tsv"
cat "$latest/policy-comparison.tsv"
cat "$latest/absolute-deadline.tsv"
cat "$latest/latency-stats.tsv"
cat "$latest/http-drain.tsv"
cat "$latest/external-deadline.tsv"
cat "$latest/termination-policy.tsv"
cat "$latest/environment.tsv"
cat "$latest/coverage.tsv"
```

On both macOS and native Ubuntu, use the same command from the repository root:

```bash
./research/INV-003/run-bench.sh
```

Compare `benchmark_code_fingerprint_sha256` between runs. Also retain `image_id`, Docker server version, kernel,
architecture, CPU count, memory and storage driver from `environment.tsv`. A matching code fingerprint proves the same
stand was run; differing kernel/runtime fields identify the environmental comparison.

Manual example:

```bash
docker build -t metricshell-inv003:prototype research/INV-003/prototype
docker run --rm metricshell-inv003:prototype \
  --total-grace=10s --policy=explicit --workload-timeout=9s --reserve=1s -- \
  /usr/local/bin/workload --term-delay=-1ms
```

Send TERM from another terminal with `docker stop --time 10 <container>`.

Deadline mode accepts either `--shutdown-deadline-unix-ms=<absolute timestamp>` or
`--shutdown-deadline-file=<path>`. The file form is read when TERM arrives and lets an external control plane publish
the actual deadline immediately before shutdown. The benchmark uses `docker cp`, not a host bind mount, so the same
runner works with Docker Desktop and native Ubuntu daemon path/security rules.

## Prototype Limits

- This is research code, not production MetricShell; finalization is a controlled synthetic delay.
- The reference environment is Docker Desktop/LinuxKit aarch64, not native Ubuntu.
- Docker does not communicate its stop timeout to a process. `--total-grace` must match `docker stop --time` or Compose
  `stop_grace_period`; otherwise no process can infer the real deadline.
- Kubernetes deadline plumbing, containerd/CRI-O, host pressure, CPU throttling, very high HTTP concurrency and slow
  diagnostic storage are not verified here.
- Timing under Docker Desktop includes LinuxKit scheduling and is architecture evidence, not an SLO.

## Additional Benchmarks

Covered in the reference run: 20-case mandatory grid with explicit assertions; three derived-policy arithmetic checks;
five real absolute-deadline states; overflow rejection; 30-run p50/p95/p99; forced KILL correctness; HTTP admission
rejection plus bounded drain; strengthened Docker 1 s/5 s deadline enforcement; and no post-exit wait during
termination.

For stronger confidence, rerun unchanged on unloaded native Ubuntu x86_64 and arm64 hosts three times, then under CPU
quotas (`--cpus=.25`), memory pressure, 100–1000 concurrent scrapes, slow filesystem/diagnostic flush, and the target
container runtime/Kubernetes version. Use an external monotonic observer (for example eBPF or `perf trace`) to validate
signal and exit timestamps independently of prototype logs.

## Decision Output

- Prototype: `prototype/`
- Runner for macOS and Ubuntu: `run-bench.sh`
- Raw evidence: `results/20260723T150539Z/`
- Detailed analysis: [report.md](report.md)
- ADR input: explicit budgets validated against total deadline, deadline-aware phase capping, no post-exit wait on TERM.
