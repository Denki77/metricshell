# EPIC-002: Managed Aggregation Implementation

**Status:** Draft for implementation planning  
**Date:** 2026-09-25

## Purpose

This epic is the complete implementation plan for the optional Managed Aggregation extension after completion of
INV-016 through INV-020 and acceptance of ADR-016 through ADR-020.

Normative chain:

```text
epic -> wave -> issue -> implementation -> acceptance test
```

## Architecture decisions in dependency order

| ADR     | Topic                          | Mandatory implementation outcome                                                                                                     |
|---------|--------------------------------|--------------------------------------------------------------------------------------------------------------------------------------|
| ADR-016 | Managed Registry semantics     | Typed descriptor-driven registry, deterministic counter/gauge/histogram semantics, conflict rejection, one empty epoch per execution |
| ADR-017 | Concurrency, ordering and ACK  | One bounded single owner, one registry-wide commit order, success ACK only after commit, explicit overload/unknown outcome           |
| ADR-018 | Local operation transport      | Versioned bounded NDJSON over Unix stream socket, one operation per connection, stateless legacy clients                             |
| ADR-019 | Materialization and limits     | Generation-based immutable encoded snapshot cache, one complete generation per representation, bounded resource controls             |
| ADR-020 | Lifecycle and Core integration | Close admission, bounded drain, single freeze, one final materialization, existing Core install/final-scrape lifecycle               |

Core ADR-003 and ADR-004 remain authoritative.

## Target production flow

```text
MetricShell PID 1
  |
  +-- snapshot mode (default, unchanged)
  |
  +-- managed-registry mode
  |      |
  |      +-- local publishers
  |             v
  |        Unix protocol v1
  |             v
  |        bounded owner queue
  |             v
  |        serialized Managed Registry
  |             v
  |        committed generation
  |             v
  |        immutable materialization
  |             v
  |        complete Core candidate
  |             v
  |        existing Core validation/install
  |
  +-- existing exposition
  |
  +-- workload exit
         +-- close admission
         +-- bounded drain
         +-- freeze
         +-- final generation
         +-- Core install
         +-- existing final wait
```

## Delivery waves

### Wave 1 — Mode boundary and semantic core

- [ISSUE-MA-001. Managed mode configuration and bootstrap](../03-issues/ISSUE-MA-001.md)
- [ISSUE-MA-002. Managed domain model and descriptor semantics](../03-issues/ISSUE-MA-002.md)
- [ISSUE-MA-003. Managed Registry and execution epoch](../03-issues/ISSUE-MA-003.md)

**Exit gate:** snapshot mode unchanged/default; managed mode explicit; ADR-016 semantics implemented and unit-tested.

### Wave 2 — Serialized mutation ownership

- [ISSUE-MA-004. Bounded single-owner mutation loop](../03-issues/ISSUE-MA-004.md)

**Exit gate:** one commit order, bounded admission, observable overload, race-clean concurrency behavior.

### Wave 3 — Protocol, Unix transport and thin clients

- [ISSUE-MA-005. Operation protocol v1 and bounded framing](../03-issues/ISSUE-MA-005.md)
- [ISSUE-MA-006. Unix socket managed-operation server](../03-issues/ISSUE-MA-006.md)
- [ISSUE-MA-007. CLI helper and PHP 5.4 reference client](../03-issues/ISSUE-MA-007.md)

**Exit gate:** real local clients work without owning registry state; malformed/partial/version/error classes are deterministic.

### Wave 4 — Resource controls and immutable materialization

- [ISSUE-MA-008. Managed resource controls](../03-issues/ISSUE-MA-008.md)
- [ISSUE-MA-009. Generation-based immutable materialization](../03-issues/ISSUE-MA-009.md)

**Exit gate:** bounded series/histogram/queue/frame; reject-before-mutation; one immutable complete generation per response.

### Wave 5 — Core bridge and runtime lifecycle

- [ISSUE-MA-010. Core candidate bridge and atomic installation](../03-issues/ISSUE-MA-010.md)
- [ISSUE-MA-011. Runtime admission barrier and bounded drain](../03-issues/ISSUE-MA-011.md)
- [ISSUE-MA-012. Freeze, final snapshot and final-scrape integration](../03-issues/ISSUE-MA-012.md)

