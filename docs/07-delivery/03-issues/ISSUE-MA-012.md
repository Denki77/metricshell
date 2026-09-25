# ISSUE-MA-012. Freeze, final snapshot and final-scrape integration

**Status:** Planned  
**Readiness:** Code-ready

**Epic:** [EPIC-002 Managed Aggregation](../02-epics/EPIC-002-managed-aggregation.md)  
**Wave:** [Wave 5](../02-epics/EPIC-002-managed-aggregation.md#wave-5)

**ADR/INV:** ADR-020 / INV-020; ADR-019 / INV-019; ADR-004 / INV-004; ADR-011 / INV-011

## Normative inputs

ADR-020, ADR-019, ADR-004, Runtime State Machine and existing final-scrape specification.

## Dependencies

ISSUE-MA-011 and ISSUE-MA-010.

## Scope

Implement one logical freeze per managed epoch, reject post-freeze publishers, select the frozen generation as the authoritative final generation, make one final candidate/install attempt and delegate waiting to existing Core final-scrape modes.

## Out of scope

New managed final wait, Kubernetes API coordination, persistence/replay.

## Configuration and observable errors

Finalization uses only the existing Core final-scrape mode/duration/count and shutdown budget; invalid lifecycle values remain startup configuration errors. One successful freeze fixes one authoritative final registry generation. Exactly one final materialization/candidate/install attempt is made for that generation. Conversion, validation or install may fail; such failure is observable, does not retry or select another generation, and preserves the previously active Core state. After freeze every publisher is rejected as late and cannot change the registry. Freeze winner/generation, late rejection, candidate outcome and retained-Core-state outcome are observable with bounded labels.

## Acceptance criteria

- Concurrent freeze attempts produce one logical winner.
- Application registry/generation is immutable after freeze.
- Late publishers reject deterministically.
- A single freeze selects one authoritative final generation and triggers one final candidate/install attempt.
- If final materialization, validation or installation fails, no final generation is installed and the previous Core state remains active.
- Failed final candidate preserves prior Core state.
- Core final-scrape eligibility/counting remains unchanged; self-metrics may evolve.
- New execution starts a fresh empty epoch.

## Required test matrix

Freeze races; late publisher storm; conversion/validation/install failures; natural/nonzero/signal exits; immediate/duration/scrapes; eligible/ineligible scrape matrix; restart/new epoch.

## Completion

Complete when all acceptance criteria and required tests pass in CI and the task preserves ADR-016...ADR-020 and snapshot-mode backward compatibility.
