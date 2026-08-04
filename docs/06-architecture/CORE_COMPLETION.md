# MetricShell Core Completion

- **Status:** Completed
- **Scope:** MetricShell Core snapshot semantics
- **Extensions:** Require a separate normative lifecycle

## 1. Purpose

This document closes the architecture-definition phase for MetricShell Core in the complete-snapshot scope accepted by
ADR-001 through ADR-015. It does not approve or pre-design aggregation or another application-state ownership model.

## 2. Completed Core Scope

MetricShell Core covers:

- PID 1 workload ownership, signal forwarding, child reaping, and workload-result preservation;
- bounded lifecycle, shutdown, finalization, and final-scrape behavior;
- ingestion of one complete application snapshot at a time;
- whole-candidate validation and atomic replacement of the last valid snapshot;
- transport-equivalent file, Unix socket, and local HTTP ingestion;
- Prometheus/OpenMetrics exposition of one immutable snapshot per scrape;
- non-root operation, resource limits, reproducible distribution, and release evidence.

The normative behavior is defined by accepted requirements, specifications, and ADR-001 through ADR-015.

## 3. Core Snapshot Contract

A successful publication represents one complete application state for one workload run. Core validates the candidate as
one indivisible unit, atomically installs it, and retains the preceding valid state after rejection.

Core does not:

- merge independent registries or snapshots;
- sum values from multiple publishers;
- replay increment, set, or observe operations;
- retain per-producer contributions or snapshot history;
- guarantee scraping or durable storage of every version;
- persist application metrics across MetricShell process restarts.

The initial zero-series snapshot and every subsequently accepted zero-series snapshot are valid complete states.

## 4. Completion Boundary

Core completion does not include managed aggregation, operation-level ingestion, cross-process registry ownership,
cross-container aggregation, remote write, durable metric storage, or Prometheus HA deduplication.

No CLI mode, environment variable, public operation protocol, internal interface, package layout, binary layout, image
layout, or deployment topology for any future aggregation capability is established by this document.

## 5. Extension Rule

Any feature that changes how application state is owned, combined, reconstructed, or produced from operations requires
its own normative chain:

~~~text
requirements
-> investigation evidence
-> accepted ADR
-> public specification
-> delivery issues and acceptance tests
~~~

An extension must explicitly define compatibility with Core snapshot semantics. It may reuse accepted Core contracts,
but
that reuse must not be assumed to approve a particular implementation structure.

## 6. Completion Statement

MetricShell Core is architecturally complete only in the accepted complete-snapshot scope. Aggregation is outside Core.
Future aggregation or operation-level publication remains unapproved until its separate normative chain is accepted.
