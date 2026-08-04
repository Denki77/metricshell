# Configuration Specification

[Russian version](../../docs-ru/04-specification/configuration.md)

> Status: Accepted normative specification
> Requirements: FR-006, FR-013, FR-024, FR-046, FR-052, FR-080–FR-082
> Acceptance criteria: AC-RUN-001, AC-ING-008–AC-ING-009, AC-CONF-001–AC-CONF-005
> Decisions: ADR-001–ADR-003, ADR-005–ADR-008, ADR-010–ADR-014

## Command syntax

~~~text
metricshell [MetricShell options] -- executable [argument ...]
metricshell --version
metricshell --help
~~~

The first standalone -- is mandatory for workload execution. Every token after it is workload argv and is passed without
shell interpretation. An empty workload argv is configuration_invalid. MetricShell options after -- are workload
arguments. Shell behavior requires an explicit shell workload such as -- /bin/sh -c command.

Core has no mode option. In particular, --mode, --mode=snapshot, and --mode=managed-registry are invalid.

## Sources and precedence

Version 1 supports command-line options, environment variables, and compiled defaults. It does not support a
configuration file. Precedence is:

~~~text
CLI > environment > compiled default
~~~

A repeatable CLI option preserves occurrence order. Repeatable environment values use the list grammar from the
[Configuration Value Grammar](configuration-value-grammar.md); version 1 has no escaping. Empty environment values are
treated as explicitly empty, not absent. Unknown options and METRICSHELL_ variables named by this specification with
invalid values are fatal before workload start.

## Core endpoints and transport

| Property              | CLI                     | Environment                       | Default                        |
|-----------------------|-------------------------|-----------------------------------|--------------------------------|
| ingestion.transport   | --ingestion-transport   | METRICSHELL_INGESTION_TRANSPORT   | unix                           |
| exposition.listen     | --exposition-listen     | METRICSHELL_EXPOSITION_LISTEN     | 0.0.0.0:9090                   |
| http_ingestion.listen | --http-ingestion-listen | METRICSHELL_HTTP_INGESTION_LISTEN | 127.0.0.1:9091                 |
| socket.path           | --unix-socket-path      | METRICSHELL_UNIX_SOCKET_PATH      | /run/metricshell/ingest.sock   |
| file.path             | --snapshot-file-path    | METRICSHELL_SNAPSHOT_FILE_PATH    | /run/metricshell/snapshot.json |
| shutdown.deadline     | --shutdown-deadline     | METRICSHELL_SHUTDOWN_DEADLINE     | empty                          |

ingestion.transport is exactly one of file, unix, or http. Only the selected ingestion listener/watcher is activated.
Explicit transport-specific options for an inactive transport are rejected to expose configuration mistakes.

The exposition server owns these fixed version 1 paths:

~~~text
GET /metrics
GET /healthz
GET /readyz
GET /debug/config
~~~

The local HTTP ingestion server owns POST /v1/metrics. Paths are not configurable in version 1. /debug/config returns
effective non-secret configuration and never workload argv or environment values. Probe semantics are defined by the
runtime state specification.

Listen addresses use Go host:port syntax; an empty host is invalid. HTTP ingestion must resolve to loopback. Unix and
file
paths must be absolute, must have a private parent directory, and must not be symlinks. MetricShell creates the Unix
socket
with mode 0660 and its runtime directory with mode 0700. The socket owner and group are the configured runtime identity;
shared producer access is granted through that group, as required by ADR-007.

## Final wait and shutdown options

| Property                    | CLI                           | Environment                             |
|-----------------------------|-------------------------------|-----------------------------------------|
| final_wait.mode             | --final-wait-mode             | METRICSHELL_FINAL_WAIT_MODE             |
| final_wait.duration         | --final-wait-duration         | METRICSHELL_FINAL_WAIT_DURATION         |
| final_wait.timeout          | --final-wait-timeout          | METRICSHELL_FINAL_WAIT_TIMEOUT          |
| final_wait.required_scrapes | --final-wait-required-scrapes | METRICSHELL_FINAL_WAIT_REQUIRED_SCRAPES |
| final_wait.completion_grace | --final-wait-completion-grace | METRICSHELL_FINAL_WAIT_COMPLETION_GRACE |
| shutdown.total_grace        | --shutdown-total-grace        | METRICSHELL_SHUTDOWN_TOTAL_GRACE        |
| shutdown.workload_timeout   | --workload-shutdown-timeout   | METRICSHELL_WORKLOAD_SHUTDOWN_TIMEOUT   |
| shutdown.reserve            | --shutdown-reserve            | METRICSHELL_SHUTDOWN_RESERVE            |

