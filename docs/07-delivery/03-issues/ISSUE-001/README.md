# ISSUE-001. Bootstrap the production Go module and command

**Status:** Done
**Readiness:** Code-ready

**Epic:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 1](../../02-epics/EPIC-001-core.md#wave-1)

Create the production package structure, `cmd/metricshell`, configuration bootstrap, version command, and build/test
targets.

**Acceptance criteria:** `metricshell --version` returns build identity; one command runs unit tests; the Linux binary
builds; research prototype packages are not imported as production shortcuts.

## Code-ready contract

- **Normative inputs:** [Configuration](../../../04-specification/configuration.md),
  [Configuration Value Grammar](../../../04-specification/configuration-value-grammar.md),
  and EPIC/ADR prerequisites.
- **Dependencies:** None; it establishes the production module and blocks every implementation issue.
- **Scope / out of scope:** Production module, command, version metadata, and build/test targets. Out of scope: runtime
  behavior and reuse of research prototype packages.
- **Configuration and observable failures:** Invalid build metadata fails the build; invalid startup configuration uses
  the configuration exit registry and structured diagnostics.
- **Acceptance criteria and required tests:** `--version`, Linux amd64/arm64 build, unit target, dependency-boundary
  check, and a test proving no production import resolves under `research/`.
- **Completion:** Complete when clean CI builds and tests the production command reproducibly without rebuilding or
  importing prototypes.

## Delivery log

- 2026-08-11: moved to `In Progress`; implementation started in the dedicated `ISSUE-001` worktree.
- 2026-08-11: moved to `Testing`; Docker verification started.
- 2026-08-11: moved to `Done`; Docker CI, static binary, runtime image, dependency-boundary, build-metadata, and README
  checks passed.
- 2026-08-11: reopened as `In Progress` to establish a repository-wide version source and correct the Make/Docker
  responsibility boundary.
- 2026-08-11: moved to `Testing`; the Docker-only Make targets and repository version source are under verification.
- 2026-08-11: moved to `Done`; the corrected responsibility boundary, repository `VERSION`, Docker CI, and native
  architecture runtime checks passed.
- 2026-08-11: reopened as `In Progress` to remove unnecessary version-validator implementation and tests.
- 2026-08-11: moved to `Testing`; simplified version-file checks and observable version output are under Docker
  verification.
- 2026-08-11: moved to `Done`; Docker CI and runtime verification passed without a custom version validator or its
  internal tests.
- 2026-08-12: reopened as `In Progress` to upgrade the production toolchain from Go 1.23 to Go 1.26.
- 2026-08-12: moved to `Testing`; the pinned Go 1.26 Docker toolchain is under CI and runtime verification.
- 2026-08-12: moved to `Done`; Docker CI and arm64 runtime verification passed with Go 1.26.5.

## Verification evidence

- `make ci` invoked the Docker `test` target; the Dockerfile directly ran the Go checks in the pinned Go 1.26 Alpine
  build layer.
- The Makefile contains Docker commands only; the Dockerfile has no Make or auxiliary CI-script dependency.
- The scratch runtime image returned the expected `--version` identity.
- Produced amd64 and arm64 binaries were verified as stripped, statically linked Linux ELF executables.
- English and Russian project and implementation READMEs passed the completeness audit.
- The repository-root `VERSION` supplied `0.1.0-dev`; no build timestamp was embedded.
- The final arm64 image reported `linux/arm64` and returned the expected version and revision at runtime.
- Version handling now tests only the observable binary output against the single non-empty line in `VERSION`.
- The production module declares Go 1.26; the pinned multi-platform Docker image resolved to Go 1.26.5.
