# Project Scope

## In Scope

- Runtime wrapper for container workloads.
- Child process lifecycle management.
- Prometheus-compatible metrics endpoint.
- Local metrics ingestion.
- Runtime self-observability.
- Configurable shutdown strategies.
- Docker, Docker Compose and Kubernetes support.

## Out of Scope

- Metrics storage.
- Service discovery.
- Prometheus configuration.
- Alerting.
- Local or distributed aggregation of metric values across producers.
- Log collection.
- Trace collection.
- Host monitoring.
- Business metrics design.

MetricShell does not retain producer-scoped metric contributions and does not aggregate metric values. The workload and
its libraries must publish one complete, conflict-free application snapshot.

---
[Project Goals](project-goals.md)

---
[Readme](README.md) | [Documentation Readme](../README.md)
