# INV-011 — Final Application State and Scrape Counting

**Status:** completed
**macOS reference run:** `results/20260803T065931Z`
**Ubuntu/LinuxKit reference run:** `results/20260803T070500Z`
**Report:** [report.md](report.md)
**Decision:** [ADR-011](../../docs/06-architecture/adr/ADR-011.md)

## Question

What remains immutable after workload exit, and when is a final scrape considered complete?

## Context and Hypotheses

ADR-004 makes the last valid complete application snapshot the final application state. MetricShell does not sum or
merge snapshots. Ingestion must close before final waiting begins; application metrics then remain immutable while
MetricShell self-metrics may change as requests and time progress.

The initial hypotheses are that the default required count should be one, `N > 1` should be optional, health/readiness
must never count, and only an eligible response whose complete bytes were written successfully may count. Serving bytes
cannot prove Prometheus TSDB persistence. A manual `curl` is indistinguishable from a scraper unless an explicit
eligibility token is configured.

## Evidence Required

- immutable application values and mutable self-metrics after finalization;
- ingestion rejection after the freeze boundary;
- immediate, fixed-duration, one-scrape and N-scrape modes;
- health/readiness exclusion and timeout behavior;
- manual, repeated same-client and concurrent scrapes;
- optional eligibility token behavior;
- disconnected large response not counted, later complete response counted;
- one matching-fingerprint Ubuntu/LinuxKit repeat.

## Confirmed Result

Both matching-fingerprint runs passed all 26 portable assertions. In each environment, ten repeated ephemeral-port
startup
cycles each returned HTTP 200 with curl/container exit 0. Application value `42` and its SHA-256
identity remained fixed while scrape-attempt self-metrics advanced; post-finalization publication returned HTTP 409.
Immediate and fixed-duration modes exited cleanly. Health/readiness left counts at zero. One manual curl satisfied
`N=1`; three requests from the same client satisfied `N=3`; all 20 concurrent clients received complete responses while
the configured count saturated at 10 during a 500 ms completion-drain window.

An 8 MiB chunked response whose TCP client disconnected was observed as an attempt but remained at zero completed
scrapes. A later full response counted and released the wait. A 500 ms timeout exited with zero completed scrapes. The
behavior is confirmed on macOS/LinuxKit aarch64 and Ubuntu/LinuxKit x86_64.

## Accepted Values

- freeze order: stop accepting application publications, freeze the last valid complete snapshot, then begin waiting;
- application state: immutable through the final-wait phase; no summation, merge or late replacement;
- self-metrics: may continue changing and are excluded from final application-state identity;
- default wait mode: one eligible completed scrape, with a mandatory bounded timeout;
- optional modes: immediate, fixed duration and configurable positive `N`;
- count point: after all response bytes are successfully written by the HTTP handler;
- exclusions: health, readiness, state/debug endpoints, failed writes and ineligible-token responses;
- concurrency: completed eligible responses count independently; the counter saturates at configured `N`;
- completion drain: after reaching `N`, keep a bounded grace period for already accepted HTTP handlers;
- uniqueness: no source-IP or scraper-identity deduplication by default;
- authentication/eligibility: optional explicit token may gate counting, but does not prove TSDB persistence;
- timeout: exit according to lifecycle policy without fabricating a completed scrape.

These values are adopted by [ADR-011](../../docs/06-architecture/adr/ADR-011.md).

## Running the Prototype

Run from the repository root on macOS or Ubuntu:

```bash
./research/INV-011/run-bench.sh
```

Inspect the latest evidence:

```bash
latest="$(cat research/INV-011/latest-results.txt)"
cat "$latest/assertions.tsv"
cat "$latest/summary.tsv"
cat "$latest/observations.tsv"
cat "$latest/environment.tsv"
cat "$latest/aborted_scrape.log"
cat "$latest/concurrent_scrapes.log"
cat "$latest/timeout.log"
```

The runner uses one Linux image and ephemeral loopback ports. It polls Docker inspect until a numeric host binding and
successful `/healthz` response both exist; a container exit before readiness is a hard failure. Short duration/timeout
cases do not publish a port. Ubuntu uses exactly the same command. Compare
`benchmark_code_fingerprint_sha256`; repository HEAD is context only. The fingerprint includes only `prototype/` and
`run-bench.sh`, so result and documentation changes do not alter benchmark identity.

