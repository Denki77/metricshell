# ISSUE-017. File publication protocol

**Status:** Done
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

## Delivery log

- 2026-08-29: moved to `In Progress`; implemented bounded `O_NOFOLLOW` target reads, raw-content deduplication,
  directory inotify and mandatory periodic reconciliation.
- 2026-08-29: moved to `Testing`; startup present/absent, rename, partial write, symlink, non-regular, oversized,
  unchanged and periodic-recovery tests passed under the Docker race gate.
- 2026-08-29: moved to `Done`; file ingestion passed `make test`, and EN/RU issue and implementation READMEs were
  audited together.

## Verification evidence

- Only the configured target is opened; `O_NOFOLLOW` plus `fstat` rejects symlinks and non-regular files before reads.
- `LimitReader` enforces the decoded-byte bound before parsing, including whitespace amplification.
- Inotify watches the containing directory for atomic replacements; overflow, invalidation and reinstallation have
  explicit recovery paths, while the finite periodic reconciliation remains authoritative after silent event loss.
- Absent, malformed and I/O states retain the last valid holder generation; identical accepted file content is not
  republished by periodic reconciliation.
