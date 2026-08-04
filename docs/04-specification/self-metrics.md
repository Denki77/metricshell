# MetricShell Self-Metrics Specification

[Russian version](../../docs-ru/04-specification/self-metrics.md)

> Status: Accepted normative specification
> Requirement: FR-051
> Acceptance criteria: AC-EXP-003, AC-OBS-001–AC-OBS-003
> Decisions: ADR-001–ADR-015

## Purpose

This specification defines the stable MetricShell self-metric namespace, metric names, types, labels and update rules.
Self-metrics describe MetricShell operation only. They never contain application metric labels or values.

## General rules

- All names begin with `metricshell_`.
- The `metricshell_` prefix is reserved; application candidates using it are rejected with `reserved_name`.
- Counters end in `_total` and reset on every MetricShell process start.
- Durations use seconds; sizes use bytes; absolute deadlines use Unix timestamp seconds.
- Every metric includes valid HELP and TYPE metadata.
- Self-metrics remain live while the application snapshot is frozen during final wait.
- Application filtering does not remove self-metrics.
- Labels use only the bounded enumerations in this document.
- Raw error text, paths, metric names, label names/values, request IDs, publication IDs, PIDs and client addresses are
  prohibited as metric labels.

## Build and runtime metrics

| Metric                               | Type    | Labels                | Semantics                                                                      |
|--------------------------------------|---------|-----------------------|--------------------------------------------------------------------------------|
| `metricshell_build_info`             | gauge   | `version`, `revision` | Constant `1` for the running build.                                            |
| `metricshell_uptime_seconds`         | gauge   | none                  | Seconds since MetricShell process start.                                       |
| `metricshell_runtime_state`          | gauge   | `state`               | One-hot state vector; every known state is emitted, exactly one has value `1`. |
| `metricshell_runtime_failures_total` | counter | `reason`              | Unrecoverable MetricShell failures by bounded class.                           |

Allowed `state` values:

```text
initializing
starting_workload
running
stopping
finalizing
final_wait
failed
terminated
```

Allowed runtime failure reasons:

```text
configuration
bind
workload_start
protocol
resource
internal
```

## Workload process metrics

| Metric                                           | Type    | Labels             | Semantics                                                                       |
|--------------------------------------------------|---------|--------------------|---------------------------------------------------------------------------------|
| `metricshell_workload_running`                   | gauge   | none               | `1` while the primary workload is running, otherwise `0`.                       |
| `metricshell_workload_process_id`                | gauge   | none               | Primary workload PID, or `0` before start/after exit.                           |
| `metricshell_workload_starts_total`              | counter | `outcome`          | Workload start attempts.                                                        |
| `metricshell_workload_exit_code`                 | gauge   | none               | Resolved MetricShell process-compatible workload result, `-1` while unresolved. |
| `metricshell_workload_signals_forwarded_total`   | counter | `signal`, `target` | Signals forwarded by MetricShell.                                               |
| `metricshell_workload_forced_terminations_total` | counter | none               | Workload/process-group SIGKILL operations after grace expiry.                   |
| `metricshell_children_reaped_total`              | counter | `kind`             | Direct or adopted child processes reaped by MetricShell.                        |

Allowed labels:

```text
outcome = started | start_failed
signal  = TERM | INT | HUP | QUIT | KILL
 target = process | process_group
kind    = direct | adopted
```

## Snapshot and ingestion metrics

| Metric                                                 | Type    | Labels                 | Semantics                                                                              |
|--------------------------------------------------------|---------|------------------------|----------------------------------------------------------------------------------------|
| `metricshell_snapshot_generation`                      | gauge   | none                   | Internal accepted-generation number; initial zero-series snapshot is generation `0`.   |
| `metricshell_snapshot_series`                          | gauge   | none                   | Application series in the active complete snapshot after acceptance, before filtering. |
| `metricshell_snapshot_bytes`                           | gauge   | none                   | Canonical uncompressed application snapshot bytes.                                     |
| `metricshell_snapshot_publications_total`              | counter | `transport`, `outcome` | Complete publication outcomes.                                                         |
| `metricshell_snapshot_rejections_total`                | counter | `transport`, `reason`  | Atomic candidate rejections by bounded reason.                                         |
| `metricshell_ingestion_inflight`                       | gauge   | `transport`            | Currently executing ingestion operations.                                              |
| `metricshell_ingestion_connections`                    | gauge   | `transport`            | Accepted active connections where applicable; file transport remains `0`.              |
| `metricshell_ingestion_last_success_timestamp_seconds` | gauge   | `transport`            | Unix timestamp of the last accepted snapshot, or `0`.                                  |

Allowed transport and outcome values:

```text
transport = file | unix | http
outcome   = accepted | rejected | busy | timeout | internal_error
```

Allowed rejection reasons:

```text
malformed
numeric_invalid
schema_version
empty_payload
payload_limit
series_limit
label_limit
name_limit
policy
duplicate_series
type_conflict
metadata_conflict
histogram_invalid
reserved_name
frozen
internal
```

A reason is the fixed whole-candidate registry from the application snapshot protocol. `busy` and `timeout` are
publication outcomes, while framing failures use the socket-frame registry below; neither category is added to this
label. Implementations must not append raw parser text to a label value.

## File and socket diagnostics

