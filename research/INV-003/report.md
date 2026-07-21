# INV-003 Report — Shutdown Time Budgeting

Status: completed for Docker Desktop/LinuxKit; native Ubuntu confirmation pending  
Run date: 2026-07-21  
Docker Server: 29.4.3  
Docker platform: linux/aarch64, LinuxKit 6.12.76  
Reference run: `results/20260723T150539Z`
Summary: `results/20260723T150539Z/shutdown-grid.tsv`

## Goal

Validate whether explicit workload timeout plus a MetricShell-owned reserve provides deterministic, operator-readable
shutdown behavior without external SIGKILL, and select admissible starting budgets.

## Prototype

The prototype is located in `research/INV-003`.

- `prototype/cmd/metricshell` starts one process group, forwards TERM/INT, applies the selected workload budget,
  force-kills an overdue workload, finalizes synthetic metric/diagnostic state, drains HTTP and preserves exit status.
- `prototype/cmd/workload` exits immediately, after a configured TERM delay, or ignores TERM.
- `prototype/Dockerfile` produces the isolated benchmark image.
- `run-bench.sh` builds the image, runs assertions, writes raw logs/TSV evidence and captures the stand fingerprint.
- The same `run-bench.sh` command is used on macOS and Ubuntu; environment differences are observations, not benchmark
  identity inputs.

The policies are implemented as explicit, fixed reserve, percentage and a distinct externally supplied absolute
deadline. Deadline mode reads a Unix-millisecond value from a flag or control-plane-provided file when TERM arrives,
then computes workload budget from actual remaining time. The runner supplies that file with `docker cp`, avoiding
host-path bind-mount differences between Docker Desktop and Ubuntu. Every phase is capped by remaining time.

## Run Commands

Full run:

```bash
./research/INV-003/run-bench.sh
```

Native Ubuntu confirmation uses the same command:

```bash
./research/INV-003/run-bench.sh
```

Results are stored in a new UTC directory. `latest-results.txt` points to the last completed run. The commands for
inspecting every summary file and manually running the prototype are documented in `README.md`.

## Run Environments

| Environment                       | Date       | Docker | Platform      | Result set                 | Fingerprint                                                        |
|-----------------------------------|------------|--------|---------------|----------------------------|--------------------------------------------------------------------|
| Docker Desktop/macOS, LinuxKit VM | 2026-07-23 | 29.4.3 | linux/aarch64 | `results/20260723T150539Z` | `27e8a991546667f92abb5965c044b834c1583f710cbb060dfef81174b29bb53c` |
| Native Ubuntu                     | pending    | —      | —             | run `run-bench.sh`         | must match the reference benchmark code fingerprint                |

The evidence records repository SHA, benchmark-only SHA-256, image ID, Docker server, container kernel, architecture,
CPU count, memory and storage driver. The benchmark fingerprint covers the prototype and runner, so documentation-only
changes do not alter stand identity.

## Results

All 20 mandatory cases passed.

| Total | Workload | Reserve | Immediate | Just before | After deadline |       Never |
|------:|---------:|--------:|----------:|------------:|---------------:|------------:|
|   1 s |   0.75 s |  0.25 s |      pass |        pass |    pass / KILL | pass / KILL |
|   5 s |      4 s |     1 s |      pass |        pass |    pass / KILL | pass / KILL |
|  10 s |      9 s |     1 s |      pass |        pass |    pass / KILL | pass / KILL |
|  30 s |     28 s |     2 s |      pass |        pass |    pass / KILL | pass / KILL |
|  60 s |     58 s |     2 s |      pass |        pass |    pass / KILL | pass / KILL |

Measured shutdown completion times:

| Window | Immediate |  Just before | After deadline |        Never |
|-------:|----------:|-------------:|---------------:|-------------:|
|    1 s | 22.484 ms |   674.507 ms |     773.309 ms |   777.234 ms |
|    5 s | 20.624 ms |  3930.539 ms |    4023.113 ms |  4022.429 ms |
|   10 s | 23.148 ms |  8935.097 ms |    9039.308 ms |  9028.187 ms |
|   30 s | 24.815 ms | 27935.941 ms |   28062.445 ms | 28024.567 ms |
|   60 s | 24.140 ms | 57941.380 ms |   58078.702 ms | 58050.850 ms |