Durations use the normative [Configuration Value Grammar](configuration-value-grammar.md); `0` is accepted only where
the defaults specification allows it. shutdown.deadline is empty or an RFC3339 timestamp with an explicit offset, for
example
2026-08-05T10:30:00Z. It is an external absolute deadline, must be in the future at startup, and caps every derived
duration. MetricShell never extends it.

## File, socket, and HTTP options

| Property                           | CLI                             | Environment                               |
|------------------------------------|---------------------------------|-------------------------------------------|
| file.reconcile_interval            | --file-reconcile-interval       | METRICSHELL_FILE_RECONCILE_INTERVAL       |
| socket.frame_bytes                 | --socket-max-frame-bytes        | METRICSHELL_SOCKET_MAX_FRAME_BYTES        |
| socket.parts                       | --socket-max-parts              | METRICSHELL_SOCKET_MAX_PARTS              |
| socket.connections                 | --socket-max-connections        | METRICSHELL_SOCKET_MAX_CONNECTIONS        |
| socket.transactions                | --socket-max-transactions       | METRICSHELL_SOCKET_MAX_TRANSACTIONS       |
| socket.transaction_timeout         | --socket-transaction-timeout    | METRICSHELL_SOCKET_TRANSACTION_TIMEOUT    |
| socket.read_timeout                | --socket-read-timeout           | METRICSHELL_SOCKET_READ_TIMEOUT           |
| socket.write_timeout               | --socket-write-timeout          | METRICSHELL_SOCKET_WRITE_TIMEOUT          |
| http_ingestion.wire_bytes          | --http-ingestion-max-wire-bytes | METRICSHELL_HTTP_INGESTION_MAX_WIRE_BYTES |
| http_ingestion.read_header_timeout | --http-read-header-timeout      | METRICSHELL_HTTP_READ_HEADER_TIMEOUT      |
| http_ingestion.read_timeout        | --http-read-timeout             | METRICSHELL_HTTP_READ_TIMEOUT             |
| http_ingestion.write_timeout       | --http-write-timeout            | METRICSHELL_HTTP_WRITE_TIMEOUT            |
| http_ingestion.idle_timeout        | --http-idle-timeout             | METRICSHELL_HTTP_IDLE_TIMEOUT             |
| http_ingestion.max_header_bytes    | --http-max-header-bytes         | METRICSHELL_HTTP_MAX_HEADER_BYTES         |

Multipart socket publication is always supported and has no enable flag. socket.parts, socket.transactions, frame size,
snapshot size, and transaction timeout are its complete configuration surface. One-part publication uses the same
BEGIN/PART/COMMIT grammar.

## Snapshot and exposition limits

