# INV-012 Report — Kubernetes Job and CronJob Viability

**Status:** in progress
**Run date:** 2026-08-03
**Runtime:** Minikube v1.38.1, Kubernetes v1.34.0
**Monitoring:** kube-prometheus-stack 88.1.2, two Prometheus replicas
**Reference run:** `results/20260803T143134Z`
**Summary:** [summary.tsv](results/20260803T143134Z/summary.tsv)

## Goal

Validate whether a Kubernetes Job Pod can keep MetricShell alive after workload exit long enough to expose one stable,
complete Prometheus snapshot to one or more real scrapers, while Kubernetes deadline, TTL, termination and CronJob
controls keep that lifetime bounded.

ADR-004 is a hard constraint: the prototype never sums snapshots. It publishes one complete snapshot and records HTTP
scrape observations only.

## Prototype

- `prototype/cmd/inv012/main.go` — post-workload HTTP server with `/metrics`, `/readyz`, bounded wait and
  observed-scrape completion.
- `prototype/Dockerfile` — non-root multi-stage image.
- `prototype/k8s/monitoring-values.yaml` — pinned two-replica Prometheus configuration.
- `run-bench.sh` — checksum-verified Minikube bootstrap, matching kubectl, isolated cluster lifecycle, Helm
  installation,
  Kubernetes scenarios and evidence capture; early failures record phase/command diagnostics instead of leaving an
  apparently empty result.
- `results/<timestamp>` — assertions, observations, manifests, events, logs and environment identity.

The prototype emits a complete Prometheus exposition on every request. A scrape increments only an internal research
counter used to decide when to end the post-exit window; it does not alter, merge or sum the exposed metric snapshot.

## Run Commands

```bash
./research/INV-012/run-bench.sh
```

```bash
cat research/INV-012/latest-results.txt
cat "$(cat research/INV-012/latest-results.txt)/summary.tsv"
cat "$(cat research/INV-012/latest-results.txt)/assertions.tsv"
cat "$(cat research/INV-012/latest-results.txt)/observations.tsv"
cat "$(cat research/INV-012/latest-results.txt)/environment.tsv"
```

For diagnosis only, keep the isolated cluster after the run:

```bash
INV012_KEEP_MINIKUBE=1 ./research/INV-012/run-bench.sh
```

Minikube must not be started manually. The runner downloads verified Minikube `v1.38.1`, creates and removes its own
`metricshell-inv012` profile/context, and uses its Kubernetes-matched kubectl `v1.34.0` instead of the system kubectl.
Minikube start is bounded by 15 minutes, failure-log capture by two minutes and cleanup by 60 seconds. A fast exit is a
failure: inspect `run-summary.tsv`, `failure.tsv` and the phase log named there.

## Run Environments

| Environment                       | Date       | Docker server | Architecture    | Result set                 | Status                  |
|-----------------------------------|------------|--------------:|-----------------|----------------------------|-------------------------|
| Docker Desktop on macOS, Minikube | 2026-08-03 |        29.6.2 | aarch64         | `results/20260803T143134Z` | 21/21 assertions passed |
| Docker on Ubuntu, Minikube        | pending    |       pending | x86_64 expected | pending                    | not run                 |

The macOS benchmark fingerprint is
`caa34fb8ba97176b3c199b56ffaeb21dcabbb654fa5efd2b4da3203d79274e6c`. Ubuntu validation must use this exact
fingerprint. `repository_head_sha` is context only and may differ after documentation changes.

## Results

| Scenario                   | Validated result                    | Result |
|----------------------------|-------------------------------------|--------|
| Toolchain                  | Minikube `1.38.1`, kubectl `1.34.0` | pass   |
| Two Prometheus replicas    | 2 running replicas                  | pass   |
| Direct Pod discovery       | final scrape observed               | pass   |
| ServiceMonitor             | at least two aggregate HTTP scrapes | pass   |
| Prometheus replica 0 TSDB  | stored series present, value `42`   | pass   |
| Prometheus replica 1 TSDB  | stored series present, value `42`   | pass   |
| Ready Pod                  | bounded window ended by timeout     | pass   |
| Unready ServiceMonitor Pod | bounded window ended by timeout     | pass   |
| Unready PodMonitor Pod     | at least two scrapes                | pass   |
| Active deadline            | Job failed with `DeadlineExceeded`  | pass   |
| TTL after finish           | Job object deleted                  | pass   |
| Explicit termination       | Pod removed                         | pass   |
| CronJob schedule           | first scheduled Job created         | pass   |
| CronJob policy             | `Forbid` retained                   | pass   |
| Overlap                    | one active Pod and no second Job    | pass   |

Measured observations:

| Observation                           |         Value |
|---------------------------------------|--------------:|
| Direct discovery completion           |  8,017.520 ms |
| ServiceMonitor completion             | 63,724.188 ms |
| Aggregate ServiceMonitor HTTP scrapes |            51 |
| Unready ServiceMonitor scrapes        |             1 |
| Explicit Pod deletion                 |  2,022.144 ms |

