# ISSUE-027. Complete-response counting and drain

**Status:** Open
**Readiness:** Code-ready

**Epic:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 5](../../02-epics/EPIC-001-core.md#wave-5)

Count only a fully successful, non-cancelled write and apply bounded completion grace.

## Code-ready contract

- **Normative inputs:** ADR-011 and
  ADR-014, [Runtime State Machine](../../../04-specification/runtime-state-machine.md),
  and [Runtime Defaults](../../../04-specification/runtime-defaults-and-resource-limits.md).
- **Dependencies:** ISSUE-024 and ISSUE-026.
- **Scope / out of scope:** Count only a fully written eligible `/metrics` response for the frozen generation, then
  drain already accepted responses within completion grace. Out of scope: counting headers or probes.
- **Configuration and observable failures:** Cancelled, timed-out, partial, wrong-generation, and probe responses emit
  not-counted outcomes; drain expiry is bounded.
- **Acceptance criteria and required tests:** Full/partial/zero-byte writes; disconnect after headers/body; simultaneous
  threshold; wrong path/generation; completion-grace 0/boundary; race detector.
- **Completion:** Complete when count and drain decisions are made at one documented write-completion point.
