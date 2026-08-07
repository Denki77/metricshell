# ISSUE-018. Unix socket framed protocol

**Status:** Open
**Readiness:** Code-ready

**Epic:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 4](../../02-epics/EPIC-001-core.md#wave-4)

**ADR/INV:** ADR-007 / INV-007. Versioned frame, length bound, read deadline, truncated/oversized rejection, linearized
activation.

## Code-ready contract

- **Normative inputs:** ADR-007 /
  INV-007, [Application Snapshot Protocol](../../../04-specification/application-snapshot-protocol.md), [Configuration](../../../04-specification/configuration.md),
  and [Runtime Defaults](../../../04-specification/runtime-defaults-and-resource-limits.md).
- **Dependencies:** ISSUE-016.
- **Scope / out of scope:** Implement MSP/1 BEGIN/PART/COMMIT, `BEGIN` and indexed frame ACKs, unpadded base64url,
  bounded transactions, and linearized commit. Out of scope: alternate framing or shared memory.
- **Configuration and observable failures:** Socket mode is 0660; protocol/frame/transaction failures stay separate from
  candidate reasons; every timeout or NACK is observable.
- **Acceptance criteria and required tests:** Valid one/multipart; padded/invalid base64; duplicate/out-of-order/missing
  part; declared-size mismatch; exact capacity formula and default 1MiB transfer; capacity below snapshot limit rejected;
  all limits; expiry/disconnect; concurrent commits.
- **Completion:** Complete when protocol golden transcripts and cross-adapter corpus pass with exact frames and enums.
