# ISSUE-021. Enforce mmap as non-primary

**Status:** Done
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

## Delivery log

- 2026-08-31: moved to `In Progress`; added explicit rejection of mmap/shared-memory CLI and environment surfaces and
  strengthened the production dependency boundary.
- 2026-08-31: moved to `Testing`; negative configuration, public API/primitive, resolved dependency, module-license and
  research-unavailable Docker-context checks passed in the Docker race/build gate.
- 2026-08-31: moved to `Done`; the production artifact built for amd64/arm64 without research or shared-memory ABI, and
  EN/RU issue and implementation READMEs were audited together.

## Verification evidence

- mmap/shared-memory CLI spellings and the reserved environment spellings are rejected before workload start; the
  three documented stable transports remain the only architectural ingestion set.
- The production AST scan fails on exported shared-memory APIs and direct mmap/shm primitives, while the resolved
  package/module scans reject prototype dependencies.
- Every future external module must expose a discoverable license file to pass the same boundary test.
- Docker tests assert that the research tree is absent from the production build context, proving production code does
  not compile by accidental access to prototypes.
