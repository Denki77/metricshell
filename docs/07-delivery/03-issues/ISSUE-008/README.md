# ISSUE-008. Shutdown budget model

**Status:** Open
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
