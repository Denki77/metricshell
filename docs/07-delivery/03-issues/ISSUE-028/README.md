# ISSUE-028. Final-wait observability

**Status:** Done
**Readiness:** Code-ready

**Epic:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 5](../../02-epics/EPIC-001-core.md#wave-5)

## Normative inputs

- ADR-002, ADR-003, ADR-011, ADR-014.
- [Runtime State Machine](../../../04-specification/runtime-state-machine.md).
- [Self-Metrics Specification](../../../04-specification/self-metrics.md).
- [Structured Logging Specification](../../../04-specification/structured-logging.md).

## Dependencies

ISSUE-007 lifecycle, ISSUE-015 self-metrics, ISSUE-026 final-wait state machine, and ISSUE-027 completed-response
counting.

## Scope

Implement the complete final-wait self-metric registry and structured events for start, counted/not-counted responses,
completion, state transitions, timeout, external termination, and runtime failure. Expose frozen snapshot generation,
mode, deadline, required and completed scrapes, bounded attempts/outcomes, and terminal reason.

## Out of scope

Scraper identity, high-cardinality request/client labels, durable audit storage, Prometheus parsing confirmation, and
application payload logging.

## Configuration and observable errors

Use final_wait.mode/duration/timeout/required_scrapes/completion_grace and the absolute shutdown deadline. Metrics and
logs
must use the closed mode/outcome/reason/error registries. Invalid configuration fails before workload start; runtime
failure emits runtime.failed and preserves bounded cleanup.

## Acceptance criteria

- Every public state transition emits exactly one runtime.state_changed event and updates one-hot state metrics.
- Start and completion events are exactly once with mode/deadline and terminal reason.
- Counted, ineligible, cancelled, write-error, timeout, and external-termination paths update matching metrics/log
  fields.
- Request/publication IDs appear only where allowed and never become metric labels.
- Rate limiting emits logging.suppression_summary without suppressing terminal lifecycle events.
- Frozen generation identity remains stable throughout final_wait.

## Required test matrix

Immediate/duration/scrapes modes; N=1 and N>1; timeout; concurrent responses at threshold; cancellation and partial
write;
probe/debug exclusion; external signal races; runtime failure; log schema/type/enum validation; suppression windows;
cardinality assertions; and race detector.

## Completion

Complete when every final-wait transition has matching metrics and schema-valid events, enum parity tests with
self-metrics pass, terminal events are exactly once, and no unbounded or sensitive field is observable.

## Delivery log

- 2026-09-07: moved to `In Progress`; added final-wait lifecycle diagnostics, final scrape counted/not-counted
  diagnostics, bounded request IDs, and self-metric updates for mode, active state, deadlines, required/completed
  scrapes, attempts and terminal reasons.
- 2026-09-07: moved to `Testing`; covered duration, scrape-response outcomes, terminal gauges, reason enum parity,
  cardinality boundaries and real-container N=2 final scrape observability.
- 2026-09-07: moved to `Done`; Docker unit/race/multi-architecture and integration gates passed, and implementation
  EN/RU README scope now marks Wave 5 complete.

## Verification evidence

- `make test IMAGE=metricshell-wave5-028-observability`
- `make integration IMAGE=metricshell-wave5-028-observability`
- `make wave5 IMAGE=metricshell-wave5-028-observability`
- README audit: implementation EN/RU command list includes `make wave5`; package scope and normative context include
  ISSUE-028 final-wait observability with no remaining Wave 5 implementation scope.
