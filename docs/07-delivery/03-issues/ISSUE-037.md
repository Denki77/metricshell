# ISSUE-037. Release supply-chain pipeline

**Status:** Done
**Readiness:** Code-ready

**Epic:** [EPIC-001 Core](../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 6](../02-epics/EPIC-001-core.md#wave-6)

Checksums, full SBOM, signatures, provenance, vulnerability scanning, independent clean-builder validation.

## Code-ready contract

- **Normative inputs:** ADR-013, [Docker and Compose Examples](../../04-specification/docker-compose-examples.md),
  and release outputs from ISSUE-032 and ISSUE-036.
- **Dependencies:** ISSUE-032, ISSUE-035, and ISSUE-036.
- **Scope / out of scope:** Generate checksums, complete SBOM, signatures, provenance, vulnerability results, and
  independent clean-builder verification. Out of scope: unsigned manual release artifacts.
- **Configuration and observable failures:** Any missing/invalid signature, checksum, provenance subject, SBOM
  component, or policy-blocking vulnerability fails publication.
- **Acceptance criteria and required tests:** Tampered binary/checksum/signature; incomplete SBOM; wrong provenance
  subject; clean rebuild; amd64/arm64 OCI verification; offline verification instructions.
- **Completion:** Complete when every published artifact is traceable, signed, reproducible, and independently
  verifiable.

## Delivery log

- 2026-09-09: moved to `In Progress`; added local release evidence generation for checksums, detached Ed25519
  signature from an external BuildKit signing-key secret, provenance subjects, SBOM components from `go list -m -json
  all`, vulnerability policy decision from `govulncheck -json`, and offline verification instructions.
- 2026-09-09: moved to `Testing`; added tamper tests for binary/checksum mismatch, signature corruption, incomplete
  SBOM, wrong provenance subject, and policy-blocking vulnerability reports.
- 2026-09-09: moved to `Done`; Docker `supply-chain-artifacts` stage and Makefile targets `supply-chain` /
  `supply-chain-ci` export independently verifiable release evidence from the static release output without storing a
  private signing key in the repository.
- 2026-09-10: tightened release evidence verification; the signed `SHA256SUMS` manifest now covers raw scanner/module
  inputs and generated evidence files, and SBOM verification compares every Go module against signed `MODULES.jsonl`
  with matching versions and sums.

## Verification evidence

- `go test ./internal/supplychain ./internal/releaseverify`
- `make supply-chain-ci IMAGE=metricshell-issue037-supply-chain`
- `make test IMAGE=metricshell-issue037-supply-chain`
- `make wave6 IMAGE=metricshell-wave6`
