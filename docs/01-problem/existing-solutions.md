# Existing Solutions

> Status: Draft

## Purpose

This document surveys existing approaches for exposing metrics from CLI, worker and finite workloads.

## Problem

Prometheus is fundamentally based on a pull model where collectors periodically scrape HTTP endpoints exposed by
monitored targets [P-01](references.md#p-01--writing-exporters).

For service-level batch jobs Prometheus recommends Pushgateway only for selected scenarios and documents important
limitations when used as a general replacement for pull
scraping [P-02](references.md#p-02--when-to-use-the-pushgateway).

For instance-level batch workloads Prometheus explicitly states that **there is no clear recommended pattern** and lists
multiple alternatives instead of a single solution [P-01](references.md#p-01--writing-exporters).

This motivates architectural investigation rather than assuming one universally correct approach.

## Existing approaches

### Embedded HTTP endpoint

References: [P-01](references.md#p-01--writing-exporters), [P-03](references.md#p-03--exposition-formats)

Advantages:

- Native Prometheus model.
- Mature libraries.
- No extra infrastructure.

Limitations:

- Monitoring becomes the application responsibility.
- CLI applications must host HTTP.

### Pushgateway

References: [P-02](references.md#p-02--when-to-use-the-pushgateway)

Advantages:

- Recommended for selected service-level batch jobs.

Limitations:

- Additional infrastructure.
- Separate lifecycle.
- Stale metrics management.
- Loss of normal `up` semantics.

### Node Exporter Textfile Collector

References: [P-01](references.md#p-01--writing-exporters)

Advantages:

- Simple.

Limitations:

- Node-scoped.
- Requires collector.

### Sidecar exporter

References: [K-01](references.md#k-01--sidecar-containers)

Advantages:

- Separation of concerns.

Limitations:

- Extra deployment artifact.
- Kubernetes-oriented pattern.

### OpenTelemetry

References: [O-01](references.md#o-01--specification), [O-02](references.md#o-02--metrics-specification)

Advantages:

- Vendor-neutral.
- Rich telemetry.

Limitations:

- Broader than Prometheus exposition.

## Conclusion

No reviewed approach is universally superior.

Each optimizes different trade-offs. The Architecture Investigation phase will evaluate all approaches using a common
comparison matrix before any architectural decisions are made.

See:

- [references.md](references.md)
- [existing-solutions-evaluation.md](../03-requirements/existing-solutions-evaluation.md)
