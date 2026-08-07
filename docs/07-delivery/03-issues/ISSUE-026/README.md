# ISSUE-026. Final scrape state machine

**Status:** Open
**Readiness:** Code-ready

**Epic:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 5](../../02-epics/EPIC-001-core.md#wave-5)

Default N=1 plus finite timeout; immediate, duration, and positive-N modes; probes excluded.

## Code-ready contract

- **Normative inputs:** ADR-011 and ADR-012 / INV-011,
  INV-012, [Runtime State Machine](../../../04-specification/runtime-state-machine.md), [Runtime Defaults](../../../04-specification/runtime-defaults-and-resource-limits.md),
  and [Self-Metrics](../../../04-specification/self-metrics.md).
- **Dependencies:** ISSUE-007, ISSUE-010, ISSUE-023, and ISSUE-025.
- **Scope / out of scope:** Implement immediate, duration, and scrape-count final-wait modes with finite timeout and
  frozen generation. Out of scope: probe-based or partial-response completion.
- **Configuration and observable failures:** Exactly one closed completion reason is emitted; timeout/external
  termination is bounded and ISSUE-006 preserves workload result.
- **Acceptance criteria and required tests:** Every mode; N=1/N>1; timeout; no scraper; external signal; concurrent
  threshold responses; probes; stale-marker-aware verifier behavior.
- **Completion:** Complete when the state machine terminates exactly once for every event ordering.
