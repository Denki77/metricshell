# ISSUE-010. Health and readiness contract

**Status:** Open
**Readiness:** Code-ready

**Epic:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 2](../../02-epics/EPIC-001-core.md#wave-2)

Define probe behavior per lifecycle state. Health/readiness/debug requests never count as final scrapes.

## Code-ready contract

- **Normative inputs:** ADR-002 and
  ADR-011, [Configuration](../../../04-specification/configuration.md), [Runtime State Machine](../../../04-specification/runtime-state-machine.md),
  and [Structured Logging](../../../04-specification/structured-logging.md).
- **Dependencies:** ISSUE-007; blocks probe handling in ISSUE-023 and final-scrape logic in ISSUE-026.
- **Scope / out of scope:** Implement fixed `/healthz` and `/readyz` semantics for every public state. Out of scope:
  configurable probe paths and counting probes as scrapes.
- **Configuration and observable failures:** Probe responses are bounded and state-derived; unavailable/failed states
  return deterministic statuses without mutating lifecycle.
- **Acceptance criteria and required tests:** State-by-endpoint status table; transition races; requests during
  shutdown; method/path errors; proof probes never increment final-scrape count.
- **Completion:** Complete when the specification table and HTTP integration fixtures agree for every state.
