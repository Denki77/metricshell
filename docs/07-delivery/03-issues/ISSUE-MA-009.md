# ISSUE-MA-009. Generation-based immutable materialization

**Status:** Planned  
**Readiness:** Code-ready

**Epic:** [EPIC-002 Managed Aggregation](../02-epics/EPIC-002-managed-aggregation.md)  
**Wave:** [Wave 4](../02-epics/EPIC-002-managed-aggregation.md#wave-4)

**ADR/INV:** ADR-019 / INV-019

## Normative inputs

ADR-019 / INV-019, ADR-004, ADR-017.

## Dependencies

ISSUE-MA-003, ISSUE-MA-004 and ISSUE-MA-008.

## Scope

Implement immutable encoded snapshot caching keyed by complete registry generation, with reuse for unchanged generation, stale marking after commit and safe concurrent/slow-reader lifetime.

## Out of scope

Core install, lifecycle freeze, guarantee that every intermediate generation is published.

## Configuration and observable errors

Materialization consumes a complete committed generation and has no independent public mode switch. An impossible/unsupported registry value or encoding failure rejects that candidate and preserves the previous immutable cache entry and Core state; it must not publish partial bytes. Concurrent requests for an unchanged generation may reuse the same immutable representation, while a commit only marks it stale for future requests. Cache generation, hit/miss/rebuild/failure and coalescing are observable with bounded labels. Slow readers retain their issued bytes and cannot extend mutable registry ownership or force unbounded retained generations beyond the accepted resource policy.

## Acceptance criteria

- Every encoded body belongs to one registry-wide generation.
- No mixed-generation response under concurrent mutation/readers.
- Unchanged generation reuses representation.
- New commit makes cache stale for future materialization without mutating issued bytes.
- Slow readers retain immutable bytes safely.
- Intermediate generations may coalesce.

## Required test matrix

Linked cross-family consistency test; concurrent mutation/materialization; slow readers; cache hit/miss; stale rebuild; race detector; memory lifetime checks.

## Completion

Complete when all acceptance criteria and required tests pass in CI and the task preserves ADR-016...ADR-020 and snapshot-mode backward compatibility.
