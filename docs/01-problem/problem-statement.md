# Problem Statement

> **Status:** Draft for architecture review

## Purpose

This document defines the engineering problem. It intentionally focuses on the problem itself rather than on a specific
implementation.

The goal is to establish a common understanding of the constraints, terminology and success criteria before discussing
architecture or implementation.

---

## Background

Modern observability platforms such as Prometheus primarily use a **pull model**: the monitoring system periodically
requests metrics from an HTTP endpoint exposed by the monitored workload.

This model is natural for long-running HTTP services because they already own a persistent network endpoint.

However, not every application is an HTTP service.

Many production systems execute as:

- queue workers;
- message consumers;
- scheduled jobs;
- CLI applications;
- maintenance commands;
- import/export processes;
- database migrations;
- finite batch workloads.

These applications often have no reason to expose an HTTP interface.

---

## The Engineering Problem

The absence of a persistent HTTP endpoint creates several practical challenges.

### Long-running workers

A worker may execute continuously for days while processing jobs.

Although it is alive, it often exposes no network interface that Prometheus can scrape.

Adding an embedded HTTP server increases application complexity and couples monitoring with business logic.

### Short-lived workloads

Finite workloads introduce another challenge.

The application may finish before Prometheus performs its next scrape.

As a result, the final metric state may never become observable.

### Environment portability

Monitoring solutions should ideally work consistently in:

- Docker;
- Docker Compose;
- Kubernetes;
- local development;
- CI environments.

Solutions tightly coupled to a specific orchestrator reduce portability.

---

## Existing Approaches

Several mature approaches already exist.

Examples include:

- embedded HTTP servers;
- Pushgateway;
- textfile collectors;
- sidecar exporters;
- OpenTelemetry Collector.

Each solves part of the problem and remains valid for many scenarios.

This project does **not** attempt to replace those technologies.

Instead, it investigates whether there is room for a reusable runtime abstraction that targets a different set of
trade-offs.

A detailed comparison is intentionally deferred to [existing-solutions.md](existing-solutions.md).

---

## Desired Characteristics

A potential solution should:

- preserve the Prometheus pull model;
- avoid unnecessary deployment complexity;
- minimise application changes;
- remain independent from a specific orchestrator;
- preserve workload lifecycle semantics;
- remain usable for both long-running and finite workloads.

These characteristics are goals rather than implementation requirements.

---

## Success Criteria

The problem can be considered addressed if a solution can:

1. expose metrics for non-HTTP workloads;
2. work in multiple container environments without redesigning deployments;
3. preserve normal process lifecycle behaviour;
4. remain compatible with standard Prometheus scraping;
5. avoid introducing mandatory central infrastructure for every deployment.

---

## Out of Scope

This document intentionally does **not** define:

- architecture;
- protocols;
- APIs;
- deployment model;
- runtime behaviour;
- implementation language.

Those topics are covered by later documents.

---
[Next: Existing solutions](existing-solutions.md)

---
[Readme](README.md) | [Documentation Readme](../README.md)
