# Non-functional Requirements

> Status: Draft for architecture investigation

## Purpose

This document defines quality attributes and measurable engineering expectations. Final numeric targets are established
by prototypes and benchmarks; `TBD` marks values the architecture investigation must resolve.

## Reliability

### NFR-REL-001 — Workload independence

Metrics ingestion or exposition failure MUST NOT terminate the workload by default.

### NFR-REL-002 — Deterministic lifecycle

The same workload outcome, configuration, and external event sequence MUST produce the same lifecycle outcome.

### NFR-REL-003 — Bounded waiting

Shutdown, ingestion flush, scrape wait, and child-process waits MUST be bounded.

### NFR-REL-004 — Crash containment

Malformed input, client disconnects, concurrent scrapes, and unsupported values MUST NOT crash **MetricShell**.

### NFR-REL-005 — Final-state consistency

Final application metrics MUST remain consistent across post-workload scrapes.

### NFR-REL-006 — Failure visibility

Ingestion and exposition failures MUST be externally observable through documented HTTP behavior, self-metrics, logs, or
a combination.

## Performance

### NFR-PERF-001 — Startup overhead

Startup overhead excluding workload startup MUST be benchmarked on a documented reference environment.
> Stable-release target: `TBD`.

### NFR-PERF-002 — Ingestion performance

Each transport MUST have documented sustained throughput and latency for low, moderate, and high update rates.
> Targets: `TBD`.

### NFR-PERF-003 — Scrape latency

For a registry within supported limits, scrape latency MUST remain materially below Prometheus's normal scrape timeout.
> Exact registry size and latency target: `TBD`.

### NFR-PERF-004 — Resource overhead

Idle, reference-registry, sustained-ingestion, and concurrent-scrape CPU and memory overhead MUST be measured and
published.

### NFR-PERF-005 — Backpressure

Capacity exhaustion MUST use bounded rejection, timeout, dropping, or replacement policy. Unbounded queues are
prohibited.

## Scalability

### NFR-SCALE-001 — Enforced limits

**MetricShell** MUST enforce configurable series, labels, payload, connection, and memory-related limits.

### NFR-SCALE-002 — Predictable degradation

At a limit, behavior MUST be deterministic and avoid uncontrolled resource growth.

### NFR-SCALE-003 — Cardinality diagnostics

Cardinality-limit rejections MUST be observable.

## Portability

### NFR-PORT-001 — Orchestrator independence

The core executable MUST NOT depend on Kubernetes APIs or Kubernetes-only conventions.

### NFR-PORT-002 — OCI compatibility

**MetricShell** MUST support OCI-compatible Linux containers. Other platforms MAY be added separately.

### NFR-PORT-003 — Minimal assumptions

Standalone integration SHOULD require no shell, init system, package manager, or language runtime beyond documented
requirements.

### NFR-PORT-004 — Reproducible provenance

Published artifacts MUST identify their source revision and release version.

## Process correctness

### NFR-PROC-001 — PID 1 correctness

When running as PID 1, MetricShell MUST correctly implement the responsibilities of its selected process model.

### NFR-PROC-002 — Process groups

Child and descendant termination semantics MUST be defined and tested.

### NFR-PROC-003 — Exit integrity

Workload failure MUST NOT silently become success. Runtime failure and forced-termination precedence MUST be documented.

## Security

### NFR-SEC-001 — Non-root

**MetricShell** MUST support non-root operation.

### NFR-SEC-002 — Local ingestion by default

Socket and push ingestion MUST default to a scope unavailable outside the workload container unless explicitly
configured.

### NFR-SEC-003 — Untrusted input

Metric names, labels, values, files, frames, and requests MUST be validated as untrusted input.

### NFR-SEC-004 — Exhaustion protection

Oversized payloads, excessive connections, series, labels, and slow clients MUST be bounded.

### NFR-SEC-005 — Secret safety

**MetricShell** MUST NOT intentionally expose environment variables, arguments, credentials, or file contents as labels or
normal logs.

### NFR-SEC-006 — Configurable HTTP binding

The scrape bind address MUST be configurable and externally binding must be documented as a security decision.

## Compatibility

### NFR-COMP-001 — Exposition compatibility

Stable releases MUST state supported Prometheus/OpenMetrics formats and validate output with official or
standards-compatible tooling.

### NFR-COMP-002 — Protocol versioning

Socket and push protocols MUST have a compatibility strategy before stable release.

### NFR-COMP-003 — File versioning

A structured proprietary file format MUST have an explicit schema version; direct adoption of an externally versioned
standard is exempt.

### NFR-COMP-004 — Client matrix

The project MUST publish runtime/client compatibility.

## Maintainability

### NFR-MAINT-001 — Documentation separation

Requirements, behavioral specification, architecture, and ADRs MUST remain separate layers.

### NFR-MAINT-002 — Traceability

Every mandatory functional requirement MUST map to automated acceptance coverage.

### NFR-MAINT-003 — ADR coverage

Process model, protocols, final-scrape semantics, and failure precedence MUST be recorded as ADRs.

### NFR-MAINT-004 — Stable surface

Configuration, protocols, and self-metrics MUST NOT be declared stable before a compatibility policy exists.

## Operability

### NFR-OPS-001 — Wait reason

Operators MUST be able to determine why **MetricShell** remains alive after workload exit.

### NFR-OPS-002 — Effective configuration

Effective non-secret configuration SHOULD be observable.

### NFR-OPS-003 — Health semantics

Health/readiness behavior MUST be documented for each lifecycle phase.

### NFR-OPS-004 — Troubleshooting

Each transport and shutdown mode MUST have failure and troubleshooting documentation.

## Testing

### NFR-TEST-001 — Concurrency

Concurrent ingestion, scrape, workload exit, and signals MUST have automated race/concurrency coverage.

### NFR-TEST-002 — Container E2E

Docker and Kubernetes usage models MUST have end-to-end tests before stable release.

### NFR-TEST-003 — Fault injection

Tests MUST cover malformed input, disconnects, stalled shutdown, bind failure, file replacement races, and scrape
timeout.

### NFR-TEST-004 — Benchmarks

Release documentation SHOULD publish benchmark methodology and results.
