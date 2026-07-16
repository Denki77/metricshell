# Glossary

## CLI

Command Line Interface.

## PID 1

The primary process inside a container is responsible for signal handling and child process reaping.

## Pull Model

A monitoring model where the monitoring system periodically requests metrics from a target.

## Prometheus

A monitoring and time-series database system that primarily collects metrics by scraping configured targets.

## Metrics Endpoint

HTTP endpoint exposing metrics in Prometheus format.

## Sidecar

A companion container running alongside the main application container.

## Pushgateway

A Prometheus component designed for selected push-based batch scenarios.

## Runtime Layer

A reusable execution layer added to an application image that provides runtime capabilities independently from business
code.

## Operator

The entity responsible for configuring, deploying, and operating MetricShell together with the managed workload.

Depending on the environment, the operator may be:

- a developer;
- a DevOps engineer;
- a CI/CD pipeline;
- Docker Compose;
- Kubernetes;
- another orchestration platform.

The term **Operator** does **not** refer to the Kubernetes Operator pattern unless explicitly stated.

---
[Readme](README.md) | [Documentation Readme](../README.md)
