# ISSUE-013. Atomic last-valid state holder

**Status:** Open
**Readiness:** Code-ready

**Epic:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 3](../../02-epics/EPIC-001-core.md#wave-3)

Concurrent readers see exactly one generation; rejection preserves last-valid state; omitted series disappear on
replacement.

## Code-ready contract

- **Normative inputs:** ADR-004 and
  ADR-014, [Application Snapshot Protocol](../../../04-specification/application-snapshot-protocol.md),
  and [Runtime State Machine](../../../04-specification/runtime-state-machine.md).
- **Dependencies:** ISSUE-011 and ISSUE-012.
- **Scope / out of scope:** Atomically install immutable validated snapshots with one monotonic generation. Out of
  scope: merge, history, replay, or per-producer state.
- **Configuration and observable failures:** Rejected/frozen candidates retain the prior pointer and generation;
  internal swap failures use the closed internal reason and structured event.
- **Acceptance criteria and required tests:** Concurrent readers/writers; replacement deletes omitted series;
  zero-series replacement; rejection retention; generation ordering; race detector and allocation ownership.
- **Completion:** Complete when readers can observe only complete old or complete new generations under stress.
