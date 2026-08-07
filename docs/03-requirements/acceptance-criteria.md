# Acceptance Criteria

> Status: Accepted normative acceptance criteria

## Traceability

| Requirements  | Scenarios |
|---------------|-----------|
| FR-001–FR-006 | AC-RUN-*  |
| FR-010–FR-016 | AC-ING-*  |
| FR-020–FR-026 | AC-MET-*  |
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

The same complete application snapshot MUST run against socket, file, and local HTTP transports.

### AC-ING-001 — File snapshot equivalence

Given a complete dataset containing counters, gauges, and histograms, file ingestion exposes the expected dataset.

### AC-ING-002 — Socket snapshot equivalence

The same complete dataset submitted as one framed socket snapshot produces exposition identical to AC-ING-001.

### AC-ING-003 — HTTP snapshot equivalence

The same complete dataset submitted in one local HTTP request produces exposition identical to AC-ING-001.

### AC-ING-004 — Atomic file visibility

During file replacement, a scrape sees either the previous valid state or the new valid state, never a mixed or partial
state.

### AC-ING-005 — Malformed socket input

Invalid socket input is rejected atomically, leaves the last accepted snapshot unchanged, is diagnosed, and does not
terminate the workload.

### AC-ING-006 — Malformed file

Invalid file input is rejected atomically under the documented fallback policy and leaves the last accepted snapshot
unchanged.

### AC-ING-007 — Malformed HTTP input

Malformed HTTP input receives the documented atomic rejection and leaves the last accepted snapshot unchanged.

### AC-ING-008 — Capacity limit

A candidate exceeding a configured limit is rejected atomically without changing the last accepted snapshot or causing
uncontrolled memory growth.

### AC-ING-009 — Explicit transport

The selected transport is observable in effective configuration; disabled transports are not silently accepted.

### AC-ING-010 — Transport failure isolation

Failure of the selected metrics transport does not terminate the workload by default.

### AC-ING-011 — Empty body is not an empty snapshot

An empty HTTP body is rejected as a malformed transport payload and leaves the last accepted snapshot unchanged. A
syntactically valid complete snapshot containing zero metric families or series is accepted as a zero-series snapshot.

### AC-ING-012 — Acknowledgement and acceptance order

A socket or local HTTP success acknowledgement is returned only after the candidate is validated and atomically
installed. Concurrent accepted candidates receive one linear acceptance order, and the last candidate in that order is
active. Producer timestamps do not change that order.

## Metric model

### AC-MET-001 — Counter

A structurally valid counter in one snapshot is exposed with correct Prometheus counter representation. MetricShell does
not compare successive snapshots to enforce business-level counter monotonicity.

### AC-MET-002 — Gauge

A valid gauge can increase and decrease.

### AC-MET-003 — Histogram

A valid classic histogram snapshot has ordered boundaries, cumulative bucket values, a `+Inf` bucket equal to `count`,
and a structurally valid numeric `sum`; it is exposed correctly.

### AC-MET-004 — Invalid names and labels

A candidate snapshot containing any invalid metric or label name is rejected atomically. The last accepted snapshot
remains unchanged.

### AC-MET-005 — Duplicate series

A candidate snapshot containing the same series more than once is rejected atomically and does not change active state.

### AC-MET-006 — Type-binding conflict

A type conflict within a candidate snapshot or with an established metric-family name-to-type binding is rejected
atomically. Omitting all series of the family and later reusing its name with a different type remains a conflict during
the same workload execution. A HELP change alone does not violate the type binding when the candidate remains internally
consistent.

### AC-MET-007 — Series and label limits

A candidate exceeding configured series or label limits is rejected atomically, the last accepted snapshot remains
unchanged, and diagnostic signals are emitted.

### AC-MET-008 — Payload limit

Oversized input is rejected without unbounded allocation.

### AC-MET-009 — Last valid snapshot retention

Given an accepted snapshot followed by a malformed or conflicting candidate snapshot, exposition continues to serve the
previously accepted snapshot unchanged.

### AC-MET-010 — No cross-producer aggregation

Given an accepted complete application snapshot containing a series with value `2`, and a subsequently accepted complete
application snapshot containing the same series with value `3`, exposition contains `3`. MetricShell never derives `5`
and never retains producer-scoped contributions. Workload components must coordinate before publication.

### AC-MET-011 — Zero-series snapshot

Given an accepted snapshot containing application series, accepting a correctly encoded zero-series snapshot removes
all application series while MetricShell self-metrics remain available.

## Exposition

### AC-EXP-001 — Parseable response

A successful metrics request returns a declared compatible content type and payload parseable by compatible Prometheus
tooling.

### AC-EXP-002 — Consistent concurrent scrape

Concurrent ingestion and scraping never produce a torn or syntactically invalid response.

### AC-EXP-003 — Response contents

The response contains the active application snapshot—initially zero-series or subsequently accepted—and documented
MetricShell self-metrics, but no unrelated host-wide metrics.

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

### AC-FIN-015 — No application publication

If the workload terminates without any accepted application snapshot, the final application state contains zero
application series and documented MetricShell self-metrics remain available.

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
