# ISSUE-019. Official client writer serialization

**Status:** Open
**Readiness:** Code-ready

**Epic:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 4](../../02-epics/EPIC-001-core.md#wave-4)

A single writer or mutex guarantees contiguous frames when multiple application threads publish through one connection.

## Code-ready contract

- **Normative inputs:** ADR-007 and
  ADR-004, [Application Snapshot Protocol](../../../04-specification/application-snapshot-protocol.md).
- **Dependencies:** ISSUE-018.
- **Scope / out of scope:** Provide the official connection writer that serializes complete MSP/1 frames and correlates
  responses. Out of scope: retry policy after ambiguous disconnect.
- **Configuration and observable failures:** Short writes, closed connections and mismatched publication IDs return
  typed client errors without interleaving bytes or exposing payloads.
- **Acceptance criteria and required tests:** Concurrent goroutines on one connection; forced short writes; server
  NACK/timeout; disconnect before ACK; response mismatch; race detector.
- **Completion:** Complete when a byte-level stress test proves every emitted frame is contiguous and attributable.
