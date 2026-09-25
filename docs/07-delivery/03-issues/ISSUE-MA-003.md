# ISSUE-MA-003. Managed Registry and execution epoch

**Status:** Planned  
**Readiness:** Code-ready

**Epic:** [EPIC-002 Managed Aggregation](../02-epics/EPIC-002-managed-aggregation.md)  
**Wave:** [Wave 1](../02-epics/EPIC-002-managed-aggregation.md#wave-1)

**ADR/INV:** ADR-016 / INV-016; ADR-020 / INV-020

## Normative inputs

ADR-016, ADR-020, Managed Aggregation requirements.

## Dependencies

ISSUE-MA-002.

## Scope

Implement one in-memory Managed Registry per MetricShell/workload execution, family/series storage, committed generation tracking, deterministic mutation API and fresh empty epoch creation.

## Out of scope

Concurrency owner, transport, materialization cache, persistence/replay.

## Configuration and observable errors

Registry construction has no independent public configuration beyond the managed-mode boundary from ISSUE-MA-001. Failure to create the in-memory registry is a startup error before workload start. Each execution creates generation zero with no families or series; persistence, replay and reuse of a previous epoch are invalid states. Runtime semantic rejection returns the domain error from ISSUE-MA-002 and preserves the current generation and complete committed registry. Successful mutation advances generation exactly once; registry creation and reads do not. Generation transitions and rejected-mutation classes must be observable without exposing application label values.

## Acceptance criteria

- Accepted mutation advances generation exactly once.
- Rejected mutation preserves generation/state.
- Complete registry read corresponds to one generation.
- New execution starts empty.
- No persistence/replay/recovery path exists.
- Publisher disconnect does not delete unrelated state.

## Required test matrix

Registry unit tests for generation accounting, rejected mutations, empty epoch, restart/new instance, disconnect independence and complete-state reads.

## Completion

Complete when all acceptance criteria and required tests pass in CI and the task preserves ADR-016...ADR-020 and snapshot-mode backward compatibility.
