# ISSUE-024. Response pre-encoding and bounds

**Status:** Done
**Readiness:** Code-ready

**Epic:** [EPIC-001 Core](../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 5](../02-epics/EPIC-001-core.md#wave-5)

Fail oversized responses before committing success headers; compression does not change identity.

## Code-ready contract

- **Normative inputs:** ADR-010 and
  ADR-014, [Runtime Defaults](../../04-specification/runtime-defaults-and-resource-limits.md), [Metric Filtering](../../04-specification/metrics-filtering.md),
  and [Self-Metrics](../../04-specification/self-metrics.md).
- **Dependencies:** ISSUE-013 and ISSUE-015; blocks ISSUE-023.
- **Scope / out of scope:** Pre-encode one complete identity response before success headers and enforce
  response/concurrency/write limits. Out of scope: streaming partial success.
- **Configuration and observable failures:** Oversize/encode failure returns 503 before success headers and records the
  closed exposition outcome; timeout/cancellation never counts as a final scrape.
- **Acceptance criteria and required tests:** At limit/limit+1; text/OpenMetrics; self-metrics-only; filter extremes;
  concurrent limit; gzip negotiation; short/slow/cancelled writes.
- **Completion:** Complete when no failure path can expose a successful partial metric family or exceed configured
  bounds.

## Delivery log

- 2026-09-02: moved to `In Progress`; implemented immutable application/self-metric pre-encoding for Prometheus and
  OpenMetrics, exact uncompressed response bounds, gzip preparation, write classification and a non-blocking limiter.
- 2026-09-02: moved to `Testing`; exact-limit, limit-minus-one, both formats, self-metrics-only, gzip identity,
  saturation, short-write and cancellation cases passed in the Docker race gate.
- 2026-09-02: moved to `Done`; Docker vet/race/dependency/multi-architecture checks and EN/RU README audit passed.

## Verification evidence

- A response is complete in memory and within the uncompressed limit before status `200` can be committed.
- Gzip changes only transport bytes; selected generation, format body and uncompressed byte accounting remain stable.
- Short writes, cancellation and invalid encoding never return the success outcome; concurrency exhaustion is
  non-blocking and bounded.
