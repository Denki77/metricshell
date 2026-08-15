# MetricShell production implementation

This directory is the production Go module. Code under `research/` is evidence and is deliberately outside the module
and dependency graph.

## Current scope

ISSUE-005 enables Linux child-subreaper mode before starting the workload and uses one `wait4` owner for the primary and
all adopted descendants. The primary PID and process-compatible result remain authoritative and are emitted exactly
once; every reaped process is classified as `direct` or `adopted` without exposing child PIDs. MetricShell waits until
all adopted children are reaped. Forced shutdown budgets and post-exit lifecycle behavior remain assigned to later
issues.

## Requirements

- Docker with a running daemon. The build uses a digest-pinned Go 1.26 toolchain inside Docker; a host Go toolchain is
  neither required nor used by the project workflow.

## Commands

Run the same Docker-only gate used by CI. Make only supplies the repository revision and invokes Docker:

```sh
make ci
```

Build a minimal runtime image for a selected Linux architecture:

```sh
make build PLATFORM=linux/amd64 IMAGE=metricshell
```

The Makefile only invokes Docker targets; `make ci` runs build/unit checks and real-container acceptance fixtures. The
Dockerfile has no Make dependency, and no host Go command or auxiliary shell orchestration script is used.

Run a workload without shell interpretation:

```sh
docker run --rm metricshell:local -- /path/to/workload "argument with spaces"
```

Every token after the first standalone `--` is passed directly as workload argv. Shell behavior requires an explicit
shell workload such as `-- /bin/sh -c 'command'`.

## Build identity

The repository-root [`VERSION`](../VERSION) file is the single version source for the whole project. The build requires
it to contain exactly one non-empty line. `REVISION` is the source commit supplied by the Makefile. No build timestamp
is embedded because it is not required by the architecture and would weaken reproducibility. Given the same source,
toolchain, architecture, and identity, build flags remove local paths, VCS probing, and random build IDs.

```text
metricshell version=0.1.0-dev revision=0123456
```

## Package layout

- `cmd/metricshell`: production executable entrypoint.
- `internal/buildinfo`: linker-provided build identity.
- `internal/cli`: bootstrap command surface.
- `internal/config`: configuration failure registry bootstrap.
- `internal/diagnostic`: ordered structured lifecycle diagnostics for one runtime identity.
- `internal/workload`: owned process-group execution, signal forwarding, subreaper adoption, and child reaping.
- `internal/testfixture`: binaries used only by real-container acceptance tests.
- `internal/dependencyboundary`: automated production/research isolation test.
- `../VERSION`: repository-wide project version.

## Normative context

Implementation follows ISSUE-005, EPIC-001, ADR-001, ADR-003, the Runtime State Machine, Self-Metrics and Structured
Logging specifications, ADR-013 static multi-architecture distribution, and the cross-cutting definition of done.
Child reaping does not add grace-budget or forced-kill policy from ISSUE-009.

## Engineering contract for subsequent issues

- Accepted requirements, specifications, and ADRs define observable behavior; delivery issues define implementation
  scope and required acceptance tests. A narrower issue must not implement later runtime behavior implicitly.
- Production code lives in this module. Research prototypes are evidence only and must never become production imports
  or copied architectural shortcuts.
- Core accepts one complete candidate snapshot, validates it as a whole, and atomically replaces the last valid state.
  It does not aggregate, merge producer state, replay operations, or retain snapshot history.
- Resources, queues, payloads, concurrency, and waits must be explicitly bounded. Failures use the normative exit-code
  and structured-diagnostic registries without exposing workload arguments, environment values, or payload data.
- Docker is the only build and test environment; Go runs inside its pinned build layer while Make remains an outer
  Docker-command interface. Production Linux artifacts are CGO-free and cover amd64 and arm64 when applicable.
- Every issue ends with maintainable automated tests. Concurrency work also passes the race detector, and relevant
  container behavior is verified against the running Docker daemon.
- English and Russian documentation change together. Each issue moves through `In Progress`, `Testing`, and `Done`,
  records verification evidence, and finishes with a completeness audit of the affected READMEs.

For ISSUE-005, `make ci` additionally verifies orphan adoption through a double spawn, including an external Docker init
above MetricShell, both primary-before-child and child-before-primary completion orders, primary-PID reuse, exactly one
primary result, `direct|adopted` reaping diagnostics, a 64-child exit burst with zero stable zombies, sanitized
subreaper/reaper failures, guaranteed cleanup of the direct child, and race-detector execution. Existing
signal-forwarding acceptance coverage remains active.
