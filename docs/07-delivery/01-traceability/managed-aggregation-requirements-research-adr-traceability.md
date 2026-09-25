# MetricShell: Managed Aggregation Traceability

**Status:** Draft for implementation planning  
**Date:** 2026-09-25

## Purpose

This document links the accepted Managed Aggregation requirements to the completed research and architecture decisions,
then maps those decisions to the implementation delivery plan.

Normative chain:

```text
requirement -> INV-016...INV-020 -> ADR-016...ADR-020 -> specification -> delivery issue -> acceptance test
```

Document scope:

```text
FR-MA / NFR-MA -> ADR -> Specification -> ISSUE
```

## Statuses

- **Closed architecturally** — accepted ADRs define the architectural policy.
- **Implementation pending** — architecture is accepted, but production code and delivery verification are not complete.
- **Specification follow-up** — the final normative Managed Aggregation specification must be completed during implementation without changing accepted ADR boundaries.

## Traceability matrix

| Requirement | Requirement topic          | Related ADRs                       | Specification / contract                                                                                                                                                                                                                               | Delivery issues                                                                    | Status                 | What is established                                                                                                       |
|-------------|----------------------------|------------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|------------------------------------------------------------------------------------|------------------------|---------------------------------------------------------------------------------------------------------------------------|
| FR-MA-001   | Optional capability        | ADR-016, ADR-020                   | [Managed Aggregation requirements](../../03-requirements/FR-NFR-managed-aggregation-requirements-extension.md); [Configuration](../../04-specification/configuration.md)                                                                               | ISSUE-MA-001, ISSUE-MA-016                                                         | Implementation pending | Managed Aggregation is opt-in; snapshot mode remains default and unchanged.                                               |
| FR-MA-002   | Registry ownership         | ADR-016, ADR-017, ADR-020          | [Managed behavioral model](../../04-specification/managed-aggregation-behavioral-model-draft.md); [Runtime State Machine](../../04-specification/runtime-state-machine.md)                                                                             | ISSUE-MA-002, ISSUE-MA-003, ISSUE-MA-012                                           | Implementation pending | One MetricShell execution owns one in-memory managed registry for one workload epoch.                                     |
| FR-MA-003   | Instrumentation operations | ADR-016, ADR-017                   | [Managed behavioral model](../../04-specification/managed-aggregation-behavioral-model-draft.md)                                                                                                                                                       | ISSUE-MA-002, ISSUE-MA-003, ISSUE-MA-005                                           | Implementation pending | Counter, gauge, and classic histogram operation semantics are descriptor-driven and deterministic.                        |
| FR-MA-004   | Thin clients               | ADR-016, ADR-018                   | [Managed behavioral model](../../04-specification/managed-aggregation-behavioral-model-draft.md)                                                                                                                                                       | ISSUE-MA-005, ISSUE-MA-007                                                         | Implementation pending | Clients submit operations and need not own a complete registry or serialize snapshots.                                    |
| FR-MA-005   | Legacy compatibility       | ADR-018                            | [Managed behavioral model](../../04-specification/managed-aggregation-behavioral-model-draft.md)                                                                                                                                                       | ISSUE-MA-007, ISSUE-MA-015                                                         | Implementation pending | Unix stream protocol is usable by PHP 5.4 and shell/CLI without a modern Prometheus client library.                       |
| FR-MA-006   | Multiple local publishers  | ADR-017, ADR-018                   | [Managed behavioral model](../../04-specification/managed-aggregation-behavioral-model-draft.md)                                                                                                                                                       | ISSUE-MA-004, ISSUE-MA-006, ISSUE-MA-014                                           | Implementation pending | Multiple publishers share one bounded serialized commit order for the same workload registry.                             |
| FR-MA-007   | Complete snapshot boundary | ADR-004, ADR-016, ADR-019, ADR-020 | [Application Snapshot Protocol](../../04-specification/application-snapshot-protocol.md); [Managed behavioral model](../../04-specification/managed-aggregation-behavioral-model-draft.md)                                                             | ISSUE-MA-009, ISSUE-MA-010                                                         | Implementation pending | Managed state enters Core only as one complete candidate snapshot.                                                        |
| FR-MA-008   | Core reuse                 | ADR-004, ADR-019, ADR-020          | [Application Snapshot Protocol](../../04-specification/application-snapshot-protocol.md); [Runtime State Machine](../../04-specification/runtime-state-machine.md)                                                                                     | ISSUE-MA-010, ISSUE-MA-011, ISSUE-MA-012                                           | Implementation pending | Managed mode reuses existing Core validation, atomic replacement, exposition, and final-scrape lifecycle.                 |
| FR-MA-009   | Lifecycle integration      | ADR-020                            | [Runtime State Machine](../../04-specification/runtime-state-machine.md); [Managed behavioral model](../../04-specification/managed-aggregation-behavioral-model-draft.md)                                                                             | ISSUE-MA-011, ISSUE-MA-012, ISSUE-MA-014                                           | Implementation pending | Finalization is close admission -> bounded drain -> freeze -> one final candidate/install attempt -> existing final wait. |
| FR-MA-010   | Execution epoch            | ADR-016, ADR-020                   | [Runtime State Machine](../../04-specification/runtime-state-machine.md)                                                                                                                                                                               | ISSUE-MA-003, ISSUE-MA-012, ISSUE-MA-014                                           | Implementation pending | Every process/workload execution starts a new empty managed epoch; no persistence or replay.                              |
| FR-MA-011   | Safe rejection             | ADR-016, ADR-017, ADR-019, ADR-020 | [Managed behavioral model](../../04-specification/managed-aggregation-behavioral-model-draft.md); [Runtime Defaults and Resource Limits](../../04-specification/runtime-defaults-and-resource-limits.md)                                               | ISSUE-MA-002, ISSUE-MA-003, ISSUE-MA-004, ISSUE-MA-008, ISSUE-MA-012               | Implementation pending | Invalid, conflicting, late, or over-limit operations reject without corrupting committed valid state.                     |
| FR-MA-012   | Atomic batches             | ADR-016                            | [Managed behavioral model](../../04-specification/managed-aggregation-behavioral-model-draft.md)                                                                                                                                                       | ISSUE-MA-002, ISSUE-MA-003                                                         | Implementation pending | Implementation follows ADR-016 batch semantics; supported batches are all-or-nothing.                                     |
| FR-MA-013   | Descriptors                | ADR-016, ADR-017                   | [Managed behavioral model](../../04-specification/managed-aggregation-behavioral-model-draft.md)                                                                                                                                                       | ISSUE-MA-002, ISSUE-MA-003                                                         | Implementation pending | Descriptor/type/HELP/labels/histogram shape are explicit semantic state with deterministic conflict handling.             |
| FR-MA-014   | Resource bounds            | ADR-017, ADR-018, ADR-019          | [Configuration](../../04-specification/configuration.md); [Configuration Value Grammar](../../04-specification/configuration-value-grammar.md); [Runtime Defaults and Resource Limits](../../04-specification/runtime-defaults-and-resource-limits.md) | ISSUE-MA-004, ISSUE-MA-005, ISSUE-MA-006, ISSUE-MA-008, ISSUE-MA-013               | Implementation pending | Frame, series, histogram, queue, and related resource exposure must be bounded and observable.                            |
| FR-MA-015   | Failure isolation          | ADR-017, ADR-018, ADR-019, ADR-020 | [Managed behavioral model](../../04-specification/managed-aggregation-behavioral-model-draft.md); [Runtime State Machine](../../04-specification/runtime-state-machine.md)                                                                             | ISSUE-MA-006, ISSUE-MA-008, ISSUE-MA-010, ISSUE-MA-011, ISSUE-MA-012, ISSUE-MA-014 | Implementation pending | Malformed, slow, disconnected, crashing, or late publishers cannot corrupt valid registry/Core state.                     |
| FR-MA-016   | Core contracts             | ADR-004, ADR-016, ADR-019, ADR-020 | [Application Snapshot Protocol](../../04-specification/application-snapshot-protocol.md); [Runtime State Machine](../../04-specification/runtime-state-machine.md)                                                                                     | ISSUE-MA-010, ISSUE-MA-011, ISSUE-MA-012, ISSUE-MA-016                             | Implementation pending | Managed Aggregation remains upstream of Core and must not create a second Core path.                                      |
| NFR-MA-001  | Integration simplicity     | ADR-018, ADR-020                   | [Configuration](../../04-specification/configuration.md); [Docker/Compose examples](../../04-specification/docker-compose-examples.md)                                                                                                                 | ISSUE-MA-001, ISSUE-MA-006, ISSUE-MA-007, ISSUE-MA-016                             | Implementation pending | One binary/process remains sufficient; no mandatory daemon, sidecar, or external service.                                 |
| NFR-MA-002  | Backward compatibility     | ADR-016, ADR-020                   | [Configuration](../../04-specification/configuration.md)                                                                                                                                                                                               | ISSUE-MA-001, ISSUE-MA-016                                                         | Implementation pending | Existing snapshot-mode integrations continue unchanged.                                                                   |
| NFR-MA-003  | Architectural isolation    | ADR-016, ADR-017, ADR-019          | [Managed behavioral model](../../04-specification/managed-aggregation-behavioral-model-draft.md)                                                                                                                                                       | ISSUE-MA-001...ISSUE-MA-010                                                        | Implementation pending | Protocol parsing, semantics, owner loop, materialization, and Core bridge are separate responsibilities.                  |
| NFR-MA-004  | Determinism                | ADR-016, ADR-017, ADR-020          | [Managed behavioral model](../../04-specification/managed-aggregation-behavioral-model-draft.md)                                                                                                                                                       | ISSUE-MA-002, ISSUE-MA-003, ISSUE-MA-004, ISSUE-MA-012, ISSUE-MA-014               | Implementation pending | Same accepted operation order and configuration produce deterministic registry state and lifecycle outcomes.              |
| NFR-MA-005  | Bounded resources          | ADR-017, ADR-018, ADR-019, ADR-020 | [Configuration](../../04-specification/configuration.md); [Runtime Defaults and Resource Limits](../../04-specification/runtime-defaults-and-resource-limits.md)                                                                                       | ISSUE-MA-004, ISSUE-MA-005, ISSUE-MA-006, ISSUE-MA-008, ISSUE-MA-011, ISSUE-MA-013 | Implementation pending | Memory, framing, queueing, processing, and finalization waits are bounded.                                                |
| NFR-MA-006  | Observability              | ADR-017, ADR-019, ADR-020          | [Self-Metrics Specification](../../04-specification/self-metrics.md); [Structured Logging Specification](../../04-specification/structured-logging.md)                                                                                                 | ISSUE-MA-013                                                                       | Implementation pending | Acceptance, rejection, overload, limits, generations, materialization/cache, and lifecycle outcomes are observable.       |
| NFR-MA-007  | Testability                | ADR-016...ADR-020                  | [Managed Aggregation requirements](../../03-requirements/FR-NFR-managed-aggregation-requirements-extension.md); [Managed behavioral model](../../04-specification/managed-aggregation-behavioral-model-draft.md)                                       | ISSUE-MA-002...ISSUE-MA-015                                                        | Implementation pending | Semantics are independently testable and covered end-to-end through Core.                                                 |
| NFR-MA-008  | Reproducible research      | ADR-016...ADR-020                  | [Architecture investigation](../../05-architecture-investigation/managed-aggregation-architecture-investigation.md); [Architecture research](../../05-architecture-investigation/managed-aggregation-architecture-research.md)                         | ISSUE-MA-015, ISSUE-MA-016                                                         | Closed architecturally | Accepted decisions have reproducible evidence; implementation validation must preserve that discipline.                   |