**Exit gate:** one Core path only; close admission first; bounded drain; single freeze; final candidate installed through existing Core; existing final wait unchanged.

### Wave 6 — Observability and production behavior verification

- [ISSUE-MA-013. Managed self-metrics and structured diagnostics](../03-issues/ISSUE-MA-013.md)
- [ISSUE-MA-014. Concurrency, shutdown, restart and failure E2E suite](../03-issues/ISSUE-MA-014.md)

**Exit gate:** bounded observability, production E2E, race detector, failure and shutdown coverage.

### Wave 7 — Compatibility, performance validation and release readiness

- [ISSUE-MA-015. Legacy, performance, resource and production validation](../03-issues/ISSUE-MA-015.md)
- [ISSUE-MA-016. Documentation, examples and release readiness](../03-issues/ISSUE-MA-016.md)

**Exit gate:** PHP 5.4/shell compatibility; production validation; normative spec accepted; EN/RU docs synchronized; snapshot regressions green.

## Issue definitions

### ISSUE-MA-001 — Managed mode configuration and bootstrap

Implement explicit managed-mode selection, keep snapshot default, reject invalid/hybrid config, instantiate managed subsystem only when enabled.

### ISSUE-MA-002 — Managed domain model and descriptor semantics

Implement production descriptor/family/series types, canonical labels, counter/gauge/histogram semantics, conflicts, and batch decision from ADR-016.

### ISSUE-MA-003 — Managed Registry and execution epoch

Implement one in-memory registry per execution, committed generation tracking, empty new epoch, no persistence/replay.

### ISSUE-MA-004 — Bounded single-owner mutation loop

Implement one owner loop, bounded queue, per-connection order, global commit order, post-commit success boundary, overload rejection.

### ISSUE-MA-005 — Operation protocol v1 and bounded framing

Implement versioned NDJSON request/response, one operation per connection, bounded parser, deterministic protocol errors.

### ISSUE-MA-006 — Unix socket managed-operation server

Implement Unix stream endpoint, permissions/lifecycle, startup readiness, connection handling and close-admission behavior.

### ISSUE-MA-007 — CLI helper and PHP 5.4 reference client

Provide shell-friendly CLI and stateless PHP 5.4 client with documented retry/unknown-outcome rules.

### ISSUE-MA-008 — Managed resource controls

Add configurable active-series, histogram-bucket, queue and frame limits with reject-before-mutation behavior.

### ISSUE-MA-009 — Generation-based immutable materialization

Implement immutable generation cache, reuse/invalidation and concurrent slow-reader safety.

### ISSUE-MA-010 — Core candidate bridge and atomic installation

Convert a complete managed generation to existing Core candidate format and reuse production validation/atomic install.

### ISSUE-MA-011 — Runtime admission barrier and bounded drain

Integrate close-admission on exit/termination and drain only already validated/admitted work within remaining ADR-003 budget.

### ISSUE-MA-012 — Freeze, final snapshot and final-scrape integration

Implement one logical freeze, reject late publishers, one final generation, and delegate to existing Core final-wait modes.

### ISSUE-MA-013 — Managed self-metrics and structured diagnostics

Add bounded self-metrics/logging for acceptance, rejection, overload, limits, queue, generations, materialization, freeze and finalization.

### ISSUE-MA-014 — Concurrency, shutdown, restart and failure E2E suite

Cover publishers, ACK loss boundary, malformed/partial clients, overload, natural exit, SIGTERM, budget exhaustion, late publishers, restart and final scrape.

### ISSUE-MA-015 — Legacy, performance, resource and production validation

Run PHP 5.4/shell and key INV-019/020 scenarios against production implementation; retain raw evidence and environment fingerprint.

### ISSUE-MA-016 — Documentation, examples and release readiness

Finalize managed normative specification and update configuration, limits, self-metrics, logging, lifecycle, examples, EN/RU traceability and release notes.

## Specification-to-issue traceability

