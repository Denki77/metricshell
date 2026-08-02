# INV-012 — Kubernetes Job and CronJob Viability

**Status:** in progress
**Reference run:** `results/20260803T143134Z`
**Report:** [report.md](report.md)
**Decision:** pending Ubuntu confirmation and ADR

## Question

Can Prometheus continue discovering and scraping the MetricShell endpoint while a completed workload is held alive
inside a still-running Kubernetes Job Pod?

## Context

The post-workload metrics window is useful only if Kubernetes discovery keeps the Pod reachable long enough for the
configured Prometheus instances to collect the final complete snapshot. Under ADR-004, MetricShell replaces its current
snapshot atomically; it never adds snapshots together. This investigation therefore counts successful scrapes of one
complete, stable exposition and does not aggregate metric values across scrapes.

The Kubernetes target used by this prototype is explicitly Minikube. It is an isolated local cluster, not a claim about
every managed Kubernetes implementation.

## Candidates

### A. Direct Pod discovery

Prometheus discovers annotated Pod IPs with `kubernetes_sd_configs` and scrapes them regardless of a Service.

### B. ServiceMonitor

Prometheus Operator discovers a Service and its endpoints through an actual `ServiceMonitor` CRD.

### C. PodMonitor

Prometheus Operator discovers Pod IPs directly through an actual `PodMonitor` CRD.

## Initial Hypotheses

- a Job Pod can remain alive after workload exit and expose a frozen, complete metrics snapshot;
- two Prometheus replicas can both scrape that endpoint before the Job completes;
- readiness affects ServiceMonitor target visibility and must not be assumed to suppress all scrapes without evidence;
- PodMonitor is the more direct discovery model for an intentionally unready post-workload Pod;
- `activeDeadlineSeconds`, `ttlSecondsAfterFinished`, deletion and CronJob concurrency policy bound Pod lifetime;
- `concurrencyPolicy: Forbid` prevents overlapping long post-exit CronJob executions.

## Evidence Required

- a real Minikube Kubernetes cluster;
- two real Prometheus replicas installed through Prometheus Operator;
- actual ServiceMonitor and PodMonitor CRDs;
- direct Kubernetes Pod discovery through a real Prometheus configuration;
- Job completion driven by observed scrapes, not a simulated callback;
- deadline, TTL, explicit termination and scheduler evidence;
- portable assertions separated from environment-dependent timings.

## Experiments

### E-012.1 — Direct Pod discovery

A standalone Prometheus `v3.5.0` uses `kubernetes_sd_configs` with `role: pod`. An annotated Job completes after its
first observed scrape.

### E-012.2 — ServiceMonitor and two Prometheus replicas

`kube-prometheus-stack` `88.1.2` installs Prometheus Operator and two Prometheus replicas. A real ServiceMonitor scrapes
a ready Job for a bounded 60-second window. After the Job completes, the runner port-forwards to each Prometheus Pod
separately and queries `inv012_final_snapshot` at an evaluation time inside the final active window.

### E-012.3 — Readiness variants

The runner measures ServiceMonitor behavior for an unready Pod without requiring zero scrapes, then verifies that a
PodMonitor can scrape an unready Pod directly.

### E-012.4 — Lifetime controls

The runner checks `activeDeadlineSeconds`, `ttlSecondsAfterFinished` and explicit Pod deletion with a one-second grace
period.

### E-012.5 — CronJob overlap

A real once-per-minute CronJob runs longer than the following schedule tick. The runner checks that
`concurrencyPolicy: Forbid` leaves one active Pod and one Job.

## Evaluation Criteria

- real scrape discovery rather than mocked requests;
- complete snapshot availability after workload exit;
- multiple-scraper viability;
- bounded Job and Pod lifetime;
- scheduler overlap behavior;
- reproducibility with one benchmark fingerprint on macOS and Ubuntu hosts.

## Open Questions

- Does the same fingerprint pass on the Ubuntu validation host?
- Do managed Kubernetes implementations differ in endpoint publication or termination timing?
- Should production use PodMonitor by default when post-workload readiness is false?
- What production post-exit window is sufficient for the configured Prometheus interval and replica count?

## Results

| Environment                       | Date       | Result set                 | Summary                                             | Benchmark fingerprint                                              |
|-----------------------------------|------------|----------------------------|-----------------------------------------------------|--------------------------------------------------------------------|
| Docker Desktop on macOS, Minikube | 2026-08-03 | `results/20260803T143134Z` | [summary.tsv](results/20260803T143134Z/summary.tsv) | `caa34fb8ba97176b3c199b56ffaeb21dcabbb654fa5efd2b4da3203d79274e6c` |
| Docker on Ubuntu, Minikube        | pending    | pending                    | pending                                             | must match macOS                                                   |

Key findings from the macOS reference run:

- all 21 portable assertions and all 14 scenario summaries passed;
- checksum-verified Minikube `v1.38.1` supplied effective kubectl `v1.34.0`; the system kubectl was not used;
- direct Pod discovery completed after an observed scrape in `8,017.520 ms`;
- the ServiceMonitor handler observed 51 aggregate HTTP scrapes during its `63,724.188 ms` bounded execution;
- Prometheus replicas 0 and 1 were queried separately after Job completion; each TSDB returned the exact
  `inv012_final_snapshot` series for the current `service-ready` Job Pod with sample value `42`;
- the unready ServiceMonitor scenario observed 1 scrape in this run. The count remains an environment-dependent
  observation rather than a portable invariant;