| Metric                                     | Type    | Labels               | Semantics                                                    |
|--------------------------------------------|---------|----------------------|--------------------------------------------------------------|
| `metricshell_file_reconciliations_total`   | counter | `trigger`, `outcome` | File state reconciliation attempts.                          |
| `metricshell_file_watch_events_total`      | counter | `event`              | Watch overflow, invalidation and successful reinstallation.  |
| `metricshell_socket_transactions_inflight` | gauge   | none                 | Bounded multipart candidate transactions currently retained. |
| `metricshell_socket_frames_rejected_total` | counter | `reason`             | Frame-level rejections that do not install a snapshot.       |

Allowed values:

```text
trigger = startup | event | periodic | overflow | watch_reinstall
outcome = accepted | unchanged | absent | invalid | error
event   = overflow | invalidated | reinstalled
reason  = malformed | protocol_version | frame_limit | part_limit | duplicate_part | missing_part | transaction_invalid | transaction_expired
```

## Filtering metrics

| Metric                        | Type  | Labels    | Semantics                                                            |
|-------------------------------|-------|-----------|----------------------------------------------------------------------|
| `metricshell_filter_rules`    | gauge | `kind`    | Effective normalized include/exclude rule count.                     |
| `metricshell_filter_families` | gauge | `outcome` | Families included/excluded in the currently encoded exposition view. |

Allowed values:

```text
kind    = include | exclude
outcome = included | excluded
```

## Exposition metrics

| Metric                                  | Type    | Labels              | Semantics                                               |
|-----------------------------------------|---------|---------------------|---------------------------------------------------------|
| `metricshell_exposition_requests_total` | counter | `format`, `outcome` | `/metrics` response outcomes after the result is known. |
| `metricshell_exposition_inflight`       | gauge   | none                | Active `/metrics` handlers.                             |
| `metricshell_exposition_response_bytes` | gauge   | `format`            | Last complete uncompressed encoded response size.       |

Allowed values:

```text
format  = prometheus | openmetrics
outcome = success | write_error | response_limit | encoding_error | timeout
```

`success` increments only after the complete response write finishes without a cancelled request context.

## Final-wait metrics

| Metric                                              | Type    | Labels    | Semantics                                              |
|-----------------------------------------------------|---------|-----------|--------------------------------------------------------|
| `metricshell_final_wait_active`                     | gauge   | none      | `1` during a natural-completion final wait.            |
| `metricshell_final_wait_mode_info`                  | gauge   | `mode`    | Constant `1` for the effective final-wait mode.        |
| `metricshell_final_wait_required_scrapes`           | gauge   | none      | Effective required scrape count.                       |
| `metricshell_final_wait_completed_scrapes`          | gauge   | none      | Saturating eligible completed response count.          |
| `metricshell_final_scrape_attempts_total`           | counter | `outcome` | Final-state scrape attempts classified after handling. |
| `metricshell_final_wait_completions_total`          | counter | `reason`  | Final-wait terminal reason.                            |
| `metricshell_final_wait_deadline_timestamp_seconds` | gauge   | none      | Absolute timeout deadline, or `0` outside timed wait.  |

Allowed values:

```text
mode    = immediate | duration | scrapes
outcome = completed | ineligible | write_error | cancelled
reason  = immediate | duration_elapsed | required_scrapes | timeout | external_termination | runtime_failure
```

Health, readiness and debug endpoints never increment `metricshell_final_scrape_attempts_total`.

## Shutdown metrics

| Metric                                            | Type      | Labels  | Semantics                                                           |
|---------------------------------------------------|-----------|---------|---------------------------------------------------------------------|
| `metricshell_shutdown_active`                     | gauge     | none    | `1` after external termination begins and before process exit.      |
| `metricshell_shutdown_deadline_timestamp_seconds` | gauge     | none    | Effective absolute external deadline, or `0` when unknown/inactive. |
| `metricshell_shutdown_phase_duration_seconds`     | histogram | `phase` | Completed phase durations.                                          |

Allowed phases:

```text
workload_wait
forced_termination
finalization
http_drain
total
```

The initial histogram buckets are:

```text
0.001, 0.005, 0.01, 0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10, 30, 60
```

## Cardinality and lifecycle rules

- All label value registries are closed enums except build `version` and `revision`.
- At most one build-info series exists.
- State, mode and other one-hot metrics emit the full known enum set for stable time-series identity.
- Snapshot generation increments only after atomic installation, including a valid zero-series snapshot.
- Rejected candidates do not change generation, series or snapshot bytes.
- Metrics are process-local and reset on a new MetricShell execution.

## Compatibility

Removing or renaming a metric or label is a breaking change. Adding an enum value is a compatibility change that must be
announced. New metrics may be added in a minor release. Experimental metrics must use the prefix
`metricshell_experimental_` and are not covered by the stable compatibility promise.

## Conformance requirements

Tests must validate all HELP/TYPE metadata, exact names, label enum rejection, no unbounded labels, reset behavior,
one-hot state/mode behavior, generation updates, last-valid retention, final-wait live self-metrics, filtering immunity
and Prometheus/OpenMetrics parsing.

## References

- [Functional Requirements](../03-requirements/functional-requirements.md#fr-051--self-metrics)
- [ADR-010](../06-architecture/adr/ADR-010.md)
- [ADR-011](../06-architecture/adr/ADR-011.md)
- [ADR-014](../06-architecture/adr/ADR-014.md)
- [Metric Filtering Specification](metrics-filtering.md)