These are enforced results, not report-only observations. `shutdown-grid-assertions.tsv` requires correct exit and
forced state, exactly one shutdown/finalization/HTTP marker, correct budget-expiry count, shutdown before total grace,
finalization and HTTP within caps, and no forced kill before workload budget.

Fixed, percentage and explicit policies computed the expected 4 s workload share for a controlled 5 s example.
Absolute deadline is not included in that arithmetic comparison. Its dedicated cases observed remaining/budget pairs
of `4955/3955`, `3930/2930`, `194/0`, `0/0` and `421/0 ms` for full, partly spent, nearly expired, expired and
reserve-exceeds-remaining states. Explicit overcommit was rejected with configuration exit `64`.

Thirty immediate shutdown repetitions all passed. Total shutdown p50/p95/p99 was `6.825/7.710/7.986 ms`, with
`5.609 ms` minimum and `9.092 ms` maximum. This measures the prototype's internal handling and 5 ms synthetic
finalization, excluding Docker CLI signal transport.

The HTTP listener closes as soon as shutdown begins. In both cases a second post-TERM request was not admitted normally
(`HTTP 000`). The pre-TERM 100 ms scrape drained in `73.423 ms`; the 1500 ms scrape hit the 700 ms cap in
`704.135 ms`. Thus admission stops immediately while already-active work remains bounded.

`docker stop --time 1` and `docker stop --time 5` independently enforced external deadlines while internal totals were
deliberately smaller: 750 ms and 4 s. Internal completion was `527.823 ms` and `3529.925 ms`; host-side stop was
`642 ms` and `3708 ms`; safety margin was `472.177 ms` and `1470.075 ms`. Each had exactly one completion marker, so
exit `137` is supported as preserved workload KILL rather than external MetricShell KILL. No `post_exit` event occurred.

## Hypothesis Evaluation

### Explicit workload timeout plus reserve is easier to reason about

Supported. It exposes phase ownership and enables validation before shutdown. Fixed and percentage policies can produce
the same number but hide either the workload budget or absolute safety margin. Explicit values still require a known
total external grace; they are not independent in the mathematical sense.

### MetricShell can complete deterministically before external SIGKILL

Supported for all tested windows. Overdue and non-cooperative workloads were killed at their budget and finalization
completed inside the reserve. Independent Docker deadline cases confirmed the completion marker was written before
Docker's SIGKILL boundary.

### A bounded reserve can drain active HTTP without unbounded waiting

Supported. A fitting request drained and an overlong request timed out at the configured HTTP cap. Starting a new
post-exit scrape waiting interval after TERM is rejected: it competes directly with the external deadline.

## Acceptable Values and Policies

- Primary API: explicit `workload_timeout` and `shutdown_reserve`, validated against a known `total_grace` or absolute
  deadline.
- Validation: reject zero/negative totals, negative phase values, ratios outside `[0,1]`, and any explicit sum greater
  than total grace.
- Short emergency window: `750ms workload + 250ms reserve` for a 1 s total is experimentally viable but leaves little
  margin for throttled or contended hosts.
- Normal 5–10 s window: reserve at least 1 s.
- Normal 30–60 s window: reserve 2 s and give the remainder to the workload.
- Fixed reserve: acceptable as a deployment default after deriving it from finalization and HTTP-drain limits.
- Percentage-only reserve: reject as the sole rule; it gives inconsistent absolute safety margins.
- Absolute deadline: preferred additional input where the orchestrator can provide it reliably.
- Budget overflow: fail configuration before relying on the unsafe budget; never silently clamp the explicit contract.
- HTTP: stop admission when shutdown begins, drain only active requests, cap drain by both its phase timeout and the
  absolute remaining deadline.
