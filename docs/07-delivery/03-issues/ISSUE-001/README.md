# ISSUE-001. Bootstrap the production Go module and command

**Status:** Open
**Readiness:** Code-ready

**Epic:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 1](../../02-epics/EPIC-001-core.md#wave-1)

Create the production package structure, `cmd/metricshell`, configuration bootstrap, version command, and build/test
targets.

**Acceptance criteria:** `metricshell --version` returns build identity; one command runs unit tests; the Linux binary
builds; research prototype packages are not imported as production shortcuts.

## Code-ready contract

- **Normative inputs:
  ** [Configuration](../../../04-specification/configuration.md), [Configuration Value Grammar](../../../04-specification/configuration-value-grammar.md),
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