| Property                      | CLI                         | Environment                           |
|-------------------------------|-----------------------------|---------------------------------------|
| limits.snapshot_bytes         | --max-snapshot-bytes        | METRICSHELL_MAX_SNAPSHOT_BYTES        |
| limits.decoded_input_bytes    | --max-decoded-input-bytes   | METRICSHELL_MAX_DECODED_INPUT_BYTES   |
| limits.series                 | --max-series                | METRICSHELL_MAX_SERIES                |
| limits.labels_per_series      | --max-labels-per-series     | METRICSHELL_MAX_LABELS_PER_SERIES     |
| limits.metric_name_bytes      | --max-metric-name-bytes     | METRICSHELL_MAX_METRIC_NAME_BYTES     |
| limits.label_name_bytes       | --max-label-name-bytes      | METRICSHELL_MAX_LABEL_NAME_BYTES      |
| limits.label_value_bytes      | --max-label-value-bytes     | METRICSHELL_MAX_LABEL_VALUE_BYTES     |
| limits.help_bytes             | --max-help-bytes            | METRICSHELL_MAX_HELP_BYTES            |
| limits.concurrent_ingestions  | --max-concurrent-ingestions | METRICSHELL_MAX_CONCURRENT_INGESTIONS |
| limits.pending_ingestions     | --max-pending-ingestions    | METRICSHELL_MAX_PENDING_INGESTIONS    |
| exposition.response_bytes     | --max-response-bytes        | METRICSHELL_MAX_RESPONSE_BYTES        |
| exposition.concurrent_scrapes | --max-concurrent-scrapes    | METRICSHELL_MAX_CONCURRENT_SCRAPES    |
| exposition.write_timeout      | --exposition-write-timeout  | METRICSHELL_EXPOSITION_WRITE_TIMEOUT  |

Byte sizes and counts use the normative [Configuration Value Grammar](configuration-value-grammar.md). Exact defaults,
ranges, and cross-field validation are normative in the defaults and resource-limits specification.

## Logging options

| Property            | CLI                         | Environment                       |
|---------------------|-----------------------------|-----------------------------------|
| log.level           | --log-level                 | METRICSHELL_LOG_LEVEL             |
| log.selector_values | --log-selector-values       | METRICSHELL_LOG_SELECTOR_VALUES   |

`log.level` is exactly `info` or `debug`. `log.selector_values` uses the normative boolean grammar. MetricShell-owned
JSON Lines always go to stderr in version 1; the destination is not configurable. Filtering adds the repeatable
--metrics-include and --metrics-exclude options and their environment variables exactly as defined by the filtering
specification. No other Core CLI option exists in version 1.

## File-descriptor validation

Before workload start, MetricShell computes:

~~~text
required_nofile =
  16
  + socket.connections when transport=unix
  + exposition.concurrent_scrapes
  + limits.concurrent_ingestions
~~~

The reserve 16 covers stdio, signal/process handles, listeners, file watch, reconciliation file, and bounded internal
overhead. If the soft RLIMIT_NOFILE is lower than required_nofile, configuration is rejected. MetricShell does not raise
the limit automatically. The same formula is reported by /debug/config.

## Startup validation

All static validation, directory checks, listener binds, and file-watch setup occur before workload start. Failures
close
all partially acquired resources. Contradictory values, an unavailable required endpoint, invalid absolute deadline,
insufficient RLIMIT_NOFILE, or an explicitly configured inactive transport option are fatal.

## MetricShell-owned exit codes

The permanent version 1 registry is:

| Code | Symbol                | Meaning                                                                             |
|-----:|-----------------------|-------------------------------------------------------------------------------------|
|   64 | configuration_invalid | CLI, environment, cross-field, path, or resource-limit validation failed.           |
|   70 | internal_failure      | Invariant violation or uncategorized MetricShell software failure.                  |
|   71 | resource_unavailable  | Required OS resource or runtime facility is unavailable.                            |
|   72 | endpoint_bind_failed  | Required exposition, ingestion, socket, or watch endpoint could not be established. |
|   73 | workload_start_failed | Workload executable could not be started.                                           |

The normative mapping from these symbols to runtime failure reasons, structured events, and `error_code` values is the
single table in the [Structured Logging Specification](structured-logging.md#closed-field-and-error-registries).

A started workload result is otherwise preserved, including 0, arbitrary non-zero codes, and 128+signal mapping. Because
a workload may return any byte-sized code, logs and the workload-started flag are authoritative for origin; MetricShell
never remaps an already obtained workload result merely because it numerically equals a registry value.

## References

- [Runtime State Machine](runtime-state-machine.md)
- [Application Snapshot Protocol](application-snapshot-protocol.md)
- [Runtime Defaults and Resource Limits](runtime-defaults-and-resource-limits.md)
- [Configuration Value Grammar](configuration-value-grammar.md)
- [Metric Filtering](metrics-filtering.md)
- [Structured Logging](structured-logging.md)
