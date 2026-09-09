# Final sample verification

For each configured Prometheus replica, query the stored sample independently. The verifier records:

- target pod name and `metricshell_replica`;
- last successful scrape timestamp;
- final sample timestamp and value;
- query start/end time;
- whether a later stale marker or missing target was observed.

An instant query after target disappearance is not sufficient evidence. Use a range query that includes the final sample
and ends before the stale marker.
