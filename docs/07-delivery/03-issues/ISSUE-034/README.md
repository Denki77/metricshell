# ISSUE-034. Configurable capacity and timeout limits

**Status:** Open
**Readiness:** Code-ready

**Epic:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 6](../../02-epics/EPIC-001-core.md#wave-6)

## Normative inputs

- ADR-003, ADR-005–ADR-008, ADR-010, ADR-011, ADR-014.
- [Configuration Specification](../../../04-specification/configuration.md).
- [Configuration Value Grammar](../../../04-specification/configuration-value-grammar.md).
- [Runtime Defaults and Resource Limits](../../../04-specification/runtime-defaults-and-resource-limits.md).
- [Application Snapshot Protocol](../../../04-specification/application-snapshot-protocol.md).

## Dependencies

ISSUE-001 configuration bootstrap, ISSUE-008 shutdown budget, ISSUE-016 ingestion interface, ISSUE-023 exposition, and
all
transport adapters.

## Scope

Implement the complete version 1 CLI/environment surface, shared scalar/list grammar, and precedence; transport
selection; fixed endpoints and paths; all decoded-input, snapshot, file, socket, HTTP, exposition, final-wait, shutdown,
filtering, and concurrency limits; absolute
deadline;
logging level/selector-value policy; cross-field socket capacity validation; required_nofile formula; effective
non-secret configuration; permanent internal exit codes; and their structured failure mapping.

## Out of scope

Configuration files, dynamic reload, auto mode, managed-registry mode, inactive transport listeners, arbitrary endpoint
paths, and automatic RLIMIT changes.

## Configuration and observable errors

Every property, option, environment variable, default, range, and unit is defined by the configuration
specifications. Invalid/unknown/contradictory input, inactive-transport options, unsafe paths, unavailable binds,
insufficient nofile, and expired deadline fail before workload start with the documented exit code and structured error.
Runtime limit exhaustion uses protocol NACK, HTTP status, self-metrics, and logs without partial activation.

## Acceptance criteria

- CLI overrides environment, which overrides defaults; workload argv after -- is preserved byte-for-byte.
- All static validation and required binds complete before the workload starts.
- Effective configuration redacts secrets and omits workload argv/environment values.
- required_nofile is computed exactly and low soft limit is rejected without mutation.
- Each internal startup failure returns the permanent code 64, 70, 71, 72, or 73 as specified.
- Every capacity/timeout is enforced at the owning boundary and preserves last-valid state.

## Required test matrix

Every option/environment/default; precedence and repeatable filters; invalid units/ranges/unknown options; cross-field
pairs including exact socket decoded capacity at limit/limit-1; both logging levels and selector-value boolean; all
transports active/inactive; safe/unsafe paths and symlinks; bind conflicts; nofile at required-1/required;
absolute deadlines before/after now; each limit at limit/limit+1; overload/timeouts; debug redaction; exit-code
registry and structured error-code mapping;
and platform/container E2E.

## Completion

Complete when exhaustive configuration-table tests cover every public property, startup performs no workload side effect
on failure, all boundaries have observable deterministic errors, and container E2E validates limits and exit codes.
