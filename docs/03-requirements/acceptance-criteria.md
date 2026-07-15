# Acceptance Criteria

> Status: Draft for architecture investigation

## Traceability

| Requirements  | Scenarios |
|---------------|-----------|
| FR-001–FR-006 | AC-RUN-*  |
| FR-010–FR-014 | AC-ING-*  |
| FR-020–FR-025 | AC-MET-*  |
| FR-030–FR-034 | AC-EXP-*  |
| FR-040–FR-047 | AC-FIN-*  |
| FR-050–FR-052 | AC-OBS-*  |
| FR-060–FR-063 | AC-DIST-* |
| FR-070–FR-073 | AC-PORT-* |
| FR-080–FR-082 | AC-CONF-* |

## Runtime lifecycle

### AC-RUN-001 — Execute command

Given a valid executable, arguments, and environment, when MetricShell starts, then the workload is started with those
values.

### AC-RUN-002 — Preserve success

Given the workload exits `0`, when configured shutdown behavior completes, then MetricShell exits successfully unless it
has an independent runtime failure.

### AC-RUN-003 — Preserve failure

Given the workload exits `17`, when MetricShell completes, then the container remains failed with the workload outcome
unless a documented higher-precedence runtime failure occurred.

### AC-RUN-004 — Start failure

Given the executable cannot be started, then MetricShell reports workload start failure, does not claim the workload
ran, and exits with a documented runtime result.

### AC-RUN-005 — Signal forwarding

Given a workload records signals, when MetricShell receives its configured termination signal, then the workload or
selected process group receives the expected signal within a bounded interval.

### AC-RUN-006 — Graceful shutdown

Given the workload exits during its grace period, MetricShell does not forcibly terminate it before that deadline.

### AC-RUN-007 — Forced shutdown

Given the workload ignores graceful termination, when the deadline expires, then documented forced termination is
applied and MetricShell terminates within a bound.

### AC-RUN-008 — Child reaping

Given managed child processes exit, then no zombies for which MetricShell is responsible remain.

### AC-RUN-009 — No indefinite wait

For every mode and failure path, expiry of all configured deadlines leads to a terminal outcome.

## Ingestion transports

The same semantic dataset MUST run against socket, file, and local push transports.

### AC-ING-001 — Counter equivalence

A valid counter submitted through each transport is exposed with equivalent final semantics.

### AC-ING-002 — Gauge equivalence

The same gauge updates through each transport produce equivalent current and final values.

### AC-ING-003 — Histogram equivalence

The same observations through each transport produce equivalent buckets, count, and sum.

### AC-ING-004 — Atomic file visibility

During file replacement, a scrape sees either the previous valid state or the new valid state, never a mixed or partial
state.

### AC-ING-005 — Malformed socket input

Invalid socket input is rejected, diagnosed, and does not terminate the workload.

### AC-ING-006 — Malformed file

Invalid file input follows documented fallback/rejection behavior and does not corrupt the last valid state.

### AC-ING-007 — Malformed push

Invalid push input receives documented rejection and does not corrupt accepted state.

### AC-ING-008 — Capacity limit

Input exceeding a configured limit receives bounded deterministic treatment without uncontrolled memory growth.

### AC-ING-009 — Explicit transport

The selected transport is observable in effective configuration; disabled transports are not silently accepted.

### AC-ING-010 — Transport failure isolation

Failure of the selected metrics transport does not terminate the workload by default.

## Metric model

### AC-MET-001 — Counter

A valid counter is exposed with correct Prometheus counter semantics.

### AC-MET-002 — Gauge

A valid gauge can increase and decrease.

### AC-MET-003 — Histogram

A valid histogram exposes cumulative buckets, count, and sum.

### AC-MET-004 — Invalid names and labels

Invalid metric or label names are rejected without affecting accepted valid series.

### AC-MET-005 — Duplicate series

Duplicate updates follow one documented deterministic policy.

### AC-MET-006 — Type conflict

The same family submitted with incompatible types is rejected according to policy.

### AC-MET-007 — Series and label limits

Configured series and label limits are enforced and diagnostic signals are emitted.

### AC-MET-008 — Payload limit

Oversized input is rejected without unbounded allocation.

## Exposition

