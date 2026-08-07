# ISSUE-022. Cross-adapter conformance suite

**Status:** Open
**Readiness:** Code-ready

**Epic:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 4](../../02-epics/EPIC-001-core.md#wave-4)

Run identical complete snapshots and failure cases through file, socket, and push adapters and require identical
accepted state identity.

## Code-ready contract

- **Normative inputs:** ADR-004–ADR-008 and
  ADR-015, [Application Snapshot Protocol](../../../04-specification/application-snapshot-protocol.md), [Runtime Defaults](../../../04-specification/runtime-defaults-and-resource-limits.md),
  and [Self-Metrics](../../../04-specification/self-metrics.md).
- **Dependencies:** ISSUE-017, ISSUE-018, and ISSUE-020.
- **Scope / out of scope:** Run one immutable acceptance/rejection corpus through all three adapters. Out of scope:
  adapter-specific exceptions to semantic validation.
- **Configuration and observable failures:** Assert exact canonical bytes, generation, active state, rejection reason,
  log fields, and self-metric deltas; transport-only failures remain separately asserted.
- **Acceptance criteria and required tests:** Every candidate reason; finite/special numbers; empty state; all limits;
  concurrency; timeout; disconnect; malformed transport and recovery.
- **Completion:** Complete when adding a reason or enum requires one shared fixture and parity test updates all
  adapters.
