# ISSUE-MA-015. Legacy, performance, resource and production validation

**Status:** Done
**Readiness:** Code-ready

**Epic:** [EPIC-002 Managed Aggregation](../02-epics/EPIC-002-managed-aggregation.md)  
**Wave:** [Wave 7](../02-epics/EPIC-002-managed-aggregation.md#wave-7)

**ADR/INV:** ADR-018 / INV-018; ADR-019 / INV-019; ADR-020 / INV-020

## Normative inputs

INV-018/019/020 evidence discipline, ADR-018/019/020 and Core release benchmark policy.

## Dependencies

ISSUE-MA-014.

## Scope

Validate production implementation with real PHP 5.4 and shell clients and repeat key cardinality, histogram, owner-queue, concurrent scrape/mutation, slow client/scraper, memory pressure, shutdown/freeze and restart scenarios with reproducible evidence.

## Out of scope

Changing architecture from timing alone; turning tested endpoints into product defaults/SLA without accepted rule.

## Configuration and observable errors

Validation runs only with explicit recorded production-like configuration; research endpoints are test inputs, never implicit defaults. Invalid harness configuration or missing provenance fails the validation run before results are accepted. Under load, each configured frame/queue/series/histogram/deadline/finalization bound must produce its specified observable rejection or lifecycle outcome, and rejected work must leave the asserted registry generation/Core state intact. Fatal OOM/process loss is recorded separately from managed policy rejection. Timing and resource measurements include environment fingerprint, configuration, outcome counts and raw evidence and cannot establish a default or SLA by themselves.

## Acceptance criteria

- PHP 5.4 and shell pass production compatibility.
- Configured bounds remain enforced under load.
- Concurrent mutation/scrape never returns mixed generation.
- Slow clients/scrapers and memory pressure cannot bypass bounded policies.
- Shutdown/freeze/restart correctness matches ADR-020.
- Raw evidence and provenance are retained.
- No timing number becomes default/SLA without explicit accepted selection.

## Required test matrix

Production matrices derived from INV-018/019/020; multiple architectures/environments where practical; provenance/fingerprint; controlled resource pressure; correctness assertions.

## Completion

Complete when all acceptance criteria and required tests pass in CI and the task preserves ADR-016...ADR-020 and snapshot-mode backward compatibility.

## Delivery log

- 2026-09-27: moved to `In Progress`; added a controlled policy/concurrency/materialization validation matrix and a
  Docker artifact stage carrying raw race, benchmark, configuration, revision and Go environment evidence.
- 2026-09-27: moved to `Testing`; composed evidence generation with the real PHP 5.4 compatibility and managed
  production-process E2E gates and documented that timing selects no default or SLA.
- 2026-09-27: moved to `Done`; repeated race validation, production compatibility, artifact provenance and the full
  Docker test gate passed.

## Verification evidence

- `cd implementation && make managed-validation IMAGE=metricshell-ma015`
- `cd implementation && make test IMAGE=metricshell-ma015`
- generated artifact: `implementation/dist/managed-validation/validation/`
