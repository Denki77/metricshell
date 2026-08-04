# ISSUE-020. Local push HTTP adapter

**Status:** Open
**Readiness:** Code-ready

**Epic:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 4](../../02-epics/EPIC-001-core.md#wave-4)

**ADR/INV:** ADR-008 / INV-008. Local-only, bounded, same snapshot and error semantics.

## Code-ready contract

- **Normative inputs:** ADR-008 /
  INV-008, [Application Snapshot Protocol](../../../04-specification/application-snapshot-protocol.md), [Configuration](../../../04-specification/configuration.md),
  and [Runtime Defaults](../../../04-specification/runtime-defaults-and-resource-limits.md).
- **Dependencies:** ISSUE-016.
- **Scope / out of scope:** Implement loopback-only POST `/v1/metrics`, identity/gzip decoding, shared validation, and
  exact HTTP mapping. Out of scope: remote authentication and non-loopback bind.
- **Configuration and observable failures:** Wire, decoded, and canonical limits are enforced independently; rejection
  bodies use closed codes; no ACK precedes installation.
- **Acceptance criteria and required tests:** Bind validation; methods/media types/encodings; gzip bomb; every
  status/code row; slow read/write; busy/timeout; concurrent ordering.
- **Completion:** Complete when HTTP runs the shared corpus and matches file/socket state, generation, reason, and
  observability.
