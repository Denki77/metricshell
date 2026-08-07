# ISSUE-029. Kubernetes Job integration

**Status:** Open
**Readiness:** Code-ready

**Epic:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 6](../../02-epics/EPIC-001-core.md#wave-6)

PodMonitor/direct discovery examples, bounded final window, explicit readiness policy.

## Code-ready contract

- **Normative inputs:** ADR-012 /
  INV-012, [Docker and Compose Examples](../../../04-specification/docker-compose-examples.md), [Runtime State Machine](../../../04-specification/runtime-state-machine.md),
  and [Configuration](../../../04-specification/configuration.md).
- **Dependencies:** ISSUE-010, ISSUE-026, ISSUE-032, and ISSUE-033.
- **Scope / out of scope:** Provide executable Kubernetes Job discovery/PodMonitor examples with explicit ports, probes,
  final window, and Prometheus verification. Out of scope: declaring one operator mandatory.
- **Configuration and observable failures:** Manifest/config errors fail static verification; runtime verifier
  distinguishes a final sample from a later stale marker and reports missing targets.
- **Acceptance criteria and required tests:** Schema/kubeconform; direct discovery and PodMonitor; finite job; final
  sample query at recorded time; target disappearance; readiness transitions.
- **Completion:** Complete when examples run from clean manifests and verify the final sample without relying on an
  active target.
