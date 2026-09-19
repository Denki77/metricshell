# Scope and Requirements Extension — Managed Aggregation

- **Status:** Proposed scope extension
- **Applies to:** MetricShell
- **Depends on:** completed MetricShell Core scope and ADR-001 through ADR-015
- **Default mode:** Core snapshot mode
- **Optional mode:** managed aggregation

## 1. Purpose

This document extends MetricShell with optional managed aggregation without changing the completed MetricShell Core architecture.

Core remains independently usable and is the default operating mode. The extension targets workloads that cannot or should not maintain a complete Prometheus-compatible metric registry themselves, including legacy applications, CLI programs, shell scripts, and multi-process workers.

## 2. Problem

MetricShell Core accepts complete application metric snapshots. This works when the workload already owns complete metric state.

For legacy and simple clients, requiring the client to maintain counters, gauges, histogram buckets, descriptors, labels, concurrency and full-snapshot serialization moves too much responsibility into the workload.

Such clients naturally need operations such as:

```text
increment processed_total
set queue_depth 12
observe request_duration_seconds 0.42
```

MetricShell therefore needs an optional owner of aggregated application metric state.

## 3. Scope Extension

MetricShell SHALL support an optional managed-registry mode.

In this mode MetricShell owns an in-memory application metric registry for one logical workload execution. Clients submit instrumentation operations. MetricShell applies accepted operations and produces complete application snapshots for the existing Core path.

```text
workload
    ↓ instrumentation operations
managed aggregation
    ↓ complete application snapshot
MetricShell Core
    ↓
Prometheus / OpenMetrics
```

The aggregation capability is upstream of the existing Core snapshot boundary.

## 4. Product and Deployment Model

MetricShell SHALL remain:

- one product;
- one executable binary;
- one container-image integration layer;
- one runtime process supervising one logical workload execution;
- a small modular monolith with explicit internal architectural boundaries.

Managed aggregation SHALL NOT require a second binary, separate daemon, mandatory sidecar, or external central aggregation service.

## 5. Operating Modes

### 5.1 Default snapshot mode

```text
metricshell
```

is equivalent in behavior to explicit Core snapshot mode:

```text
metricshell --mode=snapshot
```

The workload owns the application registry and publishes complete snapshots.

### 5.2 Managed-registry mode

Aggregation is explicitly enabled, conceptually:

```text
metricshell --mode=managed-registry
```

The exact CLI/configuration contract is subject to architecture and implementation design.

### 5.3 Isolation

The initial extension SHALL NOT require a hybrid registry mixing externally supplied complete snapshots with managed operations. Such composition requires separate research and scope approval.

## 6. Functional Requirements

### FR-MA-001 — Optional capability

Managed aggregation SHALL be optional and SHALL NOT change default snapshot-mode behavior.

### FR-MA-002 — Registry ownership

In managed mode, MetricShell SHALL own the application metric registry for the lifetime of one MetricShell execution and one logical workload.

### FR-MA-003 — Instrumentation operations

Managed mode SHALL support operations sufficient for counters, gauges, and histogram observations. Exact semantics and wire representation are architecture-research subjects.

### FR-MA-004 — Thin clients

Clients SHALL NOT be required to maintain a complete metric registry or construct complete Prometheus snapshots.

### FR-MA-005 — Legacy compatibility

The design SHALL explicitly evaluate PHP 5.4 and shell/CLI integration without requiring a modern Prometheus client library.

### FR-MA-006 — Multiple local publishers

Managed mode SHALL support multiple cooperating publishers belonging to the same logical workload, subject to researched ordering, concurrency and ownership rules. Unrelated applications sharing one registry are out of scope.

### FR-MA-007 — Complete snapshot boundary

Managed aggregation SHALL produce a complete application snapshot before state enters the existing Core snapshot path.

### FR-MA-008 — Core reuse

Managed mode SHALL reuse existing Core validation, atomic replacement, exposition, lifecycle, post-exit and final-scrape behavior.

### FR-MA-009 — Lifecycle integration

Architecture SHALL define startup, normal exit, signal shutdown, in-flight operations, registry freeze, final snapshot, final scrape and restart behavior.

### FR-MA-010 — Execution epoch

A new MetricShell execution SHALL create a new managed-registry epoch by default. Persistent restoration across restarts is outside this extension.