## Architecture-decision traceability

| Capability                                         | Research | ADR     | Primary delivery issues                                |
|----------------------------------------------------|----------|---------|--------------------------------------------------------|
| Managed registry semantics                         | INV-016  | ADR-016 | ISSUE-MA-002, ISSUE-MA-003                             |
| Concurrent publishers, ordering, ACK, backpressure | INV-017  | ADR-017 | ISSUE-MA-004, ISSUE-MA-014                             |
| Legacy client and local transport                  | INV-018  | ADR-018 | ISSUE-MA-005, ISSUE-MA-006, ISSUE-MA-007               |
| Materialization and resource controls              | INV-019  | ADR-019 | ISSUE-MA-008, ISSUE-MA-009, ISSUE-MA-013, ISSUE-MA-015 |
| Lifecycle, freeze, Core integration                | INV-020  | ADR-020 | ISSUE-MA-010, ISSUE-MA-011, ISSUE-MA-012, ISSUE-MA-014 |

## Specification work required during implementation

At minimum:

- promote/update the managed behavioral draft into the accepted normative Managed Aggregation specification;
- extend configuration with explicit managed-mode selection and bounded controls;
- define protocol v1 and its error taxonomy;
- extend self-metrics and structured logging;
- update runtime lifecycle with ADR-020 composition;
- update Docker/Compose examples;
- document shell and PHP 5.4 usage.

