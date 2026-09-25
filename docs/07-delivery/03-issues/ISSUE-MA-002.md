# ISSUE-MA-002. Managed domain model and descriptor semantics

**Status:** Planned  
**Readiness:** Code-ready

**Epic:** [EPIC-002 Managed Aggregation](../02-epics/EPIC-002-managed-aggregation.md)  
**Wave:** [Wave 1](../02-epics/EPIC-002-managed-aggregation.md#wave-1)

**ADR/INV:** ADR-016 / INV-016

## Normative inputs

ADR-016 / INV-016, Managed Aggregation requirements, managed behavioral model.

## Dependencies

ISSUE-MA-001.

## Scope

Implement production domain types for descriptors, family/series identity, canonical labels, counter/gauge/classic-histogram semantics, descriptor conflict rules, reserved names, and ADR-016 batch semantics.

## Out of scope

Owner queue, socket protocol/server, Core materialization/installation, lifecycle.

## Configuration and observable errors

This task consumes descriptor and operation data, not startup properties. Invalid metric names, unsupported types or operations, duplicate/canonical-label conflicts, invalid numeric operands, non-increasing histogram buckets, reserved-name collisions and descriptor/type/HELP/label/bucket conflicts are semantic runtime rejections. A rejected single operation or batch returns its stable semantic class and leaves every descriptor, series value and generation unchanged. Batch validation completes before mutation; no prefix may commit. Capacity policy belongs to ISSUE-MA-008 and framing errors to ISSUE-MA-005.

## Acceptance criteria

- Descriptor identity is deterministic and label order does not alter series identity.
- Counter/gauge operations implement ADR-016 semantics exactly.
- Histogram observation atomically updates count, sum and applicable buckets.
- Descriptor/type/HELP/label/bucket conflicts reject without partial mutation.
- Supported batches are all-or-nothing.

## Required test matrix

Table-driven unit tests for numeric edge cases, NaN/Inf, labels, descriptor conflicts, histogram boundaries, batches and state preservation after rejection.

## Completion

Complete when all acceptance criteria and required tests pass in CI and the task preserves ADR-016...ADR-020 and snapshot-mode backward compatibility.
