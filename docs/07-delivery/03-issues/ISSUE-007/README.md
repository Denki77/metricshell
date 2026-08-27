# ISSUE-007. Runtime lifecycle state machine

**Status:** Done
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

## Delivery log

- 2026-08-24: moved to `In Progress`; the accepted state/event table was mapped into a dedicated lifecycle package.
- 2026-08-24: moved to `Testing`; valid/invalid transition, one-hot, probe-table, concurrent exit/termination and
  Docker supervisor tests passed under the race detector.
- 2026-08-24: moved to `Done`; runtime behavior now emits each effective public transition from the shared machine.

## Verification evidence

- The closed eight-state and twelve-event registries are represented by typed constants and one normative transition
  table; every `(state, event)` pair has exactly one target.
- Invalid transitions leave state unchanged and return deterministic errors; state-dependent concurrent events are
  resolved while holding the machine lock.
- Spawn timing and final-wait policy use contextual events, so runtime code only submits events and cannot select a
  target outside the machine. Tests execute every normative row through `TransitionEvent()` and reject ambiguous tables.
- `metricshell_runtime_state` data is exposed as a full one-hot vector with exactly one active state.
- Runtime logs now include `runtime.initializing` and exactly one `runtime.state_changed` record after every effective
  transition, including the initial state without `previous_state`.
- `make ci IMAGE=metricshell-wave2` passed entirely through Docker, including `go test -race` and all Wave 1 fixtures.
- `implementation/README.md` and `implementation/README_RU.md` were reviewed and updated together.
