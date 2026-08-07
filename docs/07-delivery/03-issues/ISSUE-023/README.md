# ISSUE-023. Prometheus exposition server

**Status:** Open
**Readiness:** Code-ready

**Epic:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 5](../../02-epics/EPIC-001-core.md#wave-5)

## Normative inputs

- ADR-004, ADR-010, ADR-014 / INV-010.
- [Metric Filtering Specification](../../../04-specification/metrics-filtering.md).
- [Self-Metrics Specification](../../../04-specification/self-metrics.md).
- [Configuration Specification](../../../04-specification/configuration.md).
- [Runtime Defaults and Resource Limits](../../../04-specification/runtime-defaults-and-resource-limits.md).

## Dependencies

ISSUE-013 active snapshot, ISSUE-015 self-metrics, and ISSUE-024 bounded pre-encoding.

## Scope

Bind the configured exposition listener; implement GET /metrics with Prometheus text 0.0.4 baseline and OpenMetrics 1.0
content negotiation; select one immutable application generation per request; apply accepted family filtering; append
the complete self-metrics domain; and expose the fixed health/readiness paths through the lifecycle contract.

## Out of scope

Per-request filter parameters, label/value filters, aggregation, host-wide collectors, remote write, and durable scrape
delivery.

## Configuration and observable errors

Implement exposition.listen, response byte/concurrency/write limits, metrics include/exclude rules, and fixed paths.
Unsupported methods/media negotiation return documented 4xx; saturation returns 503; encoding or response-limit failure
occurs before success headers; bind failure exits with endpoint_bind_failed; cancelled/partial writes are not success.

## Acceptance criteria

- Prometheus and OpenMetrics parsers accept successful responses with correct content type and EOF.
- A request observes one application generation and a separately consistent self-metric view.
- Include/exclude precedence, family atomicity, and reserved self-metrics match the filtering specification.
- Response limits are checked before status 200 is committed.
- Probe/debug requests never enter scrape counters or final-scrape eligibility.
- Concurrent handlers remain within configured bounds and drain within shutdown budget.

## Required test matrix

Both formats and Accept variants; format-specific counter HELP/TYPE/sample names; component-name collisions; finite,
`NaN`, `+Inf`, and `-Inf` gauge values; valid non-negative histogram snapshots; empty-family/zero-series exposition;
every filtering rule/precedence case;
self-metric presence; concurrent replacement during scrape; slow/cancelled clients; encoding and size failures;
saturation;
bind failure; health/readiness in every runtime state; graceful drain; and race detector.

## Completion

Complete when parser-based golden tests, filtering conformance, bounds/failure tests, lifecycle probes, and concurrent
snapshot-selection tests pass with all observable errors documented.
