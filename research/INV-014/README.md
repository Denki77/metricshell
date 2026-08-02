# INV-014 — Security and Resource Limits

**Status:** in progress
**macOS reference run:** `results/20260802T173138Z`
**Ubuntu/LinuxKit run:** pending
**Report:** [report.md](report.md)

## Question

Which security posture and resource limits prevent an untrusted local producer or scraper from destabilizing
MetricShell while preserving ADR-004 complete-snapshot replacement?

## Context and Hypotheses

MetricShell accepts complete candidate snapshots, validates them before mutation and atomically replaces the last valid
snapshot. Limits must therefore apply to the entire candidate; rejected payloads must never partially add or retain
series. The default container posture should be non-root, loopback-published, read-only, capability-free and bounded by
payload, cardinality, labels, concurrency, descriptors, memory and timeouts.

## Evidence Required

- complete replacement and omission-based stale-series removal;
- malformed/duplicate rejection with last-valid retention;
- payload, series, label-count and label-length limits;
- secret-like label policy and malicious-input survival;
- concurrency rejection and slow-client timeouts;
- non-root, loopback binding, read-only rootfs, no-new-privileges and dropped capabilities;
- FD, memory and private-path limits;
- endpoint bind failure and controlled cgroup OOM;
- matching Ubuntu fingerprint.

## Current Result

The macOS Docker/LinuxKit run passed 27/27 assertions. A two-series snapshot was replaced by a one-series snapshot and
the omitted series disappeared; no values were summed. Malformed, duplicate, secret-like, over-label, over-series and
oversized candidates were rejected while the last valid snapshot remained available.

Under 12 simultaneous held requests, 4 were accepted and 8 received HTTP 429. The server survived a partial slow body
and 100 malformed producer requests. The container ran non-root with a read-only rootfs, no-new-privileges, all
capabilities dropped, 64 MiB memory, 64 FDs and private mode 0700. A 128 MiB touched allocation under 32 MiB was
confirmed as `OOMKilled=true`, exit 137.

## Provisional Admissible Values

- complete candidate payload: 64 KiB default research bound;
- active application cardinality: 1,000 series;
- labels per series: 8; label name/value: 64 bytes;
- concurrent ingestion: 4; excess receives 429 without mutation;
- HTTP read-header/read/write/idle timeouts: bounded, with the prototype using 0.5/1.2/2/2 seconds;
- open descriptors: 64 research container limit;
- memory: 64 MiB normal research limit with cgroup enforcement;
- runtime: non-root, read-only rootfs, `no-new-privileges`, all Linux capabilities dropped;
- host publication: loopback by default;
- private runtime path: mode 0700;
- failure policy: whole-candidate rejection and last-valid retention;
- secrets: secret-like label names are rejected in the research policy; production needs configurable allow/deny rules.

## Running the Prototype

```bash
./research/INV-014/run-bench.sh
latest="$(cat research/INV-014/latest-results.txt)"
cat "$latest/assertions.tsv"
cat "$latest/concurrency.tsv"
cat "$latest/observations.tsv"
cat "$latest/after-replacement.metrics"
cat "$latest/environment.tsv"
```

The OOM case is intentional and restricted to a named 32 MiB test container. macOS and Ubuntu use the same runner and
fingerprint.

## Threat Model

An untrusted process with access to a local ingestion endpoint may send malformed, large, high-cardinality, label-heavy,
slow or concurrent complete candidates. A scraper may open slow or excess connections. The model does not grant the
attacker Docker control, host root, kernel compromise or access to an externally exposed endpoint. Secrets inside metric
values cannot be reliably inferred; instrumentation policy remains an application responsibility.

## Prototype Limits

- Simplified structural parser, not full Prometheus/OpenMetrics validation.
- Secret detection is deliberately conservative and name-based, not data-loss-prevention proof.
- Host loopback publication is tested through Docker; Unix socket ownership and SELinux/AppArmor profiles need
  deployment-specific tests.
- One OOM size and one FD/memory configuration were tested.
- LinuxKit aarch64 only until Ubuntu confirmation.

## Additional Benchmarks

| Benchmark                                      | Status                              |
|------------------------------------------------|-------------------------------------|
| ADR-004 replacement/no sum                     | covered                             |
| malformed/duplicate/last-valid                 | covered                             |
| payload/series/label limits                    | covered                             |
| concurrency 429                                | covered: 4 accepted, 8 rejected     |
| slow body and malformed fuzz                   | covered                             |
| non-root/read-only/NNP/cap-drop                | covered                             |
| loopback/FD/memory/path permissions            | covered                             |
| bind failure                                   | covered: exit 70                    |
| cgroup OOM                                     | covered: 32 MiB limit, exit 137     |
| Ubuntu matching fingerprint                    | pending                             |
| Unix socket/file ACL, SELinux/AppArmor/seccomp | deployment follow-up                |
| full parser corpus and property fuzzing        | production implementation follow-up |

## Better Follow-up Benchmarking

Repeat unchanged on Ubuntu, then exercise a full production parser corpus, long fuzz/soak runs, cgroup v2 `memory.high`,
several FD/PID limits and deployment MAC policies. Every failure test must assert last-valid complete-snapshot
retention.

## Decision Output

- Prototype: `prototype/`
- macOS evidence: `results/20260802T173138Z/`
- Ubuntu evidence: pending
- Report: [report.md](report.md)
- ADR: pending