## Hypothesis Evaluation

### A Job Pod can expose final metrics after workload exit

Supported in Minikube. Direct discovery, ServiceMonitor and PodMonitor each caused the prototype to observe real HTTP
scrapes before it completed its bounded post-workload window.

### Two Prometheus replicas can collect the endpoint

Supported. After the Job completed, the runner opened a separate port-forward to each Prometheus Pod and executed
`/api/v1/query` independently. Both TSDBs returned the `inv012_final_snapshot` series for the same Job Pod and sample
value `42`. The query uses a recorded historical evaluation time ten seconds before completion because Prometheus marks
the series stale when the completed Pod target disappears. Raw responses and per-replica evidence are retained.

### Readiness alone defines scrape visibility

Not established as a universal rule. The unready ServiceMonitor case observed 1 scrape in the reference run, but one
Minikube observation cannot define behavior for every endpoint controller and monitoring stack. The portable assertion
is bounded completion by timeout, while the scrape count remains an observation. PodMonitor directly scraped an unready
Pod and is the clearer option when post-workload readiness is intentionally false.

### Kubernetes controls can bound lifetime and overlap

Supported. `activeDeadlineSeconds` produced `DeadlineExceeded`; TTL removed a completed Job; explicit Pod deletion
finished in about 2.0 seconds; and `concurrencyPolicy: Forbid` kept one Job/Pod active across the next schedule tick.

## Acceptable Values and Policies

- Discovery: prefer PodMonitor/direct Pod discovery for an intentionally unready post-workload Pod.
- Snapshot semantics: expose one atomically selected complete snapshot; never add results from repeated scrapes.
- Replicas: require stored-sample evidence from every configured replica; aggregate handler counts are insufficient.
- Post-exit window: must exceed discovery plus scrape delay. The prototype's 60-second ServiceMonitor window passed;
  this is research evidence, not a production default.
- Deadline: configure `activeDeadlineSeconds` above the intended post-exit window so the cluster remains the outer
  bound.
- Cleanup: configure `ttlSecondsAfterFinished` for finished Job objects.
- CronJob: use `concurrencyPolicy: Forbid` when overlapping metrics windows are not acceptable.
- Readiness: do not encode a zero-scrape assumption for unready ServiceMonitor targets without cluster-specific proof.

## Prototype Limits

- Only Minikube was tested, with a macOS Docker Desktop host. Ubuntu confirmation is pending.
- Helm chart and Kubernetes versions are pinned; later versions may reconcile endpoints differently.
- Replica storage is verified through separate Prometheus API connections. The historical evaluation time is required
  because current-time queries correctly omit the series after the target receives a stale marker.
- Timings include cluster scheduling and operator reconciliation and are not performance guarantees.
- NetworkPolicy, disruption, node drain, API server failure and managed-cluster behavior are not covered.
- The prototype snapshot is intentionally small; payload scaling belongs to INV-015.

## Additional Benchmarking

| Benchmark                                      | Status  | Evidence                                                       |
|------------------------------------------------|---------|----------------------------------------------------------------|
| Direct `kubernetes_sd_configs` Pod discovery   | covered | `direct-job.log`                                               |
| Two-replica ServiceMonitor                     | covered | `service-ready.log`, `helm-manifest.yaml`                      |
| Stored sample in each replica TSDB             | covered | `prometheus-replica-evidence.tsv`, replica query JSON files    |
| Ready/unready ServiceMonitor behavior          | covered | `service-ready.log`, `service-unready.log`, `observations.tsv` |
| Unready PodMonitor                             | covered | `pod-unready.log`, `monitors.yaml`                             |
| Active deadline and TTL                        | covered | `assertions.tsv`, `events.txt`, `ttl.log`                      |
| Explicit termination latency                   | covered | `observations.tsv`                                             |
| Real CronJob overlap across schedule tick      | covered | `assertions.tsv`, `events.txt`                                 |
| Same-fingerprint Ubuntu run                    | pending | must create the sole Ubuntu reference result                   |
| Managed cluster/CNI comparison                 | pending | separate explicitly named environment                          |
| NetworkPolicy, disruption and node-drain cases | pending | environment-specific extension                                 |

## Conclusion

INV-012 is supported by the macOS/Minikube evidence. A Job Pod can remain available for final complete-snapshot scrapes,
including multiple scrapers, and Kubernetes can bound the lifecycle. PodMonitor/direct Pod discovery is the safest
direction when post-workload readiness is false. The unready ServiceMonitor result is recorded as an observation and
does not establish a universal zero-scrape guarantee.

Status remains **in progress** until the identical fingerprint passes under Docker/Minikube on Ubuntu and an ADR records
the selected production policy.
