# INV-018 Report — Legacy Client and Transport Viability

**Status:** in progress

**Run date:** 2026-09-24

**Docker server:** 29.8.0

**Docker platform:** LinuxKit/aarch64

**Reference run:** `results/20260924T185603Z`

**Summary:** `results/20260924T185603Z/summary.tsv`

## Goal

Validate or reject the INV-018 assumption that PHP 5.4, shell and simple CLI workloads can publish individual Managed
Aggregation operations without maintaining complete registry state, and compare Unix stream socket and local HTTP as
the initial local operation transport.

## Prototype

The prototype is located in `research/INV-018`.

- `prototype/cmd/inv018` — one binary containing the registry server and short-lived CLI client.
- `prototype/cmd/inv018 benchmark` — persistent in-container generator that records per-ACK latency without per-operation
  Compose exec or process startup.
- `clients/metricshell.php` — PHP 5.4-compatible stateless client using built-in stream functions.
- `compose.yml` — isolated server and real PHP 5.4.45 runtime stand.
- `run-bench.sh` — reproducible experiment runner and fingerprint generator.
- `results/<timestamp>` — assertions, summaries, raw replies, logs, observations and environment metadata.

The operation envelope is JSON with explicit `version`, `op`, `metric`, `value` and labels. Unix uses one newline-delimited
request and one JSON ACK per connection. HTTP uses one `POST /v1/operations` and one JSON response. This protocol is
separate from the Core snapshot protocol; clients never serialize a complete Prometheus registry.

## Run Commands

Full run:

```bash
./research/INV-018/run-bench.sh
```

Inspect the latest result:

```bash
latest="$(cat research/INV-018/latest-results.txt)"
cat "$latest/summary.tsv"
cat "$latest/assertions.tsv"
cat "$latest/benchmarks.tsv"
cat "$latest/persistent-transport-benchmarks.tsv"
cat "$latest/transport-comparison.tsv"
cat "$latest/environment.tsv"
```

The exact stand and manual commands are documented in [README.md](README.md).

## Run Environment and Fingerprint

| Environment             | Date       | Docker | Container platform             | Result                     | Fingerprint                                                        |
|-------------------------|------------|-------:|--------------------------------|----------------------------|--------------------------------------------------------------------|
| Docker Desktop on macOS | 2026-09-24 | 29.8.0 | LinuxKit/aarch64               | `results/20260924T195359Z` | `a5a256f20b747d561c24f4d3b6132c3bb486c21d23ccba6f464277d86033fb7d` |
| Ubuntu                  | pending    |      — | expected Linux/x86_64 or ARM64 | pending                    | must match reference                                               |

The fingerprint covers `prototype/`, `clients/`, `compose.yml` and `run-bench.sh`. The PHP image is also pinned by digest:
`sha256:05440cda403be37644cad1c5e201884d4e67d3a7fae13e2b561986f98afd3169`. These inputs give macOS and Ubuntu one
stand identity even when their Docker host architecture differs. Repository HEAD, kernel, image ID and host resources
are recorded separately in `environment.tsv`.

Because matching Ubuntu evidence and the ADR are not yet present, the research status remains **in progress**.

## Results

All 41/41 correctness assertions passed.

| Experiment                   | Result | Evidence                                                                                                              |
|------------------------------|--------|-----------------------------------------------------------------------------------------------------------------------|
| E-018.1 PHP 5.4 basic        | pass   | PHP 5.4.45 performed increment, set and observe and distinguished accepted, transport, protocol and rejected outcomes |
| E-018.2 PHP multiprocess     | pass   | four independent worker processes each produced exactly 25 increments; unrelated state remained                       |
| E-018.3 Shell/CLI            | pass   | helper succeeded without another daemon; transport failure exited 3 and rejection exited 4                            |
| E-018.4 Startup race         | pass   | server readiness preceded clients; 3 × 20 ms missing-endpoint retry failed in 159 ms, without unbounded wait          |
| E-018.5 Reconnect/restart    | pass   | three independent connections accumulated exactly; restart changed epoch and cleared registry                         |
| E-018.6 Transport comparison | pass   | persistent generator covered 12 transport/profile/client cells; framing/version negatives rejected                    |

The PHP worker processes shared only the socket endpoint. They did not share a PHP registry, snapshot file or IPC state.
Worker exit did not delete another worker's series.

