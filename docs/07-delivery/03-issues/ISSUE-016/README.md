# ISSUE-016. Common transport-independent ingestion interface

**Status:** Open
**Readiness:** Code-ready

**Epic:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 4](../../02-epics/EPIC-001-core.md#wave-4)

**ADR/INV:** ADR-005 / INV-005. Shared result types, error taxonomy, admission hooks, deadlines, and candidate handoff.

## Code-ready contract

- **Normative inputs:** ADR-005 /
  INV-005, [Application Snapshot Protocol](../../../04-specification/application-snapshot-protocol.md), [Configuration](../../../04-specification/configuration.md),
  and [Runtime Defaults](../../../04-specification/runtime-defaults-and-resource-limits.md).
- **Dependencies:** ISSUE-012 and ISSUE-013; blocks ISSUE-017, ISSUE-018, and ISSUE-020.
- **Scope / out of scope:** Define common admission, deadlines, candidate handoff, result, and error taxonomy. Out of
  scope: adapter wire framing.
- **Configuration and observable failures:** Candidate reasons, publication outcomes, and transport failures are
  distinct typed enums and map to shared logs/self-metrics.
- **Acceptance criteria and required tests:** Contract tests with fake adapters for
  accepted/rejected/busy/timeout/frozen/internal; cancellation; queue boundaries; enum-parity compile/test check.
- **Completion:** Complete when every adapter implements the interface without translating semantic rejection ad hoc.