| Normative specification                                                                                                                                         | Delivery issues                                                                                                                                                                                                                                                                                                                  |
|-----------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [Managed Aggregation requirements](../../03-requirements/FR-NFR-managed-aggregation-requirements-extension.md)                                                  | [ISSUE-MA-001](../03-issues/ISSUE-MA-001.md) through [ISSUE-MA-016](../03-issues/ISSUE-MA-016.md)                                                                                                                                                                                                                                |
| [Managed Aggregation behavioral model](../../04-specification/managed-aggregation-behavioral-model-draft.md)                                                    | [ISSUE-MA-002](../03-issues/ISSUE-MA-002.md), [ISSUE-MA-003](../03-issues/ISSUE-MA-003.md), [ISSUE-MA-004](../03-issues/ISSUE-MA-004.md), [ISSUE-MA-005](../03-issues/ISSUE-MA-005.md), [ISSUE-MA-008](../03-issues/ISSUE-MA-008.md), [ISSUE-MA-009](../03-issues/ISSUE-MA-009.md), [ISSUE-MA-012](../03-issues/ISSUE-MA-012.md) |
| [Application Snapshot Protocol](../../04-specification/application-snapshot-protocol.md)                                                                        | [ISSUE-MA-009](../03-issues/ISSUE-MA-009.md), [ISSUE-MA-010](../03-issues/ISSUE-MA-010.md), [ISSUE-MA-012](../03-issues/ISSUE-MA-012.md)                                                                                                                                                                                         |
| [Runtime State Machine](../../04-specification/runtime-state-machine.md)                                                                                        | [ISSUE-MA-001](../03-issues/ISSUE-MA-001.md), [ISSUE-MA-003](../03-issues/ISSUE-MA-003.md), [ISSUE-MA-006](../03-issues/ISSUE-MA-006.md), [ISSUE-MA-011](../03-issues/ISSUE-MA-011.md), [ISSUE-MA-012](../03-issues/ISSUE-MA-012.md), [ISSUE-MA-014](../03-issues/ISSUE-MA-014.md)                                               |
| [Configuration Specification](../../04-specification/configuration.md) and [Configuration Value Grammar](../../04-specification/configuration-value-grammar.md) | [ISSUE-MA-001](../03-issues/ISSUE-MA-001.md), [ISSUE-MA-004](../03-issues/ISSUE-MA-004.md), [ISSUE-MA-005](../03-issues/ISSUE-MA-005.md), [ISSUE-MA-006](../03-issues/ISSUE-MA-006.md), [ISSUE-MA-008](../03-issues/ISSUE-MA-008.md), [ISSUE-MA-011](../03-issues/ISSUE-MA-011.md), [ISSUE-MA-016](../03-issues/ISSUE-MA-016.md) |
| [Runtime Defaults and Resource Limits](../../04-specification/runtime-defaults-and-resource-limits.md)                                                          | [ISSUE-MA-004](../03-issues/ISSUE-MA-004.md), [ISSUE-MA-005](../03-issues/ISSUE-MA-005.md), [ISSUE-MA-006](../03-issues/ISSUE-MA-006.md), [ISSUE-MA-008](../03-issues/ISSUE-MA-008.md), [ISSUE-MA-011](../03-issues/ISSUE-MA-011.md), [ISSUE-MA-015](../03-issues/ISSUE-MA-015.md)                                               |
| [Self-Metrics Specification](../../04-specification/self-metrics.md) and [Structured Logging Specification](../../04-specification/structured-logging.md)       | [ISSUE-MA-013](../03-issues/ISSUE-MA-013.md), [ISSUE-MA-014](../03-issues/ISSUE-MA-014.md), [ISSUE-MA-016](../03-issues/ISSUE-MA-016.md)                                                                                                                                                                                         |
| [Docker/Compose examples](../../04-specification/docker-compose-examples.md)                                                                                    | [ISSUE-MA-006](../03-issues/ISSUE-MA-006.md), [ISSUE-MA-007](../03-issues/ISSUE-MA-007.md), [ISSUE-MA-016](../03-issues/ISSUE-MA-016.md)                                                                                                                                                                                         |

## Requirement-to-delivery traceability

