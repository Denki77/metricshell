# Application Snapshot Protocol Specification

[Russian version](../../docs-ru/04-specification/application-snapshot-protocol.md)

> Status: Accepted normative specification
> Requirements: FR-010–FR-026
> Acceptance criteria: AC-ING-001–AC-ING-012, AC-MET-001–AC-MET-011
> Decisions: ADR-004–ADR-008, ADR-010, ADR-014

## Contract and media type

All Core ingestion adapters carry one complete application snapshot using schema version 1. The media type is
application/vnd.metricshell.snapshot+json;version=1. UTF-8 is required. Compression is transport-level and never changes
snapshot identity.

An accepted document replaces the entire application snapshot. Missing families and series are deleted. An empty
transport body is invalid. A document with no series is a valid empty application state and canonicalizes to the
zero-series document below.

## Version 1 document

~~~json
{
  "schema_version": 1,
  "families": [
    {
      "name": "requests",
      "help": "Completed requests.",
      "type": "counter",
      "series": [
        {
          "labels": {
            "method": "GET"
          },
          "value": "42"
        }
      ]
    },
    {
      "name": "request_duration_seconds",
      "help": "Request duration.",
      "type": "histogram",
      "series": [
        {
          "labels": {
            "method": "GET"
          },
          "histogram": {
            "count": "7",
            "sum": "1.25",
            "buckets": [
              {
                "le": "0.1",
                "count": "2"
              },
              {
                "le": "0.5",
                "count": "6"
              },
              {
                "le": "+Inf",
                "count": "7"
              }
            ]
          }
        }
      ]
    }
  ]
}
~~~

The top-level object has exactly schema_version and families. Unknown fields are rejected in version 1.

A family has exactly:

- name: a valid Prometheus metric-family name;
- help: a UTF-8 string, which may be empty;
- type: counter, gauge, or histogram;
- series: an array that may be empty.

A counter or gauge series has exactly labels and value. A histogram series has exactly labels and histogram. Labels are
a JSON object of string name/value pairs. Metric names, label names, the reserved __name__ label, duplicate label names,
and the metricshell_ namespace are validated before activation.

## Family names and encoded sample names

`family.name` is always the base metric-family name. Filtering, series identity, metadata conflict detection, and the
lifetime name-to-type binding use this base name. Encoders derive metadata and sample names as follows:

| Type        | Prometheus text 0.0.4 metadata / samples                  | OpenMetrics 1.0 metadata / samples                        |
|-------------|-----------------------------------------------------------|-----------------------------------------------------------|
| `counter`   | HELP/TYPE `base_total`; sample `base_total`               | HELP/TYPE `base`; sample `base_total`                     |
| `gauge`     | HELP/TYPE `base`; sample `base`                           | HELP/TYPE `base`; sample `base`                           |
| `histogram` | HELP/TYPE `base`; `base_bucket`, `base_sum`, `base_count` | HELP/TYPE `base`; `base_bucket`, `base_sum`, `base_count` |

A counter base name ending in `_total` is invalid. A histogram base name ending in `_bucket`, `_sum`, or `_count` is
invalid. Before activation, the validator constructs every encoded sample name for both formats. Intersections between
different families are `metadata_conflict`; this detects cases such as counter `foo` plus gauge `foo_total`, or
histogram `foo` plus gauge `foo_bucket`. Any derived name beginning with reserved `metricshell_` is `reserved_name`.
Both base and derived names must satisfy `limits.metric_name_bytes`; an oversized derived component is `name_limit`.
Histogram series must not contain a user label named `le`; the encoder alone creates bucket `le` labels, and a collision
is `histogram_invalid`.

## Numeric representation

Numeric values are JSON strings to avoid parser-dependent JSON number conversion.

- finite decimal grammar: `-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?`;
- special float grammar is exactly `NaN`, `+Inf`, or `-Inf`;
- every finite decimal is parsed exactly as Go `strconv.ParseFloat(token, 64)`, using IEEE-754 binary64
  round-to-nearest, ties-to-even; any range error, overflow to infinity, or underflow of a non-zero token to zero is
  `numeric_invalid`;
- canonical finite rendering is exactly Go `strconv.FormatFloat(value, 'g', -1, 64)`; special values retain the tokens
  `NaN`, `+Inf`, and `-Inf`;