## Delivery issue index

| Issue        | Topic                                                   | Primary requirements                                                |
|--------------|---------------------------------------------------------|---------------------------------------------------------------------|
| ISSUE-MA-001 | Managed mode configuration and bootstrap                | FR-MA-001, NFR-MA-001, NFR-MA-002, NFR-MA-003                       |
| ISSUE-MA-002 | Managed domain model and descriptor semantics           | FR-MA-003, FR-MA-011, FR-MA-012, FR-MA-013, NFR-MA-004              |
| ISSUE-MA-003 | Managed Registry and execution epoch                    | FR-MA-002, FR-MA-003, FR-MA-010, FR-MA-011, FR-MA-013               |
| ISSUE-MA-004 | Bounded single-owner mutation loop                      | FR-MA-006, FR-MA-011, FR-MA-014, NFR-MA-004, NFR-MA-005             |
| ISSUE-MA-005 | Operation protocol v1 and framing                       | FR-MA-003, FR-MA-004, FR-MA-014, FR-MA-015                          |
| ISSUE-MA-006 | Unix socket managed-operation server                    | FR-MA-006, FR-MA-014, FR-MA-015, NFR-MA-001                         |
| ISSUE-MA-007 | CLI helper and PHP 5.4 reference client                 | FR-MA-004, FR-MA-005, NFR-MA-001                                    |
| ISSUE-MA-008 | Managed resource controls                               | FR-MA-011, FR-MA-014, FR-MA-015, NFR-MA-005                         |
| ISSUE-MA-009 | Generation-based immutable materialization              | FR-MA-007, FR-MA-014, NFR-MA-004, NFR-MA-005                        |
| ISSUE-MA-010 | Core candidate bridge and atomic installation           | FR-MA-007, FR-MA-008, FR-MA-015, FR-MA-016                          |
| ISSUE-MA-011 | Runtime admission barrier and bounded drain             | FR-MA-008, FR-MA-009, FR-MA-015, NFR-MA-005                         |
| ISSUE-MA-012 | Freeze, final snapshot, final-scrape integration        | FR-MA-002, FR-MA-008, FR-MA-009, FR-MA-010, FR-MA-011, FR-MA-016    |
| ISSUE-MA-013 | Managed self-metrics and structured diagnostics         | FR-MA-014, NFR-MA-006                                               |
| ISSUE-MA-014 | Concurrency, shutdown, restart and failure E2E suite    | FR-MA-006, FR-MA-009, FR-MA-010, FR-MA-015, NFR-MA-004, NFR-MA-007  |
| ISSUE-MA-015 | Legacy, performance, resource and production validation | FR-MA-005, FR-MA-014, FR-MA-015, NFR-MA-005, NFR-MA-007, NFR-MA-008 |
| ISSUE-MA-016 | Documentation, examples and release readiness           | FR-MA-001, FR-MA-016, NFR-MA-001, NFR-MA-002, NFR-MA-008            |

## Cross-cutting implementation invariants

- snapshot mode remains default and independently usable;
- Managed Aggregation is explicitly enabled;
- no hybrid snapshot-plus-managed ownership;
- one execution owns one managed epoch;
- no cross-epoch persistence/replay;
- one bounded owner serializes accepted mutations;
- successful ACK is never sent before commit;
- unknown-after-possible-commit remains explicit without idempotency;
- managed state reaches Core only as a complete candidate snapshot;
- invalid or over-limit input cannot partially mutate committed state;
- scrape-visible application bytes belong to one complete generation;
- finalization is bounded and reuses existing Core budgets;
- no second Core path, daemon, mandatory sidecar, or Kubernetes API dependency;
- research coverage numbers are not silently converted into production defaults.

## Completion gate

Managed Aggregation is complete only when ISSUE-MA-001 through ISSUE-MA-016 are complete, all FR-MA/NFR-MA rows have
production evidence, snapshot-mode regressions remain green, managed unit/integration/race/fault/E2E suites pass, and
EN/RU specifications are synchronized.