### AC-EXP-001 — Parseable response

A successful metrics request returns a declared compatible content type and payload parseable by compatible Prometheus
tooling.

### AC-EXP-002 — Consistent concurrent scrape

Concurrent ingestion and scraping never produce a torn or syntactically invalid response.

### AC-EXP-003 — Response contents

The response contains accepted application metrics and documented MetricShell self-metrics, but no unrelated host-wide
metrics.

### AC-EXP-004 — Filtering

Configured family/prefix filtering is deterministic.

### AC-EXP-005 — Concurrent clients

Concurrent scrapes remain safe and bounded.

### AC-EXP-006 — Exposition failure

When no valid response can be produced, documented HTTP and diagnostic behavior is used.

### AC-EXP-007 — Bind failure

If the required endpoint cannot bind before workload start, startup fails unless an explicitly documented degraded mode
is selected.

## Finite workload behavior

### AC-FIN-001 — Establish final state

After workload exit and before a final scrape may count, MetricShell establishes exactly one final observable
application state.

### AC-FIN-002 — Stable final state

Repeated post-exit scrapes return stable application metric values.

### AC-FIN-003 — Immediate mode

Immediate mode introduces no intentional post-workload wait.

### AC-FIN-004 — Fixed-duration mode

For configured duration `D`, the endpoint remains available for approximately `D`, subject to scheduling tolerance and
external termination.

### AC-FIN-005 — Scrapes do not extend delay

Scrapes in fixed-duration mode do not extend its deadline unless explicitly configured as a separate feature.

### AC-FIN-006 — One final scrape

With required count `1`, one eligible completed response satisfies the scrape condition.

### AC-FIN-007 — N final scrapes

With required count `N`, fewer than `N` eligible responses do not satisfy the condition; exactly `N` do.

### AC-FIN-008 — Timeout

If the threshold is not reached, the configured timeout ends waiting without changing the workload outcome by default.

### AC-FIN-009 — Health request excluded

Health/readiness requests never increment final-scrape count.

### AC-FIN-010 — Failed response excluded

A request that does not successfully receive the complete final response does not count.

### AC-FIN-011 — Pre-final scrape excluded

A scrape before final state establishment does not count.

### AC-FIN-012 — Concurrent final scrapes

Concurrent eligible requests follow documented atomic counting without races.

### AC-FIN-013 — External termination precedence

An external shutdown deadline ends post-exit waiting and MetricShell terminates within available grace.

### AC-FIN-014 — No durability claim

Documentation and diagnostics never claim that a served response proves TSDB or remote-write persistence.

## Observability

### AC-OBS-001

Operators can distinguish runtime running, workload running, workload completed while runtime waits, forced termination,
and runtime failure.

### AC-OBS-002

During post-exit waiting, logs or self-metrics expose active mode, remaining condition, and deadline.

### AC-OBS-003

Rejected input, endpoint failures, and forced termination are observable.

## Distribution and portability

### AC-DIST-001

A documented multi-stage Dockerfile builds and runs a reference application without an additional container.

### AC-DIST-002

A reference application inherits from the supported MetricShell base image, installs dependencies and code, and runs
successfully.

### AC-DIST-003

Standalone-copy and base-image builds pass the same core conformance suite.

### AC-DIST-004

Reference images operate as a non-root user and expose build version/revision.

### AC-PORT-001

Reference long-running and finite workloads pass Docker end-to-end tests.

### AC-PORT-002

Docker Compose Prometheus successfully scrapes a reference workload.

### AC-PORT-003

Kubernetes long-running workload exposes metrics and shuts down correctly.

### AC-PORT-004

Kubernetes finite Job-style workload follows its configured final availability and completes.

### AC-PORT-005

The same executable works in Docker without Kubernetes API access, service account, or Kubernetes-specific mounts.

## Configuration

### AC-CONF-001

Valid configuration starts and exposes effective non-secret values.

### AC-CONF-002

Malformed or negative durations and invalid scrape counts are rejected before workload start.

### AC-CONF-003

Contradictory lifecycle or transport options are rejected with an actionable error.

### AC-CONF-004

Omitted optional values resolve to documented deterministic defaults.

### AC-CONF-005

Secrets are redacted from normal logs and diagnostics.
