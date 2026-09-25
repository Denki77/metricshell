# INV-018 Report — Legacy Client and Transport Viability

**Status:** completed

**Run dates:** 2026-09-24

**Reference run:** `results/20260924T195359Z`

**Ubuntu-host confirmation:** `results/20260924T200230Z` — confirmed

**Decision:** [ADR-018](../../docs/06-architecture/adr/ADR-018.md)

## Goal

Determine whether PHP 5.4, shell and CLI workloads can submit individual Managed Aggregation operations without complete
registry state, and select an initial local operation transport and framing contract.

## Prototype

- `prototype/cmd/inv018` — server, short-lived CLI and persistent generator.
- `clients/metricshell.php` — PHP 5.4-compatible stateless client.
- `clients/Dockerfile` — pinned PHP image containing the client, avoiding runtime host bind mounts.
- `compose.yml` — stand using a named runtime volume.
- `run-bench.sh` — reproducible runner, diagnostics, assertions and fingerprint.

The registry is research-only. Core validation and production Managed Registry remain outside this transport prototype;
ADR-016 is authoritative for registry semantics.

## Run Environments

| Environment                   | Docker | Container platform       | Result                     | Assertions | Fingerprint                                                        |
|-------------------------------|-------:|--------------------------|----------------------------|-----------:|--------------------------------------------------------------------|
| macOS host / Docker LinuxKit  | 29.8.0 | LinuxKit 7.0.12, aarch64 | `results/20260924T195359Z` |      41/41 | `a5a256f20b747d561c24f4d3b6132c3bb486c21d23ccba6f464277d86033fb7d` |
| Ubuntu host / Docker LinuxKit | 27.4.0 | LinuxKit 6.10.14, x86_64 | `results/20260924T200230Z` |      41/41 | `a5a256f20b747d561c24f4d3b6132c3bb486c21d23ccba6f464277d86033fb7d` |

Both used PHP 5.4.45, 6 daemon CPUs and approximately 8 GB daemon memory. The Ubuntu-host result is not native
Ubuntu-kernel evidence. The PHP base is pinned by digest in `clients/Dockerfile`.

## Correctness Results

Both environments passed the same 41/41 assertions and all persistent matrix cells with zero operation errors.

| Experiment                 | Result | Evidence                                                                |
|----------------------------|--------|-------------------------------------------------------------------------|
| E-018.1 PHP 5.4            | pass   | increment/set/observe and accepted/transport/protocol/rejected outcomes |
| E-018.2 Multiprocess       | pass   | four independent processes, exact increments, no shared PHP registry    |
| E-018.3 CLI/permissions    | pass   | short-lived helper; allowed access and denied unprivileged UID/GID      |
| E-018.4 Startup            | pass   | endpoint ready before clients; pre-submit retry bounded                 |
| E-018.5 Reconnect/restart  | pass   | exact reconnect accumulation; restart created a new empty epoch         |
| E-018.6 Protocol/transport | pass   | parity, framing bounds, malformed/partial frames and version cases      |

The prototype accepted 65,536 bytes and rejected 65,537 bytes. This proves bounded framing, not the correct production
limit. Exact limits remain INV-019 scope.

## Performance Observations

`benchmarks.tsv` includes Compose exec and process startup per operation. It is an end-to-end short-lived helper
observation and cannot compare transport throughput.

The persistent generator starts once per cell. Profile A uses a new connection per operation; profile B reuses HTTP
connections. Unix reuse was not added because the selected Unix contract is one operation per connection.

| Environment        | Transport/profile  | 1-client ops/s | 128-client ops/s | 128-client ACK p95 | Errors |
|--------------------|--------------------|---------------:|-----------------:|-------------------:|-------:|
| macOS-host ARM64   | Unix connection/op |        8,419.9 |         31,242.6 |           9.856 ms |      0 |
| macOS-host ARM64   | HTTP connection/op |        3,950.1 |         32,800.3 |           9.350 ms |      0 |
| macOS-host ARM64   | HTTP reused        |        9,543.6 |         47,795.6 |           7.027 ms |      0 |
| Ubuntu-host x86_64 | Unix connection/op |        3,561.3 |          8,494.0 |          40.698 ms |      0 |
| Ubuntu-host x86_64 | HTTP connection/op |        1,939.5 |         10,172.0 |          29.991 ms |      0 |
| Ubuntu-host x86_64 | HTTP reused        |        3,566.0 |         15,016.9 |          22.294 ms |      0 |

These observations lack CPU pinning and controlled cgroups. They are not guarantees, limits or the primary reason for
the transport decision.

## Transport Evaluation

| Criterion         | Unix domain stream socket                | Local HTTP/TCP                               |
|-------------------|------------------------------------------|----------------------------------------------|
| PHP 5.4           | built-in `stream_socket_client`          | built-in TCP streams; HTTP handling required |
| Shell/CLI         | short-lived helper                       | curl convenient but not guaranteed           |
| Locality/security | filesystem endpoint and owner/group/mode | address, listener and network policy surface |
| Network exposure  | none                                     | TCP listener required                        |
| Framing           | simple bounded newline-delimited JSON    | standard HTTP framing and bounded body       |
| Debugging         | less convenient than curl                | curl/wget friendly                           |
| Viability         | all cases passed                         | all cases passed                             |

Unix is selected primarily because Managed Aggregation is a local same-workload boundary. Its filesystem security,
absence of a TCP listener, PHP 5.4 compatibility and operational simplicity fit the product model. HTTP is viable but
does not improve the required initial use case enough to justify a second mandatory transport.

## Protocol and Client Conclusions

- Initial version `1` is explicit. Supported version is accepted; missing, invalid and unsupported versions reject.
- Unix framing is bounded newline-delimited JSON, one operation per connection and one JSON response.
- Legacy clients keep no complete snapshot, registry or cross-process registry state.
- Outcomes distinguish accepted, transport/connect, protocol/framing and semantic/server rejection.
- The transport exposes an explicit response boundary. ADR-017 defines the exact successful-ACK registry commit semantics.
- Retry is safe before definite submission. Missing ACK after possible submission means unknown outcome; no blind retry
  is allowed without ADR-017 idempotency semantics.
- Endpoint readiness precedes publication. Prototype retries, delays and timeouts are not defaults.
- Reconnect does not rebuild state; a new MetricShell process starts a new empty epoch.
- Filesystem permissions are required, but prototype mode `0660` is not a production requirement.

## Limitations and Follow-up

- Confirmation used an Ubuntu host with Docker LinuxKit; native Ubuntu Docker Engine/kernel remains untested.
- INV-019 owns frame size, connection/queue limits, CPU/RSS budgets and thresholds.
- INV-020 owns in-flight freeze, final state and lifecycle integration.
- Production UID/GID/mode and deployment configuration remain specification/delivery work.

## Conclusion

INV-018 is complete. Unix domain stream socket is the initial local Managed Aggregation transport with a versioned,
bounded, one-operation/one-response protocol and stateless legacy clients. HTTP remains a tested viable alternative for
possible future scope. The decision is recorded in [ADR-018](../../docs/06-architecture/adr/ADR-018.md).
