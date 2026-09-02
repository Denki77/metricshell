# ISSUE-025. Finalization ingestion barrier

**Status:** Done
**Readiness:** Code-ready

**Epic:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 5](../../02-epics/EPIC-001-core.md#wave-5)

**ADR/INV:** ADR-011 / INV-011. Close publication before final wait with explicit ordering for accepted in-flight work.

## Code-ready contract

- **Normative inputs:** ADR-011 / INV-011, [Runtime State Machine](../../../04-specification/runtime-state-machine.md),
  and [Application Snapshot Protocol](../../../04-specification/application-snapshot-protocol.md).
- **Dependencies:** ISSUE-007, ISSUE-013, and ISSUE-016.
- **Scope / out of scope:** Close admission at finalization, define ordering for already admitted candidates, freeze one
  final generation, and reject later publications. Out of scope: merging late data.
- **Configuration and observable failures:** `frozen` uses the shared mapping; in-flight acceptance is linearized before
  or after the barrier and is visible in logs/metrics.
- **Acceptance criteria and required tests:** Publication before/at/after barrier; queued and validating candidate; all
  adapters; concurrent workload exit; generation freeze; race detector.
- **Completion:** Complete when every schedule produces one deterministic frozen generation and no post-barrier
  mutation.

## Delivery log

- 2026-09-02: moved to `In Progress`; added a linearized Core admission barrier, bounded finalization wait and
  exactly-once final snapshot freeze shared by every transport.
- 2026-09-02: moved to `Testing`; executing, queued, timeout, repeated-close and post-barrier file/Unix/HTTP schedules
  passed under the Docker race detector.
- 2026-09-02: moved to `Done`; frozen publications update the shared rejection metrics/diagnostics, Docker
  vet/race/dependency/multi-architecture checks and the EN/RU README audit passed.
- 2026-09-08: corrective follow-up moved to `Testing` and back to `Done`; production runtime now creates the shared
  `ingestion.Core`, routes the selected real transport through it, and freezes via `Core.CloseAndFreeze(ctx)`.

## Verification evidence

- Admission and closure share one synchronization boundary, so every publication is deterministically before or after
  the barrier.
- Work admitted before closure may install within the supplied context budget; budget expiry freezes the preceding
  last-valid generation and makes a later install fail with `frozen`.
- Every call observes the same final generation, and all three transport identities reject post-barrier publications.
- `make test IMAGE=metricshell-issue025-core-wiring`
- `make integration IMAGE=metricshell-issue025-core-wiring`
