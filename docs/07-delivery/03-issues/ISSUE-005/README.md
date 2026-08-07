# ISSUE-005. Child reaping and orphan handling

**Status:** Open
**Readiness:** Code-ready

**Epic:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 1](../../02-epics/EPIC-001-core.md#wave-1)

Reap adopted descendants, track the primary workload PID separately, and prove zero zombies after stress tests.

## Code-ready contract

- **Normative inputs:** ADR-001 /
  INV-001, [Runtime State Machine](../../../04-specification/runtime-state-machine.md), [Self-Metrics](../../../04-specification/self-metrics.md),
  and [Structured Logging](../../../04-specification/structured-logging.md).
- **Dependencies:** ISSUE-002 and ISSUE-003.
- **Scope / out of scope:** Reap direct and adopted children while tracking the primary workload separately. Out of
  scope: supervising unrelated services.
- **Configuration and observable failures:** Unexpected child outcomes are sanitized diagnostics; primary outcome
  remains authoritative; child PIDs are excluded from metric labels.
- **Acceptance criteria and required tests:** Orphan adoption, double-fork, burst exits, primary-before-child and
  child-before-primary order, zero-zombie stress, race detector.
- **Completion:** Complete when stress fixtures leave zero zombies and exactly one primary workload result.
