# INV-011 Report — Final Application State and Scrape Counting

**Status:** in progress
**Run date:** 2026-08-02
**Docker server:** 29.6.2 (macOS/LinuxKit)
**Docker platform:** linux/aarch64
**Reference run:** `results/20260803T065931Z`
**Fingerprint:** `4b8b5d48b85b5c3c2c74e94c2b0ed59494708110ce53b6f8645d16e4d5d0c7d9`

## Goal

Define the application-state freeze boundary and the smallest truthful rule for counting final scrapes across immediate,
duration, one-scrape, N-scrape, concurrent, ineligible, aborted and timeout scenarios.

## ADR-004 Boundary

The final application state is the last valid complete snapshot, or the valid zero-series initial state when no
publication was accepted. The benchmark models that state as one immutable body with a stable SHA-256 identity. It never
sums snapshots or incorporates self-metric changes into application values. Publication is rejected after finalization.
Self-metrics remain separate and can advance while MetricShell waits.

## Prototype and Commands

- `prototype/cmd/inv011` — final-state HTTP server and wait-mode state machine;
- `prototype/Dockerfile` — reproducible Linux image;
- `run-bench.sh` — complete correctness and timing-observation matrix;
- `results/<timestamp>` — assertions, observations, response bodies and per-case logs.

```bash
./research/INV-011/run-bench.sh
latest="$(cat research/INV-011/latest-results.txt)"
cat "$latest/assertions.tsv"
cat "$latest/observations.tsv"
cat "$latest/aborted_scrape.log"
cat "$latest/timeout.log"
```

macOS and Ubuntu use the same command and benchmark fingerprint.

## Run Environment

| Environment                       |       Date |  Docker | Architecture    | Result                     | Status                |
|-----------------------------------|-----------:|--------:|-----------------|----------------------------|-----------------------|
| Docker Desktop on macOS/LinuxKit  | 2026-08-03 |  29.6.2 | aarch64         | `results/20260803T065931Z` | 26/26 assertions pass |
| Docker Desktop on Ubuntu/LinuxKit |    pending | pending | x86_64 expected | pending                    | not run               |

The macOS fingerprint is `4b8b5d48b85b5c3c2c74e94c2b0ed59494708110ce53b6f8645d16e4d5d0c7d9`.
Ubuntu evidence is comparable only if that value matches.

## Results

### Freeze boundary

Two scrapes during a bounded fixed wait both exposed `application_jobs_total 42`. The MetricShell attempt counter
increased between them. The response header and final log retained application SHA-256
`66aab7e584c5d4eb1187ab30d0a46c68b7448dbbd58bc99e6c60982883090a15`. A post-finalization snapshot request returned
HTTP 409. This supports freezing application state before waiting while keeping self-metrics live.

### Exit modes

Immediate mode exited 0 with reason `immediate`. The 500 ms duration case exited 0 with reason `duration_elapsed`; its
host-observed full container lifecycle was 813.244 ms. The 500 ms scrape timeout exited 0 with reason `timeout`, zero
completed scrapes and 858.604 ms full container lifecycle. Host times include Docker startup and are observations, not
configured-budget accuracy measurements.

### Counting and eligibility

Five health and five readiness requests left state at `completed=0 attempted=0`. A normal manual curl completed N=1.
Two requests left the N=3 case running at count 2, and the third released it. This proves that default counting cannot
distinguish a manual client from Prometheus and does not deduplicate the same client.

For the concurrent case, 20 requests were launched at once. All 20 clients received complete HTTP responses. Ten
eligible handlers reached the release threshold and the counter saturated at N=10; a 500 ms completion grace drained
already accepted handlers before shutdown. `concurrent-clients.log` is empty, so no curl transport error was hidden.

Ephemeral port publication is polled through container inspect until a numeric binding and `/healthz` are both ready.
Ten dedicated repetitions all returned HTTP 200 and clean curl/container exits; their readiness ranged from about 240
to 302 ms. Across the other scenarios readiness stayed at or below about 400 ms. Early container exit is now a hard
startup failure. The short 500 ms duration and timeout cases run without publishing an HTTP port, removing Docker
startup time from readiness correctness.

With `X-Final-Scrape-Token` configured, a complete ordinary response was served but remained ineligible at
`completed=0 attempted=1`; a response with the token counted. This demonstrates a possible eligibility gate, not
scraper authentication or storage acknowledgement.

### Aborted response

The server generated an approximately 8 MiB response in 16 KiB flushed chunks with a 1 ms inter-chunk delay. A raw TCP
client disconnected after the request. State later showed zero completed and at least one attempted response. A normal
client then received the full body, counted once and released the wait. Counting after the write loop therefore
distinguished the tested disconnect from a completed handler write.

## Hypothesis Evaluation

### Default required scrape count should be one

Provisionally supported. N=1 gives the smallest useful final-wait contract. It needs a bounded timeout because no
scraper may arrive. N>1 works but increases waiting and does not by itself improve persistence certainty.

### A scrape counts only after the complete response is written successfully

