# Runtime State Machine Specification

[Russian version](../../docs-ru/04-specification/runtime-state-machine.md)

> Status: Accepted normative specification
> Requirements: FR-001–FR-006, FR-040–FR-050
> Acceptance criteria: AC-RUN-001–AC-RUN-009, AC-FIN-001–AC-FIN-015
> Decisions: ADR-001, ADR-002, ADR-003, ADR-011

## Scope

This specification is the single normative lifecycle model for MetricShell Core. State names are the exact values used
by
the runtime-state self-metric and structured logs. Implementations may use additional private substates, but they must
not expose another public state vocabulary.

## States

The closed public state set is:

~~~text
initializing
starting_workload
running
stopping
finalizing
final_wait
failed
terminated
~~~

| State             | Meaning                                                                                                  | Readiness   |
|-------------------|----------------------------------------------------------------------------------------------------------|-------------|
| initializing      | Validate all configuration and bind required resources before workload start.                            | not ready   |
| starting_workload | Attempt exactly one workload start.                                                                      | not ready   |
| running           | Workload is active; ingestion and exposition are available.                                              | ready       |
| stopping          | External termination is active; forward the signal and perform bounded workload cleanup.                 | not ready   |
| finalizing        | Capture the workload result, close ingestion, resolve in-flight ordering, and freeze the final snapshot. | not ready   |
| final_wait        | Serve the frozen snapshot under the configured natural-completion wait policy.                           | not ready   |
| failed            | An unrecoverable MetricShell-owned failure is recorded; bounded cleanup remains.                         | not ready   |
| terminated        | No endpoint or child owned by MetricShell remains; the process exits exactly once.                       | unavailable |

A workload exit is an event, not a persistent state. Forced termination is an action and outcome inside stopping, not a
public state. The duration and scrape-count policies share final_wait; immediate mode bypasses it.

## Events

The closed lifecycle event set is:

| Event                   | Valid source                                                     | Result                                  |
|-------------------------|------------------------------------------------------------------|-----------------------------------------|
| configuration_validated | initializing                                                     | starting_workload                       |
| initialization_failed   | initializing                                                     | failed                                  |
| workload_started        | starting_workload                                                | running                                 |
| workload_start_failed   | starting_workload                                                | failed                                  |
| workload_exited         | running, stopping                                                | finalizing                              |
| termination_requested   | initializing, starting_workload, running, finalizing, final_wait | stopping or terminated as defined below |
| runtime_failed          | any non-terminal state                                           | failed                                  |
| finalization_completed  | finalizing                                                       | final_wait or terminated                |
| final_wait_completed    | final_wait                                                       | terminated                              |
| cleanup_completed       | failed                                                           | terminated                              |

Repeated termination signals do not create a new state. They may shorten the remaining grace or trigger immediate forced
cleanup according to shutdown policy and must be logged.

## Normative transitions

~~~mermaid
stateDiagram-v2
    [*] --> initializing
    initializing --> starting_workload: configuration_validated
    initializing --> failed: initialization_failed
    initializing --> terminated: termination_requested
    starting_workload --> running: workload_started
    starting_workload --> failed: workload_start_failed
    starting_workload --> stopping: termination_requested after spawn
    starting_workload --> terminated: termination_requested before spawn
    running --> finalizing: workload_exited
    running --> stopping: termination_requested
    running --> failed: runtime_failed
    stopping --> finalizing: workload_exited after bounded cleanup
    stopping --> failed: runtime_failed
    finalizing --> final_wait: natural completion and mode duration or scrapes
    finalizing --> terminated: natural completion and mode immediate
    finalizing --> terminated: external termination already active
    finalizing --> terminated: termination_requested
    finalizing --> failed: runtime_failed
    final_wait --> terminated: duration elapsed, required scrapes, or timeout
    final_wait --> terminated: termination_requested
    final_wait --> failed: runtime_failed
    failed --> terminated: cleanup_completed
    terminated --> [*]
~~~

No transition may return to running after workload_exited. Invalid transitions are internal failures.

## Final-wait rules

The accepted modes are immediate, duration, and scrapes. There is no auto mode.

- immediate transitions from finalizing directly to terminated;
- duration enters final_wait until the configured duration expires;
- scrapes enters final_wait until the saturating count reaches positive N or the finite timeout expires;
- only a successful complete response for the frozen generation, completed after final_wait begins, is eligible;
- health, readiness, debug, cancelled, failed, pre-final, and ineligible responses never count;
- after N is reached, completion grace drains only handlers already accepted and cannot increase N;
- external termination ends final_wait immediately and has precedence over its normal completion conditions.

## Ingestion ordering at workload exit

The transition to finalizing closes admission first. A candidate admitted before closure may finish within the remaining
finalization budget. If accepted, it becomes the final generation; otherwise the preceding last-valid snapshot is
frozen.
Candidates not admitted before closure receive frozen.

## Probe and endpoint semantics

| State             |                                 health |   readiness | metrics                            |
|-------------------|---------------------------------------:|------------:|------------------------------------|
| initializing      | 200 while bounded progress is possible |         503 | unavailable until bound            |
| starting_workload |                                    200 |         503 | available after bind               |
| running           |                                    200 |         200 | available                          |
| stopping          |  200 while bounded cleanup is possible |         503 | available while server is open     |
| finalizing        |                                    200 |         503 | frozen view available after freeze |
| final_wait        |                                    200 |         503 | frozen view available              |
| failed            |                                    500 |         503 | best effort until drain starts     |
| terminated        |                            unavailable | unavailable | unavailable                        |

Probe requests never count as final scrapes. Readiness is intentionally false outside running.

## Termination precedence and process result

When conditions compete, MetricShell resolves them in this order:

1. unrecoverable MetricShell internal failure;
2. external termination and forced-cleanup policy;
3. preserved workload result;
4. normal final-wait completion.

A final-wait timeout is a normal bounded completion reason and does not replace the workload result. Forced cleanup is
recorded independently; it changes the result only when the workload had not already produced a result. The permanent
MetricShell-owned exit-code registry is defined by the configuration specification.

## Observability mapping

Every transition emits exactly one state-change log after the new state becomes effective. The
metricshell_runtime_state metric uses exactly the state values in this document. final_wait mode and completion reason
use the closed enums in the self-metrics specification. Structured logs use the same state, mode, outcome, and reason
registries.

## Conformance

Table-driven tests must cover every valid and invalid transition, concurrent workload-exit/signal/publication races,
probe responses in every state, all final-wait terminal conditions, repeated signals, forced cleanup, exactly one
terminated transition, and race-detector execution.

## References

- [Configuration Specification](configuration.md)
- [Self-Metrics Specification](self-metrics.md)
- [Structured Logging Specification](structured-logging.md)
- [ADR-002](../06-architecture/adr/ADR-002.md)
- [ADR-003](../06-architecture/adr/ADR-003.md)
- [ADR-011](../06-architecture/adr/ADR-011.md)
