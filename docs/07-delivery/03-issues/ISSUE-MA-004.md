# ISSUE-MA-004. Bounded single-owner mutation loop

**Status:** Planned  
**Readiness:** Code-ready

**Epic:** [EPIC-002 Managed Aggregation](../02-epics/EPIC-002-managed-aggregation.md)  
**Wave:** [Wave 2](../02-epics/EPIC-002-managed-aggregation.md#wave-2)

**ADR/INV:** ADR-017 / INV-017

## Normative inputs

ADR-017 / INV-017, ADR-016, Managed Aggregation requirements.

## Dependencies

ISSUE-MA-003.

## Scope

Implement one bounded owner goroutine/event loop that exclusively applies admitted mutations, establishes one registry-wide commit order, preserves per-connection receive order and emits success only after commit.

## Out of scope

Wire transport, materialization cache and lifecycle freeze.

## Configuration and observable errors

The owner-queue capacity is a validated positive, bounded managed-mode property supplied by the resource-control contract; zero, negative, overflowing or otherwise invalid values are startup configuration errors before workload start. At runtime, a full queue rejects admission as overload before ownership transfer and leaves registry state/generation unchanged. Cancellation or deadline expiry before admission is a rejection; after admission the owner produces one authoritative committed-or-rejected result. Only a successful committed result may be acknowledged as success. Queue depth/capacity, overload, cancellation and owner failures are observable with bounded labels; no accepted item is silently dropped.

## Acceptance criteria

- Concurrent counter increments preserve exact accepted sum.
- Gauge SET final value follows commit order.
- Histogram observation remains atomic.
- Descriptor races converge/reject deterministically.
- Queue is bounded and overload rejects observably.
- No accepted work is silently dropped.
- Race detector passes.

## Required test matrix

Concurrency matrix with 1/2/8/32/128 publishers; counter/gauge/histogram/descriptor races; queue boundaries; deadline/cancellation; race detector.

## Completion

Complete when all acceptance criteria and required tests pass in CI and the task preserves ADR-016...ADR-020 and snapshot-mode backward compatibility.
