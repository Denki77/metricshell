# INV-014 Report — Security and Resource Limits

**Status:** in progress
**Run date:** 2026-08-02
**Reference run:** `results/20260802T173138Z`
**Platform:** Docker 29.6.2, LinuxKit aarch64
**Fingerprint:** `5a5066721a556bba9ce836e691757ceeefbab9eac05e3d6f5cab7c6ae027c1b3`

## Goal and ADR-004 Boundary

Select a bounded default security posture. Each request carries one complete snapshot. Validation and all resource
checks
finish before atomic replacement; rejection never merges a candidate with active state.

## Prototype and Environment

The Go server exposes local ingestion and metrics, holds an immutable snapshot behind an atomic pointer and implements
payload/series/label/concurrency/time limits. Docker supplies identity, filesystem, capabilities, descriptor, PID and
memory isolation. The runner performs normal, overload, slow, fuzz, bind-failure and OOM cases.

## Results

All 27 assertions passed. Replacement removed an omitted `alpha` series and retained only `beta 7`. A later malformed
candidate returned 400 and did not change `beta`. Duplicate series and secret-like labels returned 400; excessive labels
or cardinality returned 422; a payload above 64 KiB returned 413.

The concurrency semaphore admitted 4 held publications and rejected 8 with 429. A two-second partial request did not
make the service unavailable. One hundred malformed requests also left health available.

| Control           |         Tested value | Result              |
|-------------------|---------------------:|---------------------|
| UID               |             non-zero | pass                |
| root filesystem   |            read-only | pass                |
| capabilities      |             drop ALL | pass                |
| no-new-privileges |              enabled | pass                |
| host binding      |            127.0.0.1 | pass                |
| open files        |                   64 | pass                |
| container memory  |               64 MiB | pass                |
| runtime path      |      0700, app-owned | pass                |
| invalid bind      |              exit 70 | pass                |
| forced allocation | 128 MiB under 32 MiB | OOMKilled, exit 137 |

## Hypothesis Evaluation

Whole-candidate limits are sufficient to preserve state integrity: supported in the tested parser. Concurrency must fail
fast instead of queueing without bound: supported. Container isolation should be default: supported without preventing
the server from operating. Resource exhaustion must be externally bounded: cgroup OOM behaved deterministically.

## Failure Policy and Provisional Defaults

Reject the entire candidate on syntax, duplicate, secret policy or any bound violation. Keep the last valid complete
snapshot. Return 400 for malformed/policy input, 413 for payload, 422 for cardinality/label bounds and 429 for
concurrency.
Expose rejection self-metrics without copying rejected labels. Use loopback/local endpoints, non-root, read-only rootfs,
no-new-privileges, no capabilities, private paths and explicit FD/PID/memory limits.

The prototype's 64 KiB, 1,000 series, 8 labels, 64-byte label field, 4 concurrent requests, 64 FDs and 64 MiB are
admissible research defaults, not final capacity limits.

## Limitations and Additional Benchmarking

The parser is intentionally smaller than production Prometheus validation. There is no TLS, authentication, Unix ACL,
SELinux/AppArmor/seccomp custom profile or distributed attacker model. All locally possible listed profiles were run;
Ubuntu, production parser fuzzing, long soak and deployment MAC tests remain.

## Conclusion

The provisional posture is non-root local-only operation with immutable complete-snapshot replacement and explicit
whole-candidate, concurrency, timeout and cgroup limits. No tested failure corrupted the active snapshot. Completion
awaits Ubuntu and production-parser validation.

## Decision Output

- Evidence: `results/20260802T173138Z/`
- Ubuntu/ADR: pending
