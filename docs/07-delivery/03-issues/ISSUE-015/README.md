# ISSUE-015. Separate self-metrics domain

**Status:** Open
**Readiness:** Code-ready

**Epic:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 3](../../02-epics/EPIC-001-core.md#wave-3)

## Normative inputs

- ADR-004, ADR-010, ADR-011, ADR-014.
- [Self-Metrics Specification](../../../04-specification/self-metrics.md).
- [Runtime State Machine](../../../04-specification/runtime-state-machine.md).

## Dependencies

ISSUE-007 lifecycle states, ISSUE-011 canonical model, and ISSUE-013 active-state holder.

## Scope

Implement the complete metricshell_ registry, metric types, HELP/TYPE metadata, all closed label enums, one-hot state
and
mode series, generation/publication/ingestion/exposition/final-wait/shutdown metrics, and lifecycle reset/update rules.

## Out of scope

Application labels or values, raw paths/IDs/error text, unbounded labels, application filtering of self-metrics, and any
effect on application snapshot identity.

## Configuration and errors

Self-metrics have no independent enable switch in version 1. They obey exposition.response_bytes and expose bounded
internal failure classes. A registry construction conflict is internal_failure and prevents workload start.

## Acceptance criteria

- Every metric and label value in the accepted specification exists with the declared type and semantics.
- The full one-hot state set matches the runtime state machine; exactly one state is 1.
- Self-metrics remain mutable while the application snapshot is frozen.
- Application candidate rejection, replacement, and filtering never add, remove, or rename self-metric series.
- Attacker-controlled strings never become labels.

## Required test matrix

Golden exposition for every lifecycle state; all transport/outcome/reason enums; zero and non-zero generations; final
wait modes and terminal reasons; concurrent updates/scrapes under the race detector; cardinality bound; reserved-name
rejection; filtering immunity; and process restart/reset.

## Completion

Complete when the whole normative registry is implemented, golden outputs and enum exhaustiveness tests pass, and
structured logging uses the same state/mode/outcome/reason values.
