# Constraints

## Default values

- Default shutdown strategy: delay
- Default delay: 30 seconds
- Default wait-for-scrape timeout: 60 seconds
- Default required scrape count: 1

## Limits

- Maximum configurable scrape wait should be bounded.
- Runtime must never wait indefinitely.
- Application failure must never become success because MetricShell completed normally.
- Final snapshot is immutable.
