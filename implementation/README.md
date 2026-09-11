# MetricShell production implementation

This directory is the production Go module. Code under `research/` is evidence and is deliberately outside the module
and dependency graph.

## Current scope

Core through Wave 6 is complete and ready for operational use in the complete-snapshot profile. Publishers replace the
whole metric set at once; partial metric updates are intentionally out of scope for this release.

ISSUE-011 through ISSUE-015 provide the immutable snapshot model, strict whole-candidate parser/validator, atomic
last-valid holder, exact generation-zero state and a separate bounded self-metrics registry. ISSUE-016 through
ISSUE-022 provide one bounded ingestion core, atomic-file reconciliation, acknowledged MSP/1 Unix ingestion and its
serialized client, bounded loopback HTTP push, the explicit mmap boundary, and an exhaustive cross-adapter conformance
corpus. ISSUE-023 through ISSUE-028 provide bounded exposition, response preparation, the finalization ingestion
barrier, the immediate/duration/scrape-count final-wait state machine, complete-response drain and final-wait
observability. ISSUE-029 through ISSUE-031 add Kubernetes Job/CronJob examples, lifecycle controls and multi-replica
Prometheus verification. ISSUE-032 through ISSUE-037 add static multi-architecture release artifacts, container
hardening defaults, configurable capacity/time limits, fault/soak/race gates, controlled release benchmarks and signed
supply-chain evidence.

Complete accepted application snapshots replace one immutable generation at a time; live self-metrics use their own
fixed-cardinality state and do not affect application identity. Production runtime creates one shared `ingestion.Core`,
routes the selected `file`, `unix` or `http` ingestion transport through it, and finalizes via
`Core.CloseAndFreeze(ctx)` before terminal exposition observes the frozen snapshot.

## Requirements

- Docker with a running daemon. The build uses a digest-pinned Go 1.26 toolchain inside Docker; a host Go toolchain is
  neither required nor used by the project workflow.

## Commands

Run the same Docker-only gate used by CI. Make only supplies the repository revision and invokes Docker:

```sh
make ci
```

Run the explicit Wave 1 exit gate (currently the same complete Docker gate):

```sh
make wave1
```

Run the Wave 2 lifecycle, shutdown, escalation and probe exit gate:

```sh
make wave2
```

Run the Wave 3 transport-independent snapshot and self-metrics state-core exit gate:

```sh
make wave3
```

Run the Wave 4 common-ingestion and cross-adapter conformance exit gate:

```sh
make wave4
```

Run the Wave 5 exposition and final-wait exit gate:

```sh
make wave5
```

Run the Wave 6 Kubernetes, hardening, release, benchmark and supply-chain exit gate:

```sh
make wave6
```

Run the fault-injection gate:

```sh
make fault
```

Export controlled benchmark artifacts:

```sh
make benchmark
```

Build static multi-architecture release artifacts:

```sh
make release
```

Export signed supply-chain evidence with release signing keys supplied as BuildKit secrets:

```sh
make supply-chain \
  RELEASE_SIGNING_KEY_FILE=/path/to/private.hex \
  RELEASE_SIGNING_PUBLIC_KEY_FILE=/path/to/public.hex
```

The Docker-contained exit verifier checks the real artifact's rejected bootstrap configuration (`64` and one
`configuration.rejected` record), all workload exits `0-255`, and INT/TERM mappings. Product exit codes are observed
inside the container so Docker CLI failures cannot be confused with MetricShell results.

Build a minimal runtime image for a selected Linux architecture:

```sh
make build PLATFORM=linux/amd64 IMAGE=metricshell
```

The Makefile only invokes Docker targets; `make ci` runs build/unit checks, real-container acceptance fixtures, fault
injection, controlled benchmark artifact generation, and supply-chain verification with an ephemeral CI signing key
outside the repository. The release `supply-chain` target requires external signing-key and public-key files supplied as
BuildKit secrets. The Dockerfile has no Make dependency, and no host Go command or auxiliary shell orchestration script
is used.

