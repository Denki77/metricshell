# ISSUE-003. Owned process group/session

**Status:** Done
**Readiness:** Code-ready

**Epic:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 1](../../02-epics/EPIC-001-core.md#wave-1)

Place the workload and descendants in an owned process group. Signals must target the workload tree without affecting
unrelated processes. Test child and grandchild processes.

## Code-ready contract

- **Normative inputs:** ADR-001 / INV-001, [Runtime State Machine](../../../04-specification/runtime-state-machine.md),
  and [Structured Logging](../../../04-specification/structured-logging.md).
- **Dependencies:** ISSUE-002.
- **Scope / out of scope:** Create and own the workload process group/session and target only that tree. Out of scope:
  unrelated container processes and Kubernetes pod-wide signaling.
- **Configuration and observable failures:** Group-creation or signaling failures emit sanitized structured errors; no
  process identifier becomes a metric label.
- **Acceptance criteria and required tests:** Child/grandchild tree; unrelated sibling process; rapid exit during setup;
  group signal delivery; race detector and zombie check.
- **Completion:** Complete when descendants are controllable as one tree and unrelated processes remain unaffected in
  every integration fixture.

## Delivery log

- 2026-08-15: moved to `In Progress`; implementation started in the dedicated `ISSUE-003` worktree.
- 2026-08-15: moved to `Testing`; Docker CI passed with the race detector and process-group acceptance fixture.
- 2026-08-15: moved to `Done`; final Docker CI, arm64 runtime-image, and EN/RU README completeness checks passed.

## Verification evidence

- `make ci` passed with the digest-pinned Go 1.26 builder and the Docker daemon; `go vet` and `go test -race` passed.
- The workload fixture observed its PID equal to its PGID. Its child and grandchild inherited that PGID.
- A sibling placed in another process group did not receive the signal sent to the owned workload group; the root,
  child, and grandchild did receive it within the bounded fixture deadline.
- Twenty-five rapid workload start/exit container runs passed. The process-tree fixture waited for its descendants and
  found no zombie process afterward.
- `workload.started` records the workload PID and PGID as structured log fields. Missing-executable startup failure
  remains sanitized as `workload.start_failed` / `WORKLOAD_START_FAILED` without exposing workload arguments.
- The production `scratch` image built for `linux/arm64`, and its `--help` entrypoint check passed.
- `implementation/README.md` and `implementation/README_RU.md` were checked and updated together; external signal
  forwarding and descendant reaping remain explicitly assigned to ISSUE-004 and ISSUE-005.
