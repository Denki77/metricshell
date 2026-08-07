# ISSUE-017. File publication protocol

**Status:** Open
**Readiness:** Code-ready

**Epic:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 4](../../02-epics/EPIC-001-core.md#wave-4)

**ADR/INV:** ADR-006, ADR-015 / INV-006, INV-015. Atomic rename contract, no partial activation, inotify plus
reconciliation fallback.

## Code-ready contract

- **Normative inputs:** ADR-006 and ADR-015 / INV-006,
  INV-015, [Application Snapshot Protocol](../../../04-specification/application-snapshot-protocol.md),
  and [Runtime Defaults](../../../04-specification/runtime-defaults-and-resource-limits.md).
- **Dependencies:** ISSUE-016.
- **Scope / out of scope:** Read only the configured atomic-rename target, combine inotify with mandatory
  reconciliation, and enforce raw decoded input limits. Out of scope: accepting partial/in-place writes.
- **Configuration and observable failures:** Absent, invalid, oversized and I/O states preserve last-valid snapshot and
  produce bounded file outcomes without logging paths or payloads.
- **Acceptance criteria and required tests:** Startup present/absent; atomic rename; in-place partial write;
  symlink/non-regular; overflow/invalidation/reinstall; whitespace-amplified file; periodic recovery.
- **Completion:** Complete when event loss and malformed files cannot cause partial activation or unbounded reads.
