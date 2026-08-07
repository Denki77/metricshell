# ISSUE-032. Static multi-architecture release artifacts

**Status:** Open
**Readiness:** Code-ready

**Epic:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 6](../../02-epics/EPIC-001-core.md#wave-6)

amd64/arm64, checksums, version, OCI metadata, pinned multi-stage copy.

The required executable image/copy examples and their release verification must conform to the accepted
[Docker and Docker Compose Examples Specification](../../../04-specification/docker-compose-examples.md).

## Code-ready contract

- **Normative inputs:** ADR-013, [Docker and Compose Examples](../../../04-specification/docker-compose-examples.md),
  and [Configuration](../../../04-specification/configuration.md).
- **Dependencies:** ISSUE-001 and ISSUE-036.
- **Scope / out of scope:** Produce static linux amd64/arm64 binaries, checksums, version metadata, pinned OCI copy
  examples, and multi-arch image metadata. Out of scope: copying prototype binaries.
- **Configuration and observable failures:** Checksum/architecture/version mismatch fails before installation; mutable
  tags alone are forbidden in normative examples.
- **Acceptance criteria and required tests:** Clean cross-build; static linkage inspection; checksum corruption; wrong
  architecture; digest-pinned copy; container `--version`; multi-arch manifest.
- **Completion:** Complete when independent clean builders reproduce verifiable artifacts for both architectures.
