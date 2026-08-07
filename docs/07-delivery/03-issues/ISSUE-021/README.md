# ISSUE-021. Enforce mmap as non-primary

**Status:** Open
**Readiness:** Code-ready

**Epic:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 4](../../02-epics/EPIC-001-core.md#wave-4)

**ADR/INV:** ADR-009 / INV-009. The first release and core API have no shared-memory ABI dependency.

## Code-ready contract

- **Normative inputs:** ADR-009 / INV-009 and [Configuration](../../../04-specification/configuration.md).
- **Dependencies:** ISSUE-001.
- **Scope / out of scope:** Enforce that Core exposes no mmap option, shared-memory ABI, or production dependency. Out
  of scope: future experimental research.
- **Configuration and observable failures:** Any undocumented mmap/shared-memory option is rejected as unknown;
  dependency checks fail CI if production imports prototype mmap code.
- **Acceptance criteria and required tests:** CLI/environment negative tests; public API scan; dependency/license scan;
  clean build with the research tree unavailable.
- **Completion:** Complete when release artifacts and public packages contain no shared-memory contract.
