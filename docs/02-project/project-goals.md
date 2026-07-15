# Project Goals

## Purpose

This document defines the long-term objectives of MetricShell.

## Vision

MetricShell provides a reusable runtime layer that makes Prometheus metrics available for CLI workloads without
requiring every application to implement and maintain its own metrics endpoint.

## Goals

- Preserve the standard Prometheus pull model.
- Simplify instrumentation of CLI applications.
- Reduce deployment complexity compared to solutions requiring additional infrastructure.
- Work consistently in Docker, Docker Compose and Kubernetes.
- Keep the solution language-agnostic.
- Encapsulate observability concerns outside business code.
- Preserve normal workload lifecycle (signals, exit codes, shutdown).
- Be easy to adopt by adding a runtime layer to an existing container image.

## Non-goals

MetricShell is **not** intended to:

- replace Prometheus;
- replace Pushgateway;
- replace OpenTelemetry Collector;
- replace sidecar exporters in every scenario;
- replace application metrics libraries.

Instead, it offers another deployment model with different trade-offs.

---
[Project Scopes](project-scope.md)

---
[Readme](README.md) | [Documentation Readme](../README.md)
