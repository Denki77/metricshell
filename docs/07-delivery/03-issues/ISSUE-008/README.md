# ISSUE-008. Shutdown budget model

**Status:** Done
**Readiness:** Code-ready

**Epic:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 2](../../02-epics/EPIC-001-core.md#wave-2)

**ADR/INV:** ADR-003 / INV-003.

Split one finite budget across signal forwarding, workload grace, final scrape, server drain, and forced cleanup.
Validate configuration before workload startup.

## Code-ready contract

- **Normative inputs:** ADR-003 /
  INV-003, [Configuration](../../../04-specification/configuration.md), [Configuration Value Grammar](../../../04-specification/configuration-value-grammar.md), [Runtime Defaults](../../../04-specification/runtime-defaults-and-resource-limits.md),
  and [Runtime State Machine](../../../04-specification/runtime-state-machine.md).
- **Dependencies:** ISSUE-007.
- **Scope / out of scope:** Derive one monotonic absolute deadline and allocate workload timeout, reserve, finalization
  and drain within it. Out of scope: extending an external deadline.
- **Configuration and observable failures:** Invalid cross-field budgets fail before workload start; exhaustion has a
  closed completion reason and structured remaining time.
- **Acceptance criteria and required tests:** Every duration boundary; timeout plus reserve equality and overflow;
  already-expired deadline; clock advancement; cancellation at each phase.
- **Completion:** Complete when no phase can exceed the absolute deadline and all validation/error paths are observable.

## Delivery log

- 2026-08-24: moved to `In Progress`; shutdown configuration and the deadline-derived phase model were implemented.
- 2026-08-24: moved to `Testing`; grammar, range, equality/overflow, expiry, clock-advance and phase-cancellation tests
  passed under the race detector in Docker.
- 2026-08-24: moved to `Done`; invalid budgets are rejected before spawn and every phase context is deadline-capped.

## Verification evidence

- Defaults are `30s` total, `28s` workload and `2s` reserve; CLI overrides environment, which overrides defaults.
- Duration parsing implements the normative integer/unit grammar and rejects signs, fractions, compounds, leading zeroes
  and overflow before range validation.
- The resolved plan retains one absolute deadline, derives zero workload grace when reserve cannot fit, and never
  extends an external deadline.
- Signal forwarding, workload grace, finalization, HTTP drain and forced cleanup receive cancellable contexts capped by
  the phase boundary and the common deadline.
- A CLI-level test proves an overcommitted budget returns `64`/`configuration.rejected` without a workload event.
- `implementation/README.md` and `implementation/README_RU.md` were reviewed and updated together.
