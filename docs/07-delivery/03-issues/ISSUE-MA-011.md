# ISSUE-MA-011. Runtime admission barrier and bounded drain

**Status:** Planned  
**Readiness:** Code-ready

**Epic:** [EPIC-002 Managed Aggregation](../02-epics/EPIC-002-managed-aggregation.md)  
**Wave:** [Wave 5](../02-epics/EPIC-002-managed-aggregation.md#wave-5)

**ADR/INV:** ADR-020 / INV-020; ADR-003 / INV-003; ADR-017 / INV-017

## Normative inputs

ADR-020 / INV-020, ADR-003, ADR-017, Runtime State Machine.

## Dependencies

ISSUE-MA-006 and ISSUE-MA-010.

## Scope

Integrate close-admission with natural workload exit and external termination; only already complete/validated/admitted work may finish within remaining existing shutdown/finalization budget.

## Out of scope

New drain timeout, unbounded drain, mandatory client flush/close handshake, final freeze implementation.

## Configuration and observable errors

This task reuses the existing shutdown/finalization budget and must not add a second drain timeout. Invalid existing lifecycle duration/count configuration remains a startup error under the Runtime State Machine contract. After admission closes, new, partial, validated-but-not-admitted and queued-without-owner-acceptance work receives a late/closed rejection and cannot mutate state. Work already admitted by the owner may finish only within the remaining budget; expiry produces an observable bounded-drain failure/abandonment outcome without claiming an uncommitted operation succeeded. Admission-close reason, admitted/in-flight counts, drain completion and budget exhaustion are observable with bounded labels.

## Acceptance criteria

- New operations reject after closure.
- Partial and received-not-admitted work cannot commit after closure.
- Admitted/committing work may finish only while budget remains.
- Budget exhaustion terminates drain deterministically.
- Shutdown stays within ADR-003 budget.
- ACK semantics remain ADR-017 compliant.

## Required test matrix

Natural exit/SIGTERM at each receive/validate/queue/commit/ACK stage; zero/small/expiring budgets; slow admitted work; late connection storm.

## Completion

Complete when all acceptance criteria and required tests pass in CI and the task preserves ADR-016...ADR-020 and snapshot-mode backward compatibility.
