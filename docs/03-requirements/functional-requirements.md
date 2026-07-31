# Functional Requirements

> Status: Draft for architecture investigation

## Purpose

This document defines externally observable capabilities. It does not prescribe internal packages, concurrency
primitives, storage engines, protocol encodings, or frameworks.

## Runtime lifecycle

### FR-001 — Start arbitrary workload

**MetricShell** MUST start an operator-provided executable with its arguments and environment, without requiring a
specific
language or framework.

### FR-002 — Distinguish workload outcomes

**MetricShell** MUST distinguish normal exit, non-zero exit, signal termination, and workload start failure.

### FR-003 — Preserve workload outcome

When **MetricShell** itself has not failed, the final container outcome MUST preserve the workload outcome. Successful
metrics delivery MUST NOT convert a failed workload into success.

### FR-004 — Forward termination

**MetricShell** MUST forward supported termination signals to the workload or its process group and allow bounded
graceful
shutdown.

### FR-005 — Reap managed children

**MetricShell** MUST NOT leave unreaped child processes for which it is responsible.

### FR-006 — Bound every wait

**MetricShell** MUST NOT keep a completed workload alive indefinitely. Every post-exit and shutdown wait MUST have an
upper
bound.

## Metrics ingestion

The following are required project capabilities. Their protocols and implementations are architecture decisions.

### FR-010 — Socket ingestion

**MetricShell** MUST support local socket-based ingestion. Architecture investigation MUST select the concrete socket
model
and protocol.

### FR-011 — File ingestion

**MetricShell** MUST support local file-based ingestion. A scrape MUST observe either the previous valid state or the
new
valid state, never a partially written state.

### FR-012 — Local push ingestion

**MetricShell** MUST support producer-to-MetricShell push ingestion local to the workload instance. This does not mean
pushing to Prometheus or a central gateway.

### FR-013 — Transport selection

The operator MUST be able to select one ingestion transport explicitly. Architecture investigation MUST select the
default transport. Simultaneous transports MAY be supported only after conflict resolution is defined.

### FR-014 — Equivalent metric semantics

Submitting the same complete application snapshot through file, socket, or local HTTP ingestion MUST produce the same
accepted application state and exposition result. Wire formats, acknowledgement behavior, and performance MAY differ.

### FR-015 — Empty and initial snapshots

Before the first accepted publication, the active application snapshot MUST contain zero application series. An empty
transport payload MUST be treated as malformed. A syntactically valid complete snapshot containing zero metric families
or series MUST be accepted as a zero-series snapshot and MUST clear the active application state. An absent, truncated,
or otherwise malformed transport payload MUST NOT be interpreted as a zero-series snapshot.

### FR-016 — Acknowledged acceptance and ordering

For socket and local HTTP ingestion, a success acknowledgement MUST mean that the complete candidate was validated and
atomically installed. MetricShell MUST assign one linear acceptance order to concurrent accepted candidates and MUST
NOT infer freshness from a producer-supplied timestamp. The workload MUST serialize publication when generation order
matters.

## Metric model

### FR-020 — Metric families

**MetricShell** MUST preserve and expose Prometheus counter, gauge, and histogram types contained in an accepted
snapshot. It MUST NOT act as an instrumentation library: increment, set, observe, or cross-producer value aggregation
are not ingestion operations.

### FR-021 — Prometheus metadata

**MetricShell** MUST preserve or produce valid metric names, label names, values, type information, and any required
exposition metadata.

### FR-022 — Input validation

Structurally invalid input MUST be rejected without crashing or terminating the workload by default. Structural
validation includes syntax, names, labels, type metadata, duplicate series, histogram structure, and configured resource
limits. MetricShell MUST NOT infer or validate the workload's business meaning for otherwise valid metric values.
Rejection MUST be observable.

### FR-023 — Series identity and conflicts

A metric name plus label set MUST identify a series. Duplicate series within a candidate snapshot, type conflicts, and
metadata conflicts MUST cause deterministic atomic rejection; MetricShell MUST NOT resolve them by aggregating values.

### FR-024 — Resource limits

Operators MUST be able to bound total series, labels per series, name/value sizes, payload size, buffered state, and
concurrent ingestion.

### FR-025 — Failure isolation

Metrics failures MUST NOT corrupt workload execution. Strict failure behavior MAY exist only as explicit configuration.

### FR-026 — Metric-family type-binding lifetime

