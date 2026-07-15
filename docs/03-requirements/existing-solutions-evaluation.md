# Existing Solutions: Evaluation Criteria and Official Sources

> Status: Research baseline for architecture investigation

## Purpose

This document defines how MetricShell should be compared with existing approaches.

The objective is not to prove universal superiority. The objective is to identify whether a reusable in-container
runtime wrapper offers a coherent combination of trade-offs that existing approaches do not provide together.

## Official evidence

Prometheus explicitly states that Pushgateway is recommended only for limited cases, mainly service-level batch jobs,
and documents lifecycle, stale-series, bottleneck, single-point-of-failure, and loss-of-`up` concerns when it is used as
a general replacement for normal pull scraping.

- Prometheus — When to use the Pushgateway  
  [https://prometheus.io/docs/practices/pushing/](https://prometheus.io/docs/practices/pushing/)

Prometheus exporter guidance states that for service-level batch metrics, Pushgateway is appropriate, while for
instance-level batch metrics “there is no clear pattern yet”; listed options include the Node Exporter textfile
collector, in-memory state, or implementing similar functionality.

- Prometheus — Writing exporters  
  [https://prometheus.io/docs/instrumenting/writing_exporters/](https://prometheus.io/docs/instrumenting/writing_exporters/)

The same exporter guidance recommends that an exporter normally monitor one application instance and run beside it. This
supports investigating a per-workload local exporter/runtime model, but does not prove the MetricShell design by itself.

Docker officially supports wrapper scripts and process managers when a container needs multiple related processes, and
notes that the main process is responsible for managing its children.

- Docker — Run multiple processes in a container  
  [https://docs.docker.com/engine/containers/multi-service_container/](https://docs.docker.com/engine/containers/multi-service_container/)

Kubernetes provides native sidecar lifecycle support, including Jobs. This is a valid alternative, but it remains a
Kubernetes-specific multi-container deployment model with separate lifecycle and resource accounting.

- Kubernetes — Sidecar Containers  
  [https://kubernetes.io/docs/concepts/workloads/pods/sidecar-containers/](https://kubernetes.io/docs/concepts/workloads/pods/sidecar-containers/)

## Evidence rules

Every comparison MUST distinguish:

- fact from official documentation;
- prototype measurement;
- project assumption;
- project inference.

No approach should be called “unnecessary”, “complex”, “better”, or “worse” without naming the workload and evaluation
criterion.

## Comparison criteria

### C-01 — Prometheus pull semantics

- Is the endpoint associated with one workload instance?
- Does target disappearance naturally end its scrape lifecycle?
- Is standard `up` behavior preserved?

### C-02 — Lifecycle coupling

- Are metrics tied to the workload/container instance?
- Can stale series survive after it disappears?
- Who performs cleanup?

### C-03 — Finite workload final-state availability

- Can final metrics remain scrapeable after process exit?
- Is availability bounded?
- Can completion be coordinated with one or more scrapes?

### C-04 — Mandatory infrastructure

- Is a gateway, collector, node agent, controller, or datastore required?
- Is a new shared failure domain introduced?

### C-05 — Deployment topology

- Is another container, shared volume, Pod mutation, node daemon, or service required?
- Can adoption happen entirely during image construction?

### C-06 — Environment portability

- Does the same artifact work in Docker, Compose, Kubernetes, CI, and local environments?
- Are Kubernetes APIs or Pod lifecycle features required?

### C-07 — Application integration

- Must the application host HTTP?
- Must it know a central service address?
- Can simple file or local IPC integration be used?

### C-08 — Process correctness

- Who owns PID 1 behavior, signals, process groups, child reaping, exit codes, and deadlines?
- Can telemetry handling alter workload outcome?

### C-09 — Isolation and upgrades

- Can the metrics component be upgraded independently?
- What process, filesystem, network, and resource isolation exists?
- What coupling is accepted?

### C-10 — Operational burden

- What must platform teams deploy, secure, discover, monitor, upgrade, and clean up?
- Is configuration per app, node, cluster, or globally shared?

### C-11 — Metric-state ownership

- Who owns counters, gauges, histograms, aggregation, expiry, and resets?
- Does state survive restarts?
- Is persistence beneficial or a stale-data risk?

### C-12 — Failure behavior

- What happens when metrics handling fails?
- Can workload processing continue?
- Is loss observable?
- Is there a central bottleneck or single point of failure?

### C-13 — Resource and cardinality controls

- Are series, labels, payloads, connections, queues, and memory bounded?
- Are rejections observable?

### C-14 — Final scrape confirmation

- Can the solution know a final response was served?
- Can it distinguish Prometheus from probes or manual requests?
- Does it claim response delivery only, or durable storage?

### C-15 — Standards compatibility

- Which Prometheus/OpenMetrics formats are supported?
- Are metric types and labels preserved?
- Can existing discovery and scrape configuration be reused?

## Approaches to compare

1. Direct application instrumentation with embedded HTTP.
2. Prometheus Pushgateway.
3. Node Exporter textfile collector.
4. Separate exporter process in the same container.
5. Kubernetes sidecar exporter.
6. OpenTelemetry SDK plus Collector.
7. StatsD-style local exporter.
8. MetricShell runtime-wrapper model.

## Initial research hypothesis

| Criterion                     | Embedded endpoint     | Pushgateway             | Textfile collector             | Kubernetes sidecar   | MetricShell target     |
|-------------------------------|-----------------------|-------------------------|--------------------------------|----------------------|------------------------|
| Per-instance pull             | Strong                | Indirect                | Via node target                | Strong               | Strong                 |
| Final finite-job availability | Only while app serves | Strong                  | File persists                  | Depends on design    | Explicit bounded modes |
| Shared component required     | No                    | Yes                     | Node exporter                  | No central component | No                     |
| Plain Docker portability      | Strong                | Requires gateway access | Requires collector arrangement | Weak as K8s pattern  | Strong                 |
| Application hosts HTTP        | Yes                   | No                      | No                             | No                   | No                     |
| Image-only adoption           | Requires app change   | No                      | Usually no                     | No                   | Target capability      |
| Independent exporter upgrade  | Coupled               | Central                 | Node-level                     | Strong               | Coupled by image       |
| Instance cleanup              | Natural               | Must be managed         | File ownership required        | Natural with Pod     | Natural with container |
| Process management included   | App-specific          | No                      | No                             | No                   | Target capability      |

This table is a hypothesis, not a conclusion. Architecture investigation and prototypes MUST validate it.

## Necessity test

MetricShell is justified only if the investigation demonstrates:

1. Existing approaches do not provide the target combination of:

    - per-instance pull endpoint;
    - no mandatory central component;
    - image-level integration;
    - orchestrator independence;
    - managed workload lifecycle;
    - bounded final-state availability.

2. The combined runtime is simpler for at least one well-defined workload class than the nearest alternative.
3. Process-wrapper risks can be contained and tested.
4. Runtime overhead is acceptable.
5. Socket, file, and local push can share coherent semantics without an unmaintainable protocol surface.
6. The project clearly documents scenarios where another solution is preferable.

If these conditions are not demonstrated, MetricShell should be narrowed or repositioned rather than justified by
ambition alone.
