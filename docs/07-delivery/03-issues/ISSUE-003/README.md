# ISSUE-003. Owned process group/session

**Status:** Open
**Readiness:** Code-ready

**Epic:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 1](../../02-epics/EPIC-001-core.md#wave-1)

Place the workload and descendants in an owned process group. Signals must target the workload tree without affecting
unrelated processes. Test child and grandchild processes.

## Code-ready contract

- **Normative inputs:** ADR-001 / INV-001, [Runtime State Machine](../../../04-specification/runtime-state-machine.md),
  and [Structured Logging](../../../04-specification/structured-logging.md).
- **Dependencies:** ISSUE-002.
- **Scope / out of scope:** Create and own the workload process group/session and target only that tree. Out of scope:
  unrelated container processes and Kubernetes pod-wide signaling.
- **Configuration and observable failures:** Group-creation or signaling failures emit sanitized structured errors; no
  process identifier becomes a metric label.
- **Acceptance criteria and required tests:** Child/grandchild tree; unrelated sibling process; rapid exit during setup;
  group signal delivery; race detector and zombie check.
- **Completion:** Complete when descendants are controllable as one tree and unrelated processes remain unaffected in
  every integration fixture.
