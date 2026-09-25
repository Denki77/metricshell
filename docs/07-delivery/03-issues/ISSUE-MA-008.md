# ISSUE-MA-008. Managed resource controls

**Status:** Planned  
**Readiness:** Code-ready

**Epic:** [EPIC-002 Managed Aggregation](../02-epics/EPIC-002-managed-aggregation.md)  
**Wave:** [Wave 4](../02-epics/EPIC-002-managed-aggregation.md#wave-4)

**ADR/INV:** ADR-019 / INV-019; ADR-017 / INV-017; ADR-018 / INV-018

## Normative inputs

ADR-019 / INV-019, ADR-017, ADR-018, Configuration, Runtime Defaults and Resource Limits.

## Dependencies

ISSUE-MA-004, ISSUE-MA-005 and ISSUE-MA-006.

## Scope

Implement configurable independent bounds for active series, histogram buckets, owner queue, protocol frame and other accepted managed dimensions; reject before mutation/unsafe allocation.

## Out of scope

Using research coverage endpoints as defaults; masking fatal OOM as normal rejection.

## Configuration and observable errors

Independent limits cover active families/series, labels, histogram buckets, batch/operation dimensions, owner-queue capacity and protocol frame size where specified by the accepted configuration contract. Missing values use only accepted documented defaults; zero, negative, overflowing, internally inconsistent or unsupported values fail configuration before workload start. At runtime each limit is checked before unsafe allocation and before mutation. Limit exhaustion is a policy rejection distinct from queue overload, protocol rejection and fatal process OOM; it preserves the complete committed registry and generation. Limit name, configured bound and rejection count are observable with bounded cardinality, without application-controlled labels.

## Acceptance criteria

- Each bound is enforced independently.
- At/under limit succeeds when otherwise valid; limit+1 rejects.
- Rejected operation preserves committed generation/state.
- Queue overload remains distinct from policy rejection.
- Fatal OOM remains a process/container failure.
- No INV-019 number becomes default without accepted rule.

## Required test matrix

Below/equal/above matrix for all limits; combined limits; configuration validation; queue overload; generation preservation; memory-pressure distinction; snapshot regression.

## Completion

Complete when all acceptance criteria and required tests pass in CI and the task preserves ADR-016...ADR-020 and snapshot-mode backward compatibility.
