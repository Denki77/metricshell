# Metric Filtering Specification

[Russian version](../../docs-ru/04-specification/metrics-filtering.md)

> Status: Accepted normative specification
> Requirements: FR-032, FR-033
> Acceptance criteria: AC-EXP-003, AC-EXP-004
> Decisions: ADR-004, ADR-010, ADR-014

## Purpose

This specification defines the syntax, defaults, precedence and observable behavior of application metric filtering.
Filtering is an exposition policy. It does not mutate the accepted application snapshot, alter snapshot ownership, merge
producer state or change the validation contract defined by ADR-004.

## Scope

Filtering applies only to application metric families. MetricShell self-metrics are never filtered by this mechanism.
The first stable filtering surface supports exact metric-family names and metric-family prefixes only.

The following are outside this specification:

- label matchers;
- value-based filtering;
- regular expressions or glob patterns;
- per-scrape filter parameters;
- producer-specific filtering;
- dynamic filter reload during one workload execution.

## Configuration

| Canonical property          | CLI option                                | Environment variable          | Default |
|-----------------------------|-------------------------------------------|-------------------------------|---------|
| `exposition.filter.include` | repeatable `--metrics-include=<selector>` | `METRICSHELL_METRICS_INCLUDE` | empty   |
| `exposition.filter.exclude` | repeatable `--metrics-exclude=<selector>` | `METRICSHELL_METRICS_EXCLUDE` | empty   |

Environment values use the comma-list rules in the
[Configuration Value Grammar](configuration-value-grammar.md): version 1 has no escaping, and comma or backslash inside
a selector is invalid. CLI options are repeatable. When the same property is present in both sources, CLI replaces the
entire environment-provided list; the two sources are not merged.

Configuration is validated before workload start and remains immutable for the workload execution.

## Selector syntax

```ebnf
selector     = selector-type, ":", metric-token ;
selector-type = "name" | "prefix" ;
metric-token = prometheus-name-start, { prometheus-name-char } ;
```

Where:

- `prometheus-name-start` is one of `A-Z`, `a-z`, `_`, `:`;
- `prometheus-name-char` additionally permits `0-9`;
- matching is case-sensitive;
- surrounding whitespace is trimmed from an environment-list item;
- whitespace inside a selector is invalid;
- an empty `name:` or `prefix:` selector is invalid;
- regular expressions, `*`, `?` and label selectors are invalid.

Examples:

```text
name:http_requests
prefix:application_
prefix:worker_queue_
```

`name:` matches the declared base metric-family name. Derived counter `_total` and histogram `_bucket`, `_sum`, and
`_count` sample names are not selector inputs and cannot be filtered independently.

## Default behavior

- Empty include list means all valid application families are eligible.
- Empty exclude list excludes nothing.
- Therefore, with both lists empty, the full active application snapshot is exposed.
- MetricShell self-metrics are always exposed.

## Evaluation order and precedence

For every selected immutable snapshot, MetricShell applies the following deterministic algorithm:

1. Parse and validate the complete candidate before any filtering decision.
2. Atomically install the complete unfiltered application snapshot after successful validation.
3. Treat all application families as candidates when the include list is empty.
4. Otherwise retain only families matching at least one include selector.
5. Remove every family matching at least one exclude selector.
6. Exclude always wins over include.
7. Emit each retained family atomically with its metadata and all component samples.
8. Append the complete required MetricShell self-metric set.

Filtering must never make an otherwise invalid candidate valid. A duplicate series, type conflict, malformed histogram
or
resource violation in a family that would later be excluded still rejects the complete candidate.

## Family atomicity

Filtering is performed at metric-family granularity:

- HELP and TYPE metadata follow the family;
- all counter, gauge or histogram samples belonging to the family follow the same decision;
- a classic histogram cannot expose only selected buckets;
- a filtered-out family leaves no stale samples in the response;
- filtering all application families is valid and produces a self-metrics-only response.

## Reserved namespace

The prefix `metricshell_` is reserved for MetricShell self-metrics. An application candidate containing a family whose
name begins with `metricshell_` is rejected atomically with reason `reserved_name`. Filtering cannot be used to hide
such
a collision.

## Duplicate rules

Repeated identical selectors are accepted and deduplicated in effective configuration. Invalid selectors fail startup.
Selectors that match no current family are valid because future accepted snapshots may contain matching families.

## Observability

Effective non-secret configuration must expose the normalized include and exclude selector counts. Selector values are
absent from logs by default. They may appear only in `configuration.validated` when the operator explicitly sets
`log.selector_values=true`; the structured-logging size and redaction rules still apply.

The following self-metrics are defined by the self-metrics specification:

```text
metricshell_filter_rules{kind="include"}
metricshell_filter_rules{kind="exclude"}
metricshell_filter_families{outcome="included"}
metricshell_filter_families{outcome="excluded"}
```

Family names must never be copied into metric labels.

## Examples

### Include one namespace and remove its debug families

```text
METRICSHELL_METRICS_INCLUDE=prefix:application_
METRICSHELL_METRICS_EXCLUDE=prefix:application_debug_
```

`application_requests_total` is exposed. `application_debug_cache_entries` is not exposed.

### Exact include overridden by exclude

```text
--metrics-include=name:http_requests
--metrics-exclude=name:http_requests
```

The family is excluded because exclude has final precedence.

### Self-metrics cannot be filtered

```text
--metrics-exclude=prefix:metricshell_
```

This selector is accepted as a no-op against the application domain. MetricShell self-metrics remain present.

## Conformance requirements

Automated tests must prove:

- empty configuration exposes all application families;
- exact and prefix matching are case-sensitive;
- include restricts the candidate family set;
- exclude wins over include;
- histogram families are atomic;
- validation occurs before filtering;
- all-application-filtered response remains valid and contains self-metrics;
- reserved `metricshell_` application families are rejected;
- concurrent publication and scrape never mix filter results from different snapshots;
- CLI source replaces the environment list for the same property.

## References

- [Functional Requirements](../03-requirements/functional-requirements.md#fr-033--filtering)
- [Acceptance Criteria](../03-requirements/acceptance-criteria.md#ac-exp-004--filtering)
- [ADR-004](../06-architecture/adr/ADR-004.md)
- [ADR-010](../06-architecture/adr/ADR-010.md)
- [Self-metrics Specification](self-metrics.md)
- [Configuration Value Grammar](configuration-value-grammar.md)
