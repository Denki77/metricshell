# Structured Logging Specification

[Russian version](../../docs-ru/04-specification/structured-logging.md)

> Status: Accepted normative specification
> Requirement: FR-052
> Acceptance criteria: AC-OBS-001–AC-OBS-003, AC-CONF-005
> Decisions: ADR-001–ADR-015

## Purpose

This specification defines the stable schema and mandatory event registry for MetricShell-owned structured logs.
Workload stdout/stderr is passed through unchanged and is not parsed, wrapped or claimed to conform to this schema.

## Encoding and destination

- One UTF-8 JSON object per line (JSON Lines).
- MetricShell logs are always written to stderr in version 1.
- No ANSI color or multi-line records.
- Timestamps use UTC RFC 3339 with nanosecond precision.
- Floating-point special values are prohibited.
- Field names are lowercase `snake_case`.
- The schema version is the string `"1"`.

`log.level=info` emits every info, warn, and error registry event. `log.level=debug` additionally emits every debug
registry event; it never suppresses higher levels. There is no `warn`, `error`, `off`, or destination setting in version
1, so mandatory lifecycle and rejection records cannot be disabled.

## Mandatory base fields

Every MetricShell log record contains:

| Field            | Type    | Meaning                                                                                                 |
|------------------|---------|---------------------------------------------------------------------------------------------------------|
| `timestamp`      | string  | UTC RFC3339Nano timestamp.                                                                              |
| `sequence`       | integer | Process-local monotonically increasing event sequence.                                                  |
| `schema_version` | string  | Structured log schema version, initially `"1"`.                                                         |
| `level`          | string  | `debug`, `info`, `warn` or `error`.                                                                     |
| `event`          | string  | Stable event identifier from the registry below.                                                        |
| `component`      | string  | `runtime`, `workload`, `ingestion`, `file`, `socket`, `http`, `exposition`, `final_wait` or `shutdown`. |
| `runtime_id`     | string  | Random identifier for one MetricShell process lifetime.                                                 |
| `state`          | string  | Effective runtime state at emission time.                                                               |
| `message`        | string  | Short human-readable summary without raw payload data.                                                  |

`sequence`, not timestamp alone, is the authoritative order inside one process.

## Common optional fields

| Field                 | Type    | Use                                                                                        |
|-----------------------|---------|--------------------------------------------------------------------------------------------|
| `duration_ms`         | number  | Completed operation duration.                                                              |
| `deadline`            | string  | UTC RFC3339Nano absolute deadline.                                                         |
| `remaining_ms`        | integer | Remaining bounded time when evaluated.                                                     |
| `pid`                 | integer | MetricShell PID.                                                                           |
| `workload_pid`        | integer | Primary workload PID.                                                                      |
| `workload_pgid`       | integer | Workload process-group ID.                                                                 |
| `exit_code`           | integer | Resolved process-compatible result.                                                        |
| `signal`              | string  | Stable signal name such as `TERM`.                                                         |
| `forced`              | boolean | Whether forced termination occurred.                                                       |
| `transport`           | string  | `file`, `unix` or `http`.                                                                  |
| `snapshot_generation` | integer | Accepted internal generation.                                                              |
| `snapshot_bytes`      | integer | Candidate/canonical bytes when safe to record.                                             |
| `series`              | integer | Candidate/active application series count.                                                 |
| `reason`              | string  | Bounded event-specific reason enum defined below.                                          |
| `previous_state`      | string  | Public state before a state transition.                                                    |
| `mode`                | string  | Final-wait mode from the self-metrics registry.                                            |
| `kind`                | string  | Child kind from the self-metrics registry.                                                 |
| `trigger`             | string  | File-reconciliation trigger from the self-metrics registry.                                |
| `outcome`             | string  | Event-specific outcome from the self-metrics registry.                                     |
| `watch_event`         | string  | File-watch event from the self-metrics registry.                                           |
| `suppressed_event`    | string  | Event identifier summarized after rate limiting.                                           |
| `suppressed_count`    | integer | Records suppressed in the summary window.                                                  |
| `window_ms`           | integer | Suppression-window duration.                                                               |
| `http_status`         | integer | HTTP status generated by MetricShell.                                                      |
| `publication_id`      | string  | Socket correlation ID; debug level only and length-bounded.                                |
| `request_id`          | string  | MetricShell-generated request correlation ID; debug level only.                            |
| `error_code`          | string  | Stable machine-readable internal code.                                                     |
| `error_message`       | string  | Sanitized error summary.                                                                   |
| `selectors`           | array   | Normalized filtering selectors; only on `configuration.validated` when explicitly enabled. |

