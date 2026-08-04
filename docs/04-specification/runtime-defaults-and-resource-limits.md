# Runtime Defaults and Resource Limits Specification

[Russian version](../../docs-ru/04-specification/runtime-defaults-and-resource-limits.md)

> Status: Accepted normative specification
> Requirements: FR-024, FR-046, FR-052, FR-080, FR-081, FR-082
> Acceptance criteria: AC-ING-008, AC-MET-007, AC-MET-008, AC-FIN-004, AC-FIN-006–AC-FIN-008, AC-CONF-002–AC-CONF-004
> Decisions: ADR-003, ADR-006, ADR-007, ADR-008, ADR-010, ADR-011, ADR-014, ADR-015

## Purpose

This specification selects deterministic first-stable-release defaults and safety ranges for waiting, ingestion,
exposition and bounded runtime resources. These values are product defaults, not SLOs or universal capacity claims.
Operators may tune values only inside the ranges below.

## Value syntax

All durations, byte sizes, counts, and environment list values use the accepted
[Configuration Value Grammar](configuration-value-grammar.md). The tables below define semantic ranges; they do not
replace or narrow the lexical grammar. Invalid, negative, overflowing, out-of-range, or contradictory values are
rejected before workload start whenever possible.

## Natural completion defaults

| Canonical property            | Environment variable                      |   Default |                      Allowed range | Meaning                                                   |
|-------------------------------|-------------------------------------------|----------:|-----------------------------------:|-----------------------------------------------------------|
| `final_wait.mode`             | `METRICSHELL_FINAL_WAIT_MODE`             | `scrapes` | `immediate`, `duration`, `scrapes` | Post-workload wait after natural completion.              |
| `final_wait.duration`         | `METRICSHELL_FINAL_WAIT_DURATION`         |     `30s` |                           `0`–`1h` | Used only in `duration` mode.                             |
| `final_wait.timeout`          | `METRICSHELL_FINAL_WAIT_TIMEOUT`          |     `60s` |                          `1s`–`1h` | Mandatory upper bound in `scrapes` mode.                  |
| `final_wait.required_scrapes` | `METRICSHELL_FINAL_WAIT_REQUIRED_SCRAPES` |       `1` |                           `1`–`16` | Eligible complete final responses required.               |
| `final_wait.completion_grace` | `METRICSHELL_FINAL_WAIT_COMPLETION_GRACE` |   `500ms` |                           `0`–`5s` | Drain for already accepted responses after the threshold. |

The default `scrapes` mode follows ADR-011. The older draft constraint that named `delay` as the default is superseded.
`30s` remains the default only for explicitly selected `duration` mode.

No post-exit wait begins after external termination has started. In that path the shutdown budget below is
authoritative.

## External termination defaults

| Canonical property          | Environment variable                    | Default | Allowed range | Validation                                              |
|-----------------------------|-----------------------------------------|--------:|--------------:|---------------------------------------------------------|
| `shutdown.total_grace`      | `METRICSHELL_SHUTDOWN_TOTAL_GRACE`      |   `30s` |     `1s`–`1h` | Must match or be lower than the external runtime grace. |
| `shutdown.workload_timeout` | `METRICSHELL_WORKLOAD_SHUTDOWN_TIMEOUT` |   `28s` |      `0`–`1h` | Capped by remaining absolute deadline.                  |
| `shutdown.reserve`          | `METRICSHELL_SHUTDOWN_RESERVE`          |    `2s` |  `250ms`–`1m` | Reserved for MetricShell finalization and HTTP drain.   |

`workload_timeout + reserve` must not exceed `total_grace`. An externally supplied absolute deadline is authoritative and may reduce the effective workload timeout to zero.

## File ingestion defaults

| Canonical property        | Environment variable                  | Default | Allowed range |
|---------------------------|---------------------------------------|--------:|--------------:|
| `file.reconcile_interval` | `METRICSHELL_FILE_RECONCILE_INTERVAL` |    `1s` |  `100ms`–`1s` |

Periodic reconciliation cannot be disabled. Relevant inotify events, overflow and watch invalidation trigger immediate
reconciliation independently of this interval.

## Snapshot and validation defaults

