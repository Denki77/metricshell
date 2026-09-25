# INV-019 — Performance, Snapshot Materialization and Resource Limits

**Status:** in progress

**Reference run:** `results/20260925T090317Z`

**Ubuntu confirmation:** pending

**Report:** [report.md](report.md)

## Question

Can the bounded single-owner Managed Registry selected by INV-017 sustain practical operation rates and cardinality
while continuously publishing consistent complete snapshots with bounded resources?

## Current Result

The macOS-host Docker Desktop/LinuxKit ARM64 reference run passed 20/20 portable assertions and all seven INV-019
experiments. It confirms the initial hypotheses: encoding after every mutation and cloning the registry on every
mutation scale poorly; cardinality and histogram buckets dominate retained and encoded size; immutable
generation-cached responses preserve a single generation during concurrent mutation and slow reads.

This is not yet a completed investigation. The same fingerprint must pass on the Ubuntu host before ADR-019 and final
limits are accepted.

## Run the Prototype

The command is identical on macOS and Ubuntu and requires Docker, `tar`, and standard POSIX shell tools:

```bash
./research/INV-019/run-bench.sh
latest="research/INV-019/$(cat research/INV-019/latest-results.txt)"
cat "$latest/summary.tsv"
cat "$latest/assertions.tsv"
cat "$latest/environment.tsv"
```

The runner builds the image, executes the complete matrix with 2 CPUs and 512 MiB, samples container CPU/RSS, then
runs a 32 MiB cgroup OOM injection and a complete run with `nofile=64`. It uses `docker create/start/cp`; host source and
result paths are not bind-mounted into the container.

## Ubuntu Stand Fingerprint

Export and verify the exact stand:

```bash
./research/INV-019/export-stand.sh /tmp/inv019-stand.tar.gz
mkdir /tmp/inv019-stand
tar -xzf /tmp/inv019-stand.tar.gz -C /tmp/inv019-stand
cd /tmp/inv019-stand
./verify-fingerprint.sh
./run-bench.sh
```

Expected portable source/runner fingerprint:

```text
2a37bb25d9d3c09167d97d8861a42e4902e0874b903fd00f6896e4328ea97112
```

The image ID is recorded but is not the portable identity because ARM64 and AMD64 images contain different machine
code. The fingerprint covers the Dockerfile, Go source, module file, runner, exporter and verifier.

## Evidence Files

- `throughput.tsv` — 100/1,000/10,000 series crossed with 1/10/100 publishers.
- `cardinality.tsv` — retained allocation, encoded size and split materialization/Core/scrape timing through 20,000
  series.
- `histograms.tsv` — 100/1,000/5,000 series crossed with 1/10/50/100 buckets.
- `snapshot-strategies.tsv` — every candidate strategy at 100/1,000/10,000 series.
- `concurrent-scrape.tsv` — full-body linked-family generation checks through the immutable cache under sustained
  mutation, concurrent readers and a delayed reader.
- `backpressure.tsv` — exact admission at capacities 1/16/64/1,024.
- `limits.tsv`, `resource-exhaustion.tsv`, `fd-limit.tsv` — policy rejection, fatal OOM and FD-bound behavior.
- `container-stats.tsv`, `environment.tsv`, `container.inspect.json` — runtime and provenance evidence.

## Prototype Limits

- It is an in-container registry/materialization microbenchmark, not an end-to-end Unix-socket client benchmark.
- Operation rates exclude transport, parsing and ACK costs; INV-018 retains those measurements.
- Core installation is represented by immutable byte ownership and validity gating, not the production Core parser.
- Docker stats sampling is coarse and may miss short peaks; Go allocation counters provide the fine-grained comparison.
- The reference host is LinuxKit ARM64, not native Linux. Ubuntu confirmation is deliberately still pending.
- Timing and resource observations are environment-sensitive; only semantic assertions are portable.
- The tested 20,000-series, 100-bucket and 1,024-queue endpoints are coverage, not production defaults. Exact defaults
  require an explicit memory/response/latency/safety-margin selection rule and remain deferred.

## Better Follow-up Benchmarking

After portable confirmation, repeat on an otherwise idle native Ubuntu host with pinned CPUs, fixed governor and at
least 30 repetitions; retain raw distributions. Add end-to-end Unix framing/parser/ACK cost, production Core validation,
GC pause histograms, heap profiles, high-frequency scrape rates, response-write cancellation, publisher connection
limits and a sustained 50/80/100/120% offered-load soak. Test cgroup v1 and v2, native AMD64 and ARM64, and run the Go
race detector at the largest retained matrix. These improve capacity estimates; they do not replace the complete local
candidate matrix already executed.
