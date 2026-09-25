# ISSUE-MA-006. Unix socket managed-operation server

**Status:** Planned  
**Readiness:** Code-ready

**Epic:** [EPIC-002 Managed Aggregation](../02-epics/EPIC-002-managed-aggregation.md)  
**Wave:** [Wave 3](../02-epics/EPIC-002-managed-aggregation.md#wave-3)

**ADR/INV:** ADR-018 / INV-018; ADR-017 / INV-017; ADR-020 / INV-020

## Normative inputs

ADR-018, ADR-017, ADR-020, Configuration Specification.

## Dependencies

ISSUE-MA-005 and ISSUE-MA-004.

## Scope

Implement production Unix stream endpoint, filesystem lifecycle, configured permissions, bounded connection deadlines, protocol handoff to owner admission, startup readiness and close-admission behavior.

## Out of scope

Local HTTP/TCP managed transport, mandatory sidecar, remote transport.

## Configuration and observable errors

Socket path, filesystem permissions and connection read/write deadlines are managed-mode configuration. Empty/unusable paths, invalid permission values, non-positive/unrepresentable deadlines, bind conflicts and unsafe pre-existing filesystem objects are startup errors; readiness is not published and the workload does not start. At runtime, deadline expiry, partial input, disconnect and closed admission produce distinct transport outcomes and never bypass framing or owner admission. Cleanup may remove only the socket instance owned by this execution. Bind/read/write/accept/cleanup failures, readiness and admission closure are observable with sanitized paths and bounded error classes.

## Acceptance criteria

- Socket is ready before workload publication.
- Concurrent local clients reach owner safely.
- Slow/partial/disconnected client cannot corrupt registry or consume unbounded resources.
- Permissions follow accepted configuration, not prototype values.
- Admission closure rejects new work deterministically.
- Listener cleanup is deterministic.

## Required test matrix

AF_UNIX E2E; concurrent clients; slow/partial deadlines; disconnect cases; path/permission/bind failures; startup readiness; admission closure; leak/race tests.

## Completion

Complete when all acceptance criteria and required tests pass in CI and the task preserves ADR-016...ADR-020 and snapshot-mode backward compatibility.
