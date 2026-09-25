# ISSUE-036. Controlled release benchmark suite

**Status:** Done
**Readiness:** Code-ready

**Epic:** [EPIC-001 Core](../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 6](../02-epics/EPIC-001-core.md#wave-6)

30+ repetitions, pinned resources, production binary/adapters, correctness separate from SLO thresholds.

## Code-ready contract

- **Normative inputs:** ADR-015 /
  INV-015, [Runtime Defaults](../../04-specification/runtime-defaults-and-resource-limits.md),
  and [Docker and Compose Examples](../../04-specification/docker-compose-examples.md).
- **Dependencies:** ISSUE-032 and ISSUE-035.
- **Scope / out of scope:** Benchmark the production binary and adapters in controlled pinned environments for at least
  30 repetitions, separating correctness from release thresholds. Out of scope: rebuilding or treating research
  prototypes as release artifacts.
- **Configuration and observable failures:** Environment drift, insufficient repetitions, correctness failure, or
  unstable variance invalidates the run with machine-readable metadata.
- **Acceptance criteria and required tests:** File/socket/HTTP; idle and load; architecture matrix; pinned CPU/memory;
  warmup; 30+ samples; raw results, summary statistics, correctness gate.
- **Completion:** Complete when an independent runner can reproduce the suite and compare distributions without
  rebuilding any prototype.

## Delivery log

- 2026-09-09: moved to `In Progress`; added a controlled release benchmark contract for file, Unix socket and HTTP
  adapters across idle/load profiles, 30 repetitions, warmups, architecture matrix and pinned resource metadata.
- 2026-09-09: moved to `Testing`; added Docker `benchmark-artifacts` stage and Makefile target `benchmark`, building
  the production binary before running the benchmark suite and exporting raw results plus machine-readable metadata.
- 2026-09-09: moved to `Done`; correctness assertions remain inside the benchmark driver and are separate from any SLO
  threshold interpretation.

## Verification evidence

- `go test ./internal/benchrelease`
- `make benchmark IMAGE=metricshell-issue036-benchmark`
- `make test IMAGE=metricshell-issue036-benchmark`
