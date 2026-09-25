# INV-020 Report — Managed Aggregation Lifecycle and Core Integration

Status: completed

Run date: 2026-09-25

macOS reference run: `results/20260925T122604Z`

Reference environment: Docker Desktop 29.8.0, LinuxKit 7.0.12, linux/aarch64, 2-CPU container limit, 256 MiB memory limit

Result: 54/54 portable assertions passed; E-020.1–E-020.7 and all listed additional local benchmarks completed

Ubuntu confirmation run: `results/20260925T121532Z`

Confirmation environment: Docker Desktop 27.4.0, LinuxKit 6.10.14, linux/x86_64, 2-CPU container limit, 256 MiB memory limit

Decision: [ADR-020](../../docs/06-architecture/adr/ADR-020.md)

## Goal and Evidence Rule

INV-020 connects ADR-016 registry semantics, ADR-017 commit/ACK ordering, ADR-018 bounded Unix framing and ADR-019
immutable generation materialization to the accepted Core lifecycle. Assertions establish portable safety properties.
Timing and container-start observations are environment-sensitive and are not promises.

Both runs used portable source/runner fingerprint
`bbdde683d7c54d83ed82181634b639297d68e41ba2fb15ca4f350b5eaf36264f`.

## Candidate Freeze Boundaries

| Candidate                      | Bounded | Observed disposition                                              | Final evaluation                                              |
|--------------------------------|--------:|-------------------------------------------------------------------|---------------------------------------------------------------|
| Immediate freeze               |     yes | one committed item retained; already received queue omitted       | deterministic but discards admissible work unnecessarily      |
| Drain all fully received work  |      no | all four modeled items retained                                   | rejected without a deadline                                   |
| Explicit flush/close handshake |      no | all four modeled items retained                                   | rejected as mandatory; crashed client can withhold completion |
| Bounded hybrid                 |     yes | admitted work retained through the bound; late admission rejected | selected by ADR-020                                           |

The bounded hybrid matches the existing Core rule: entering `finalizing` closes admission first; an item admitted before
closure may finish only within the remaining finalization budget. The owner commit order is the linearization boundary.

## Experiments and Results

### E-020.1 — Normal exit

Two committed operations produced final value `5`. Admission closed, freeze executed exactly once and the complete
generation installed. A second freeze was a no-op and a late `+7` was rejected, leaving final state `5`.

### E-020.2 — Signal shutdown and in-flight stages

| Stage at closure                 | Client-visible outcome | Mutation in final state |
|----------------------------------|------------------------|------------------------:|
| Partial frame                    | rejected               |                       0 |
| Received, not validated/admitted | rejected               |                       0 |
| Queued/admitted                  | accepted after commit  |                       1 |
| Committing                       | accepted after commit  |                       1 |
| Committed before ACK delivery    | unknown                |                       1 |
| ACK delivered                    | accepted               |                       1 |

This preserves ADR-017: success is never sent before commit. ACK loss after commit is explicitly unknown to the client,
but the committed operation remains in the final state. Blind retry remains unsafe without an idempotency key.

The additional deadline matrix covered budgets `0/1/5/25/100 µs` crossed with operation costs
`0/1/5/25/100/250 µs` (30 cells). Work whose declared cost exceeded the remaining budget did not start. Measured sleep
duration reflects Linux scheduler granularity and is retained only as an observation.

### E-020.3 — Late publisher

After freeze, 1,000 concurrent mutation attempts were rejected and the final application value stayed `11`. Late
publisher failure is local and deterministic; it does not reopen the registry or mutate Core state.

### E-020.4 — Final snapshot failure

Conversion, whole-candidate validation and atomic-install failure were injected separately. In all three cases the
candidate was not installed, prior Core value `7` remained active, and the outcome was classified as a MetricShell-owned
failure. A finalization failure must not be hidden behind the workload result; Core failure precedence remains
authoritative.

### E-020.5 — Final scrape

Immediate, duration and scrape-count modes were crossed with health, readiness, debug, cancelled, failed, pre-final and
eligible requests (21 cells). Frozen application value `13` never changed. Self-metrics advanced independently.
Only a successful complete frozen-generation response after final-wait entry was eligible; immediate mode counted none.
The prototype makes no TSDB persistence or successful Prometheus collection claim.

### E-020.6 — Restart/new epoch

Thirty prior final values (`1..30`) were followed by thirty new registries. Every new epoch began with value and
generation zero. There is no replay, recovery or attachment to a prior workload execution.

### E-020.7 — Kubernetes Job/CronJob

Job and CronJob shapes were crossed with duration and scrape-count waits. All four retained workload exit `17`, exposed
the frozen state with metrics `200`, reported readiness `503`, and required no Kubernetes API. A real container
Job-shaped run waited at least the configured 250 ms and exited `17` in both environments.
This validates container semantics, not live-cluster discovery or scrape scheduling.

## Cross-environment Confirmation

