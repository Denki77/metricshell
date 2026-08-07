# ISSUE-007. Runtime lifecycle state machine

**Status:** Open
**Readiness:** Code-ready

**Epic:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 2](../../02-epics/EPIC-001-core.md#wave-2)

**ADR/INV:** ADR-002 / INV-002.

Implement and test the exact public states from the accepted
[Runtime State Machine](../../../04-specification/runtime-state-machine.md): `initializing`, `starting_workload`,
`running`, `stopping`, `finalizing`, `final_wait`, `failed`, and `terminated`. Reject invalid transitions and run race
tests. Workload exit is an event and forced termination is an action, not additional public states.

## Code-ready contract

- **Normative inputs:** ADR-002 / INV-002 and the accepted Runtime State Machine, Self-Metrics, and Structured Logging
  specifications.
- **Dependencies:** ISSUE-001; it supplies lifecycle semantics to ISSUE-008, ISSUE-010, ISSUE-025, and ISSUE-026.
- **Scope / out of scope:** Implement only the eight public states and their transitions, probes, logs, and one-hot
  metric. Out of scope: additional public states.
- **Configuration and observable failures:** Invalid transitions fail deterministically, emit `runtime.failed` where
  terminal, and never expose two active state series.
- **Acceptance criteria and required tests:** Every valid/invalid transition; concurrent exit/signal/publication;
  one-hot metric; health/readiness table; race detector.
- **Completion:** Complete when one transition table drives runtime behavior, probes, logs, and tests.
