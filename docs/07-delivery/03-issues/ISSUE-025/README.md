# ISSUE-025. Finalization ingestion barrier

**Status:** Open
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