Once a metric-family name is accepted with a Prometheus metric type during one workload execution, the same name MUST
NOT be accepted with a different type until the next workload execution. Omitting every series of the family removes
its values from active state but MUST NOT clear this name-to-type binding. HELP, unit, label names, histogram boundaries,
and other metadata MUST be validated for internal consistency within each candidate but are not lifetime-bound by this
requirement.

## Metrics exposition

### FR-030 — Prometheus-compatible endpoint

**MetricShell** MUST expose a documented Prometheus-compatible metrics endpoint.

### FR-031 — Consistent scrape

While the workload runs, a successful scrape MUST return one internally consistent state despite concurrent ingestion.

### FR-032 — Response scope

A normal scrape MUST include all series from the active complete application snapshot that remain after explicitly
configured filtering, plus required **MetricShell** self-metrics. The active snapshot is the zero-series initial snapshot
or the last accepted snapshot. A scrape MUST NOT silently omit accepted application metrics for producer completeness
reasons or add unrelated host-wide metrics.

### FR-033 — Filtering

Operators SHOULD be able to include or exclude metric families or prefixes.

### FR-034 — Failed scrape semantics

HTTP errors, partial responses, and health indicators MUST follow one documented policy selected during architecture
investigation.

## Finite workload behavior

### FR-040 — Final observable state

After workload termination, **MetricShell** MUST establish one final observable application metric state from the last
valid accepted complete application snapshot. If no snapshot was accepted, the zero-series initial application snapshot
MUST become the final application state. The storage mechanism is an architecture decision.

### FR-041 — Stable final state

While **MetricShell** remains available after workload exit, final application metric values MUST remain stable.

### FR-042 — Immediate mode

**MetricShell** MUST provide a mode with no intentional post-workload scrape wait.

### FR-043 — Fixed-duration mode

**MetricShell** MUST provide a mode that keeps final metrics available for a configured bounded duration.

### FR-044 — Scrape-count mode

**MetricShell** MUST provide a mode that waits for a configured number of eligible final-state scrapes, a timeout, or
external termination—whichever occurs first.

### FR-045 — Eligible scrape

Only a successful response containing the final state MAY count. Health and unrelated endpoint requests MUST NOT count.
Scraper identity rules are an architecture decision.

### FR-046 — Configurable waiting

Operators MUST be able to configure the fixed duration, maximum scrape wait, and required scrape count within
implementation safety bounds.

### FR-047 — Delivery limitation

**MetricShell** MUST document that serving a response does not prove durable persistence in Prometheus or downstream
systems.

## Runtime observability

### FR-050 — Runtime state visibility

Operators MUST distinguish runtime operational, workload running, workload completed, post-exit waiting, forced
termination, and runtime failure.

### FR-051 — Self-metrics

**MetricShell** MUST expose self-metrics for its own operation. Names belong in the metric specification.

### FR-052 — Structured logs

**MetricShell** MUST log lifecycle transitions, invalid configuration, ingestion rejection, forced termination, and
final-wait completion.

## Distribution

### FR-060 — Standalone executable

**MetricShell** MUST be distributable as an executable installable into an existing image.

### FR-061 — Multi-stage Dockerfile

The project MUST provide a supported multi-stage integration that copies required artifacts into an application image.

### FR-062 — Base-image inheritance

The project MUST provide a supported base-image model from which an application image can inherit and then install
dependencies and copy application code.

### FR-063 — Equivalent behavior

Supported distribution models MUST provide equivalent core behavior for the same version and configuration.

## Portability

### FR-070 — OCI/container operation

Core **MetricShell** MUST work in ordinary OCI-compatible containers without Kubernetes APIs.

### FR-071 — Docker examples

The project MUST provide executable Docker and Docker Compose examples.

### FR-072 — Kubernetes examples

The project MUST provide examples for a long-running workload and a finite Job/CronJob-style workload.

### FR-073 — No mandatory central component

Core operation MUST NOT require a shared Pushgateway, Collector, controller, webhook, or datastore. Optional
integrations MAY exist.

## Configuration

### FR-080 — Explicit configuration

Lifecycle, transport, exposure, and final-wait behavior MUST be explicitly configurable and documented.

### FR-081 — Deterministic defaults

Every optional property MUST have a documented default. Defaults are selected after architecture investigation and
validation.

### FR-082 — Validate before workload start

Invalid or contradictory configuration MUST be rejected before starting the workload whenever possible.

## Open architecture decisions

- exact socket and push protocols;
- file format and change detection;
- registry and concurrency model;
- supported exposition versions;
- scraper eligibility and identity;
- default durations and scrape counts;
- runtime exit-code namespace;
- HTTP framework and internal package structure.
