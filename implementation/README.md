# MetricShell production implementation

This directory is the production Go module. Code under `research/` is evidence and is deliberately outside the module
and dependency graph.

## Current scope

ISSUE-003 starts each workload as the leader of a dedicated Linux process group inherited by its descendants. The
workload PID and PGID are recorded in the structured `workload.started` diagnostic. Forwarding external signals,
subreaper-based descendant reaping, and post-exit lifecycle behavior remain assigned to subsequent issues.

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
- `internal/diagnostic`: structured startup diagnostics.
- `internal/workload`: direct-child execution in an owned process group and immediate result mapping.
- `internal/testfixture`: binaries used only by real-container acceptance tests.
- `internal/dependencyboundary`: automated production/research isolation test.
- `../VERSION`: repository-wide project version.

## Normative context

Implementation follows ISSUE-003, EPIC-001, ADR-001, the Runtime State Machine and Structured Logging specifications,
ADR-013 static multi-architecture distribution, and the cross-cutting definition of done. The process-group boundary
does not add the external signal-forwarding policy assigned to ISSUE-004 or descendant reaping assigned to ISSUE-005.

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

For ISSUE-003, `make ci` additionally verifies a child/grandchild tree in the owned group, isolation of another process
group, group-signal delivery, rapid workload exits, and absence of zombies after the fixture has waited for its children.
