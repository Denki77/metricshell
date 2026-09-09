# Kubernetes Job integration

This example is an executable contract for finite Kubernetes workloads. It keeps MetricShell as PID 1, exposes only the
Prometheus endpoint, keeps ingestion local to the pod, and uses a bounded final-scrape window after workload exit.

The example supports two discovery styles:

- direct pod discovery through the `metricshell.io/scrape=true` pod label;
- Prometheus Operator `PodMonitor` targeting the named `metrics` port.

Readiness is intentionally lifecycle-derived. A pod can be unready during final wait while still scrapeable through
direct pod discovery or PodMonitor configuration that does not depend on service endpoint readiness.

Verification must query Prometheus over a range that includes the recorded final sample timestamp and ends before any
stale marker. Aggregate scrape counts are not evidence that every configured Prometheus replica stored the sample.
