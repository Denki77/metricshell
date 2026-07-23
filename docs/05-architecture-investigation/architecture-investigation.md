# Architecture Investigation

> Status: In progress  
> Purpose: Evaluate architecture options before ADRs and implementation  
> Scope: MetricShell runtime, process lifecycle, metrics ingestion, exposition and shutdown coordination

## 1. Purpose

This document records the architecture investigation for MetricShell. It is not an architecture specification and does
not approve a solution in advance.

The investigation must identify material architecture questions, document realistic candidates, state falsifiable
hypotheses, define reproducible experiments, collect evidence, evaluate trade-offs and produce ADRs only after the
evidence is sufficient.

## 2. Research Method

Every investigation item follows the same sequence:

1. **Question** — one concrete architecture question.
2. **Context** — why it matters and which requirements it affects.
3. **Candidates** — realistic alternatives, including the current preference.
4. **Hypotheses** — expected outcomes stated before experimentation.
5. **Evidence required** — official documentation, prototype, integration test, fault injection, benchmark or
   operational comparison.
6. **Experiment** — reproducible environment, workload, inputs, measurements and repetitions.
7. **Evaluation criteria** — defined before results are known.
8. **Results** — raw observations, measurements and failures.
9. **Conclusion** — accept, reject, retain as fallback, postpone or declare insufficient evidence.
10. **Decision output** — ADR, updated requirement, benchmark result, rejected-alternative record or new research item.

A good hypothesis is falsifiable:

> Directory-level inotify with periodic reconciliation detects atomic file replacement with lower idle CPU than
> polling-only mode while recovering from lost notifications.

A bad hypothesis is subjective:

> inotify is better.

## 3. Evidence Levels

Prefer evidence in this order:

1. specification or official documentation;
2. reproducible integration test;
3. reproducible benchmark;
4. prototype observation;
5. source-code inspection;
6. team experience;
7. opinion.

Lower-level evidence is useful for forming hypotheses but should not be the sole basis for a material architectural
decision.

## 4. Agreed Design Direction

The following direction is accepted and is not reopened without new contradictory evidence.

MetricShell:

- is added to an application container image;
- starts and controls one workload execution;
- exposes a Prometheus-compatible endpoint;
- accepts application metrics locally;
- may remain available for a bounded period after workload completion;
- preserves the workload result unless MetricShell itself fails;
- does not require a central metrics gateway;
- does not push application metrics to Prometheus;
- remains independent from Kubernetes APIs;
- supports Docker, Docker Compose and Kubernetes usage;
- leaves restart policy to the container runtime or orchestrator.

Architecture investigation may refine how these capabilities are implemented.

## 5. Investigation Order

1. [Process and PID 1 model](architecture-research.md#INV-001)
2. [Workload lifecycle and exit semantics](architecture-research.md#INV-002)
3. [Shutdown time budgeting](architecture-research.md#INV-003)
4. [Metric-state ownership and semantics](architecture-research.md#INV-004)
5. [Ingestion transport comparison](architecture-research.md#INV-005)
6. [File-based ingestion](architecture-research.md#INV-006)
7. [Socket-based ingestion](architecture-research.md#INV-007)
8. [Local push ingestion](architecture-research.md#INV-008)
9. [Shared-memory and mmap feasibility](architecture-research.md#INV-009)
10. [Prometheus exposition](architecture-research.md#INV-010)
11. [Final-state and scrape-count semantics](architecture-research.md#INV-011)
12. [Kubernetes Job/CronJob viability](architecture-research.md#INV-012)
13. [Distribution models](architecture-research.md#INV-013)
14. [Security and resource limits](architecture-research.md#INV-014)
15. [Benchmarks and final comparison](architecture-research.md#INV-015)

---

## 6. Investigation Tracking

| ID      | Topic                                                       | Status      | Evidence                                    | Decision                                     |
|---------|-------------------------------------------------------------|-------------|---------------------------------------------|----------------------------------------------|
| INV-001 | [PID 1 and process model](architecture-research.md#INV-001) | Completed   | [INV-001](../../research/INV-001/README.md) | [ADR-001](../06-architecture/adr/ADR-001.md) |
| INV-002 | [Workload lifecycle](architecture-research.md#INV-002)      | Completed   | [INV-002](../../research/INV-002/README.md) | [ADR-002](../06-architecture/adr/ADR-002.md) |
| INV-003 | [Shutdown budgeting](architecture-research.md#INV-003)      | Completed   | [INV-003](../../research/INV-003/README.md) | [ADR-003](../06-architecture/adr/ADR-003.md) |
| INV-004 | [Metric-state semantics](architecture-research.md#INV-004)  | Completed   | [INV-004](../../research/INV-004/README.md) | [ADR-004](../06-architecture/adr/ADR-004.md) |
| INV-005 | [Transport comparison](architecture-research.md#INV-005)    | Completed   | [INV-005](../../research/INV-005/README.md) | [ADR-005](../06-architecture/adr/ADR-005.md) |
| INV-006 | [File ingestion](architecture-research.md#INV-006)          | In progress | —                                           | —                                            |
| INV-007 | [Socket ingestion](architecture-research.md#INV-007)        | Not started | —                                           | —                                            |
| INV-008 | [Local push](architecture-research.md#INV-008)              | Not started | —                                           | —                                            |
| INV-009 | [Shared memory/mmap](architecture-research.md#INV-009)      | Not started | —                                           | —                                            |
| INV-010 | [Exposition](architecture-research.md#INV-010)              | Not started | —                                           | —                                            |
| INV-011 | [Final scrape semantics](architecture-research.md#INV-011)  | Not started | —                                           | —                                            |
| INV-012 | [Kubernetes viability](architecture-research.md#INV-012)    | Not started | —                                           | —                                            |
| INV-013 | [Distribution](architecture-research.md#INV-013)            | Not started | —                                           | —                                            |
| INV-014 | [Security and limits](architecture-research.md#INV-014)     | Not started | —                                           | —                                            |
| INV-015 | [Benchmarks](architecture-research.md#INV-015)              | Not started | —                                           | —                                            |

## 7. Completion Criteria

Architecture investigation is complete enough to begin production implementation when:

- all high-risk questions have evidence-backed conclusions;
- selected alternatives have ADRs;
- rejected alternatives are documented;
- the behavioral model is updated;
- the state machine reflects selected lifecycle semantics;
- protocols have draft specifications;
- benchmark targets are defined;
- acceptance criteria are updated;
- no unresolved question can invalidate the core runtime model.

Implementation spikes may be created during research, but they must not be treated as production architecture before
decisions are recorded.
