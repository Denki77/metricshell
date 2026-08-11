# MetricShell production implementation

This directory is the production Go module. Code under `research/` is evidence and is deliberately outside the module
and dependency graph.

## Current scope

ISSUE-001 provides the production package layout, `cmd/metricshell`, build identity, configuration-error
bootstrap, tests, static Linux builds, and Docker-based CI. Workload execution and PID 1 behavior begin in ISSUE-002.

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

The Makefile only invokes Docker targets; the Dockerfile directly runs formatting, vet, tests, dependency-boundary
checks, version smoke coverage, and static Linux builds for amd64 and arm64. The Dockerfile has no Make
dependency, and no host Go command or auxiliary shell orchestration script is used.

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
- `internal/dependencyboundary`: automated production/research isolation test.
- `../VERSION`: repository-wide project version.

## Normative context

Implementation follows ISSUE-001, EPIC-001, the accepted Configuration and Configuration Value Grammar specifications,
ADR-013 static multi-architecture distribution, and the cross-cutting definition of done. The complete-snapshot model
remains the Core invariant; ISSUE-001 does not introduce transport, aggregation, runtime lifecycle, or workload behavior.

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
