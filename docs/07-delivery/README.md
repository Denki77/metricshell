# Delivery Documentation

This section connects accepted requirements, investigations, ADRs, and specifications to implementation work.

## Project status — production baseline v0.2.2

Core and optional Managed Registry are implemented. The production gate passed and the first successful GitHub Release
is published for Linux amd64/arm64. The current production baseline is `v0.2.2`; it remains pre-1.0 without an unlimited
API compatibility promise. No new functional stages are planned. Further changes are limited to defects, security,
compatibility and maintenance. The delivery plans below are historical implementation records, not a new roadmap.

- [Production installation and verification](production-deployment.md)
- [Final release notes](releases/v0.2.2.md)
- [Published artifact and acceptance audit](releases/v0.2.2-audit.md)
- [Owner actions and future release infrastructure improvements](releases/v0.2.2-maintenance.md)

## Navigation

- [Requirements-to-ADR traceability](01-traceability/requirements-research-adr-traceability.md)
- [EPIC-001: MetricShell Core](02-epics/EPIC-001-core.md)
- [ISSUE-001](03-issues/ISSUE-001.md) through [ISSUE-037](03-issues/ISSUE-037.md)

## Normative relationship

Delivery documents are implementation plans. Accepted requirements, ADRs, and specifications define observable
behavior. English and Russian versions are equal and must be updated together. An issue is code-ready only when it links
those inputs and contains scope, acceptance criteria, required tests, observable failures, dependencies, and completion
conditions.
