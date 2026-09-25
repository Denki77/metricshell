# ISSUE-006. Workload result preservation

**Status:** Done
**Readiness:** Code-ready

**Epic:** [EPIC-001 Core](../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 1](../02-epics/EPIC-001-core.md#wave-1)

Preserve exit 0, non-zero exit, and signal termination distinctly. Final-scrape timeout must not silently replace the
workload result. Supervisor failures use a separate exit-code range.

## Code-ready contract

- **Normative inputs:** ADR-001, ADR-002 / INV-001,
  INV-002, [Configuration](../../04-specification/configuration.md),
  and [Runtime State Machine](../../04-specification/runtime-state-machine.md).
- **Dependencies:** ISSUE-002 and ISSUE-005.
- **Scope / out of scope:** Preserve exit 0, non-zero and signal outcomes through post-exit work. Out of scope:
  remapping a started workload result to a MetricShell-owned code.
- **Configuration and observable failures:** Pre-start failures use the closed MetricShell exit registry; post-start
  diagnostics record origin without replacing the obtained workload result.
- **Acceptance criteria and required tests:** All byte-sized exits including registry collisions; TERM/INT mapping;
  final-wait timeout; internal error before/after workload start.
- **Completion:** Complete when the exit propagation matrix is table-driven and passes container integration tests.

## Delivery log

- 2026-08-24: moved to `In Progress`; implementation started in the dedicated `ISSUE-006` worktree, based on merged
  ISSUE-005.
- 2026-08-24: moved to `Testing`; the table-driven unit matrix and Docker-contained `0-255` plus TERM/INT result matrix
  passed together with all prior Wave 1 integration checks.
- 2026-08-24: moved to `Done`; the explicit Wave 1 Docker gate, arm64 production image, and EN/RU README completeness
  checks passed.

## Verification evidence

- `make wave1` passed against Docker Desktop/LinuxKit aarch64 with the digest-pinned Go 1.26 builder. It includes
  `gofmt`, `go vet`, `go test -race`, dependency-boundary checks, both static Linux architectures, and all real-container
  acceptance fixtures accumulated by ISSUE-001 through ISSUE-006.
- The table-driven unit matrix resolves all exits `0-255`, `SIGINT -> 130`, and `SIGTERM -> 143` as started-workload
  results.
- One Docker-contained verifier ran 256 independent MetricShell/workload lifecycles plus INT and TERM. Every lifecycle
  executed the workload exactly once, emitted exactly one `workload.started` and one matching `workload.exited`, and
  returned the expected result.
- The same verifier executes the real artifact with rejected bootstrap configuration and requires exit `64`, empty
  stdout, and exactly one `configuration.rejected`/`CONFIG_INVALID` record. This keeps the assertion inside Docker and
  prevents a Docker CLI exit from being mistaken for a MetricShell result.
- Workload exits numerically equal to the MetricShell registry (`64`, `70`, `71`, `72`, and `73`) remained workload
  results and produced neither `runtime.failed` nor `workload.start_failed`; the lifecycle records preserve origin.
- Existing unit failure injection distinguishes an internal failure before workload start from one after start but
  before primary resolution. Both use the MetricShell-owned `70` path and retain the correct `Started` fact.
- The primary result remains stored separately while post-exit adopted children are reaped. Normal post-exit completion,
  including the normative future final-wait timeout, has no path that replaces it. ISSUE-026 remains responsible for
  implementing and timing the actual final-wait modes.
- The Wave 1 audit confirmed ISSUE-001 through ISSUE-006 are `Done` in both languages and that MetricShell can act as a
  basic PID 1 entrypoint with metrics ingestion disabled: argv preservation, process-group ownership, signal forwarding,
  external-init subreaping, zero-zombie burst handling, and result preservation all pass in Docker.
- The production `scratch` image built for `linux/arm64`, and its `--help` entrypoint check passed.
- `implementation/README.md` and `implementation/README_RU.md` were reviewed and updated together.
