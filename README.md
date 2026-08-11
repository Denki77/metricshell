# MetricShell

> Status: Draft

MetricShell is a container-native runtime wrapper for exposing Prometheus metrics from CLI workloads.

## What is it?

MetricShell runs an arbitrary command as a managed child process and exposes application metrics through a Prometheus-compatible HTTP endpoint without requiring the application itself to implement an HTTP server.

## Current status

Core architecture is complete. Production implementation is in progress under [`implementation/`](implementation/README.md).

## Documentation

See the `docs/` directory.