The prototype accepted exactly 65,536 bytes and rejected 65,537 bytes. This proves enforcement of its bounded framing,
not that 64 KiB is the correct production limit. The value remains an initial research safety bound pending INV-019.
The Unix path also rejected a connection closed before its newline delimiter. Supported version `1` was accepted;
missing, invalid and unsupported versions were rejected deterministically.

## End-to-End Short-Lived Helper Observation

| Transport | Concurrent clients | Operations | Elapsed ms | Observed ops/s | Errors |
|-----------|-------------------:|-----------:|-----------:|---------------:|-------:|
| Unix      |                  1 |        100 |  9,488.566 |           10.5 |      0 |
| Unix      |                  8 |         96 |  1,611.486 |           59.6 |      0 |
| Unix      |                 32 |         96 |  1,587.146 |           60.5 |      0 |
| HTTP      |                  1 |        100 |  9,424.338 |           10.6 |      0 |
| HTTP      |                  8 |         96 |  1,744.837 |           55.0 |      0 |
| HTTP      |                 32 |         96 |  1,482.650 |           64.7 |      0 |

These numbers include one `docker compose exec` for every CLI operation. They measure the deliberately simple shell
integration path plus Docker orchestration, not server throughput or transport latency. The near-equal Unix/HTTP values
therefore do not prove transport equivalence and are not production limits. Their useful result is that all tested
client-count cells completed without an operation error.

## Persistent-Generator Transport Observations

The generator process starts once per complete cell and performs all operations internally. Profile A opens, sends one
operation, receives one ACK and closes for every operation. Profile B reuses HTTP connections. The Unix candidate remains
one-operation-per-connection; its reuse profile is explicitly rejected instead of silently changing protocol semantics.

| Transport | Profile                  | Clients | Operations |    Ops/s | ACK p50 ms | ACK p95 ms | ACK p99 ms | Errors |
|-----------|--------------------------|--------:|-----------:|---------:|-----------:|-----------:|-----------:|-------:|
| Unix      | connection per operation |       1 |        200 |  9,648.9 |      0.091 |      0.175 |      0.343 |      0 |
| Unix      | connection per operation |       8 |      1,600 | 32,335.5 |      0.190 |      0.554 |      0.772 |      0 |
| Unix      | connection per operation |      32 |      6,400 | 32,136.1 |      0.768 |      2.432 |      3.385 |      0 |
| Unix      | connection per operation |     128 |     25,600 | 31,407.0 |      3.187 |     10.052 |     14.445 |      0 |
| HTTP      | connection per operation |       1 |        200 |  4,686.5 |      0.203 |      0.296 |      0.455 |      0 |
| HTTP      | connection per operation |       8 |      1,600 | 23,440.2 |      0.273 |      0.815 |      1.055 |      0 |
| HTTP      | connection per operation |      32 |      6,400 | 34,239.3 |      0.701 |      2.082 |      3.386 |      0 |
| HTTP      | connection per operation |     128 |     25,600 | 32,502.1 |      3.185 |      9.528 |     13.279 |      0 |
| HTTP      | reused connection        |       1 |        200 |  8,906.1 |      0.106 |      0.152 |      0.170 |      0 |
| HTTP      | reused connection        |       8 |      1,600 | 46,437.6 |      0.128 |      0.414 |      0.782 |      0 |
| HTTP      | reused connection        |      32 |      6,400 | 55,226.8 |      0.362 |      1.651 |      2.364 |      0 |
| HTTP      | reused connection        |     128 |     25,600 | 45,910.0 |      2.071 |      7.163 |      9.940 |      0 |

These are LinuxKit/ARM64 observations without CPU pinning or controlled cgroups. They are secondary evidence and must not
be treated as portable guarantees, production throughput or the primary reason to prefer Unix.

## Transport Evaluation

| Criterion              | Unix stream socket                           | Local HTTP                                                    |
|------------------------|----------------------------------------------|---------------------------------------------------------------|
| PHP 5.4 dependency     | built-in `stream_socket_client`              | built-in TCP streams; HTTP parsing needed without curl        |
| Shell path             | short-lived helper                           | curl is convenient but not guaranteed; helper still preferred |
| Framing                | simple newline candidate; bounded frame      | standard HTTP framing; bounded body                           |
| Local security         | filesystem owner/group/mode                  | loopback or network namespace policy                          |
| Debugging              | helper required for convenient use           | curl/wget friendly                                            |
| Listener exposure      | no TCP listener                              | requires TCP listener even when local-only                    |
| Startup/failure        | bind before workload; connect/reject visible | bind before workload; status and connect errors visible       |
| Operational complexity | no network configuration                     | TCP listener/address and network policy                       |
| Prototype correctness  | all variants passed                          | all variants passed                                           |

