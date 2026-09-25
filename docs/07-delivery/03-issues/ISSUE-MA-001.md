# ISSUE-MA-001. Managed mode configuration and bootstrap

**Status:** Planned  
**Readiness:** Code-ready

**Epic:** [EPIC-002 Managed Aggregation](../02-epics/EPIC-002-managed-aggregation.md)  
**Wave:** [Wave 1](../02-epics/EPIC-002-managed-aggregation.md#wave-1)

**ADR/INV:** ADR-016 / INV-016; ADR-020 / INV-020

## Normative inputs

Managed Aggregation requirements, ADR-016, ADR-020, Configuration Specification, Configuration Value Grammar.

## Dependencies

None; starts EPIC-002 after Core completion.

## Scope

Add explicit managed-registry mode selection while keeping snapshot mode as the default; select the managed bootstrap path only when configured; reject contradictory/hybrid ownership configuration before workload start.

## Out of scope

Managed Registry construction, empty epoch creation and registry semantics (ISSUE-MA-003); transport server, materialization, Core bridge and lifecycle drain/freeze implementation.

## Configuration and observable errors

The mode selector is the only public property owned by this task. Omitted or explicit snapshot mode selects the existing bootstrap path; explicit managed mode selects the managed bootstrap boundary. Unknown values, conflicting aliases and any configuration that requests simultaneous snapshot and managed ownership are startup configuration errors and fail before the workload starts. This task does not create a registry, epoch, socket or committed state, so failure cannot partially initialize any of them. The selected mode and a sanitized rejection reason are observable through the existing startup diagnostics.

## Acceptance criteria

- Default invocation remains snapshot mode and preserves existing behavior.
- Explicit snapshot mode is equivalent to the default.
- Explicit managed mode selects exactly one managed bootstrap path; ISSUE-MA-003 creates the empty registry epoch.
- Invalid/hybrid ownership configuration rejects deterministically before workload start where statically knowable.
- Snapshot-mode regression suite stays green.

## Required test matrix

Configuration table tests for default/snapshot/managed/unknown mode, precedence, contradictory options, bootstrap/no-listener assertions, snapshot regression suite.

## Completion

Complete when all acceptance criteria and required tests pass in CI and the task preserves ADR-016...ADR-020 and snapshot-mode backward compatibility.
