# ISSUE-MA-014. Concurrency, shutdown, restart and failure E2E suite

**Status:** Planned  
**Readiness:** Code-ready

**Epic:** [EPIC-002 Managed Aggregation](../02-epics/EPIC-002-managed-aggregation.md)  
**Wave:** [Wave 6](../02-epics/EPIC-002-managed-aggregation.md#wave-6)

**ADR/INV:** ADR-016 / INV-016; ADR-017 / INV-017; ADR-018 / INV-018; ADR-019 / INV-019; ADR-020 / INV-020

## Normative inputs

ADR-016 through ADR-020 and all accepted Managed Aggregation specifications.

## Dependencies

ISSUE-MA-012 and ISSUE-MA-013.

## Scope

Build production-process E2E/race coverage across real Unix clients, owner queue, registry, materialization, Core install/exposition, workload lifecycle, overload, failure injection and restart.

## Out of scope

Performance certification/default selection; research prototype execution as substitute for production tests.

## Configuration and observable errors

The suite must exercise accepted minimum/equal/over-limit values and invalid startup configuration for every managed property owned by ISSUE-MA-001 and ISSUE-MA-004...013. Runtime assertions distinguish protocol, semantic, resource, overload, late, timeout and unknown outcomes and verify the committed registry generation plus active Core state after each rejection. Failure injection covers bind/read/write, owner, materialization, validation/install, diagnostics and budget exhaustion. Tests must fail on silent drops, success without commit, post-freeze mutation, mixed generations, unbounded waits or leaked resources and must capture the bounded observable outcome that identified the failure.

## Acceptance criteria

- Multiple publishers preserve exact accepted state and one commit order.
- ACK-loss keeps committed mutation and exposes unknown client outcome.
- Malformed/partial/disconnected/overload cases preserve valid Core state.
- Natural/nonzero exit, SIGTERM, budget exhaustion and late publishers follow ADR-020.
- Restart starts empty epoch.
- Final scrape exposes frozen generation with existing eligibility rules.
- Race detector passes.

## Required test matrix

Production scenario matrix derived from INV-017/018/020; repeated races; process-level containers; race detector; leak/deadlock timeouts.

## Completion

Complete when all acceptance criteria and required tests pass in CI and the task preserves ADR-016...ADR-020 and snapshot-mode backward compatibility.
