# ISSUE-014. Initial zero-series state

**Status:** Open
**Readiness:** Code-ready

**Epic:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 3](../../02-epics/EPIC-001-core.md#wave-3)

Expose a valid empty application state before the first publication and when a workload publishes nothing.

## Code-ready contract

- **Normative inputs:**
  ADR-004, [Application Snapshot Protocol](../../../04-specification/application-snapshot-protocol.md),
  and [Self-Metrics](../../../04-specification/self-metrics.md).
- **Dependencies:** ISSUE-011 and ISSUE-013.
- **Scope / out of scope:** Install the explicit generation-zero, zero-series application state before publication. Out
  of scope: treating empty transport input as an empty snapshot.
- **Configuration and observable failures:** Empty payload is rejected and observable; absence before first publication
  remains a valid self-metrics-only exposition.
- **Acceptance criteria and required tests:** Startup without publication; explicit zero-series publication; zero-byte
  file/body/socket transaction; generation and self-metric assertions.
- **Completion:** Complete when startup exposition is valid and all empty-input cases preserve the correct generation.
