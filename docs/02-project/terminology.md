# Terminology

| Term            | Definition                                                                                                                                                                                                                                            |
|-----------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Runtime         | MetricShell executable acting as the container runtime wrapper.                                                                                                                                                                                       |
| Workload        | Application started by MetricShell.                                                                                                                                                                                                                   |
| Producer        | Component publishing metrics to the runtime.                                                                                                                                                                                                          |
| Snapshot        | Current exported metric state.                                                                                                                                                                                                                        |
| Final Snapshot  | Last snapshot after workload completion.                                                                                                                                                                                                              |
| Scrape          | HTTP request performed by Prometheus or another compatible collector.                                                                                                                                                                                 |
| Exit Strategy   | Policy controlling runtime behaviour after workload completion.                                                                                                                                                                                       |
| Local Ingestion | Internal communication between workload and MetricShell.                                                                                                                                                                                              |
| Operator        | A person, team, automation system, or orchestration platform responsible for building, configuring, deploying, and operating a workload together with MetricShell. The operator is not the workload itself and not necessarily a Kubernetes Operator. |

---
[Readme](README.md) | [Documentation Readme](../README.md)
