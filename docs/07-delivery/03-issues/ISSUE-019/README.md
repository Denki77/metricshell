# ISSUE-019. Official client writer serialization

**Status:** Done
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

## Delivery log

- 2026-08-29: moved to `In Progress`; implemented the public official MSP/1 writer with whole-publication
  serialization, bounded frame splitting, response correlation and typed failures.
- 2026-08-29: moved to `Testing`; concurrent goroutine stress, forced short writes, NACK, timeout, disconnect,
  mismatched ID, capacity and cancellation tests passed under the Docker race detector.
- 2026-08-29: moved to `Done`; the client package passed `make test`, and EN/RU issue and implementation READMEs were
  audited together.

## Verification evidence

- A cancellable single-owner gate covers BEGIN through final ACK/NACK, so bytes from separate publications cannot
  interleave on one connection.
- Every frame is written with a complete-write loop and independently checked against the configured frame bound;
  partial writes are completed while zero-progress writes fail deterministically.
- Every FRAME_ACCEPTED, ACK and NACK is correlated to the requested publication ID. Mismatch, malformed response,
  rejection, timeout and closed connection are distinct typed client errors without payload text.
- The writer intentionally has no retry policy after an ambiguous disconnect.
