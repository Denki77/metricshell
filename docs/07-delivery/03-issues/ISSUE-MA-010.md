# ISSUE-MA-010. Core candidate bridge and atomic installation

**Status:** Planned  
**Readiness:** Code-ready

**Epic:** [EPIC-002 Managed Aggregation](../02-epics/EPIC-002-managed-aggregation.md)  
**Wave:** [Wave 5](../02-epics/EPIC-002-managed-aggregation.md#wave-5)

**ADR/INV:** ADR-019 / INV-019; ADR-020 / INV-020; ADR-004 / INV-004

## Normative inputs

ADR-004, ADR-019, ADR-020, Application Snapshot Protocol.

## Dependencies

ISSUE-MA-009.

## Scope

Convert one complete managed generation into the existing Core candidate representation and reuse production whole-candidate validation, atomic active-state install and exposition path.

## Out of scope

Managed-specific Core store/path, direct registry exposition, changed Core semantics.

## Configuration and observable errors

The bridge introduces no managed-specific Core configuration or alternate store. Conversion errors and existing Core validation/installation errors reject the whole candidate; no partial family or series becomes active and the previous Core state remains authoritative. Only successful completion of the existing atomic Core install changes active state. Candidate generation, conversion result, validation rejection and install result are observable through bounded internal diagnostics, without exposing a second managed exposition path. Existing Core size/parse constraints apply unchanged rather than being silently relaxed.

## Acceptance criteria

- Managed generation produces a self-contained complete candidate.
- Candidate uses the same validator as snapshot mode.
- Successful install uses the same atomic Core state holder.
- Invalid candidate preserves prior active state.
- Exposition has no separate managed path.
- ADR-004 semantics remain unchanged.

## Required test matrix

Managed-to-Core conformance corpus; valid/invalid conversion; validator failures; atomic replacement; concurrent scrapes; equivalent snapshot exposition comparison.

## Completion

Complete when all acceptance criteria and required tests pass in CI and the task preserves ADR-016...ADR-020 and snapshot-mode backward compatibility.