Supported within server-observable semantics. The disconnected large response did not count; a later full write did.
This remains weaker than remote receipt, Prometheus parsing or TSDB commit.

### Health and readiness never count

Supported. Both endpoints remained outside attempted and completed scrape counters.

### Concurrent scrapes count independently

Supported, with a saturating threshold. Each eligible completed handler may claim one count until N. No scraper identity
or source-IP uniqueness is required.

### Serving a response does not prove TSDB persistence

Confirmed by protocol reasoning. HTTP write completion has no causal acknowledgement from Prometheus storage. An
optional request token can prove configured eligibility but still cannot prove durable ingestion.

## Evaluation Against Criteria

| Criterion                       | Provisional result                            |
|---------------------------------|-----------------------------------------------|
| immutable application state     | stable value and SHA-256 across wait          |
| live self-metrics               | attempt counter changed independently         |
| freeze ordering                 | late publication returned 409                 |
| immediate/duration modes        | passed                                        |
| N=1 and N>1                     | passed at 1, 3 and concurrent 10              |
| health/readiness exclusion      | passed                                        |
| manual and same-client behavior | counted independently by default              |
| optional eligibility            | token gate demonstrated                       |
| abort handling                  | 8 MiB disconnected write did not count        |
| timeout                         | bounded exit with zero fabricated completions |
| Ubuntu reproducibility          | pending matching-fingerprint run              |

## Provisional Acceptable Values and Policies

- Freeze the last valid complete application snapshot and close ingestion before entering final wait.
- Keep application metric identity immutable; self-metrics remain mutable and separate.
- Default to waiting for one eligible completed scrape with a mandatory finite timeout.
- Support immediate, fixed-duration and positive configurable N modes.
- Increment only after the handler successfully writes the full response and sees no cancelled request context.
- Never count health, readiness, debug/state, write failures or explicitly ineligible responses.
- Count concurrent completed responses independently using an atomic saturating counter capped at N.
- After reaching N, use a bounded completion grace before server shutdown to drain already accepted handlers.
- Do not deduplicate by IP or inferred scraper identity by default.
- Treat an eligibility token as optional policy, not evidence of storage.
- On timeout, follow lifecycle/exit policy and report the timeout; never synthesize completion.

## Prototype Limits

- One macOS/LinuxKit environment; Ubuntu is pending.
- Synthetic already-final workload state rather than full process supervision and signal races.
- Go server-side write completion cannot prove remote or durable receipt.
- Token gate is illustrative, with no credential lifecycle or threat model.
- HTTP/1.1 direct connection only; no proxy, TLS or HTTP/2 buffering behavior.
- Host lifecycle timings are not shutdown budget limits.
- The tested 500 ms completion grace drained 20 local clients; production values still require deployment-specific
  proxy/network testing.

## Additional Benchmarking

| Item                                        | Status      | Evidence/Reason                                            |
|---------------------------------------------|-------------|------------------------------------------------------------|
| application freeze and self-metric mutation | covered     | two bodies and stable hash                                 |
| ingestion close before wait                 | covered     | HTTP 409                                                   |
| immediate and duration                      | covered     | per-case logs                                              |
| one and N completed scrapes                 | covered     | N=1, 3 and 10                                              |
| health/readiness exclusion                  | covered     | zero attempts/counts                                       |
| manual and repeated same client             | covered     | all eligible by default                                    |
| concurrent completions and drain            | covered     | 20 complete responses, saturation at 10                    |
| ephemeral port/readiness startup            | covered     | inspect polling, numeric binding and HTTP readiness        |
| repeated ephemeral-port lifecycle           | covered     | 10/10 HTTP 200, curl 0 and container 0 cycles              |
| eligibility token                           | covered     | ineligible and eligible responses                          |
| aborted connection                          | covered     | 8 MiB chunked response                                     |
| bounded timeout                             | covered     | 500 ms setting, zero count                                 |
| Ubuntu matching fingerprint                 | pending     | required before completion                                 |
| real Prometheus/TSDB visibility             | follow-up   | illustrates non-equivalence, cannot be inferred from write |
| proxy/TLS/HTTP2 abort matrix                | recommended | deployment-dependent buffering                             |
| real workload/signal finalization race      | recommended | production integration with INV-003                        |

## Conclusion

The macOS evidence provisionally supports a default of one eligible completed scrape plus a finite timeout. Application
metrics freeze before waiting; self-metrics remain live. Health/readiness never count, repeated and concurrent eligible
responses count independently up to N, and a server-observed failed/aborted write does not count.

The guarantee must be worded narrowly: complete successful server write, not TSDB persistence. The investigation stays
in progress until the identical fingerprint passes on Ubuntu and no final ADR is produced before that confirmation.

## Decision Output

- Prototype: `prototype/`
- Runner: `run-bench.sh`
- macOS raw evidence: `results/20260803T065931Z/`
- Ubuntu raw evidence: pending
- Provisional direction: freeze then wait; default N=1; count complete eligible writes; finite timeout
- ADR: pending
