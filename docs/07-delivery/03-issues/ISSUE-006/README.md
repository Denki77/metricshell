# ISSUE-006. Workload result preservation

**Status:** Open
**Readiness:** Code-ready

**Epic:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 1](../../02-epics/EPIC-001-core.md#wave-1)

Preserve exit 0, non-zero exit, and signal termination distinctly. Final-scrape timeout must not silently replace the
workload result. Supervisor failures use a separate exit-code range.

## Code-ready contract

- **Normative inputs:** ADR-001, ADR-002 / INV-001,
  INV-002, [Configuration](../../../04-specification/configuration.md),
  and [Runtime State Machine](../../../04-specification/runtime-state-machine.md).
- **Dependencies:** ISSUE-002 and ISSUE-005.
- **Scope / out of scope:** Preserve exit 0, non-zero and signal outcomes through post-exit work. Out of scope:
  remapping a started workload result to a MetricShell-owned code.
- **Configuration and observable failures:** Pre-start failures use the closed MetricShell exit registry; post-start
  diagnostics record origin without replacing the obtained workload result.
- **Acceptance criteria and required tests:** All byte-sized exits including registry collisions; TERM/INT mapping;
  final-wait timeout; internal error before/after workload start.
- **Completion:** Complete when the exit propagation matrix is table-driven and passes container integration tests.
