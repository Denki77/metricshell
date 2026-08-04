# ISSUE-011. Canonical publication model

**Status:** Open
**Readiness:** Code-ready

**Epic:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 3](../../02-epics/EPIC-001-core.md#wave-3)

## Normative inputs

- ADR-004 / INV-004.
- [Application Snapshot Protocol](../../../04-specification/application-snapshot-protocol.md).
- [Runtime Defaults and Resource Limits](../../../04-specification/runtime-defaults-and-resource-limits.md).

## Dependencies

ISSUE-001 configuration/bootstrap. This issue blocks ISSUE-012, ISSUE-013, and all ingestion adapters.

## Scope

Implement immutable CandidateSnapshot, ValidatedSnapshot, and ActiveSnapshot representations; schema-version dispatch;
base-family/encoded-sample naming; family/series identity; counter, gauge, and classic-histogram values; empty-family
normalization and the explicit zero-series state; binary64 canonicalization; deterministic ordering; and generation metadata.

## Out of scope

Transport framing, exposition encoding, filtering, aggregation, per-producer state, operation replay, and history.

## Configuration and errors

Apply limits.snapshot_bytes, limits.series, limits.labels_per_series, metric/label/help byte limits, and the reserved
metricshell_ namespace. Return only rejection codes from the snapshot protocol registry; errors must not mutate active
state.

## Acceptance criteria

- Every version 1 document maps to one immutable candidate or one deterministic rejection.
- Canonical identity is independent of JSON member and input series ordering.
- Counter/gauge/histogram and zero-series semantics match the public schema exactly.
- Duplicate identity, metadata/type conflict, invalid histogram, unknown fields, and resource limits reject atomically.
- No mutable input buffer or caller-owned collection is retained after construction.

## Required test matrix

Golden JSON/canonical fixtures; zero-series and empty-family normalization; float64 overflow/underflow/rounding and
boundary collisions; negative histogram boundaries/sums; family component collisions and histogram `le`; duplicate
families/series/labels; histogram ordering and cumulative counts; reserved names; all limits at limit and limit+1; fuzzing;
and race-detector tests for concurrent readers.

## Completion

Complete when public protocol fixtures create deterministic immutable values, every rejection code is covered,
downstream
validator/state-holder tests consume these types, and documentation and conformance corpus are linked.
