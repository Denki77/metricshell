# INV-014 Report — Security and Resource Limits

**Status:** completed
**Run dates:** 2026-08-02–2026-08-03
**Reference runs:** `results/20260802T173138Z`, `results/20260803T071713Z`
**Fingerprint:** `5a5066721a556bba9ce836e691757ceeefbab9eac05e3d6f5cab7c6ae027c1b3`
**Decision:** [ADR-014](../../docs/06-architecture/adr/ADR-014.md)

## Goal and ADR-004 Boundary

Select a bounded security posture for local producers and scrapers. Each publication is one complete candidate snapshot.
Syntax, policy and resource checks finish before atomic replacement; rejection never merges a candidate with active
state and never retains omitted series from an older snapshot.

## Threat Model

An untrusted local process may send malformed, oversized, high-cardinality, label-heavy, slow or concurrent complete
candidates. A scraper may open slow or excess connections. The attacker does not have Docker control, host root, kernel
control or permission to expose the endpoint externally. Instrumentation owners remain responsible for secrets placed
inside otherwise allowed metric values.

## Prototype

The Go research server exposes local ingestion and metrics, stores an immutable complete snapshot behind an atomic
pointer and enforces payload, series, label, concurrency and timeout bounds. Docker supplies user, filesystem,
capability, descriptor and memory isolation. The runner covers normal replacement, validation failures, overload, slow
clients, malformed-input repetition, bind failure and an intentional bounded cgroup OOM.

## Run Environments

| Environment                       | Date       | Docker | Architecture | Result                     | Status                |
|-----------------------------------|------------|-------:|--------------|----------------------------|-----------------------|
| Docker Desktop on macOS/LinuxKit  | 2026-08-02 | 29.6.2 | aarch64      | `results/20260802T173138Z` | 27/27 assertions pass |
| Docker Desktop on Ubuntu/LinuxKit | 2026-08-03 | 27.4.0 | x86_64       | `results/20260803T071713Z` | 27/27 assertions pass |

Both runs have the same fingerprint and all 27 assertions passed in each environment. Both container environments use
LinuxKit; native non-LinuxKit Linux and deployment-specific mandatory-access-control behavior are not covered.

## Results

### Complete replacement and validation

A two-series snapshot was replaced by a one-series snapshot. The omitted `alpha` series disappeared and only `beta 7`
remained; no values were added across snapshots. A later malformed candidate returned HTTP 400 and did not change the
last valid state. Duplicate series and secret-like label names returned 400, label/cardinality violations returned 422,
and a payload above 64 KiB returned 413.

### Concurrency and hostile clients

In both environments, 12 simultaneous held publications produced the same aggregate outcome: 4 accepted requests and 8
HTTP 429 responses. Request ordering differed, as expected, but the configured concurrency ceiling did not. A partial
slow request and 100 malformed requests left health available and did not mutate the active snapshot.

| Observation                      | macOS/LinuxKit | Ubuntu/LinuxKit |
|----------------------------------|---------------:|----------------:|
| Concurrent publications accepted |              4 |               4 |
| Concurrent publications rejected |              8 |               8 |
| Private path mode/UID/GID        |    700:100:101 |     700:100:101 |

### Container and resource controls

| Control           | Tested value         | Result              |
|-------------------|----------------------|---------------------|
| UID               | non-zero             | pass in both runs   |
| root filesystem   | read-only            | pass in both runs   |
| capabilities      | drop ALL             | pass in both runs   |
| no-new-privileges | enabled              | pass in both runs   |
| host binding      | 127.0.0.1            | pass in both runs   |
| open files        | 64                   | pass in both runs   |
| normal memory     | 64 MiB               | pass in both runs   |
| private path      | mode 0700, app-owned | pass in both runs   |
| invalid bind      | internal exit 70     | pass in both runs   |
| forced allocation | 128 MiB under 32 MiB | OOMKilled, exit 137 |

## Hypothesis Evaluation

### Whole-candidate bounds preserve metric-state integrity

Confirmed. Every syntax, policy and resource rejection preserved the previous complete snapshot. No failure caused
partial addition, summation or stale-series retention.

### Excess concurrency should fail fast

Confirmed. A bounded semaphore admitted four requests and rejected excess work with 429 rather than creating an
unbounded queue. The same aggregate behavior appeared in both architectures.

### Container hardening should be the default

Confirmed for the tested local server. Non-root, read-only rootfs, no-new-privileges and dropped capabilities did not
prevent normal operation.

### External resource enforcement is required

Confirmed. The controlled allocation exceeded the 32 MiB cgroup bound and terminated as OOMKilled/137 without changing
the validity of the earlier evidence.

## Accepted Values and Policies

- Reject a whole candidate on syntax, duplicate, secret policy or any configured bound violation.
- Preserve the last valid complete snapshot on rejection.
- Use 400 for malformed/policy input, 413 for payload size, 422 for cardinality/label bounds and 429 for concurrency.
- Keep ingestion local by default and publish container ports on loopback only.
- Run non-root with read-only rootfs, `no-new-privileges`, no Linux capabilities and private runtime paths.
- Configure explicit payload, series, labels, concurrency, HTTP timeout, FD, PID and memory limits.
- Do not copy attacker-controlled rejected labels into self-metrics.
- Treat 64 KiB, 1,000 series, 8 labels, 64-byte label fields, concurrency 4, 64 FDs and 64 MiB as tested research
  starting values, not universal production capacity limits.

## Limitations

- Both container environments use LinuxKit; native non-LinuxKit Linux was not tested.
- The parser is intentionally smaller than the production Prometheus/OpenMetrics validator.
- Secret detection is conservative and name-based, not a data-loss-prevention guarantee.
- TLS, authentication, Unix socket ACLs and custom SELinux/AppArmor/seccomp profiles were not implemented.
- One normal memory/FD setting and one OOM shape were tested.
- The threat model excludes Docker control, host root and kernel compromise.

## Additional Benchmarking

| Benchmark                                      | Status    | Evidence/Boundary                       |
|------------------------------------------------|-----------|-----------------------------------------|
| ADR-004 complete replacement/no sum            | covered   | replacement response and assertions     |
| malformed/duplicate/last-valid retention       | covered   | HTTP codes and snapshot checks          |
| payload/series/label limits                    | covered   | assertions                              |
| concurrency ceiling                            | covered   | 4 accepted, 8 rejected in both runs     |
| slow body and malformed-input repetition       | covered   | health and state assertions             |
| non-root/read-only/NNP/cap-drop                | covered   | container assertions                    |
| loopback/FD/memory/path permissions            | covered   | environment and assertions              |
| bind failure and cgroup OOM                    | covered   | exit 70; OOMKilled/137                  |
| matching Ubuntu fingerprint                    | covered   | 27/27 assertions, identical fingerprint |
| full production parser corpus and fuzz/soak    | follow-up | production implementation               |
| Unix ACL and SELinux/AppArmor/seccomp profiles | follow-up | deployment-specific validation          |

## Conclusion

INV-014 is confirmed in matching macOS/LinuxKit and Ubuntu/LinuxKit environments. MetricShell uses local non-root
operation, immutable complete-snapshot replacement, fail-fast concurrency and explicit whole-candidate, timeout and
cgroup bounds. No tested failure corrupted active metric state. The decision is recorded in
[ADR-014](../../docs/06-architecture/adr/ADR-014.md).

## Decision Output

- macOS evidence: `results/20260802T173138Z/`
- Ubuntu evidence: `results/20260803T071713Z/`
- ADR: [ADR-014](../../docs/06-architecture/adr/ADR-014.md)
