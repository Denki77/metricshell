# ISSUE-005. Child reaping and orphan handling

**Status:** Done
**Readiness:** Code-ready

**Epic:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 1](../../02-epics/EPIC-001-core.md#wave-1)

Reap adopted descendants, track the primary workload PID separately, and prove zero zombies after stress tests.

## Code-ready contract

- **Normative inputs:** ADR-001 /
  INV-001, [Runtime State Machine](../../../04-specification/runtime-state-machine.md), [Self-Metrics](../../../04-specification/self-metrics.md),
  and [Structured Logging](../../../04-specification/structured-logging.md).
- **Dependencies:** ISSUE-002 and ISSUE-003.
- **Scope / out of scope:** Reap direct and adopted children while tracking the primary workload separately. Out of
  scope: supervising unrelated services.
- **Configuration and observable failures:** Unexpected child outcomes are sanitized diagnostics; primary outcome
  remains authoritative; child PIDs are excluded from metric labels.
- **Acceptance criteria and required tests:** Orphan adoption, double-fork, burst exits, primary-before-child and
  child-before-primary order, zero-zombie stress, race detector.
- **Completion:** Complete when stress fixtures leave zero zombies and exactly one primary workload result.

## Delivery log

- 2026-08-15: moved to `In Progress`; implementation started in the dedicated `ISSUE-005` worktree.
- 2026-08-15: moved to `Testing`; subreaper-based adoption, single-owner child reaping, primary-result separation, and
  Docker acceptance fixtures are implemented; the Docker unit/race gate and integration suite passed.
- 2026-08-15: moved to `Done`; final Docker CI, arm64 production image, and EN/RU README completeness checks passed.
- 2026-08-15: returned to `In Progress`; review found primary-PID reuse and external-init subreaper coverage gaps.
- 2026-08-15: moved back to `Testing`; primary PID reuse is handled once-only and orphan adoption now passes with an
  external Docker init above MetricShell.
- 2026-08-15: moved back to `Done`; review fixes passed the final Docker CI and EN/RU README completeness checks.

## Verification evidence

- `make ci` passed with the digest-pinned Go 1.26 builder; `gofmt`, `go vet`, `go test -race`, dependency-boundary
  checks, and all existing container acceptance tests passed.
- MetricShell enables Linux child-subreaper mode before workload spawn and uses one blocking `wait4` owner, so direct
  and adopted children cannot be consumed by competing waiters.
- Real-container fixtures cover double-spawn orphan adoption in both primary-before-child and child-before-primary
  orders while preserving exit codes `21` and `22` as the sole authoritative primary results. The latter runs with
  Docker `--init`, proving subreaper adoption when MetricShell is not PID 1.
- The burst fixture creates 64 adopted children, observes 64 `kind=adopted` and one `kind=direct` reap records, and
  reports zero stable zombies through `/proc` inspection.
- `workload.exited` is emitted exactly once with `forced=false`; `child.reaped` exposes only the closed
  `direct|adopted` kind and no child PID.
- Subreaper and unexpected reaper failures use sanitized `runtime.failed` / `INTERNAL_FAILURE`. Unit tests verify no
  workload starts after subreaper setup failure and that reaper, observer, and forwarding error paths reap the direct
  child before returning.
- A deterministic PID-reuse test feeds primary PID, another child, and the same numeric PID again; the original primary
  result and single primary event are preserved, while the reused PID is classified as adopted.
- The production `scratch` image built for `linux/arm64`, and its `--help` entrypoint check passed.
- `implementation/README.md` and `implementation/README_RU.md` were reviewed and updated together. Grace budgeting and
  forced descendant cleanup remain explicitly assigned to ISSUE-009.
