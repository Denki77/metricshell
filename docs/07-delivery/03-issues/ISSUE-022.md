# ISSUE-022. Cross-adapter conformance suite

**Status:** Done
**Readiness:** Code-ready

**Epic:** [EPIC-001 Core](../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 4](../02-epics/EPIC-001-core.md#wave-4)

Run identical complete snapshots and failure cases through file, socket, and push adapters and require identical
accepted state identity.

## Code-ready contract

- **Normative inputs:** ADR-004–ADR-008 and
  ADR-015, [Application Snapshot Protocol](../../04-specification/application-snapshot-protocol.md), [Runtime Defaults](../../04-specification/runtime-defaults-and-resource-limits.md),
  and [Self-Metrics](../../04-specification/self-metrics.md).
- **Dependencies:** ISSUE-017, ISSUE-018, and ISSUE-020.
- **Scope / out of scope:** Run one immutable acceptance/rejection corpus through all three adapters. Out of scope:
  adapter-specific exceptions to semantic validation.
- **Configuration and observable failures:** Assert exact canonical bytes, generation, active state, rejection reason,
  log fields, and self-metric deltas; transport-only failures remain separately asserted.
- **Acceptance criteria and required tests:** Every candidate reason; finite/special numbers; empty state; all limits;
  concurrency; timeout; disconnect; malformed transport and recovery.
- **Completion:** Complete when adding a reason or enum requires one shared fixture and parity test updates all
  adapters.

## Delivery log

- 2026-09-01: moved to `In Progress`; implemented one shared semantic corpus over real file reconciliation, MSP/1
  commit and HTTP request paths, plus common ingestion diagnostics.
- 2026-09-01: moved to `Testing`; every candidate/state rejection reason, finite/special values, zero state, resource
  limits, replacement, admission, transport failure and recovery cases passed under Docker race testing.
- 2026-09-01: moved to `Done`; the complete Docker `make wave4` gate passed and all EN/RU implementation and issue
  READMEs were audited together.

## Verification evidence

- The immutable corpus is exhaustive against `snapshot.RejectionReasons`; adding a reason fails coverage until the one
  shared fixture is updated.
- File, Unix and HTTP produce identical outcome/reason, canonical bytes, active generation and last-valid retention for
  the same complete candidate. Accepted counter/gauge/histogram values include finite and all supported special gauges.
- Each adapter increments the same bounded publication/rejection metric labels and emits the normative accepted or
  rejected structured event with transport, generation/bytes/series or reason fields.
- Complete replacement removes omitted state. Malformed file content, a wrong MSP version and malformed HTTP gzip do
  not mutate state, and every adapter accepts the following valid publication.
- Busy and timeout admission outcomes use the common counters; busy emits the bounded `ingestion.overloaded` event.
  Socket disconnect/expiry and client timeout remain covered by their transport-specific suites.
- `make wave4` is an explicit alias of the full Docker CI gate, so CI executes the conformance suite, race detector,
  vet, dependency boundary, multi-architecture builds and real-container lifecycle acceptance tests.
