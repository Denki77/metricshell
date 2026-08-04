# ISSUE-009. Termination escalation

**Status:** Open
**Readiness:** Code-ready

**Epic:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 2](../../02-epics/EPIC-001-core.md#wave-2)

Graceful signal, bounded wait, then forced process-group kill. No descendant may outlive MetricShell.

## Code-ready contract

- **Normative inputs:** ADR-003 /
  INV-003, [Runtime State Machine](../../../04-specification/runtime-state-machine.md), [Runtime Defaults](../../../04-specification/runtime-defaults-and-resource-limits.md),
  and [Structured Logging](../../../04-specification/structured-logging.md).
- **Dependencies:** ISSUE-003, ISSUE-004, and ISSUE-008.
- **Scope / out of scope:** Send the graceful signal, wait within the derived budget, then force the owned process
  group. Out of scope: unbounded retries.
- **Configuration and observable failures:** Escalation emits the normative forwarding/forced events; missing processes
  are idempotent; saved workload result follows ISSUE-006.
- **Acceptance criteria and required tests:** Cooperative, ignoring and fork-after-signal workloads; zero remaining
  budget; repeated signal; disappearing group; no surviving descendant.
- **Completion:** Complete when every termination path finishes inside the budget with no descendant left behind.
