# ISSUE-015. Separate self-metrics domain

**Status:** Done
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

## Delivery log

- 2026-08-29: moved to `In Progress`; the bounded self-metric registry, update domains and immutable scrape views were
  implemented.
- 2026-08-29: moved to `Testing`; registry exhaustiveness, golden text formats, lifecycle/mode one-hot, label rejection,
  generation projection, histogram, freeze, restart and concurrent scrape/update tests passed in Docker.
- 2026-08-29: moved to `Done`; the complete transport-independent Wave 3 state core passed the explicit `make wave3`
  gate.

## Verification evidence

- All 38 normative families and their 187 bounded initial series exist with HELP/TYPE metadata and closed label values;
  application rejection reasons and lifecycle states are consumed from their authoritative registries.
- Registry updates reject unknown metrics, labels, enum values, non-finite/negative values and semantic gauge violations
  without creating series.
- Runtime state and final-wait mode are atomic full one-hot vectors; active generation, series and canonical bytes update
  under one registry lock.
- Self-metrics remain mutable after the application holder freezes, while process restart reconstructs all counters and
  gauges from their normative defaults.
- Prometheus and OpenMetrics encoders preserve counter-family naming, escape labels, emit all metadata and produce
  cumulative shutdown histograms including `+Inf`.
- Concurrent readers/updates pass the race detector, returned views own their labels/buckets, and cardinality remains
  fixed.
- `implementation/README.md` and `implementation/README_RU.md` were reviewed and updated together.
