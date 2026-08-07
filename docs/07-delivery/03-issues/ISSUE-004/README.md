# ISSUE-004. Signal forwarding

**Status:** Open
**Readiness:** Code-ready

**Epic:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 1](../../02-epics/EPIC-001-core.md#wave-1)

Forward SIGTERM, SIGINT, SIGHUP, and selected operational signals. Define repeated-signal behavior and cover
startup/post-exit races.

## Code-ready contract

- **Normative inputs:** ADR-001, ADR-003 / INV-001,
  INV-003, [Runtime State Machine](../../../04-specification/runtime-state-machine.md),
  and [Structured Logging](../../../04-specification/structured-logging.md).
- **Dependencies:** ISSUE-002 and ISSUE-003.
- **Scope / out of scope:** Forward TERM, INT, HUP and documented operational signals with deterministic repeated-signal
  behavior. Out of scope: inventing workload-specific signal policy.
- **Configuration and observable failures:** Forwarding and ignored late signals are observable; an unsupported signal
  or failed target never panics and uses a bounded error path.
- **Acceptance criteria and required tests:** TERM/INT/HUP; repeated signals; signal before exec, during exit, and after
  reap; process-group disappearance; race detector.
- **Completion:** Complete when every supported signal has a deterministic state-dependent outcome and integration
  coverage.
