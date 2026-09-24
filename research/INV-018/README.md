# INV-018 — Legacy Client and Transport Viability

**Status:** in progress (macOS/LinuxKit evidence collected; Ubuntu confirmation and ADR pending)

**Reference run:** `results/20260924T185603Z`

**Report:** [report.md](report.md)

## Question

Can PHP 5.4, shell and simple CLI workloads submit managed-registry operations without maintaining a complete registry,
and should the initial local operation transport be a Unix stream socket or local HTTP?

## Evidence Rule

The prototype tests client and transport viability, not the production concurrency implementation. Correctness assertions
are portable. Timings include Docker orchestration and are observations only. The research remains in progress until an
Ubuntu run with the same benchmark fingerprint is retained and the decision is recorded.

## Current Evidence

The macOS/LinuxKit ARM64 reference run passed 41/41 assertions. It used real PHP 5.4.45 under an amd64 container, four
independent PHP worker processes, the short-lived CLI helper, Unix and HTTP operation paths, bounded startup retry,
reconnect, server restart/new epoch, exact 64 KiB payload acceptance, 64 KiB + 1 rejection, malformed/partial input and a
1/8/32/128-client persistent-generator matrix with ACK p50/p95/p99 observations.

Benchmark fingerprint: `d89830368f7df46e2f9ea628ef54c7f6d195d49ffa4baa3efb21274e5a0af146`.

## Provisional Conclusion

- A stateless PHP 5.4 client is viable with built-in stream functions; it holds neither a registry nor a Prometheus
  snapshot.
- Shell should invoke `metricshell metric`-style short-lived helper commands and use their exit status. Shell should not
  implement JSON framing.
- A versioned one-operation request/one-ACK protocol is viable. The PHP research client reports `accepted`,
  `transport_error`, `protocol_error` and `rejected` categories. Its numeric CLI mappings are prototype behavior, not an
  architecture decision.
- Unix stream socket remains the preferred initial local transport because its filesystem ownership/mode is a clearer
  same-container security boundary and it requires no exposed TCP listener. Local HTTP remains a useful diagnostic or
  compatibility fallback, not a required second production transport.
- A bounded frame is required. The demonstrated 64 KiB inclusive boundary is a research safety bound and initial
  candidate only; INV-019 owns production resource-limit selection.
- The endpoint should be bound before workload start. Optional convenience retry must be finite; the prototype's
  3 retries at 20 ms are demonstrated behavior, not yet a production default.

## Running the Prototype

Run the same command on macOS and Ubuntu:

```bash
./research/INV-018/run-bench.sh
latest="$(cat research/INV-018/latest-results.txt)"
cat "$latest/summary.tsv"
cat "$latest/assertions.tsv"
  cat "$latest/benchmarks.tsv"
cat "$latest/persistent-transport-benchmarks.tsv"
cat "$latest/transport-comparison.tsv"
cat "$latest/environment.tsv"
```

Increase short-lived helper observations with `INV018_REPETITIONS=1000` and persistent-generator operations per client
with `INV018_TRANSPORT_OPERATIONS_PER_CLIENT=1000`. The runner builds the server image, creates an isolated
Compose network and runtime volume, executes the PHP/CLI scenarios, records evidence, and removes the containers,
network and volume on exit.

Manual stand start:

```bash
docker compose -p inv018-manual -f research/INV-018/compose.yml up -d server
docker compose -p inv018-manual -f research/INV-018/compose.yml exec server \
  inv018 client --transport=unix --endpoint=/run/metricshell/managed.sock \
  --op=inc --metric=jobs_total --value=1 --labels=worker=shell
docker compose -p inv018-manual -f research/INV-018/compose.yml exec server \
  wget -qO- http://127.0.0.1:8080/debug/state
docker compose -p inv018-manual -f research/INV-018/compose.yml down -v
```

PHP 5.4 call against the manual stand:

```bash
docker compose -p inv018-manual -f research/INV-018/compose.yml --profile tools run --rm --no-deps php54 \
  /clients/metricshell.php unix /run/metricshell/managed.sock inc php_jobs_total 1 worker=php
```

## Prototype Limits

- This is research code, not production MetricShell, and uses a mutex registry rather than the eventual INV-017 model.
- It demonstrates counter increment/add, gauge set and non-negative histogram observation only. Descriptor negotiation,
  batches, idempotency keys and production snapshot installation are outside this prototype.
- HTTP has no authentication because it is only bound inside the isolated test network.
- PHP runs as `linux/amd64`; Docker Desktop emulates it on ARM64. This validates PHP 5.4 compatibility but makes its
  timings unsuitable for transport comparison.
- `benchmarks.tsv` invokes `docker compose exec` for every operation and is only an end-to-end short-lived helper
  observation. It is not a transport throughput benchmark.
- `persistent-transport-benchmarks.tsv` runs one in-container generator per complete matrix cell. Its timing values are
  environment-sensitive observations, not portable guarantees or production sizing.
- Production timeout, retry, queue and connection limits remain INV-017/INV-019 inputs.
- A successful prototype ACK carries an accepted outcome. Whether production ACK means queued, committed or deduplicated
  is intentionally left to INV-017.
- Bounded retry is safe only before a request was definitely submitted. A lost ACK creates an unknown outcome and must
  not be blindly retried before INV-017 selects idempotency semantics.

## Additional Benchmarking

The default runner covers both transports at 1/8/32/128 clients with operations/sec and ACK p50/p95/p99, plus sequential
reconnects, independent PHP processes, deterministic PHP success/transport/protocol/rejection categories, malformed JSON,
a partial Unix frame, supported/missing/invalid/unsupported versions, 64 KiB and 64 KiB + 1 payloads, Unix allowed/denied
filesystem access, bounded pre-submit retry, readiness ordering and restart/new epoch. HTTP connection reuse is measured
separately. Unix multi-operation connection reuse is explicitly deferred because the current candidate framing permits
one operation per connection.

INV-019 should extend the persistent generator with pinned server/client CPUs, cgroup memory, CPU/RSS sampling, warmups,
longer 10k–1m operation cells, slow readers and saturation. It must not reuse the short-lived orchestration rates as
transport capacity numbers. Ubuntu confirmation should use the unmodified fingerprint above; if executable sources
change, rerun both environments.

## Decision Output

- Prototype: `prototype/`
- Legacy client: `clients/metricshell.php`
- Stand: `compose.yml`
- Runner: `run-bench.sh`
- Current evidence: `results/20260924T185603Z/`
- Detailed analysis: [report.md](report.md)
- Pending: matching-fingerprint Ubuntu evidence and ADR-018/rejected-alternative record
