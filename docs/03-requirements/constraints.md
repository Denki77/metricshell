# Constraints

## Default values

Normative values are defined by
[Runtime Defaults and Resource Limits](../04-specification/runtime-defaults-and-resource-limits.md).

- Default natural-completion final-wait mode: `scrapes`.
- Default fixed duration when `duration` mode is selected: `30s`.
- Default wait-for-scrape timeout: `60s`.
- Default required scrape count: `1`.
- Default external shutdown total grace: `30s`.
- Default workload shutdown timeout: `28s`.
- Default MetricShell shutdown reserve: `2s`.

## Limits

- Maximum configurable scrape wait is `1h`.
- Runtime must never wait indefinitely.
- Application failure must never become success because MetricShell completed normally.
- Final application snapshot is immutable.
- Every candidate and response is subject to explicit bounded resource limits.
