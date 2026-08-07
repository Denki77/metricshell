# MetricShell: Complete Implementation Plan

**Status:** Draft for implementation planning  
**Date:** 2026-08-04

## Purpose

This document is the complete implementation entry point for MetricShell. It links the full architecture starting with
PID 1 and workload supervision .

Normative chain:

```text
epic -> wave -> task -> acceptance test
```

## Architecture decisions in dependency order

| ADR     | Topic                   | Mandatory implementation outcome                                                               |
|---------|-------------------------|------------------------------------------------------------------------------------------------|
| ADR-001 | PID 1 and process model | MetricShell is PID 1, owns the workload process group, forwards signals, and reaps descendants |
| ADR-002 | Workload lifecycle      | Formal state machine for startup, execution, workload exit, and post-exit processing           |
| ADR-003 | Shutdown budgeting      | One bounded shutdown budget with grace periods and forced escalation                           |
| ADR-004 | Metric-state semantics  | Last valid complete snapshot, whole-candidate validation, atomic replacement, no aggregation   |
| ADR-005 | Transport comparison    | Shared transport-independent publication contract and error model                              |
| ADR-006 | File ingestion          | Atomic file publication with event notification and reconciliation fallback                    |
| ADR-007 | Socket ingestion        | Framed local socket protocol with bounded requests and concurrent complete candidates          |
| ADR-008 | Local push              | Optional local HTTP/push adapter with identical snapshot semantics                             |
| ADR-009 | Shared memory/mmap      | mmap is not the primary transport and the core contract has no ABI dependency                  |
| ADR-010 | Prometheus exposition   | One immutable complete snapshot per scrape with text/OpenMetrics negotiation                   |
| ADR-011 | Final scrape semantics  | Freeze ingestion, default N=1, complete-response counting, finite timeout                      |
| ADR-012 | Kubernetes viability    | Bounded post-workload Pod lifetime, discovery, replica-specific validation                     |
| ADR-013 | Distribution            | Static amd64/arm64 artifacts, checksums, pinned multi-stage integration                        |
| ADR-014 | Security and limits     | Non-root/local-only defaults and whole-candidate resource bounds                               |
| ADR-015 | Final benchmark policy  | Bounded architecture, event-driven reconciliation, separate release performance certification  |

## Normative ADR-004 scope

One successful publication represents one complete snapshot for one workload/application scope.

```text
complete candidate
-> complete validation
-> atomic replacement of last-valid snapshot
-> repeated safe scrapes
```

Until a separate aggregation investigation is accepted, MetricShell does not perform registry merging, snapshot
summation, per-producer state, operation replay, snapshot history, guaranteed delivery of every version, or
reconciliation of multiple producer timelines.

## Target production flow

```text
MetricShell PID 1
  |
  +-- start workload in owned process group
  |      |
  |      +-- client library / file / socket / local push
  |              |
  |              v
  |        complete candidate snapshot
  |              |
  |              v
  |        whole-candidate validation
  |              |
  |              v
  |        atomic active-state replacement
  |
  +-- Prometheus exposition of one immutable snapshot
  |
  +-- workload exits
         |
         +-- close ingestion
         +-- freeze last-valid application snapshot
         +-- bounded final scrape wait
         +-- drain, terminate descendants, return workload result
```

## Delivery waves

### Wave 1

Supervisor foundation and PID 1

**Goal:** deliver a minimally correct runtime that safely starts and owns a workload as PID 1.

- [ISSUE-001. Bootstrap the production Go module and command](../03-issues/ISSUE-001/README.md)
- [ISSUE-002. PID 1 entrypoint and workload command parsing](../03-issues/ISSUE-002/README.md)
- [ISSUE-003. Owned process group/session](../03-issues/ISSUE-003/README.md)
- [ISSUE-004. Signal forwarding](../03-issues/ISSUE-004/README.md)
- [ISSUE-005. Child reaping and orphan handling](../03-issues/ISSUE-005/README.md)
- [ISSUE-006. Workload result preservation](../03-issues/ISSUE-006/README.md)

