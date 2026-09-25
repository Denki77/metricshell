# ISSUE-MA-016. Documentation, examples and release readiness

**Status:** Planned  
**Readiness:** Code-ready

**Epic:** [EPIC-002 Managed Aggregation](../02-epics/EPIC-002-managed-aggregation.md)  
**Wave:** [Wave 7](../02-epics/EPIC-002-managed-aggregation.md#wave-7)

**ADR/INV:** ADR-016 / INV-016; ADR-017 / INV-017; ADR-018 / INV-018; ADR-019 / INV-019; ADR-020 / INV-020

## Normative inputs

Managed Aggregation requirements; ADR-016 through ADR-020; Configuration, Runtime State Machine, Self-Metrics, Structured Logging, Defaults/Resource Limits, Docker/Compose docs.

## Dependencies

ISSUE-MA-015 and all prior Managed Aggregation issues.

## Scope

Finalize normative Managed Aggregation specification and update EN/RU implementation-facing specs, traceability, mode/protocol/socket security, limits, lifecycle, CLI/PHP examples, Docker/Compose/Job-shaped usage, troubleshooting and release notes.

## Out of scope

New architecture decisions, hybrid mode, distributed/persistent aggregation.

## Configuration and observable errors

Every public managed property must be documented with owner, syntax, default source, valid domain, startup-versus-runtime validation point and observable failure. Examples must use valid bounded values and demonstrate protocol/semantic/resource/overload/late/unknown outcomes without promising retry safety or success before commit. Documentation must state that rejected operations preserve committed registry/Core state, and that final candidate validation/install can fail while prior Core state remains active. Missing EN/RU parity, broken links, undocumented configuration or examples that disagree with accepted ADR/specification behavior are release-blocking documentation failures.

## Acceptance criteria

- Behavioral draft is promoted/replaced by accepted normative spec.
- Configuration/defaults/resource limits cover every public managed property.
- Self-metrics/logging specs contain final managed registry/enums.
- Runtime lifecycle documents ADR-020 composition.
- CLI/PHP/Docker/Compose examples are verified per docs policy.
- Traceability maps every FR-MA/NFR-MA to code/tests.
- Snapshot-mode compatibility/regression evidence is recorded.
- EN/RU parity checks pass.

## Required test matrix

Documentation link/parity checks; example automation; configuration completeness audit; traceability completeness audit; full CI and snapshot/managed regression suites.

## Completion

Complete when all acceptance criteria and required tests pass in CI and the task preserves ADR-016...ADR-020 and snapshot-mode backward compatibility.