Supply-chain artifacts use `SHA256SUMS` as the signed manifest. It covers the release binaries, `go.mod`, raw
`MODULES.jsonl` / `GOVULNCHECK.json` inputs, and every generated evidence file. SBOM verification compares module
components against the signed module graph, including module versions and sums.

Kubernetes examples live under `examples/kubernetes/` and cover direct Job discovery, lifecycle controls, CronJob
concurrency policy, PodMonitor integration and multi-replica Prometheus scrape validation. Docker hardening examples
live under `examples/docker/` and document non-root, read-only, no-new-privileges and tmpfs runtime defaults.

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
metricshell version=0.1.2 revision=0123456
```

## Package layout

- `client`: public official MSP/1 connection writer with complete-publication serialization and typed errors.
- `cmd/metricshell`: production executable entrypoint.
- `internal/benchrelease`: controlled release benchmark contract and artifact checks.
- `internal/buildinfo`: linker-provided build identity.
- `internal/cli`: bootstrap command surface.
- `internal/config`: validated workload, shutdown and exposition bootstrap configuration.
- `internal/conformance`: shared file/Unix/HTTP semantic, state and observability corpus.
- `internal/diagnostic`: ordered structured lifecycle diagnostics for one runtime identity.
- `internal/exposition`: immutable application/self-metric encoding, response bounds, compression, write outcomes and
  final-response drain.
- `internal/finalwait`: validated natural-completion policies, frozen-generation threshold and terminal decisions.
- `internal/hardening`: container-hardening example and runtime-default verification.
- `internal/ingestion`: transport-independent admission, cancellation, finalization barrier, result taxonomy and
  complete-candidate handoff.
- `internal/httpingest`: loopback-only POST adapter with independent wire/decoded limits and exact HTTP mapping.
- `internal/fileingest`: bounded no-follow file reconciliation and Linux directory-inotify recovery.
- `internal/kubeexamples`: Kubernetes Job, CronJob, lifecycle and Prometheus example verification.
- `internal/socketingest`: bounded MSP/1 transactions, exact ACK/NACK framing and mode-0660 Unix listener.
- `internal/lifecycle`: synchronized public runtime state and transitions.
- `internal/probe`: bounded HTTP health/readiness responses derived only from lifecycle state.
- `internal/promverify`: multi-replica Prometheus scrape and label-cardinality verification helpers.
- `internal/releaseverify`: release, Dockerfile, pinned-image and supply-chain contract checks.
- `internal/shutdown`: validated shutdown budgets, deadlines, phase contexts, and completion reasons.
- `internal/selfmetric`: bounded self-metric registry, immutable scrape views and Prometheus/OpenMetrics text encoding.
- `internal/snapshot`: immutable application snapshot model, parser, canonicalization, atomic holder, limits and rejection registry.
- `internal/supplychain`: signed release evidence, SBOM, provenance and vulnerability verification.
- `internal/workload`: owned process-group execution, signal forwarding, subreaper adoption, and child reaping.
- `internal/testfixture`: binaries used only by real-container acceptance, fault, release and supply-chain tests.
- `internal/dependencyboundary`: production/research, public-API, primitive, module and license boundary checks.
- `../VERSION`: repository-wide project version.

## Normative context

Implementation follows ISSUE-011 through ISSUE-037, EPIC-001, ADR-001–ADR-015, the Application Snapshot Protocol,
Configuration, Runtime State Machine, Self-Metrics and Structured Logging specifications, ADR-012 Kubernetes viability,
ADR-013 static multi-architecture distribution, ADR-014 security and limits, ADR-015 final benchmark policy, and the
cross-cutting definition of done through Wave 6.

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

For ISSUE-006, `make wave1` retains every earlier PID 1, argv, process-group, signal-forwarding and descendant-reaping
check. Its Docker-contained result verifier additionally runs 256 independent MetricShell lifecycles for exits `0-255`
and TERM/INT cases, proves one workload execution and one authoritative result per lifecycle, and rejects registry
collisions misclassified as MetricShell failures. Pre-result internal failures remain covered before and after workload
start; post-exit descendant work proves the resolved workload result is retained until completion.