**Wave 1 exit gate:** MetricShell can replace a basic container entrypoint and correctly perform PID 1 duties even with
metrics ingestion disabled.

### Wave 2

Lifecycle and shutdown coordination

- [ISSUE-007. Runtime lifecycle state machine](../03-issues/ISSUE-007/README.md)
- [ISSUE-008. Shutdown budget model](../03-issues/ISSUE-008/README.md)
- [ISSUE-009. Termination escalation](../03-issues/ISSUE-009/README.md)
- [ISSUE-010. Health and readiness contract](../03-issues/ISSUE-010/README.md)

**Wave 2 exit gate:** lifecycle and shutdown behavior are deterministic and bounded before transport work begins.

### Wave 3

Metric state core

- [ISSUE-011. Canonical publication model](../03-issues/ISSUE-011/README.md)
- [ISSUE-012. Whole-candidate parser and validator](../03-issues/ISSUE-012/README.md)
- [ISSUE-013. Atomic last-valid state holder](../03-issues/ISSUE-013/README.md)
- [ISSUE-014. Initial zero-series state](../03-issues/ISSUE-014/README.md)
- [ISSUE-015. Separate self-metrics domain](../03-issues/ISSUE-015/README.md)

**Wave 3 exit gate:** ADR-004 is implemented independently of transport.

### Wave 4

Transport contract and adapters

- [ISSUE-016. Common transport-independent ingestion interface](../03-issues/ISSUE-016/README.md)
- [ISSUE-017. File publication protocol](../03-issues/ISSUE-017/README.md)
- [ISSUE-018. Unix socket framed protocol](../03-issues/ISSUE-018/README.md)
- [ISSUE-019. Official client writer serialization](../03-issues/ISSUE-019/README.md)
- [ISSUE-020. Local push HTTP adapter](../03-issues/ISSUE-020/README.md)
- [ISSUE-021. Enforce mmap as non-primary](../03-issues/ISSUE-021/README.md)
- [ISSUE-022. Cross-adapter conformance suite](../03-issues/ISSUE-022/README.md)

**Wave 4 exit gate:** all three promised integration methods use the same state core and semantics.

### Wave 5

Exposition and final metrics

- [ISSUE-023. Prometheus exposition server](../03-issues/ISSUE-023/README.md)
- [ISSUE-024. Response pre-encoding and bounds](../03-issues/ISSUE-024/README.md)
- [ISSUE-025. Finalization ingestion barrier](../03-issues/ISSUE-025/README.md)
- [ISSUE-026. Final scrape state machine](../03-issues/ISSUE-026/README.md)
- [ISSUE-027. Complete-response counting and drain](../03-issues/ISSUE-027/README.md)
- [ISSUE-028. Final-wait observability](../03-issues/ISSUE-028/README.md)

**Wave 5 exit gate:** MetricShell exposes a frozen final snapshot after workload exit and terminates within budget.

### Wave 6

Kubernetes, distribution, hardening, and release evidence

- [ISSUE-029. Kubernetes Job integration](../03-issues/ISSUE-029/README.md)
- [ISSUE-030. Kubernetes lifecycle controls](../03-issues/ISSUE-030/README.md)
- [ISSUE-031. Multi-replica Prometheus integration test](../03-issues/ISSUE-031/README.md)
- [ISSUE-032. Static multi-architecture release artifacts](../03-issues/ISSUE-032/README.md)
- [ISSUE-033. Container hardening defaults](../03-issues/ISSUE-033/README.md)
- [ISSUE-034. Configurable capacity and timeout limits](../03-issues/ISSUE-034/README.md)
- [ISSUE-035. Fault, soak, and race suite](../03-issues/ISSUE-035/README.md)
- [ISSUE-036. Controlled release benchmark suite](../03-issues/ISSUE-036/README.md)
- [ISSUE-037. Release supply-chain pipeline](../03-issues/ISSUE-037/README.md)

**Wave 6 exit gate:** the release candidate is ready for a production pilot with auditable operational bounds.

## Specification-to-issue traceability

