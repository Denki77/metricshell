# ISSUE-035. Fault, soak, and race suite

**Status:** Open
**Readiness:** Code-ready

**Epic:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 6](../../02-epics/EPIC-001-core.md#wave-6)

Malformed repetition, slow clients, disconnects, queue saturation, bind failure, forced OOM, signal races, graceful
drain.

## Code-ready contract

- **Normative inputs:** ADR-001–ADR-015 and all accepted specifications,
  especially [Runtime Defaults](../../../04-specification/runtime-defaults-and-resource-limits.md)
  and [Structured Logging](../../../04-specification/structured-logging.md).
- **Dependencies:** ISSUE-001–ISSUE-034 implementation surfaces.
- **Scope / out of scope:** Automate fault injection, soak, fuzz, and race coverage for bounded runtime behavior. Out of
  scope: redefining normative limits from benchmark results.
- **Configuration and observable failures:** Every injected failure asserts exit/result origin, last-valid state, closed
  log/self-metric enums, and bounded completion.
- **Acceptance criteria and required tests:** Malformed flood; slow clients; disconnects; saturation; bind/path failure;
  OOM container; signal/publication/scrape races; long reconciliation and drain.
- **Completion:** Complete when the production binary passes documented-duration suites with reproducible seeds and
  retained failure artifacts.