| Portable evidence                               |    ARM64 reference |      Ubuntu x86_64 | Result |
|-------------------------------------------------|-------------------:|-------------------:|--------|
| Portable fingerprint                            | `bbdde683...6264f` | `bbdde683...6264f` | match  |
| Assertions                                      |              54/54 |              54/54 | match  |
| E-020.1–E-020.7                                 |           7/7 PASS |           7/7 PASS | match  |
| Process lifecycle cases                         |                3/3 |                3/3 | match  |
| Natural / signal / Job-shaped exit              |      17 / 143 / 17 |      17 / 143 / 17 | match  |
| Single-winner freeze rounds                     |              30/30 |              30/30 | match  |
| Late publishers accepted after freeze           |            0/1,000 |            0/1,000 | match  |
| Final-state/failure/epoch/Kubernetes invariants |               PASS |               PASS | match  |

The semantic evidence matches exactly. Timing does not: the main container wall time was 220 ms on ARM64 and 3,811 ms
on Ubuntu x86_64; the Job-shaped end-to-end Docker lifecycle was 430 ms and 4,245 ms respectively. Freeze-race maximum
per-round p99 was 0.010208 ms and 0.031733 ms. These scheduler, runtime and host observations are not portable contracts,
defaults or SLAs.

## Additional Benchmarks Executed

No locally executable planned variant was replaced by a future-work agreement:

- all four freeze-boundary candidates;
- all six receive/validate/queue/commit/ACK stages;
- 30 drain deadline/cost cells;
- 21 final-wait/request-class cells;
- conversion, validation and atomic-install failures;
- 1,000 concurrent late publishers;
- 30 new-epoch repetitions;
- 30 freeze races with 128 concurrent contenders (3,840 calls), with exactly one winner in every round;
- natural workload exit `17`, signal-driven exit `143`, and Job-shaped final wait preserving exit `17`;
- Job and CronJob shapes under both duration and scrape-count policies;
- fixed 2-CPU/256-MiB container envelope and environment fingerprint capture.

The two retained result directories are the matching-fingerprint ARM64 reference and Ubuntu x86_64 confirmation. No
superseded INV-020 evidence set remains.

## Lifecycle Contract and Acceptable Values

The matching-fingerprint evidence supports:

- freeze policy: bounded hybrid;
- closure order: stop new connections/admission first, then resolve already admitted work;
- drain population: only complete, validated and admitted operations; partial and merely received frames do not enter;
- drain time: capped by the existing remaining finalization/shutdown budget; never a new unbounded timeout;
- success ACK: only after commit;
- commit followed by lost ACK: final state includes the operation; client outcome is unknown;
- post-freeze mutation: deterministic rejection;
- freeze/install: exactly one successful final complete generation;
- finalization failure: explicit MetricShell-owned failure; invalid candidate never replaces prior Core active state;
- application metrics: immutable after freeze; self-metrics may continue;
- final wait: existing immediate/duration/scrapes modes and Core eligibility rules remain unchanged;
- external termination: no new post-exit wait; use only the remaining shutdown reserve;
- epoch: one empty managed registry per MetricShell/workload execution; no restoration;
- workload exit: natural `17` and signal `143` were preserved in the process-level checks;
- numeric drain/post-exit default: not selected. The tested `0–100 µs` synthetic drain budgets and `250 ms` final wait
  are coverage values, not product defaults; accepted ADR-003 budgets remain authoritative.

## Environment Fingerprint and Ubuntu Procedure

The portable fingerprint is:

```text
bbdde683d7c54d83ed82181634b639297d68e41ba2fb15ca4f350b5eaf36264f
```

`export-stand.sh` packages the exact source, runner, verifier and expected fingerprint. On Ubuntu, unpack it, run
`./verify-fingerprint.sh`, then `./run-bench.sh`. Portable assertion names and results, experiment coverage and
fingerprint must match exactly. Timings and architecture-specific image IDs are observations and need not match.

## Limitations and Better Benchmarking

The stand is intentionally contract-focused. It does not substitute for production Unix parsing, the production owner
queue, production Core validation/install or a real HTTP server/control plane. The Job/CronJob matrix does not prove
Prometheus discovered or persisted a sample. Both tested hosts use Docker Desktop/LinuxKit; native-kernel Ubuntu,
containerd and CRI-O remain outside the current evidence.

Repeat on native Linux with production modules, pinned CPU and at least 30 process-level
repetitions. Exercise maximum cardinality, queue saturation, slow partial sockets, ACK disconnects, CPU throttling,
cgroup memory pressure, slow/cancelled scrapers and randomized concurrent exit/signal races under the race detector.
Measure queue residence and final materialization/install distributions against an explicitly configured reserve.

## Conclusion

Matching-fingerprint macOS ARM64 and Ubuntu x86_64 evidence supports the bounded hybrid freeze boundary and composes it
with Core without changing Core semantics. It rejects unbounded full drain and a mandatory client handshake; immediate
freeze remains a bounded fallback but loses already admitted complete work. All application state becomes immutable
before final-wait entry, while self-metrics and Core-defined eligible scrape counting may continue.

ADR-020 accepts this lifecycle contract. INV-020 is **completed**, and the Managed Aggregation research sequence
INV-016–INV-020 is complete.
