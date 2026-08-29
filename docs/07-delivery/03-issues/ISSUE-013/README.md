# ISSUE-013. Atomic last-valid state holder

**Status:** Done
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

## Delivery log

- 2026-08-28: moved to `In Progress`; the last-valid holder and linear installation boundary were implemented.
- 2026-08-28: moved to `Testing`; replacement, omission, lifetime binding, freeze, overflow, ownership and concurrent
  reader/writer tests passed under the race detector in Docker.
- 2026-08-28: moved to `Done`; readers now load one immutable old or new generation while writers install or reject a
  complete candidate atomically.

## Verification evidence

- Installation and freeze share one serialized boundary; active reads use one atomic pointer load.
- Accepted replacements receive consecutive generations and remove omitted families and series without merge or history.
- Frozen, lifetime type-conflicting and internal generation-overflow candidates preserve both content and generation.
- Empty families create no lifetime type binding, matching their zero-series normalization semantics.
- Returned active/validated representations remain caller-owned copies and concurrent stress passes the race detector.
- `implementation/README.md` and `implementation/README_RU.md` were reviewed and updated together.
