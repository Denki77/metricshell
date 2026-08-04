# ISSUE-033. Container hardening defaults

**Status:** Open
**Readiness:** Code-ready

**Epic:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 6](../../02-epics/EPIC-001-core.md#wave-6)

Non-root, read-only rootfs, dropped capabilities, no-new-privileges, private paths, loopback/local socket.

## Code-ready contract

- **Normative inputs:** ADR-007, ADR-008, and
  ADR-013, [Configuration](../../../04-specification/configuration.md), [Runtime Defaults](../../../04-specification/runtime-defaults-and-resource-limits.md),
  and [Docker and Compose Examples](../../../04-specification/docker-compose-examples.md).
- **Dependencies:** ISSUE-018, ISSUE-020, and ISSUE-032.
- **Scope / out of scope:** Ship non-root, read-only-rootfs, dropped-capability, no-new-privileges examples with private
  runtime paths and socket group ownership. Out of scope: privileged fallback.
- **Configuration and observable failures:** Permission/bind/path failures use normative startup errors; socket mode is
  exactly 0660 and HTTP ingestion remains loopback-only.
- **Acceptance criteria and required tests:** Arbitrary non-root UID; shared producer GID; read-only rootfs; no
  capabilities; unwritable/missing runtime dir; socket mode/ownership assertions.
- **Completion:** Complete when hardened examples run without privilege and a second UID in the configured group can
  publish.
