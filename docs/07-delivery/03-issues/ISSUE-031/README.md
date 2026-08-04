# ISSUE-031. Multi-replica Prometheus integration test

**Status:** Open
**Readiness:** Code-ready

**Epic:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 6](../../02-epics/EPIC-001-core.md#wave-6)

Query each configured replica independently; aggregate request counts are insufficient.

## Code-ready contract

- **Normative inputs:** ADR-012 /
  INV-012, [Docker and Compose Examples](../../../04-specification/docker-compose-examples.md),
  and [Self-Metrics](../../../04-specification/self-metrics.md).
- **Dependencies:** ISSUE-029 and ISSUE-030.
- **Scope / out of scope:** Verify each configured replica independently through Prometheus labels and historical
  queries. Out of scope: accepting aggregate-only success.
- **Configuration and observable failures:** Missing, duplicate, stale-only, or cross-replica samples fail with
  replica-specific diagnostics and bounded query time.
- **Acceptance criteria and required tests:** Two or more replicas; one missing publication; duplicate label; target
  disappearance/stale marker; delayed scrape; aggregate count false positive.
- **Completion:** Complete when the test fails if any named replica lacks its own expected final sample.