### FR-MA-011 — Safe rejection

Invalid or conflicting operations SHALL NOT corrupt already valid registry state. Acceptance/rejection semantics SHALL be deterministic.

### FR-MA-012 — Atomic batches

Architecture research SHALL determine whether related operations require atomic batching. If supported, rejected batches SHALL NOT be partially applied.

### FR-MA-013 — Descriptors

Architecture SHALL define ownership and lifecycle of metric type, HELP metadata, label schema and histogram configuration, including deterministic conflict handling.

### FR-MA-014 — Resource bounds

Managed mode SHALL provide configurable protection against unbounded payloads, series/cardinality, labels, histogram configuration, concurrent publishers, queued work and memory consumption.

### FR-MA-015 — Failure isolation

Malformed, slow, disconnected or crashing publishers SHALL NOT corrupt valid registry state or break Core exposition of the last valid state.

### FR-MA-016 — Core contracts

Managed aggregation SHALL preserve accepted Core contracts. Any required change to an accepted Core contract requires explicit scope revision and a superseding ADR.

## 7. Non-Functional Requirements

### NFR-MA-001 — Integration simplicity

The extension SHALL preserve the existing low-friction model: one MetricShell layer in the workload image and explicit mode selection. No additional service is required for local aggregation.

### NFR-MA-002 — Backward compatibility

Existing snapshot-mode integrations SHALL continue to work unchanged.

### NFR-MA-003 — Architectural isolation

Operation parsing, registry mutation and aggregation semantics SHALL be isolated behind explicit internal module boundaries.

### NFR-MA-004 — Determinism

Given the same accepted operation order and configuration, registry semantics SHALL be deterministic. Concurrency ordering rules SHALL be explicit.

### NFR-MA-005 — Bounded resources

Memory, buffering, concurrency and input processing SHALL have explicit limits or demonstrably bounded behavior.

### NFR-MA-006 — Observability

MetricShell SHALL expose sufficient self-metrics for managed-mode acceptance, rejection, overload and resource limits without copying unbounded application label cardinality into self-metrics.

### NFR-MA-007 — Testability

Aggregation semantics SHALL be independently testable and SHALL also have end-to-end coverage through Core.

### NFR-MA-008 — Reproducible research

Architecture decisions SHALL be supported by reproducible investigations using the evidence discipline established for Core.

## 8. Explicitly Out of Scope

This extension does not introduce:

- distributed aggregation across MetricShell instances;
- central aggregation clusters;
- persistent application metric state;
- guaranteed history/delivery of every instrumentation event;
- remote write;
- Prometheus HA deduplication;
- registry sharing between unrelated workloads;
- automatic merging of independent complete snapshots;
- a second MetricShell executable;
- a mandatory sidecar;
- hybrid snapshot-plus-operation ownership.

## 9. Compatibility with Core

The existing Core boundary remains:

```text
complete candidate snapshot
→ whole-candidate validation
→ atomic active-state replacement
→ exposition
```

Managed aggregation adds only a new producer:

```text
instrumentation operations
→ managed registry
→ complete candidate snapshot
→ existing Core
```

Core does not become an operation processor and does not merge independent registries.

## 10. Required Architecture Research

Before implementation semantics become normative, dedicated investigations SHALL cover at least:

- managed-registry semantics;
- concurrent publishers;
- legacy-client viability;
- performance and resource limits;
- lifecycle integration.

The Managed Aggregation Research Plan tracks these investigations. Their conclusions are expected to be recorded in explicit ADRs.

## 11. Acceptance Criteria for This Scope Extension

This scope extension is accepted when the project agrees that:

1. MetricShell Core remains architecturally complete and unchanged in responsibility.
2. Snapshot mode remains the default.
3. Managed aggregation is an explicitly enabled mode of the same MetricShell binary.
4. Aggregation is architecturally isolated inside the small modular monolith.
5. Clients can publish instrumentation operations without owning complete metric state.
6. Aggregation produces complete snapshots for the existing Core contract.
7. Unresolved semantics are decided by architecture investigations, not prematurely by product requirements.

Acceptance of this document authorizes the managed-aggregation investigation cycle. It does not itself select synchronization algorithms, framing, storage structures or protocol details.
