# ISSUE-020. Local push HTTP adapter

**Status:** Done
**Readiness:** Code-ready

**Epic:** [EPIC-001 Core](../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 4](../02-epics/EPIC-001-core.md#wave-4)

**ADR/INV:** ADR-008 / INV-008. Local-only, bounded, same snapshot and error semantics.

## Code-ready contract

- **Normative inputs:** ADR-008 /
  INV-008, [Application Snapshot Protocol](../../04-specification/application-snapshot-protocol.md), [Configuration](../../04-specification/configuration.md),
  and [Runtime Defaults](../../04-specification/runtime-defaults-and-resource-limits.md).
- **Dependencies:** ISSUE-016.
- **Scope / out of scope:** Implement loopback-only POST `/v1/metrics`, identity/gzip decoding, shared validation, and
  exact HTTP mapping. Out of scope: remote authentication and non-loopback bind.
- **Configuration and observable failures:** Wire, decoded, and canonical limits are enforced independently; rejection
  bodies use closed codes; no ACK precedes installation.
- **Acceptance criteria and required tests:** Bind validation; methods/media types/encodings; gzip bomb; every
  status/code row; slow read/write; busy/timeout; concurrent ordering.
- **Completion:** Complete when HTTP runs the shared corpus and matches file/socket state, generation, reason, and
  observability.

## Delivery log

- 2026-08-31: moved to `In Progress`; implemented the loopback-only versioned HTTP handler, bounded identity/gzip
  decoding, exact response mapping and finite server timeouts.
- 2026-08-31: moved to `Testing`; bind, method, path, media type, encoding, wire/decoded limits, gzip bomb, every
  candidate status row, busy, timeout, cancellation and concurrent-order tests passed under the Docker race detector.
- 2026-08-31: moved to `Done`; the adapter passed `make test`, and EN/RU issue and implementation READMEs were audited
  together.

## Verification evidence

- Only `POST /v1/metrics` accepts `application/json` or the version-1 MetricShell media type with identity/gzip
  encoding; other method/media/encoding classes have closed bounded response codes.
- Wire bytes are bounded before decompression, decoded bytes are bounded during decompression, and canonical bytes stay
  bounded by the shared parser. A compressed amplification candidate cannot reach unbounded allocation.
- Every candidate reason uses the normative HTTP status table; busy and timeout remain publication outcomes with 429
  and 408. ACK is written only from the common Core's post-install accepted result.
- Listener configuration rejects empty, wildcard and non-loopback hosts and preserves finite header/read/write/idle
  timeouts and the header-byte bound.
- Concurrent requests receive unique generations from the same atomic holder used by file and Unix ingestion.