Unknown fields must be ignored by consumers. Existing fields cannot change type within schema version `1`.

## Mandatory event registry

| Event                         | Minimum level | Required event fields                                                         | Emission rule                                       |
|-------------------------------|---------------|-------------------------------------------------------------------------------|-----------------------------------------------------|
| `runtime.initializing`        | info          | `pid`                                                                         | Once after logger initialization.                   |
| `runtime.state_changed`       | info          | `previous_state` except initial transition                                    | Once for every public state transition.             |
| `configuration.validated`     | info          | none                                                                          | Once before starting workload; secrets omitted.     |
| `configuration.rejected`      | error         | `reason`, `error_code`                                                        | Every terminal invalid configuration.               |
| `endpoint.bound`              | info          | `component`                                                                   | Each required listener/socket successfully created. |
| `endpoint.bind_failed`        | error         | `component`, `reason`, `error_code`                                           | Required bind failure.                              |
| `workload.starting`           | info          | none                                                                          | Immediately before start attempt.                   |
| `workload.started`            | info          | `workload_pid`, `workload_pgid`                                               | After successful start.                             |
| `workload.start_failed`       | error         | `reason`, `error_code`                                                        | Failed start attempt.                               |
| `workload.signal_forwarded`   | info          | `signal`, `workload_pid` or `workload_pgid`                                   | Every forwarded control/termination signal.         |
| `workload.exited`             | info          | `exit_code`, `forced`                                                         | Exactly once after outcome resolution.              |
| `child.reaped`                | debug         | `kind`                                                                        | Managed direct/adopted child reaped.                |
| `snapshot.accepted`           | debug         | `transport`, `snapshot_generation`, `snapshot_bytes`, `series`, `duration_ms` | After atomic installation.                          |
| `snapshot.rejected`           | warn          | `transport`, `reason`, `duration_ms`                                          | Every candidate rejection; no payload content.      |
| `ingestion.overloaded`        | warn          | `transport`, `outcome`                                                        | Queue/semaphore/connection refusal.                 |
| `file.reconciled`             | debug         | `trigger`, `outcome`, `duration_ms`                                           | Startup/event/periodic reconciliation.              |
| `file.watch_recovered`        | warn          | `watch_event`                                                                 | Overflow, invalidation or watch reinstall recovery. |
| `socket.transaction_expired`  | warn          | `reason`                                                                      | Bounded multipart transaction expiry.               |
| `exposition.failed`           | warn          | `outcome`, `http_status`                                                      | Encoding, response-limit or write failure.          |
| `final_wait.started`          | info          | `mode`; `deadline` for duration/scrapes                                       | Once after final snapshot freeze.                   |
| `final_scrape.counted`        | debug         | `request_id`, `snapshot_generation`                                           | Eligible complete final response counted.           |
| `final_scrape.not_counted`    | debug         | `request_id`, `outcome`                                                       | Ineligible, cancelled or failed final response.     |
| `final_wait.completed`        | info          | `reason`, `duration_ms`                                                       | Exactly once for the final-wait terminal condition. |
| `shutdown.started`            | info          | `signal`, `deadline`, `remaining_ms`                                          | External termination begins.                        |
| `shutdown.forced`             | warn          | `signal`, `workload_pid` or `workload_pgid`                                   | Workload grace expires and force is applied.        |
| `shutdown.completed`          | info          | `duration_ms`, `exit_code`                                                    | Shutdown phases finish.                             |
| `runtime.failed`              | error         | `reason`, `error_code`                                                        | Unrecoverable MetricShell failure.                  |
| `runtime.terminated`          | info          | `exit_code`, `duration_ms`                                                    | Final MetricShell-owned record before process exit. |
| `logging.suppression_summary` | warn          | `suppressed_event`, `suppressed_count`, `window_ms`; optional `reason`        | Periodic summary for rate-limited records.          |

High-frequency success events are debug-level so default info logging does not scale with publication or scrape rate.
Rejections and lifecycle boundaries remain visible at normal levels.

When `log.selector_values=true`, `configuration.validated` may contain the normalized `selectors` array. When false,
the field is absent. No other event contains selector values.

## Closed field and error registries

Shared fields use the exact self-metrics values: `kind=direct|adopted`; file `trigger` and `outcome`; `watch_event` uses
the self-metric `event` values; final-wait `mode` and completion `reason`; snapshot rejection `reason`; socket-frame
`reason`; and exposition `outcome`. `ingestion.overloaded` uses `outcome=busy`. Event-specific fields must not be
substituted with a generic `reason`.

The following table is the single normative mapping for MetricShell-owned process failures:

| Failure symbol          | Exit | Runtime failure reason | Structured event         | `error_code`            |
|-------------------------|-----:|------------------------|--------------------------|-------------------------|
| `configuration_invalid` |   64 | `configuration`        | `configuration.rejected` | `CONFIG_INVALID`        |
| `internal_failure`      |   70 | `internal`             | `runtime.failed`         | `INTERNAL_FAILURE`      |
| `resource_unavailable`  |   71 | `resource`             | `runtime.failed`         | `RESOURCE_UNAVAILABLE`  |
| `endpoint_bind_failed`  |   72 | `bind`                 | `endpoint.bind_failed`   | `BIND_FAILED`           |
| `workload_start_failed` |   73 | `workload_start`       | `workload.start_failed`  | `WORKLOAD_START_FAILED` |

Non-process operational failures additionally use the closed codes `SNAPSHOT_MALFORMED`, `SNAPSHOT_LIMIT`,
`INGESTION_BUSY`, `SOCKET_PROTOCOL`, and `FINAL_WAIT_TIMEOUT`. No other schema-version-1 error code is valid. Raw
parser,
filesystem, socket, or HTTP errors appear only in a sanitized `error_message`; they never become enum values.

## Privacy and redaction

MetricShell must not log by default:

- environment variable values;
- complete command arguments;
- metric payloads;
- metric or label values;
- credentials, tokens, headers or cookies;
- raw file contents;
- arbitrary remote addresses;
- unbounded parser errors.

Known secret configuration keys are replaced with `"[REDACTED]"`. Newline, tab and control characters in external error
text are escaped by JSON encoding. Error messages are truncated after sanitization.

## Size and rate bounds

- Maximum encoded MetricShell event size: `16KiB`.
- Maximum `message` or `error_message`: `4KiB` after UTF-8 validation.
- Oversized optional fields are omitted and `truncated: true` is added.
- Repeated identical warn/error events are rate-limited per `event + reason` to 10 records/second with burst 20.
- The first suppressed event remains observable and `logging.suppression_summary` is emitted at least every 60 seconds
  while suppression continues and once when the window closes.
- Lifecycle terminal events are never rate-limited.

## State transition rule

Every externally observable runtime state transition emits `runtime.state_changed` once. The record carries the new
`state` and `previous_state` except for the initial transition. It is emitted after the state becomes effective and
before work specific to the new state can complete.

## Examples

```json
{
  "timestamp": "2026-08-04T09:30:00.123456789Z",
  "sequence": 12,
  "schema_version": "1",
  "level": "info",
  "event": "workload.started",
  "component": "workload",
  "runtime_id": "7bd6c74b",
  "state": "running",
  "message": "workload started",
  "workload_pid": 42,
  "workload_pgid": 42
}
```

```json
{
  "timestamp": "2026-08-04T09:30:03.100000000Z",
  "sequence": 31,
  "schema_version": "1",
  "level": "warn",
  "event": "snapshot.rejected",
  "component": "ingestion",
  "runtime_id": "7bd6c74b",
  "state": "running",
  "message": "candidate snapshot rejected",
  "transport": "unix",
  "reason": "duplicate_series",
  "snapshot_bytes": 8120,
  "duration_ms": 1.42
}
```

```json
{
  "timestamp": "2026-08-04T09:31:00.000000000Z",
  "sequence": 58,
  "schema_version": "1",
  "level": "info",
  "event": "final_wait.completed",
  "component": "final_wait",
  "runtime_id": "7bd6c74b",
  "state": "final_wait",
  "message": "final wait completed",
  "reason": "required_scrapes",
  "duration_ms": 842.7,
  "exit_code": 17
}
```

## Compatibility

Event identifiers and mandatory fields form a stable surface. Removing an event, renaming a field or changing a field
type requires a major schema version. Adding optional fields is backward compatible. Consumers must tolerate unknown
events and fields.

## Conformance requirements

Tests must validate JSON Lines encoding, mandatory fields, monotonic sequence, UTC timestamps, event-specific fields,
redaction, size truncation, rate limiting, exactly-once terminal lifecycle events, no raw payload logging and
consistency between log reasons and self-metric label enums. The matrix must cover info/debug selection,
selector-values false/true, all five process-failure mapping rows, and every supplementary operational error code.

## References

- [Functional Requirements](../03-requirements/functional-requirements.md#fr-052--structured-logs)
- [Non-functional Requirements](../03-requirements/non-functional-requirements.md)
- [Configuration](configuration.md)
- [Runtime Defaults and Resource Limits](runtime-defaults-and-resource-limits.md)
- [Self-metrics Specification](self-metrics.md)
- [Runtime State Machine](runtime-state-machine.md)