| Canonical property             | Environment variable                    | Default |    Allowed range |
|--------------------------------|-----------------------------------------|--------:|-----------------:|
| `limits.snapshot_bytes`        | `METRICSHELL_MAX_SNAPSHOT_BYTES`        |  `1MiB` |  `64KiB`–`64MiB` |
| `limits.decoded_input_bytes`   | `METRICSHELL_MAX_DECODED_INPUT_BYTES`   |  `2MiB` | `64KiB`–`128MiB` |
| `limits.series`                | `METRICSHELL_MAX_SERIES`                | `10000` |     `1`–`100000` |
| `limits.labels_per_series`     | `METRICSHELL_MAX_LABELS_PER_SERIES`     |     `8` |         `0`–`64` |
| `limits.metric_name_bytes`     | `METRICSHELL_MAX_METRIC_NAME_BYTES`     |   `256` |       `1`–`1024` |
| `limits.label_name_bytes`      | `METRICSHELL_MAX_LABEL_NAME_BYTES`      |   `128` |       `1`–`1024` |
| `limits.label_value_bytes`     | `METRICSHELL_MAX_LABEL_VALUE_BYTES`     |  `1024` |      `1`–`16KiB` |
| `limits.help_bytes`            | `METRICSHELL_MAX_HELP_BYTES`            |  `4KiB` |      `0`–`64KiB` |
| `limits.concurrent_ingestions` | `METRICSHELL_MAX_CONCURRENT_INGESTIONS` |     `4` |         `1`–`64` |
| `limits.pending_ingestions`    | `METRICSHELL_MAX_PENDING_INGESTIONS`    |     `0` |         `0`–`64` |

`pending_ingestions=0` means fail-fast overload rejection. It does not prohibit bounded multipart socket transactions;
those use the separate limits below.

A candidate violating any limit is rejected atomically and cannot partially modify active state.

## Unix socket defaults

| Canonical property           | Environment variable                     | Default |  Allowed range |
|------------------------------|------------------------------------------|--------:|---------------:|
| `socket.frame_bytes`         | `METRICSHELL_SOCKET_MAX_FRAME_BYTES`     |  `8KiB` | `1KiB`–`64KiB` |
| `socket.parts`               | `METRICSHELL_SOCKET_MAX_PARTS`           |   `256` |     `1`–`1024` |
| `socket.connections`         | `METRICSHELL_SOCKET_MAX_CONNECTIONS`     |     `8` |       `1`–`64` |
| `socket.transactions`        | `METRICSHELL_SOCKET_MAX_TRANSACTIONS`    |     `4` |       `1`–`32` |
| `socket.transaction_timeout` | `METRICSHELL_SOCKET_TRANSACTION_TIMEOUT` |    `5s` |   `100ms`–`1m` |
| `socket.read_timeout`        | `METRICSHELL_SOCKET_READ_TIMEOUT`        |    `5s` |   `100ms`–`1m` |
| `socket.write_timeout`       | `METRICSHELL_SOCKET_WRITE_TIMEOUT`       |    `5s` |   `100ms`–`1m` |

For part index `i`, define the conservative decoded payload capacity:

```text
payload_chars(i) = socket.frame_bytes
  - byte_len("MSP/1 SNAPSHOT_PART ")
  - 64                         # maximum publication-id bytes
  - 1 - decimal_digits(i) - 1 # separators and index
  - 1                          # LF
decoded_part_capacity(i) = floor(payload_chars(i) * 3 / 4)
effective_socket_decoded_capacity = sum(decoded_part_capacity(i), i=0..socket.parts-1)
```

Negative `payload_chars` means zero capacity. Startup requires
`effective_socket_decoded_capacity >= limits.snapshot_bytes`. The default `8KiB × 256` configuration satisfies this
invariant after worst-case MSP/1 overhead and unpadded base64url expansion. The assembled input remains bounded by
`limits.decoded_input_bytes`, and its canonical form by `limits.snapshot_bytes`.

## Local HTTP ingestion defaults

| Canonical property                   | Environment variable                        | Default |    Allowed range |
|--------------------------------------|---------------------------------------------|--------:|-----------------:|
| `http_ingestion.wire_bytes`          | `METRICSHELL_HTTP_INGESTION_MAX_WIRE_BYTES` |  `2MiB` | `64KiB`–`128MiB` |
| `http_ingestion.read_header_timeout` | `METRICSHELL_HTTP_READ_HEADER_TIMEOUT`      |    `2s` |    `100ms`–`30s` |
| `http_ingestion.read_timeout`        | `METRICSHELL_HTTP_READ_TIMEOUT`             |   `10s` |     `100ms`–`1m` |
| `http_ingestion.write_timeout`       | `METRICSHELL_HTTP_WRITE_TIMEOUT`            |   `30s` |     `100ms`–`2m` |
| `http_ingestion.idle_timeout`        | `METRICSHELL_HTTP_IDLE_TIMEOUT`             |   `30s` |        `1s`–`5m` |
| `http_ingestion.max_header_bytes`    | `METRICSHELL_HTTP_MAX_HEADER_BYTES`         |  `8KiB` |   `1KiB`–`64KiB` |

