# ISSUE-009. Termination escalation

**Status:** Done
**Readiness:** Code-ready

**Epic:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 2](../../02-epics/EPIC-001-core.md#wave-2)

Graceful signal, bounded wait, then forced process-group kill. No descendant may outlive MetricShell.

## Code-ready contract

- **Normative inputs:** ADR-003 /
  INV-003, [Runtime State Machine](../../../04-specification/runtime-state-machine.md), [Runtime Defaults](../../../04-specification/runtime-defaults-and-resource-limits.md),
  and [Structured Logging](../../../04-specification/structured-logging.md).
- **Dependencies:** ISSUE-003, ISSUE-004, and ISSUE-008.
- **Scope / out of scope:** Send the graceful signal, wait within the derived budget, then force the owned process
  group. Out of scope: unbounded retries.
- **Configuration and observable failures:** Escalation emits the normative forwarding/forced events; missing processes
  are idempotent; saved workload result follows ISSUE-006.
- **Acceptance criteria and required tests:** Cooperative, ignoring and fork-after-signal workloads; zero remaining
  budget; repeated signal; disappearing group; no surviving descendant.
- **Completion:** Complete when every termination path finishes inside the budget with no descendant left behind.

## Delivery log

- 2026-08-24: moved to `In Progress`; the budget-aware escalation controller and shutdown diagnostics were implemented.
- 2026-08-24: moved to `Testing`; race-enabled unit tests and real-container cooperative, ignoring,
  fork-after-signal, repeated-signal, zero-budget and disappearing-group scenarios passed in Docker.
- 2026-08-24: moved to `Done`; termination now completes with one authoritative result after every managed child is
  reaped, and forced cleanup is idempotent and bounded by the accepted shutdown plan.

## Verification evidence

- The first TERM/INT resolves one absolute shutdown deadline, forwards the graceful signal and starts only the workload
  grace timer; a repeated termination signal may force the owned process group immediately.
- Grace expiry sends SIGKILL to the entire process group and produces exit `137`; a group that already disappeared is
  treated idempotently without a false forced-termination event.
- Container fixtures prove cooperative exit, forced exit for an ignoring workload, and reaping of a descendant forked
  after the graceful signal.
- `shutdown.started`, `shutdown.forced`, `shutdown.completed` and the `forced` workload result field follow the
  structured-event registry.
- `implementation/README.md` and `implementation/README_RU.md` were reviewed and updated together.
