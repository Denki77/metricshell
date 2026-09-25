# ISSUE-016. Common transport-independent ingestion interface

**Status:** Done
**Readiness:** Code-ready

**Epic:** [EPIC-001 Core](../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 4](../02-epics/EPIC-001-core.md#wave-4)

**ADR/INV:** ADR-005 / INV-005. Shared result types, error taxonomy, admission hooks, deadlines, and candidate handoff.

## Code-ready contract

- **Normative inputs:** ADR-005 /
  INV-005, [Application Snapshot Protocol](../../04-specification/application-snapshot-protocol.md), [Configuration](../../04-specification/configuration.md),
  and [Runtime Defaults](../../04-specification/runtime-defaults-and-resource-limits.md).
- **Dependencies:** ISSUE-012 and ISSUE-013; blocks ISSUE-017, ISSUE-018, and ISSUE-020.
- **Scope / out of scope:** Define common admission, deadlines, candidate handoff, result, and error taxonomy. Out of
  scope: adapter wire framing.
- **Configuration and observable failures:** Candidate reasons, publication outcomes, and transport failures are
  distinct typed enums and map to shared logs/self-metrics.
- **Acceptance criteria and required tests:** Contract tests with fake adapters for
  accepted/rejected/busy/timeout/frozen/internal; cancellation; queue boundaries; enum-parity compile/test check.
- **Completion:** Complete when every adapter implements the interface without translating semantic rejection ad hoc.

## Delivery log

- 2026-08-29: moved to `In Progress`; implemented the shared transport/result/failure registries, bounded admission,
  cancellable queue and complete-candidate handoff to the atomic holder.
- 2026-08-29: moved to `Testing`; acceptance, rejection, busy, timeout, frozen, internal, cancellation, queue-boundary,
  linearization and enum-parity tests passed under the race detector in Docker.
- 2026-08-29: moved to `Done`; the production module passed the Docker `make test` gate and both implementation READMEs
  were audited.

## Verification evidence

- File, Unix and HTTP use one `Publisher` contract and return one typed `Result`; candidate reasons remain the snapshot
  registry while wire failures use a separate typed registry.
- Admission bounds executing and pending work independently. Cancellation is checked before parsing and again before
  atomic installation, so timed-out work cannot mutate active state.
- The shared metrics observer updates inflight, publication, rejection, last-success and active-snapshot metrics without
  introducing attacker-controlled labels.
- Concurrent accepted candidates receive unique, monotonically assigned generations from the same holder.
