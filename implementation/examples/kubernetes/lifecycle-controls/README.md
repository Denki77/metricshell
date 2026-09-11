# Kubernetes lifecycle controls

These manifests encode the outer Kubernetes bounds that must be larger than MetricShell's internal shutdown and final
wait bounds. They are static examples for CI validation and cluster qualification.

- `terminationGracePeriodSeconds: 32` gives a measurable two-second margin over the default
  `METRICSHELL_SHUTDOWN_TOTAL_GRACE=30s`.
- `activeDeadlineSeconds` bounds the whole Job, including final wait.
- `ttlSecondsAfterFinished` delegates finished Job cleanup to Kubernetes.
- `restartPolicy: Never` preserves workload-result semantics.
- `CronJob.concurrencyPolicy: Forbid` prevents overlapping final-metric windows.
