# Changelog

All notable changes to MetricShell are recorded here.

The project follows semantic versioning after the first stable release. While `0.x` is in development, incompatible
runtime-contract changes may appear in minor versions.

## [Unreleased] - 2026-09-27

- Added the opt-in `managed-registry` ownership mode with bounded descriptor-driven counter, gauge and classic
  histogram operations over a private Unix protocol.
- Added serialized commit acknowledgement, resource controls, immutable materialization and the shared Core install
  path with admission-close, bounded drain, freeze and existing final-scrape lifecycle.
- Added stateless shell and PHP 5.4 clients, fixed-cardinality observability, production E2E and reproducible validation
  evidence. Snapshot mode remains the default and retains its existing contract.

## [0.1.2] - 2026-09-11

- Completed the Core implementation waves under `implementation`.
- Marked the complete-snapshot runtime profile ready for operational use.
- Documented that publishers replace the whole metric set; partial metric updates are out of scope for this release.
- Added Docker-only CI, integration, fault, benchmark and supply-chain gates.
- Added signed release evidence, SBOM and vulnerability-scanning pipeline.
- Added GitHub-facing README, community profile files and MIT license.
