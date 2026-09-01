# ISSUE-018. Unix socket framed protocol

**Status:** Done
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

## Delivery log

- 2026-08-29: moved to `In Progress`; implemented bounded MSP/1 line framing, multipart transaction storage,
  FRAME_ACCEPTED/ACK/NACK responses and a mode-0660 Unix listener.
- 2026-08-29: moved to `Testing`; golden one/multipart transcripts, base64, ordering, duplication, missing-part,
  size/capacity, expiry, frame drain, default 1MiB and concurrent-commit tests passed under Docker race testing.
- 2026-08-29: moved to `Done`; the socket adapter passed `make test`, and EN/RU issue and implementation READMEs were
  audited together.

## Verification evidence

- Every line is bounded and oversized input is drained before the next frame; only unpadded RFC 4648 base64url is
  decoded.
- Transactions reserve bounded shared slots, enforce canonical IDs/indexes/sizes, strict part order, exact decoded size
  and finite expiry; disconnect releases every reservation.
- ACK is emitted only from the common Core's accepted result after installation and includes the assigned generation.
  Candidate reasons, admission outcomes and wire failures retain their separate registries.
- The exact conservative capacity formula rejects configurations below the canonical snapshot limit; the default
  8KiB × 256 setup transfers a complete 1MiB decoded candidate.
- Real AF_UNIX listener coverage verifies mode 0660 and end-to-end installation; concurrent commits receive unique
  linear generations.
