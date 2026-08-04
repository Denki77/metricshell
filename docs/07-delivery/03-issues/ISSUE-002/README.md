# ISSUE-002. PID 1 entrypoint and workload command parsing

**Status:** Open
**Readiness:** Code-ready

**Epic:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 1](../../02-epics/EPIC-001-core.md#wave-1)

**ADR/INV:** ADR-001 / INV-001.

Preserve workload argv without default shell interpretation, distinguish workload launch errors from workload exit, and
run an integration test with MetricShell as container PID 1.

## Code-ready contract

- **Normative inputs:** ADR-001 /
  INV-001, [Configuration](../../../04-specification/configuration.md), [Runtime State Machine](../../../04-specification/runtime-state-machine.md),
  and [Structured Logging](../../../04-specification/structured-logging.md).
- **Dependencies:** ISSUE-001.
- **Scope / out of scope:** PID-1 entrypoint and byte-preserving workload argv after `--`. Out of scope: shell parsing
  unless the shell is the explicit workload.
- **Configuration and observable failures:** Empty argv is `configuration_invalid`; exec failure is
  `workload_start_failed`; workload exit is never reported as supervisor startup failure.
- **Acceptance criteria and required tests:** PID-1 container test; spaces, empty arguments, Unicode and option-like
  argv; missing executable; exit 0/non-zero/signal; startup signal race.
- **Completion:** Complete when argv and outcome fixtures pass in a real container and startup failures use the
  normative log and exit registries.
