# Multi-replica Prometheus verification

The release verifier must query each configured Prometheus replica independently and map every returned final sample to
one `metricshell_replica` label. The success condition is per-replica, not aggregate:

```text
prometheus-0 -> metricshell_replica=final-sample, value=42
prometheus-1 -> metricshell_replica=final-sample, value=42
```

Failures are replica-specific: missing sample, duplicate target labels, stale-only data, delayed scrape outside the
recorded range, or an aggregate count that hides a missing replica.