Manual example:

```bash
docker build -t metricshell-inv011:prototype research/INV-011/prototype
docker run --rm -p 127.0.0.1:19111:19111 metricshell-inv011:prototype \
  --mode=scrapes --required=1 --wait=5s
curl http://127.0.0.1:19111/metrics
```

## Prototype Limits

- Research lifecycle server, not a complete MetricShell supervisor.
- “Successfully written” means the Go HTTP handler completed all writes without an error and the request context was
  not cancelled. Kernel/socket success cannot prove remote parsing, Prometheus acceptance or TSDB persistence.
- The eligibility token is a research mechanism, not a selected authentication protocol.
- The synthetic application snapshot is already final when the HTTP server starts; workload termination ordering is
  represented, not integrated with a real workload process.
- Counter saturation at N makes the release condition deterministic. A bounded completion grace lets already accepted
  concurrent handlers finish their HTTP responses before server shutdown.
- Timing observations include Docker startup and host polling and are not shutdown-budget recommendations.
- An operator observed several intermittent empty HTTP replies outside the retained benchmark evidence. The reference
  lifecycle matrix reproduced none of them (10/10 HTTP 200 with clean curl/container exits), so the observation is not
  classified as a failed assertion or as a proven server defect. A future reproduction must retain client stderr,
  server logs, container state and port-binding state for the same request.
- Both container environments use LinuxKit: macOS/LinuxKit aarch64 and Ubuntu/LinuxKit x86_64. Native non-LinuxKit
  Linux is not covered; Kubernetes discovery/lifecycle is covered separately by INV-012.

## Additional Benchmarks

| Benchmark                                               | Status                                                                        |
|---------------------------------------------------------|-------------------------------------------------------------------------------|
| immutable application snapshot and mutable self-metrics | covered                                                                       |
| ingestion rejected after finalization                   | covered: HTTP 409                                                             |
| immediate exit                                          | covered                                                                       |
| fixed-duration exit                                     | covered at 500 ms test setting                                                |
| one completed scrape                                    | covered                                                                       |
| configurable N                                          | covered at N=3 and N=10                                                       |
| health/readiness exclusion                              | covered                                                                       |
| manual curl eligibility                                 | covered: counts without an explicit gate                                      |
| same-client uniqueness                                  | covered: repeated requests count independently                                |
| concurrent counting and response drain                  | covered: 20 complete responses, count saturated at 10                         |
| ephemeral port publication/readiness race               | covered: inspect polling plus HTTP readiness; early exit fails                |
| repeated ephemeral-port lifecycle                       | covered: 10/10 HTTP 200, curl 0 and container 0 cycles                        |
| operator-observed intermittent empty reply              | not reproduced; no retained per-request evidence                              |
| optional eligibility token                              | covered                                                                       |
| aborted response                                        | covered with 8 MiB chunked response                                           |
| bounded timeout                                         | covered at 500 ms with zero fabricated completions                            |
| matching-fingerprint Ubuntu/LinuxKit repeat             | covered: 26/26 assertions with identical fingerprint                          |
| real Prometheus scrape and TSDB query                   | cannot establish causal persistence without explicit acknowledgement protocol |
| HTTP/2, proxy buffering and TLS                         | recommended after deployment topology is selected                             |
| shutdown race with real workload/signals                | integrate after INV-003 and production lifecycle code                         |

## Better Follow-up Benchmarking

For follow-up characterization, repeat abort timing at several body sizes/socket buffer settings,
exercise HTTP/2 and a reverse proxy, and use a real Prometheus instance to demonstrate the distinction between response
completion and later query visibility. Do not reinterpret multiple responses as metric aggregation: every response must
still contain the same frozen complete application snapshot plus independently changing self-metrics.

## Decision Output

- Prototype: `prototype/`
- Runner: `run-bench.sh`
- macOS raw evidence: `results/20260803T065931Z/`
- Ubuntu raw evidence: `results/20260803T070500Z/`
- Detailed analysis: [report.md](report.md)
- ADR: [ADR-011](../../docs/06-architecture/adr/ADR-011.md)
