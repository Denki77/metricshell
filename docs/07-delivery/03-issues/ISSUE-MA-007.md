# ISSUE-MA-007. CLI helper and PHP 5.4 reference client

**Status:** Planned  
**Readiness:** Code-ready

**Epic:** [EPIC-002 Managed Aggregation](../02-epics/EPIC-002-managed-aggregation.md)  
**Wave:** [Wave 3](../02-epics/EPIC-002-managed-aggregation.md#wave-3)

**ADR/INV:** ADR-018 / INV-018

## Normative inputs

ADR-018 / INV-018, Managed Aggregation requirements.

## Dependencies

ISSUE-MA-006.

## Scope

Provide shell-friendly CLI operations and a minimal PHP 5.4-compatible stateless reference client implementing protocol v1 and documented safe retry boundaries.

## Out of scope

Persistent local agent, exactly-once retry guarantees, modern Prometheus client dependency.

## Configuration and observable errors

CLI/client configuration is limited to the socket endpoint and a bounded client deadline. Missing/invalid endpoint or deadline is a local invocation error before connection. Invalid operation arguments fail locally without sending a frame. Server `rejected`, `overload` and protocol responses are distinct non-zero machine-readable outcomes; connect/read/write failures are transport outcomes. EOF or timeout after a complete request was sent but before a response is `unknown`, because commit may have occurred, and the helper must not advertise blind retry as safe. Diagnostics exclude credentials and unbounded application payloads.

## Acceptance criteria

- Shell uses operations without building snapshots.
- PHP 5.4 client keeps no registry state.
- Accepted/rejected/connect/protocol/unknown outcomes are machine-detectable.
- Reconnect requires no registry reconstruction.
- Unknown outcome is not blindly retried.

## Required test matrix

Real PHP 5.4 container tests; shell CLI tests; accepted/rejected/connect/protocol/unknown cases; reconnect; quoting/labels/numeric edges.

## Completion

Complete when all acceptance criteria and required tests pass in CI and the task preserves ADR-016...ADR-020 and snapshot-mode backward compatibility.
