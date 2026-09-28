# Kubernetes Job integration

This example is an executable contract for finite Kubernetes workloads. It keeps MetricShell as PID 1, exposes only the
Prometheus endpoint, keeps ingestion local to the pod, and uses a bounded final-scrape window after workload exit.

The example supports two discovery styles:

- direct pod discovery through the `metricshell.io/scrape=true` pod label;
- Prometheus Operator `PodMonitor` targeting the named `metrics` port.

Readiness is intentionally lifecycle-derived. A pod can be unready during final wait while still scrapeable through
direct pod discovery or PodMonitor configuration that does not depend on service endpoint readiness.

Managed-registry workloads share a private writable `/run/metricshell` directory with the PID-1 wrapper. The managed
Unix socket is local to the pod, mode `0600`/`0660`, and disappears at process cleanup; only the named `metrics` HTTP
port is exposed. Configure managed cardinality/queue bounds and the independent Core snapshot-size limit explicitly.

The same container shape works for a long-running worker in a Deployment: MetricShell periodically coalesces committed
registry generations into complete Core snapshots while the worker remains alive. Job/CronJob additionally relies on
the configured final-wait policy after exit. Graceful termination uses the existing pod termination budget, closes
admission, interrupts partial frames and performs final freeze/install without Kubernetes API access. Every restart
starts an empty managed epoch.

Verification must query Prometheus over a range that includes the recorded final sample timestamp and ends before any
stale marker. Aggregate scrape counts are not evidence that every configured Prometheus replica stored the sample.
