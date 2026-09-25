# INV-018 — Legacy Client and Transport Viability

**Status:** completed

**Reference run:** `results/20260924T195359Z`

**Ubuntu-host confirmation:** `results/20260924T200230Z` — confirmed

**Report:** [report.md](report.md)

**Decision:** [ADR-018](../../docs/06-architecture/adr/ADR-018.md)

## Question

Can PHP 5.4, shell and simple CLI workloads submit Managed Aggregation operations without maintaining a complete
registry, and which initial local transport/protocol is justified?

## Evidence and Portable Confirmation

The macOS-host/LinuxKit ARM64 reference run and Ubuntu-host/Docker LinuxKit x86_64 confirmation run both passed 41/41
correctness assertions and all 12 persistent transport-matrix cells with zero operation errors. Both used real PHP
5.4.45 and the same benchmark fingerprint:

```text
a5a256f20b747d561c24f4d3b6132c3bb486c21d23ccba6f464277d86033fb7d
```

The Ubuntu-host run used Docker 27.4.0 with a LinuxKit 6.10.14 x86_64 container kernel. It is cross-host and
cross-architecture confirmation, not native Ubuntu-kernel evidence. Timing differences are observations only.

## Final Conclusions

- The initial local transport is a Unix domain stream socket. It matches the same-workload boundary, provides filesystem
  ownership/group/mode control, needs no TCP listener or network configuration, works with PHP 5.4 stream APIs and fits
  the one-binary/one-process MetricShell model.
- The initial Unix protocol uses version `1`, bounded newline-delimited JSON, one operation per connection and one JSON
  response. Missing, invalid and unsupported versions and malformed, partial or over-bound frames reject deterministically.
- Frames must be bounded. The prototype's 64 KiB boundary is research evidence only; INV-019 selects production limits.
- Legacy clients are stateless with respect to Managed Registry: no complete snapshot, local metric registry or
  cross-process registry state is required.
- Clients distinguish accepted, transport/connect failure, protocol/framing failure and semantic/server rejection.
  Prototype numeric exit codes are not an architecture contract.
- The endpoint is ready before supervised workload clients publish. Bounded retry is safe only before definite
  submission. Missing ACK after possible submission is an unknown outcome; ADR-017 defines idempotency and exact
  successful-ACK commit semantics.
- Reconnect does not reconstruct registry state. A new MetricShell process starts a new empty epoch, consistent with
  ADR-016.
- Local HTTP was tested successfully and is technically viable, but is not an initial mandatory transport. It remains a
  possible future adapter if product scope changes.

Performance was not the primary selection criterion. Rates and ACK percentiles are observations, not guarantees or SLAs.

## Running the Stand

The command is identical on macOS and Ubuntu hosts and requires no runtime host bind mount:

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

Increase observations with `INV018_REPETITIONS=1000` and `INV018_TRANSPORT_OPERATIONS_PER_CLIENT=1000`.

## Limits and Follow-up

- The mutex registry is a transport stand, not production Managed Registry. ADR-016 controls registry semantics;
  ADR-017 controls concurrency, ordering, idempotency and successful-ACK meaning.
- `benchmarks.tsv` is an end-to-end short-lived helper observation, not a transport throughput benchmark.
- The persistent generator covers Unix/HTTP at 1/8/32/128 clients. INV-019 owns CPU/RSS sizing, controlled cgroups,
  frame/connection limits and performance thresholds; INV-020 owns lifecycle integration.
- Socket mode `0660` proved positive/negative filesystem access but is not a mandatory production mode.
- Native Ubuntu Docker Engine/kernel coverage was not collected; Ubuntu-host confirmation used Docker LinuxKit.

## Decision Output

- Prototype and CLI: `prototype/`
- PHP 5.4 reference client: `clients/metricshell.php`
- Stand and runner: `compose.yml`, `run-bench.sh`
- Reference evidence: `results/20260924T195359Z/`
- Ubuntu-host confirmation: `results/20260924T200230Z/`
- Detailed analysis: [report.md](report.md)
- Accepted decision: [ADR-018](../../docs/06-architecture/adr/ADR-018.md)
