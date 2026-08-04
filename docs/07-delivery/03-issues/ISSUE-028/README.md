# ISSUE-028. Final-wait observability

**Status:** Open
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
