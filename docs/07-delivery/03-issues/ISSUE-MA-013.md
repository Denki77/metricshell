# ISSUE-MA-013. Managed self-metrics and structured diagnostics

**Status:** Planned  
**Readiness:** Code-ready

**Epic:** [EPIC-002 Managed Aggregation](../02-epics/EPIC-002-managed-aggregation.md)  
**Wave:** [Wave 6](../02-epics/EPIC-002-managed-aggregation.md#wave-6)

**ADR/INV:** ADR-017 / INV-017; ADR-019 / INV-019; ADR-020 / INV-020

## Normative inputs

NFR-MA-006, Self-Metrics Specification, Structured Logging Specification, ADR-017, ADR-019, ADR-020.

## Dependencies

ISSUE-MA-008 and ISSUE-MA-012.

## Scope

Add bounded managed-mode self-metrics/logs for accepted/rejected operations, rejection class, overload, queue depth/capacity, active series, generations, materialization/cache, protocol/server failures, freeze and finalization.

## Out of scope

Arbitrary application metric names/labels or per-client identifiers in self-metric labels.

## Configuration and observable errors

No application-controlled metric name, label value, client identity, socket path or raw payload may become a self-metric label. Any optional diagnostic level/sink follows the existing logging configuration; invalid enum/sink configuration fails before workload start. Unknown rejection/result enum values are implementation errors and must not be collapsed into success. Metrics/logs distinguish protocol, semantic, resource, overload, late, unknown-client-outcome, materialization and final-install results; diagnostic emission failure must not mutate registry/Core state or change an operation result. Cardinality bounds, redaction and stable enum schemas are testable contracts.

## Acceptance criteria

- Semantic/protocol/overload/resource rejection are distinguishable.
- Queue/resource state is observable without application cardinality leakage.
- Materialization/cache/lifecycle transitions have stable metrics/logs.
- Redaction rules remain intact.
- Snapshot-mode observability does not change unintentionally.

## Required test matrix

Metric registry golden tests; bounded-label audit; log schema/enums; rejection/lifecycle paths; redaction; snapshot regression.

## Completion

Complete when all acceptance criteria and required tests pass in CI and the task preserves ADR-016...ADR-020 and snapshot-mode backward compatibility.