- Post-exit wait: allowed after natural workload completion only; disabled after external TERM/INT begins.

These are admissible research values for the measured environment, not production defaults across all runtimes. A
matching-fingerprint native Ubuntu run is required before widening the evidence claim.

## Prototype Limits

- Synthetic finalization models time budgeting, not real registry serialization, filesystem durability or log sinks.
- The reference run is LinuxKit aarch64. Native Ubuntu x86_64/arm64 evidence is prepared but not fabricated.
- Docker does not pass `docker stop --time` to PID 1; deployment configuration must supply the same total/deadline to
  MetricShell.
- CPU throttling, memory pressure, storage stalls, large cardinality, high scrape concurrency, containerd/CRI-O and
  Kubernetes termination behavior remain outside this run.
- A successful drain means handler completion inside the server timeout; it does not guarantee Prometheus received or
  persisted a final sample.

## Additional Benchmarking

| Benchmark item                                        | Status              | Evidence                                              |
|-------------------------------------------------------|---------------------|-------------------------------------------------------|
| Windows 1/5/10/30/60 × four workload behaviors        | Covered, 20/20 pass | `shutdown-grid.tsv`, raw logs                         |
| Explicit grid invariants                              | Covered, all pass   | `shutdown-grid-assertions.tsv`                        |
| Fixed/percentage/explicit policy arithmetic           | Covered, 3/3 pass   | `policy-comparison.tsv`                               |
| Absolute deadline full/spent/near/expired/reserve     | Covered, 5/5 pass   | `absolute-deadline.tsv`                               |
| Explicit budget overflow                              | Covered, rejected   | `policy-comparison.tsv`, `inv003-policy-overflow.log` |
| 30 repetitions with p50/p95/p99                       | Covered, 30/30 pass | `repetitions.tsv`, `latency-stats.tsv`                |
| Active HTTP request fits reserve                      | Covered             | `http-drain.tsv`, `http-fits.body`                    |
| Active HTTP request exceeds cap                       | Covered             | `http-drain.tsv`, `http-overruns.body`                |
| New HTTP request after shutdown begins                | Covered, rejected   | `http-drain.tsv`, raw logs                            |
| Independent Docker external deadline                  | Covered at 1 s/5 s  | `external-deadline.tsv`                               |
| Post-exit policy during external termination          | Covered             | `termination-policy.tsv`                              |
| Environment and stand fingerprint                     | Covered             | `environment.tsv`                                     |
| Native Ubuntu x86_64/arm64                            | Prepared, not run   | the same `run-bench.sh`                               |
| CPU quota/pressure and 100–1000 concurrent scrapes    | Recommended next    | not part of current evidence                          |
| Slow disk/diagnostic sink and real metric cardinality | Recommended next    | requires production-like finalizer                    |
| containerd/CRI-O and Kubernetes                       | Recommended next    | runtime-specific integration                          |

For the Ubuntu confirmation, use an otherwise idle native host, run the script three times without editing benchmark
files, retain complete result directories, and compare the code fingerprint. Record `docker info`, `uname`, image ID,
CPU/memory and storage driver already emitted by the runner. For stronger independent timing, collect signal/process
exit events with eBPF or `perf trace` and correlate them with `EVENT elapsed_ms`.

## Conclusion

INV-003 is confirmed for Docker Desktop/LinuxKit aarch64. Explicit workload timeout plus explicit reserve is the
clearest
configuration contract, provided MetricShell also knows the external total/deadline and rejects overcommit. A fixed
reserve is suitable as a documented deployment default; percentage-only budgeting is not.

Recommended starting policy is a 1 s reserve for 5–10 s totals and 2 s for 30–60 s totals, with 250 ms demonstrated as
the minimum tested reserve for a constrained 1 s total. Shutdown must stop new HTTP admission, drain only active
requests within a cap, skip post-exit waiting, and make every phase deadline-aware. Native Ubuntu confirmation remains
an explicit evidence boundary and is reproducible through the same `run-bench.sh` command.