- counter values are finite with a clear sign bit (`-0` is invalid); histogram counts are non-negative;
- counts are base-10 unsigned 64-bit integers without sign or leading zeroes, except 0;
- gauge values are finite decimals or special float values; implementations preserve all four classes without
  interpreting their business meaning;
- histogram sums are non-negative finite decimals or `+Inf`; negative zero, negative values, `NaN`, and `-Inf` are
  `histogram_invalid`;
- histogram `le` is a non-negative finite decimal or `+Inf`; negative zero and negative boundaries are invalid;
- histogram buckets are strictly increasing, cumulative counts never decrease, the last bucket is +Inf, and its count
  equals histogram count. Boundaries are compared after binary64 conversion; duplicate or non-increasing converted
  values are `histogram_invalid`, even when their input strings differ.

Timestamps, exemplars, summaries, native histograms, and untyped families are not part of version 1.

## Identity, conflicts, and canonical form

Series identity is family name plus the sorted label-name/value pairs. A candidate is rejected atomically when it
contains duplicate series, duplicate family names, conflicting metadata, or a family name whose type differs from the
type binding already established during this MetricShell run.

After validation, canonical form is produced by:

1. sorting families by name;
2. sorting each labels object by label name;
3. removing families whose `series` array is empty;
4. sorting family series lexicographically by their label pairs;
5. replacing finite numeric strings with their canonical binary64 rendering and preserving validated bucket order;
6. encoding UTF-8 JSON with no insignificant whitespace and a final newline.

`limits.decoded_input_bytes` applies to uncompressed/decoded bytes before parsing, including whitespace. Snapshot byte
limits apply separately to the canonical uncompressed form. Transport wire limits apply before decompression or base64
decoding as specified by each adapter.

## Valid zero-series snapshot

The exact semantic zero-series document is:

~~~json
{
  "schema_version": 1,
  "families": []
}
~~~

A family with an empty `series` array is fully name/metadata/label validated, but creates no lifetime type binding, emits
no HELP/TYPE lines, and is removed from canonical form. A document containing only empty families therefore canonicalizes
exactly to the document above. Accepting either representation atomically clears all application series and advances
generation. A final newline is permitted.
A zero-byte file, empty HTTP body, or multipart transaction assembling zero bytes is empty_payload and does not change
active state.

## File adapter

The configured snapshot file contains exactly one version 1 document and optional final newline. Publication uses a
temporary file in the same directory, flush/close, and atomic rename to the configured path. MetricShell reads only the
configured target after rename/inotify or reconciliation. Partial JSON, non-regular files, symlinks, deletion, and
content exceeding `limits.decoded_input_bytes` are rejected or treated as absent according to the file adapter policy;
none clears active state. The limit is enforced while reading, before JSON parsing.

## HTTP adapter

The endpoint is POST /v1/metrics. Content-Type must be the snapshot media type or application/json. Content-Encoding may
be identity or gzip. Other methods, media types, and encodings are rejected. Decompression stops with `payload_limit` as
soon as output would exceed `limits.decoded_input_bytes`; wire and canonical bytes retain their independent limits.

Success response after atomic installation:

~~~json
{
  "schema_version": 1,
  "status": "ack",
  "generation": 7
}
~~~

Failure response:

~~~json
{
  "schema_version": 1,
  "status": "nack",
  "code": "duplicate_series"
}
~~~

ACK is never sent before validation and installation. HTTP status follows the single rejection mapping below. Busy and
timeout are publication outcomes: HTTP uses 429 and 408 respectively, while socket uses NACK `busy` and `timeout`; they
do not enter the candidate-rejection reason registry.

## Unix socket protocol

The Unix stream protocol is UTF-8 line-framed ASCII control data with unpadded RFC 4648 base64url payload parts. Padding
`=` and non-url-safe alphabet characters are invalid. Every line ends in LF and must fit socket.frame_bytes. Spaces are
single ASCII spaces. publication-id matches [A-Za-z0-9_-]{1,64}; indexes and sizes are canonical unsigned decimal
integers.

Client frames:

~~~text
MSP/1 SNAPSHOT_BEGIN <publication-id> <part-count> <decoded-bytes>
MSP/1 SNAPSHOT_PART <publication-id> <zero-based-index> <base64url-data>
MSP/1 SNAPSHOT_COMMIT <publication-id>
~~~

Server frames:

~~~text
MSP/1 FRAME_ACCEPTED <publication-id> BEGIN
MSP/1 FRAME_ACCEPTED <publication-id> <zero-based-index>
MSP/1 ACK <publication-id> <generation>
MSP/1 NACK <publication-id> <code>
~~~

SNAPSHOT_BEGIN reserves one bounded transaction and returns `FRAME_ACCEPTED <publication-id> BEGIN`. part-count is
1..socket.parts and decoded-bytes is 1..limits.decoded_input_bytes. Every index must occur exactly once. A retained part
returns `FRAME_ACCEPTED <publication-id> <index>`. Parts may arrive in index order only; their decoded bytes
are concatenated. COMMIT is valid only when all declared parts and exact decoded size are present. The assembled bytes
must be one version 1 JSON document. FRAME_ACCEPTED acknowledges bounded retention only; ACK means atomic installation.

One connection writer must serialize complete frames. Concurrent publications use separate transactions and may validate
concurrently, but installation and generation assignment have one linear order. Disconnect before ACK is ambiguous; a
retry republishes a complete snapshot and is safe but may advance generation again.

## Closed candidate-rejection mapping

This table is the single normative mapping for whole-candidate rejection. `snapshot.rejected` logs and
`metricshell_snapshot_rejections_total{reason}` use exactly the reason column.

| Reason              | HTTP | Socket NACK code    | File/self-metric outcome     |
|---------------------|-----:|---------------------|------------------------------|
| `malformed`         |  400 | `malformed`         | `rejected/malformed`         |
| `numeric_invalid`   |  400 | `numeric_invalid`   | `rejected/numeric_invalid`   |
| `schema_version`    |  400 | `schema_version`    | `rejected/schema_version`    |
| `empty_payload`     |  400 | `empty_payload`     | `rejected/empty_payload`     |
| `payload_limit`     |  413 | `payload_limit`     | `rejected/payload_limit`     |
| `series_limit`      |  422 | `series_limit`      | `rejected/series_limit`      |
| `label_limit`       |  422 | `label_limit`       | `rejected/label_limit`       |
| `name_limit`        |  422 | `name_limit`        | `rejected/name_limit`        |
| `policy`            |  422 | `policy`            | `rejected/policy`            |
| `duplicate_series`  |  400 | `duplicate_series`  | `rejected/duplicate_series`  |
| `type_conflict`     |  400 | `type_conflict`     | `rejected/type_conflict`     |
| `metadata_conflict` |  400 | `metadata_conflict` | `rejected/metadata_conflict` |
| `histogram_invalid` |  400 | `histogram_invalid` | `rejected/histogram_invalid` |
| `reserved_name`     |  400 | `reserved_name`     | `rejected/reserved_name`     |
| `frozen`            |  409 | `frozen`            | `rejected/frozen`            |
| `internal`          |  500 | `internal`          | `rejected/internal`          |

The socket response is `MSP/1 NACK <publication-id> <code>`. Socket framing failures use a separate closed transport
registry: `malformed`, `protocol_version`, `frame_limit`, `part_limit`, `duplicate_part`, `missing_part`,
`transaction_invalid`, and `transaction_expired`. Admission and deadline outcomes are `busy` and `timeout`; they appear
in the common ingestion outcome metrics, not in candidate or socket-frame rejection labels. Adapters must not embed
parser text, paths, names, labels, addresses, or IDs in codes.
Equivalent invalid candidates produce the same reason across file, socket, and HTTP adapters.

## Conformance

A shared corpus must exercise valid counter/gauge/histogram and zero-series documents plus every rejection class through
all three adapters. Accepted canonical bytes, generation, exposition, and rejection code must match. Tests must include
float64 overflow/underflow and rounding collisions; negative histogram bounds/sums; empty-family normalization;
format-specific counter names and component collisions; histogram `le`; truncation; duplicate/out-of-order parts;
decompression limits; disconnects; concurrent validation; and atomic replacement.

## References

- [Configuration Specification](configuration.md)
- [Runtime Defaults and Resource Limits](runtime-defaults-and-resource-limits.md)
- [ADR-004](../06-architecture/adr/ADR-004.md)
- [ADR-006](../06-architecture/adr/ADR-006.md)
- [ADR-007](../06-architecture/adr/ADR-007.md)
- [ADR-008](../06-architecture/adr/ADR-008.md)
