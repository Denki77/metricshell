# ISSUE-012. Whole-candidate parser and validator

**Status:** Done
**Readiness:** Code-ready

**Epic:** [EPIC-001 Core](../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 3](../02-epics/EPIC-001-core.md#wave-3)

Validate syntax, metadata conflicts, duplicate series, histogram consistency, payload, cardinality, labels, and policy
before activation.

Parsing and rejection codes must conform to the accepted
[Application Snapshot Protocol](../../04-specification/application-snapshot-protocol.md).

## Code-ready contract

- **Normative inputs:** ADR-004 and ADR-014 /
  INV-004, [Application Snapshot Protocol](../../04-specification/application-snapshot-protocol.md),
  and [Runtime Defaults](../../04-specification/runtime-defaults-and-resource-limits.md).
- **Dependencies:** ISSUE-011; blocks ISSUE-013 and all adapters.
- **Scope / out of scope:** Parse, validate, canonicalize, and return only the shared candidate result. Out of scope:
  transport-specific parsing and active-state mutation.
- **Configuration and observable failures:** Use the closed rejection mapping, including `schema_version` and `policy`;
  decoded and canonical limits are independent and last-valid state is untouched.
- **Acceptance criteria and required tests:** Shared corpus for every reason; binary64 range, canonical rendering,
  rounding collisions and special floats; histogram sign rules; base/component-name and `le` collisions; empty-family
  normalization; decoded amplification; exact limits; malformed/fuzz input; race-safe concurrent validation.
- **Completion:** Complete when one validator and corpus are consumed unchanged by file, socket, and HTTP adapters.

## Delivery log

- 2026-08-28: moved to `In Progress`; strict whole-document JSON parsing was connected to the canonical validator.
- 2026-08-28: moved to `Testing`; golden, closed-rejection, exact-limit, ownership, malformed/fuzz and concurrent race
  tests passed in the Docker test stage.
- 2026-08-28: moved to `Done`; one transport-independent parser now returns only a complete validated snapshot or a
  closed rejection reason.

## Verification evidence

- Version 1 objects are shape-checked recursively; missing, unknown and duplicate members cannot be silently ignored.
- Empty and invalid UTF-8 payloads, malformed documents, unsupported schema versions and decoded/canonical size excess
  map deterministically to the accepted rejection registry.
- Numeric, identity, metadata, histogram, namespace and cardinality rules are delegated to the single ISSUE-011
  validator after complete decoding.
- Parsed snapshots retain neither the input byte buffer nor decoder-owned mutable collections.
- `implementation/README.md` and `implementation/README_RU.md` were reviewed and updated together.
