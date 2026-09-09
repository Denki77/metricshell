# ISSUE-030. Kubernetes lifecycle controls

**Status:** Done
**Readiness:** Code-ready

**Epic:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 6](../../02-epics/EPIC-001-core.md#wave-6)

activeDeadlineSeconds, ttlSecondsAfterFinished, termination grace, and CronJob Forbid examples/tests.

## Code-ready contract

- **Normative inputs:** ADR-003 and
  ADR-012, [Runtime Defaults](../../../04-specification/runtime-defaults-and-resource-limits.md),
  and [Docker and Compose Examples](../../../04-specification/docker-compose-examples.md).
- **Dependencies:** ISSUE-008, ISSUE-009, and ISSUE-029.
- **Scope / out of scope:** Define Job/CronJob deadlines, TTL, restart/concurrency policy, and termination grace with
  measurable margin over internal grace. Out of scope: cluster-specific admission policy.
- **Configuration and observable failures:** Static checks reject external grace not greater than internal total; forced
  deadline and scheduling outcomes are visible in Kubernetes status and logs.
- **Acceptance criteria and required tests:** Deadline before/after workload; 32s external versus 30s internal baseline;
  TTL cleanup; CronJob Forbid; restart Never; forced termination.
- **Completion:** Complete when manifests encode every lifecycle bound and conformance tests measure the safety margin.

## Delivery log

- 2026-09-09: moved to `In Progress`; added Kubernetes lifecycle control manifests for finite Jobs and CronJob overlap
  prevention.
- 2026-09-09: moved to `Testing`; static conformance checks assert 32s external grace over 30s internal grace,
  activeDeadlineSeconds, TTL, restartPolicy Never, backoffLimit 0 and CronJob Forbid.
- 2026-09-09: moved to `Done`; lifecycle examples document the external-versus-internal bound relationship.

## Verification evidence

- `go test ./internal/kubeexamples`
