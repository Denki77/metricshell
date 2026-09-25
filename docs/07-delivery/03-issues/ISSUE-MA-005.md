# ISSUE-MA-005. Operation protocol v1 and bounded framing

**Status:** Planned  
**Readiness:** Code-ready

**Epic:** [EPIC-002 Managed Aggregation](../02-epics/EPIC-002-managed-aggregation.md)  
**Wave:** [Wave 3](../02-epics/EPIC-002-managed-aggregation.md#wave-3)

**ADR/INV:** ADR-018 / INV-018; ADR-017 / INV-017

## Normative inputs

ADR-018 / INV-018, ADR-017, configuration/resource-limit contracts.

## Dependencies

ISSUE-MA-004.

## Scope

Implement protocol v1 request/response structures, bounded newline-delimited JSON framing, one operation per connection, deterministic version/framing/protocol/semantic outcomes and bounded parsing.

## Out of scope

Unix listener lifecycle, CLI/PHP wrappers, unsupported alternate managed transports.

## Configuration and observable errors

The configured maximum frame size and parsing bounds must be positive, finite and representable; invalid values are startup configuration errors. Empty, truncated, malformed, multi-frame, oversized or unsupported-version input is a protocol rejection before owner admission and cannot mutate registry state. A parsed and semantically admitted request is not committed merely because parsing/admission succeeded. The protocol emits success only from the successful committed owner result defined by ISSUE-MA-004; semantic rejection, overload and pre-commit cancellation remain non-success outcomes, while disconnect/timeout after admission but before response is an explicit unknown client outcome. Outcome class and frame-limit rejection are observable without logging payloads.

## Acceptance criteria

- Valid v1 operation parses to one domain command.
- Partial frame never reaches mutation.
- Oversized frame rejects within bounded memory.
- Missing/invalid/unsupported version rejects deterministically.
- Transport/protocol/semantic/overload/unknown outcomes remain distinct.
- Success is derived only from ISSUE-MA-004's successful committed owner result; parsing or admission alone can never produce success.

## Required test matrix

Golden corpus; exact frame limit/+1; malformed JSON; partial EOF; versions; semantic rejection mapping; fuzz/property tests; no mutation on parse failure.

## Completion

Complete when all acceptance criteria and required tests pass in CI and the task preserves ADR-016...ADR-020 and snapshot-mode backward compatibility.
