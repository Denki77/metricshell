# ISSUE-037. Release supply-chain pipeline

**Status:** Open
**Readiness:** Code-ready

**Epic:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 6](../../02-epics/EPIC-001-core.md#wave-6)

Checksums, full SBOM, signatures, provenance, vulnerability scanning, independent clean-builder validation.

## Code-ready contract

- **Normative inputs:** ADR-013, [Docker and Compose Examples](../../../04-specification/docker-compose-examples.md),
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