- PodMonitor scraped an unready Pod at least twice;
- active deadline produced `DeadlineExceeded`, TTL deleted the completed Job, and explicit Pod deletion took
  `2,022.144 ms`;
- the real CronJob retained one active Pod and one Job across the next schedule tick under `Forbid`.

## Conclusion

The macOS/Minikube evidence supports the architecture direction: a post-workload Job Pod can expose its final complete
snapshot long enough for multiple Prometheus scrapers, and Kubernetes lifetime controls can bound that window.
PodMonitor is viable for direct discovery of an unready Pod. ServiceMonitor readiness behavior must be treated as
observed implementation behavior, not reduced to an assumed zero-scrape rule.

The investigation remains **in progress** until the exact benchmark fingerprint is rerun under Docker on Ubuntu and an
ADR is accepted.

## Decision output

- Prototype: `prototype/`
- Runner: `run-bench.sh`
- macOS raw evidence: `results/20260803T143134Z/`
- Ubuntu raw evidence: pending
- Report: [report.md](report.md)
- Recommended ADR input: use direct Pod discovery/PodMonitor for an intentionally unready post-workload endpoint;
  retain explicit lifetime and overlap limits; never sum snapshots or scrape values.

## Running the Prototype

Prerequisites: Docker, Helm, `curl` and network access to the configured binary/image/chart registries. A system
Minikube or `kubectl` is not required.

```bash
./research/INV-012/run-bench.sh
```

The command creates the isolated Minikube profile `metricshell-inv012`, installs the pinned monitoring chart, performs
all scenarios, writes one timestamped result directory, updates `latest-results.txt`, and deletes the profile. Preserve
the cluster for inspection only when needed:

Do not start Minikube manually. The runner downloads official Minikube `v1.38.1`, verifies its platform-specific
SHA-256,
deletes any previous profile with this research-only name and creates its own Docker-backed cluster with Kubernetes
`v1.34.0`. All Kubernetes commands use Minikube's matching kubectl `v1.34.0`; a system kubectl such as Ubuntu `1.30` is
recorded for diagnosis but never used. A successful run takes several minutes; a fast exit means failure, not an empty
successful investigation.

The Minikube start phase has a hard 15-minute stand timeout. Failure diagnostics are then collected for at most two
minutes, and profile cleanup is bounded by 60 seconds. A stalled `kubeadm init` therefore terminates with explicit
`minikube_start` evidence instead of leaving the one-command run blocked for hours.

```bash
INV012_KEEP_MINIKUBE=1 ./research/INV-012/run-bench.sh
```

Inspect the latest result:

```bash
cat research/INV-012/latest-results.txt
cat "$(cat research/INV-012/latest-results.txt)/summary.tsv"
cat "$(cat research/INV-012/latest-results.txt)/assertions.tsv"
cat "$(cat research/INV-012/latest-results.txt)/observations.tsv"
cat "$(cat research/INV-012/latest-results.txt)/environment.tsv"
```

Inspect a failed early run:

```bash
latest="$(cat research/INV-012/latest-results.txt)"
cat "$latest/run-summary.tsv"
cat "$latest/failure.tsv"
cat "$latest/docker-info.log"
cat "$latest/minikube-download.log"
cat "$latest/minikube-start.log"
cat "$latest/minikube-cluster.log"
cat "$latest/helm-install.log"
```

`run-summary.tsv` records `status=error`, the failing phase, line, exit code, command and normalized detail. The same
detail is printed immediately in the terminal without host paths. Before running, `docker info` must succeed as the
current user without `sudo`. Typical Ubuntu failures are a missing prerequisite, lack of permission to access the
Docker daemon, insufficient Docker CPU/memory allocation, or network access failure while resolving images or the Helm
repository.

Run the same command on macOS and Ubuntu. Compare
`benchmark_code_fingerprint_sha256`; do not compare runs with different fingerprints as one cross-environment pair.

## Prototype Limits

- The target is checksum-verified Minikube `v1.38.1` with Kubernetes and effective kubectl `v1.34.0`, not a managed
  production cluster.
- The macOS host uses Docker Desktop and an arm64 Docker engine; Ubuntu evidence is still pending.
- Prometheus Operator is installed by `kube-prometheus-stack` `88.1.2`; a separate Prometheus `v3.5.0` covers direct
  Pod discovery.
- Scheduler and scrape timings are observations, not production SLOs.
- The prototype uses a synthetic complete Prometheus snapshot and counts HTTP scrapes; it does not implement all
  production MetricShell ingestion paths.
- No assertion requires a specific scrape count for an unready ServiceMonitor target; that count is an
  environment-dependent observation.

## Additional Benchmarks

Covered by the current runner:

- checksum-verified Minikube bootstrap and cluster-matched kubectl assertion;
- direct annotated-Pod discovery;
- actual ServiceMonitor plus a separate post-completion PromQL query against each Prometheus replica TSDB;
- ready and unready ServiceMonitor scenarios;
- actual PodMonitor against an unready Pod;
- `activeDeadlineSeconds` failure reason;
- `ttlSecondsAfterFinished` deletion;
- explicit Pod termination latency;
- real CronJob scheduling and `Forbid` overlap across a second tick;
- Helm manifest, monitor CRDs, cluster events, raw logs and environment fingerprint capture.

Still useful on additional environments:

- rerun this identical fingerprint on Docker/Ubuntu;
- repeat on a managed Kubernetes cluster with its native CNI and endpoint controller;
- vary Prometheus scrape intervals, replica counts and post-exit windows;
- test network policies, Pod disruption, node drain and control-plane interruption;
- measure larger complete exposition payloads without summing or merging snapshots.