| Requirement | Delivery issues                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
|-------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| FR-MA-001   | [ISSUE-MA-001](../03-issues/ISSUE-MA-001.md), [ISSUE-MA-016](../03-issues/ISSUE-MA-016.md)                                                                                                                                                                                                                                                                                                                                                                                 |
| FR-MA-002   | [ISSUE-MA-002](../03-issues/ISSUE-MA-002.md), [ISSUE-MA-003](../03-issues/ISSUE-MA-003.md), [ISSUE-MA-012](../03-issues/ISSUE-MA-012.md)                                                                                                                                                                                                                                                                                                                                   |
| FR-MA-003   | [ISSUE-MA-002](../03-issues/ISSUE-MA-002.md), [ISSUE-MA-003](../03-issues/ISSUE-MA-003.md), [ISSUE-MA-005](../03-issues/ISSUE-MA-005.md)                                                                                                                                                                                                                                                                                                                                   |
| FR-MA-004   | [ISSUE-MA-005](../03-issues/ISSUE-MA-005.md), [ISSUE-MA-007](../03-issues/ISSUE-MA-007.md)                                                                                                                                                                                                                                                                                                                                                                                 |
| FR-MA-005   | [ISSUE-MA-007](../03-issues/ISSUE-MA-007.md), [ISSUE-MA-015](../03-issues/ISSUE-MA-015.md)                                                                                                                                                                                                                                                                                                                                                                                 |
| FR-MA-006   | [ISSUE-MA-004](../03-issues/ISSUE-MA-004.md), [ISSUE-MA-006](../03-issues/ISSUE-MA-006.md), [ISSUE-MA-014](../03-issues/ISSUE-MA-014.md)                                                                                                                                                                                                                                                                                                                                   |
| FR-MA-007   | [ISSUE-MA-009](../03-issues/ISSUE-MA-009.md), [ISSUE-MA-010](../03-issues/ISSUE-MA-010.md)                                                                                                                                                                                                                                                                                                                                                                                 |
| FR-MA-008   | [ISSUE-MA-010](../03-issues/ISSUE-MA-010.md), [ISSUE-MA-011](../03-issues/ISSUE-MA-011.md), [ISSUE-MA-012](../03-issues/ISSUE-MA-012.md)                                                                                                                                                                                                                                                                                                                                   |
| FR-MA-009   | [ISSUE-MA-011](../03-issues/ISSUE-MA-011.md), [ISSUE-MA-012](../03-issues/ISSUE-MA-012.md), [ISSUE-MA-014](../03-issues/ISSUE-MA-014.md)                                                                                                                                                                                                                                                                                                                                   |
| FR-MA-010   | [ISSUE-MA-003](../03-issues/ISSUE-MA-003.md), [ISSUE-MA-012](../03-issues/ISSUE-MA-012.md), [ISSUE-MA-014](../03-issues/ISSUE-MA-014.md)                                                                                                                                                                                                                                                                                                                                   |
| FR-MA-011   | [ISSUE-MA-002](../03-issues/ISSUE-MA-002.md), [ISSUE-MA-003](../03-issues/ISSUE-MA-003.md), [ISSUE-MA-004](../03-issues/ISSUE-MA-004.md), [ISSUE-MA-008](../03-issues/ISSUE-MA-008.md), [ISSUE-MA-012](../03-issues/ISSUE-MA-012.md)                                                                                                                                                                                                                                       |
| FR-MA-012   | [ISSUE-MA-002](../03-issues/ISSUE-MA-002.md), [ISSUE-MA-003](../03-issues/ISSUE-MA-003.md)                                                                                                                                                                                                                                                                                                                                                                                 |
| FR-MA-013   | [ISSUE-MA-002](../03-issues/ISSUE-MA-002.md), [ISSUE-MA-003](../03-issues/ISSUE-MA-003.md)                                                                                                                                                                                                                                                                                                                                                                                 |
| FR-MA-014   | [ISSUE-MA-004](../03-issues/ISSUE-MA-004.md), [ISSUE-MA-005](../03-issues/ISSUE-MA-005.md), [ISSUE-MA-006](../03-issues/ISSUE-MA-006.md), [ISSUE-MA-008](../03-issues/ISSUE-MA-008.md), [ISSUE-MA-013](../03-issues/ISSUE-MA-013.md)                                                                                                                                                                                                                                       |
| FR-MA-015   | [ISSUE-MA-006](../03-issues/ISSUE-MA-006.md), [ISSUE-MA-008](../03-issues/ISSUE-MA-008.md), [ISSUE-MA-010](../03-issues/ISSUE-MA-010.md), [ISSUE-MA-011](../03-issues/ISSUE-MA-011.md), [ISSUE-MA-012](../03-issues/ISSUE-MA-012.md), [ISSUE-MA-014](../03-issues/ISSUE-MA-014.md)                                                                                                                                                                                         |
| FR-MA-016   | [ISSUE-MA-010](../03-issues/ISSUE-MA-010.md), [ISSUE-MA-011](../03-issues/ISSUE-MA-011.md), [ISSUE-MA-012](../03-issues/ISSUE-MA-012.md), [ISSUE-MA-016](../03-issues/ISSUE-MA-016.md)                                                                                                                                                                                                                                                                                     |
| NFR-MA-001  | [ISSUE-MA-001](../03-issues/ISSUE-MA-001.md), [ISSUE-MA-006](../03-issues/ISSUE-MA-006.md), [ISSUE-MA-007](../03-issues/ISSUE-MA-007.md), [ISSUE-MA-016](../03-issues/ISSUE-MA-016.md)                                                                                                                                                                                                                                                                                     |
| NFR-MA-002  | [ISSUE-MA-001](../03-issues/ISSUE-MA-001.md), [ISSUE-MA-016](../03-issues/ISSUE-MA-016.md)                                                                                                                                                                                                                                                                                                                                                                                 |
| NFR-MA-003  | [ISSUE-MA-001](../03-issues/ISSUE-MA-001.md), [ISSUE-MA-002](../03-issues/ISSUE-MA-002.md), [ISSUE-MA-003](../03-issues/ISSUE-MA-003.md), [ISSUE-MA-004](../03-issues/ISSUE-MA-004.md), [ISSUE-MA-005](../03-issues/ISSUE-MA-005.md), [ISSUE-MA-006](../03-issues/ISSUE-MA-006.md), [ISSUE-MA-007](../03-issues/ISSUE-MA-007.md), [ISSUE-MA-008](../03-issues/ISSUE-MA-008.md), [ISSUE-MA-009](../03-issues/ISSUE-MA-009.md), [ISSUE-MA-010](../03-issues/ISSUE-MA-010.md) |
| NFR-MA-004  | [ISSUE-MA-002](../03-issues/ISSUE-MA-002.md), [ISSUE-MA-003](../03-issues/ISSUE-MA-003.md), [ISSUE-MA-004](../03-issues/ISSUE-MA-004.md), [ISSUE-MA-012](../03-issues/ISSUE-MA-012.md), [ISSUE-MA-014](../03-issues/ISSUE-MA-014.md)                                                                                                                                                                                                                                       |
| NFR-MA-005  | [ISSUE-MA-004](../03-issues/ISSUE-MA-004.md), [ISSUE-MA-005](../03-issues/ISSUE-MA-005.md), [ISSUE-MA-006](../03-issues/ISSUE-MA-006.md), [ISSUE-MA-008](../03-issues/ISSUE-MA-008.md), [ISSUE-MA-011](../03-issues/ISSUE-MA-011.md), [ISSUE-MA-013](../03-issues/ISSUE-MA-013.md)                                                                                                                                                                                         |
| NFR-MA-006  | [ISSUE-MA-013](../03-issues/ISSUE-MA-013.md)                                                                                                                                                                                                                                                                                                                                                                                                                               |
| NFR-MA-007  | [ISSUE-MA-002](../03-issues/ISSUE-MA-002.md) through [ISSUE-MA-015](../03-issues/ISSUE-MA-015.md)                                                                                                                                                                                                                                                                                                                                                                          |
| NFR-MA-008  | [ISSUE-MA-015](../03-issues/ISSUE-MA-015.md), [ISSUE-MA-016](../03-issues/ISSUE-MA-016.md)                                                                                                                                                                                                                                                                                                                                                                                 |

## Cross-cutting Definition of Done

Every task must preserve snapshot mode, accepted ADR semantics, bounded resources/waits, Core ADR-004 semantics,
race-safety where relevant, bounded observability, EN/RU parity, and full requirement -> INV -> ADR -> specification ->
issue -> test traceability.