Unix remains preferred primarily because Managed Aggregation is a local same-workload boundary. It provides filesystem
ownership/mode control, requires no TCP listener, works with PHP 5.4 stream APIs and fits the one-binary/local-workload
model. The stand demonstrated both an allowed client and a UID/GID without socket permission being denied. `0660` is a
prototype mode, not a production permission decision. HTTP's debugging advantage is real, but it does not justify
requiring two production transports. Performance observations are secondary evidence.

## Acceptable Values Selected for Follow-up

These are inputs to ADR-018 and later limit research, not accepted production defaults:

- operation protocol version candidate: integer `1`; supported, missing, invalid and unsupported behavior is tested;
- one operation and one explicit ACK per request;
- bounded operation framing is required; 64 KiB inclusive is only the prototype safety bound and initial candidate;
- transport/connect error and semantic rejection must be distinguishable to CLI callers;
- listener readiness before workload start is the primary startup contract;
- retry, when offered by a convenience client, must have a finite count and delay;
- reconnect requires no registry reconstruction;
- server restart creates a new empty epoch.

The demonstrated `3 × 20 ms` retry and 2-second client I/O deadline are prototype controls, not selected product values.
A startup/connect retry is safe when the connection was not established and the request definitely was not submitted.
If a write may have succeeded but the ACK is lost, the outcome is unknown and the client must not blindly retry.
The transport carries explicit accepted/rejected outcomes, but whether a production ACK means queued, committed or
deduplicated is resolved by INV-017.

## Limitations

- The registry is a research mutex implementation and does not replace INV-017 concurrency evidence.
- Core snapshot validation and the production Managed Registry are outside this transport stand. ADR-016 remains
  authoritative for registry semantics.
- The legacy image is amd64 and emulated on the reference ARM64 host; compatibility is demonstrated, performance is not.
- Descriptor declaration, batch atomicity, idempotency keys, ACK-loss behavior, backpressure and lifecycle freeze remain
  outside this prototype.
- Allowed/denied Unix access was tested; production UID/GID deployment matrices and final socket mode remain integration concerns.
- Only the macOS/LinuxKit reference environment is retained so far.

## Additional Benchmarks

The runner covers Unix and HTTP at 1/8/32/128 clients, connection-per-operation and HTTP connection reuse, ACK
p50/p95/p99, PHP and CLI, four independent legacy workers, reconnect, bounded pre-submit retry, readiness-before-client,
restart/new epoch, exact and over-limit payloads, malformed JSON, incomplete Unix framing, version cases, deterministic
PHP outcomes and allowed/denied Unix access. Negative results are retained in individual logs.

INV-019 should add CPU/RSS, warmups, pinned CPUs/cgroup memory, longer cells, slow readers and saturation. ACK loss,
unknown outcome and deduplication remain INV-017 scope. Repeat this exact runner on Ubuntu; the fingerprint must remain
`d89830368f7df46e2f9ea628ef54c7f6d195d49ffa4baa3efb21274e5a0af146`.

## Conclusion

The viability assumption is supported by the current environment: real PHP 5.4 and shell-friendly short-lived commands
can submit managed operations without owning a complete registry. Unix socket is provisionally preferred over local HTTP,
with a versioned framing candidate, explicit outcome and bounded frames. The exact production bound is not selected.

This is not yet a final architecture decision. Ubuntu matching-fingerprint confirmation and ADR review are still required,
so INV-018 remains in progress.

## Decision Output

- Prototype and CLI: `prototype/`
- PHP 5.4 reference client: `clients/metricshell.php`
- Runner and portable stand: `run-bench.sh`, `compose.yml`
- Reference evidence: `results/20260924T185603Z/`
- Provisional future ADR input: Unix socket + versioned per-operation request/ACK; stateless clients; bounded framing
- Pending: Ubuntu confirmation and ADR-018 or rejected-alternative record
