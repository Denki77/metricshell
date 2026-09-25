# Managed Aggregation Architecture Investigation

> Status: Planned
> Purpose: Evaluate the architecture of the optional Managed Aggregation extension before ADRs and implementation
> Scope: managed registry semantics, concurrent publishers, legacy-client viability, performance/resource limits, lifecycle and Core integration
> Depends on: completed MetricShell Core investigation INV-001–INV-015, accepted ADR-001–ADR-015, Managed Aggregation requirements and draft specification

## 1. Purpose

This document starts a new architecture-investigation cycle for the optional Managed Aggregation extension.

The completed MetricShell Core architecture is not reopened by this investigation. The extension is evaluated as a new upstream producer of complete application snapshots for the existing Core boundary.

The investigation must determine whether the required managed-registry behavior can be implemented while preserving the existing product constraints:

- one MetricShell product;
- one executable binary;
- one container-image integration layer;
- one runtime process supervising one logical workload execution;
- snapshot mode remains the default;
- managed aggregation is opt-in;
- no required sidecar, central daemon or external aggregation service;
- existing Core lifecycle, snapshot, exposition and final-scrape contracts remain valid.

## 2. Research Method

Each investigation follows the same evidence sequence used by MetricShell Core:

1. **Question** — one concrete architecture question.
2. **Context** — why it matters and which requirements/specification clauses it affects.
3. **Candidates** — realistic alternatives, including intentionally simple options.
4. **Initial hypotheses** — falsifiable expectations stated before experimentation.
5. **Evidence required** — documentation, prototype, integration test, fault injection, benchmark or source inspection.
6. **Experiments** — reproducible environment, workload, inputs, repetitions and measurements.
7. **Assertions** — portable correctness conditions that must pass independently of timing.
8. **Observations** — environment-sensitive measurements and behavior.
9. **Evaluation criteria** — criteria fixed before results.
10. **Results** — retained raw evidence.
11. **Conclusion** — accept, reject, retain as fallback, postpone or declare insufficient evidence.
12. **Decision output** — ADR, specification update, requirement update, benchmark record or follow-up research.

Assertions must represent semantic or safety properties. Timing, throughput, CPU, RSS and scheduler behavior are observations unless a requirement defines a hard bound.

## 3. Existing Architectural Boundary

The accepted Core boundary remains:

```text
complete candidate snapshot
→ whole-candidate validation
→ atomic active-state replacement
→ Prometheus/OpenMetrics exposition
```

Managed Aggregation may add only an upstream path:

```text
instrumentation operations
→ managed registry
→ complete candidate snapshot
→ existing Core
```

The investigation must not silently convert Core into an operation log, event store, distributed registry or merge engine.

## 4. Investigation Order

1. [INV-016 — Managed Registry Semantics](managed-aggregation-architecture-research.md#inv-016)
2. [INV-017 — Concurrent Publishers and Ordering](managed-aggregation-architecture-research.md#inv-017)
3. [INV-018 — Legacy Client and Transport Viability](managed-aggregation-architecture-research.md#inv-018)
4. [INV-019 — Performance, Snapshot Materialization and Resource Limits](managed-aggregation-architecture-research.md#inv-019)
5. [INV-020 — Lifecycle and Core Integration](managed-aggregation-architecture-research.md#inv-020)

INV-016 must complete first because the later investigations need an explicit semantic model to test.

INV-017 and INV-018 may then proceed in parallel once INV-016 fixes the operation and ownership semantics sufficiently for prototypes.

INV-019 depends on a representative candidate implementation from INV-016/017.

INV-020 validates the full lifecycle integration and must use the selected semantics from the earlier investigations.

## 5. Investigation Tracking

| ID      | Topic                                                                                         | Status      | Evidence            | Decision                                     |
|---------|-----------------------------------------------------------------------------------------------|-------------|---------------------|----------------------------------------------|
| INV-016 | [Managed Registry Semantics](managed-aggregation-architecture-research.md#inv-016)            | Completed   | `research/INV-016/` | [ADR-016](../06-architecture/adr/ADR-016.md) |
| INV-017 | [Concurrent Publishers and Ordering](managed-aggregation-architecture-research.md#inv-017)    | Completed   | `research/INV-017/` | [ADR-017](../06-architecture/adr/ADR-017.md) |
| INV-018 | [Legacy Client and Transport Viability](managed-aggregation-architecture-research.md#inv-018) | Completed   | `research/INV-018/` | [ADR-018](../06-architecture/adr/ADR-018.md) |
| INV-019 | [Performance and Resource Limits](managed-aggregation-architecture-research.md#inv-019)       | Completed   | `research/INV-019/` | [ADR-019](../06-architecture/adr/ADR-019.md) |
| INV-020 | [Lifecycle and Core Integration](managed-aggregation-architecture-research.md#inv-020)        | In progress | `research/INV-020/` | ADR-020 / lifecycle specification update     |

ADR numbering is provisional until each investigation proves that a separate decision record is warranted. Multiple investigations may feed one ADR, and one investigation may produce multiple ADRs if independent decisions require it.

## 6. Cross-Investigation Invariants

The following are input constraints, not research conclusions:

- snapshot mode remains the default;
- managed aggregation is explicitly enabled;
- Core remains independently usable without managed aggregation;
- one managed registry belongs to one logical workload execution;
- independent applications do not share one managed registry;
- a new MetricShell execution starts a new managed-registry epoch by default;
- persistence/replay across MetricShell restarts is not required;
- managed state must enter Core as a complete application snapshot;
- invalid managed input must not corrupt previously valid state;
- the extension must remain inside the same MetricShell binary/process model;
- no hybrid external-snapshot-plus-managed-operation ownership is required in the initial extension;
- no investigation may assume guaranteed storage of every intermediate operation or snapshot unless the product scope is explicitly changed.

If evidence shows one of these constraints is impossible or materially harmful, the result is not permission to violate it silently: the investigation must stop and propose a requirements/scope revision.

## 7. Evidence Discipline

Every `research/INV-016` through `research/INV-020` package should retain, where applicable:

- README describing the question and runner;
- prototype source;
- deterministic assertions;
- raw observations/results;
- environment fingerprint;
- commit SHA and benchmark-scope fingerprint;
- command needed to reproduce;
- report separating facts from interpretation;
- explicit links back to affected requirements/specification clauses.

Environment-sensitive results must not be presented as portable guarantees.

Negative results are first-class evidence and must be retained when they reject an attractive design.

## 8. Completion Criteria

The Managed Aggregation architecture investigation is complete enough for production implementation when:

- operation semantics for counter, gauge and histogram are unambiguous;
- descriptor and series identity rules are fixed;
- concurrency and ordering semantics are deterministic;
- duplicate/idempotency behavior is decided;
- legacy PHP/shell integration is demonstrated or explicitly rejected with evidence;
- transport/protocol choice has reproducible evidence;
- resource bounds and overload behavior are defined;
- mutation-to-snapshot behavior is defined;
- acknowledgement semantics are consistent with actual acceptance guarantees;
- lifecycle freeze and in-flight operation handling are defined;
- restart/new-epoch behavior is verified;
- final managed state reaches the existing Core contract without weakening it;
- required ADRs are accepted;
- draft specification is updated into accepted normative behavior;
- no unresolved question can invalidate the Managed Aggregation operating model.

Implementation spikes may be created during research, but they are evidence, not production architecture, until the corresponding decisions are recorded.

---
[Managed Aggregation research topics](managed-aggregation-architecture-research.md) | [Documentation index](../README.md)
