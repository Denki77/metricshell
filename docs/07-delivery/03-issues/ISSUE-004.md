# ISSUE-004. Signal forwarding

**Status:** Done
**Readiness:** Code-ready

**Epic:** [EPIC-001 Core](../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 1](../02-epics/EPIC-001-core.md#wave-1)

Forward SIGTERM, SIGINT, SIGHUP, and selected operational signals. Define repeated-signal behavior and cover
startup/post-exit races.

## Code-ready contract

- **Normative inputs:** ADR-001, ADR-003 / INV-001,
  INV-003, [Runtime State Machine](../../04-specification/runtime-state-machine.md),
  and [Structured Logging](../../04-specification/structured-logging.md).
- **Dependencies:** ISSUE-002 and ISSUE-003.
- **Scope / out of scope:** Forward TERM, INT, HUP and documented operational signals with deterministic repeated-signal
  behavior. Out of scope: inventing workload-specific signal policy.
- **Configuration and observable failures:** Forwarding and ignored late signals are observable; an unsupported signal
  or failed target never panics and uses a bounded error path.
- **Acceptance criteria and required tests:** TERM/INT/HUP; repeated signals; signal before exec, during exit, and after
  reap; process-group disappearance; race detector.
- **Completion:** Complete when every supported signal has a deterministic state-dependent outcome and integration
  coverage.

## Delivery log

- 2026-08-15: moved to `In Progress`; implementation started in the dedicated `ISSUE-004` worktree.
- 2026-08-15: moved to `Testing`; Docker CI passed with race and signal-forwarding acceptance coverage.
- 2026-08-15: moved to `Done`; final Docker CI, arm64 runtime-image, and EN/RU README completeness checks passed.
- 2026-08-15: returned to `In Progress` after review found pre-start and error-path lifecycle gaps.
- 2026-08-15: moved back to `Testing`; deterministic pre-start termination, guaranteed error-path Wait, and normative
  ignored-signal logging are implemented.
- 2026-08-15: moved back to `Done`; review fixes passed final Docker CI, arm64 runtime-image, and README/specification
  completeness checks.

## Verification evidence

- `make ci` passed with the digest-pinned Go 1.26 builder and the Docker daemon; `gofmt`, `go vet`, and
  `go test -race` passed.
- Docker delivered TERM and INT to MetricShell as container PID 1; the workload observed each forwarded group signal.
- The signal fixture observed TERM, INT, HUP, QUIT, and a repeated TERM. Every delivered signal produced an ordered
  `workload.signal_forwarded` record with the workload PGID.
- One runtime ID and monotonically increasing sequence are retained across lifecycle records. After TERM/INT, later
  control-signal records remain in `stopping` rather than returning to `running`.
- Twenty-five workload-exit/forwarding races completed without hangs or panics. Disappeared process groups,
  unsupported signals, and queued post-wait signals use bounded ignored paths.
- The existing immediate Docker startup-signal race remains covered, and startup failure classification is preserved.
- A queued pre-start TERM returns `143` without invoking the workload start observer and without producing
  `workload.start_failed`; the test uses a non-existent command to prove no exec attempt occurred.
- Observer and forwarding failures both terminate and `Wait` for a real direct-child helper. `Wait4` returns `ECHILD`
  after `Run`, and an ineffective group-kill test exercises the bounded direct-process fallback.
- `workload.signal_ignored` and its closed `target_exited|unsupported_signal` reasons are now normative in both Structured
  Logging specification languages.
- The production `scratch` image built for `linux/arm64`, and its `--help` entrypoint check passed.
- `implementation/README.md` and `implementation/README_RU.md` were checked and updated together. Descendant reaping
  remains ISSUE-005; grace budgeting and forced cleanup remain ISSUE-009.
