# Behavioral Model Draft

> Status: Non-normative input for architecture investigation

## Purpose

This document summarises the currently expected external behaviour of MetricShell.

It is not a normative specification and does not define conformance requirements.
Its contents may change as a result of architecture investigation and ADRs.

---

## Scope

This specification defines:

- runtime lifecycle;
- process lifecycle;
- metrics lifecycle;
- ingestion interfaces;
- exposition behaviour;
- shutdown semantics;
- configuration model;
- observable guarantees.

It intentionally does not define internal architecture or implementation details.

---

## Runtime Model

MetricShell acts as the primary runtime process for a workload.

Responsibilities:

- start the workload;
- monitor its lifecycle;
- expose metrics;
- preserve exit code;
- coordinate shutdown.

---

## Supported Integration Models

1. Multi-stage Docker build.
2. Base runtime image.
3. Runtime package installed into an existing image.

All models shall provide identical runtime behaviour.

---

## Metrics Ingestion

Supported transports:

- Unix Domain Socket
- Atomic Snapshot File
- Local Push API

The client API must remain transport-independent.

---

## Metrics Exposition

MetricShell shall expose a Prometheus-compatible `/metrics` endpoint.

Supported exposition modes:

- live registry;
- frozen final snapshot;
- latest available snapshot.

---

## Shutdown Strategies

The current required capabilities are defined
in [Functional Requirements](../03-requirements/functional-requirements.md).

---

## Process Guarantees

MetricShell shall:

- preserve workload exit code;
- forward signals;
- operate correctly as PID 1;
- reap child processes.

---

## Failure Model

Metric publication failures must not silently change workload execution semantics.

Timeouts must always result in deterministic shutdown.

---

## Compatibility

The runtime shall remain compatible with standard Prometheus pull scraping.

It shall not require Kubernetes-specific functionality.

---
[Runtime State Machine](runtime-state-machine.md)

---
[Readme](README.md) | [Documentation Readme](../README.md)