| Accepted specification or contract                                                                                   | Delivery issues                                                            |
|----------------------------------------------------------------------------------------------------------------------|----------------------------------------------------------------------------|
| [Runtime State Machine](../../04-specification/runtime-state-machine.md)                                             | ISSUE-007…ISSUE-010, ISSUE-025…ISSUE-028                                   |
| [Application Snapshot Protocol](../../04-specification/application-snapshot-protocol.md)                             | ISSUE-011…ISSUE-014, ISSUE-016…ISSUE-022                                   |
| [Configuration](../../04-specification/configuration.md)                                                             | ISSUE-001, ISSUE-008, ISSUE-010, ISSUE-016…ISSUE-020, ISSUE-023, ISSUE-034 |
| [Metric Filtering](../../04-specification/metrics-filtering.md)                                                      | ISSUE-023, ISSUE-024                                                       |
| [Self-Metrics](../../04-specification/self-metrics.md)                                                               | ISSUE-015, ISSUE-023, ISSUE-028                                            |
| [Structured Logging](../../04-specification/structured-logging.md)                                                   | ISSUE-028, ISSUE-034, ISSUE-035                                            |
| [Runtime Defaults and Resource Limits](../../04-specification/runtime-defaults-and-resource-limits.md)               | ISSUE-008, ISSUE-017…ISSUE-020, ISSUE-023, ISSUE-026, ISSUE-034            |
| [Docker and Compose Examples](../../04-specification/docker-compose-examples.md)                                     | ISSUE-029, ISSUE-030, ISSUE-032, ISSUE-035, ISSUE-037                      |
| Internal exit-code registry in [Configuration](../../04-specification/configuration.md#metricshell-owned-exit-codes) | ISSUE-002, ISSUE-006, ISSUE-034                                            |
| [Configuration value grammar](../../04-specification/configuration-value-grammar.md)                                 | ISSUE-001, ISSUE-008, ISSUE-034                                            |

## Requirement-to-delivery traceability

| Capability                  | INV     | ADR     | Tasks                           |
|-----------------------------|---------|---------|---------------------------------|
| PID 1/process ownership     | INV-001 | ADR-001 | ISSUE-002...ISSUE-005           |
| Workload lifecycle/result   | INV-002 | ADR-002 | ISSUE-006, ISSUE-007, ISSUE-010 |
| Shutdown budget/escalation  | INV-003 | ADR-003 | ISSUE-004, ISSUE-008, ISSUE-009 |
| Complete snapshot semantics | INV-004 | ADR-004 | ISSUE-011...ISSUE-015           |
| Common transport contract   | INV-005 | ADR-005 | ISSUE-016, ISSUE-022            |
| File ingestion              | INV-006 | ADR-006 | ISSUE-017, ISSUE-022            |
| Socket ingestion            | INV-007 | ADR-007 | ISSUE-018, ISSUE-019, ISSUE-022 |
| Local push                  | INV-008 | ADR-008 | ISSUE-020, ISSUE-022            |
| mmap/shared memory          | INV-009 | ADR-009 | ISSUE-021                       |
| Exposition                  | INV-010 | ADR-010 | ISSUE-023, ISSUE-024            |
| Final scrape                | INV-011 | ADR-011 | ISSUE-025...ISSUE-028           |
| Kubernetes                  | INV-012 | ADR-012 | ISSUE-029...ISSUE-031           |
| Distribution                | INV-013 | ADR-013 | ISSUE-032, ISSUE-037            |
| Security/limits             | INV-014 | ADR-014 | ISSUE-033...ISSUE-035           |
| Final benchmarks            | INV-015 | ADR-015 | ISSUE-036                       |

## Cross-cutting Definition of Done

Every production task must:

- avoid copying prototype architecture as a production shortcut;
- have tests at the appropriate level;
- pass the race detector where concurrency is involved;
- preserve ADR-004 semantics;
- use bounded resources and timeouts;
- add bounded diagnostics/self-metrics;
- update English and Russian documentation synchronously;
- link requirement -> INV -> ADR -> specification -> task -> test;
- not introduce aggregation without a new requirement, investigation, and ADR.
