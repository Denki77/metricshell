# ISSUE-010. Health and readiness contract

**Status:** Done
**Readiness:** Code-ready

**Epic:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 2](../../02-epics/EPIC-001-core.md#wave-2)

Define probe behavior per lifecycle state. Health/readiness/debug requests never count as final scrapes.

## Code-ready contract

- **Normative inputs:** ADR-002 and
  ADR-011, [Configuration](../../../04-specification/configuration.md), [Runtime State Machine](../../../04-specification/runtime-state-machine.md),
  and [Structured Logging](../../../04-specification/structured-logging.md).
- **Dependencies:** ISSUE-007; blocks probe handling in ISSUE-023 and final-scrape logic in ISSUE-026.
- **Scope / out of scope:** Implement fixed `/healthz` and `/readyz` semantics for every public state. Out of scope:
  configurable probe paths and counting probes as scrapes.
- **Configuration and observable failures:** Probe responses are bounded and state-derived; unavailable/failed states
  return deterministic statuses without mutating lifecycle.
- **Acceptance criteria and required tests:** State-by-endpoint status table; transition races; requests during
  shutdown; method/path errors; proof probes never increment final-scrape count.
- **Completion:** Complete when the specification table and HTTP integration fixtures agree for every state.

## Delivery log

- 2026-08-24: moved to `In Progress`; a bounded HTTP probe adapter was implemented directly over the lifecycle state
  source.
- 2026-08-24: moved to `Testing`; the full state/endpoint matrix, shutdown states, concurrent transitions,
  method/path errors and final-scrape exclusion passed with the race detector and a real HTTP server in Docker.
- 2026-08-24: moved to `Done`; the normative EN/RU table and the Docker fixture agree for all eight public states.

## Verification evidence

- Exact `GET /healthz` and `GET /readyz` routes return state-derived bounded text responses; other methods return `405`
  with `Allow: GET`, and other paths return `404`.
- Initializing, starting, stopping, finalizing and final-wait remain healthy but unready; running is healthy and ready;
  failed is `500`/`503`; an accepted request observing terminated receives deterministic `503 unavailable`.
- Probe handling only reads the synchronized lifecycle state. Concurrent transition/request tests pass under the race
  detector and probe routing cannot invoke `/metrics` final-scrape completion.
- The HTTP integration fixture performs 18 requests over a real loopback server inside the scratch integration image.
- `implementation/README.md` and `implementation/README_RU.md` were reviewed and updated together.