HTTP wire bytes are bounded before decompression. Gzip or identity content is then bounded by
`limits.decoded_input_bytes` before parsing, and its canonical form by `limits.snapshot_bytes`. The same decoded limit
applies to raw file content and assembled socket content, preventing whitespace or decompression amplification.

## Exposition defaults

| Canonical property              | Environment variable                   | Default |   Allowed range |
|---------------------------------|----------------------------------------|--------:|----------------:|
| `exposition.response_bytes`     | `METRICSHELL_MAX_RESPONSE_BYTES`       |  `8MiB` | `64KiB`–`64MiB` |
| `exposition.concurrent_scrapes` | `METRICSHELL_MAX_CONCURRENT_SCRAPES`   |    `32` |       `1`–`128` |
| `exposition.write_timeout`      | `METRICSHELL_EXPOSITION_WRITE_TIMEOUT` |   `30s` |       `1s`–`2m` |

The response byte limit is checked against the complete uncompressed encoded body before success headers are committed.
Compression does not increase the configured application or response limit.

## Logging defaults

| Canonical property    | Environment variable              | Default | Allowed values  |
|-----------------------|-----------------------------------|---------|-----------------|
| `log.level`           | `METRICSHELL_LOG_LEVEL`           | `info`  | `info`, `debug` |
| `log.selector_values` | `METRICSHELL_LOG_SELECTOR_VALUES` | `false` | `true`, `false` |

Info, warn, and error events are always emitted. Selecting `debug` additionally emits registry events whose level is
debug. Selector values are omitted unless `log.selector_values=true`; the destination remains fixed to stderr.

## Reference container resource profile

The executable does not pretend to enforce all operating-system limits internally. Supported Docker and Compose examples
use the following initial profile:

| Resource               | Reference value |
|------------------------|----------------:|
| memory limit           |         `64MiB` |
| PID limit              |            `64` |
| `nofile` soft/hard     |         `64/64` |
| runtime directory mode |          `0700` |
| Unix socket mode       |          `0660` |

A deployment may raise these values. Lower values are unsupported unless the full conformance suite passes. Hard memory
containment is provided by cgroups.

## Cross-field validation

Startup fails when any of the following is true:

- an explicit value is outside its allowed range;
- workload timeout plus reserve exceeds total grace;
- final wait mode is `scrapes` without a positive timeout or positive required count;
- response limit is lower than the minimum valid self-metrics-only response;
- decoded input limit is lower than the canonical snapshot limit;
- socket frame size exceeds the decoded input limit;
- effective socket decoded capacity is lower than the canonical snapshot limit;
- configured concurrency cannot fit under the process file-descriptor limit with the reserved runtime descriptors;
- an adapter omits the common decoded/uncompressed input limit before parsing;
- a zero or unlimited timeout is requested where the table requires a finite value.

## Limit behavior

- malformed or policy input: deterministic rejection according to the snapshot protocol mapping;
- payload, series, label, name, and policy limits use the HTTP/socket/self-metric mapping defined by the snapshot
  protocol; this document does not assign alternative statuses;
- concurrency exhaustion: HTTP `429` or protocol-equivalent busy publication outcome;
- response-size limit: HTTP `503` before partial success;
- timeout: request or transaction is cancelled and last-valid state remains active;
- cgroup OOM: process exits according to runtime behavior and cannot be represented as a valid workload success.

## Compatibility

Changing a default in a stable release is a compatibility change and must be called out in release notes. Raising a hard
safety ceiling requires benchmark and security evidence. Lowering a default requires a migration note.

## Conformance requirements

Tests must cover every default, both range boundaries, one value outside each boundary, contradictory budget values,
atomic state retention after every rejection class, fail-fast overload, final-wait timeout, external termination
precedence, exact socket capacity at snapshot limit and one byte below, both logging levels and selector-value settings,
and response preflight failure.

## References

- [Constraints](../03-requirements/constraints.md)
- [Configuration Value Grammar](configuration-value-grammar.md)
- [ADR-003](../06-architecture/adr/ADR-003.md)
- [ADR-006](../06-architecture/adr/ADR-006.md)
- [ADR-007](../06-architecture/adr/ADR-007.md)
- [ADR-008](../06-architecture/adr/ADR-008.md)
- [ADR-011](../06-architecture/adr/ADR-011.md)
- [ADR-014](../06-architecture/adr/ADR-014.md)
- [ADR-015](../06-architecture/adr/ADR-015.md)
