# INV-019 Report — Performance, Snapshot Materialization and Resource Limits

Status: in progress

Run date: 2026-09-25

Reference run: `results/20260925T090317Z`

Reference environment: Docker Desktop 29.8.0, LinuxKit 7.0.12, linux/aarch64, 2-CPU container limit, 512 MiB memory limit

Result: 20/20 portable assertions passed; all seven required experiments and the additional local benchmarks completed

Ubuntu confirmation: pending; ADR-019 is intentionally not created yet

## Goal and Evidence Rule

INV-019 evaluates the representative ADR-016/017 managed registry: explicit bounded state, one serialized commit order
and complete registry-wide snapshots. Assertions establish portable safety properties. Throughput, latency, CPU, RSS,
allocation and scheduler-dependent sample counts are observations and are not portable promises.

The run used one portable source/runner fingerprint. The reference result keeps the research in progress until that
fingerprint is confirmed on Ubuntu.

## Candidates

| Candidate                        | Mutation cost                          | Scrape behavior                                      | Disposition before Ubuntu confirmation                  |
|----------------------------------|----------------------------------------|------------------------------------------------------|---------------------------------------------------------|
| Re-encode after every mutation   | full registry encoding per operation   | pre-encoded                                          | reject; cost scales with operation rate and cardinality |
| Materialize on scrape            | minimal mutation                       | full encode for every scrape                         | viable fallback; repeated unchanged scrapes repeat work |
| Generation-based immutable cache | minimal mutation plus dirty generation | encode once per eligible generation; immutable reads | preferred                                               |
| Copy-on-write state              | registry clone per operation           | immutable state is easy to expose                    | reject for the tested single-owner registry             |

The preferred shape is not publication of every intermediate generation. The owner advances committed state; snapshot
materialization publishes the newest eligible complete generation and caches its immutable encoding. A later mutation
marks the cache stale. Concurrent scrapers retain their immutable response even as the registry advances.

## Experiments and Results

### E-019.1 — Operation throughput

The complete 100/1,000/10,000-series × 1/10/100-publisher matrix accepted all 180,000 attempted mixed
counter/gauge/histogram mutations. No accepted operation was lost. Observed rates ranged from 3.52 to 7.46 million
in-process operations/s. Across cells, p50 was 0.000042–0.000250 ms and p99 0.000084–0.001125 ms.

These very small values measure only the Go owner-state critical section. They exclude Unix transport, JSON parsing,
queue residence and ACK delivery and therefore must not be used as an end-to-end capacity claim. Queue-depth behavior
is measured independently in E-019.6.

### E-019.2 — Cardinality scaling

| Series | Retained allocation | Encoded bytes | Encode time | Core-like install |
|-------:|--------------------:|--------------:|------------:|------------------:|
|    100 |             9,408 B |       9,519 B |    0.069 ms |          0.001 ms |
|  1,000 |            89,408 B |     100,719 B |    0.696 ms |          0.004 ms |
| 10,000 |           885,312 B |   1,057,719 B |    8.017 ms |          0.133 ms |
| 20,000 |         1,762,336 B |   2,170,951 B |   19.019 ms |          0.781 ms |

Retained prototype allocation stabilized at about 88–94 B/series and encoded output at 95–108 B/series for this mixed
schema. A 20,001st series rejected before construction with `series_limit`; the existing generation remained unchanged.
The bytes/series are workload-shape observations, not a universal formula because labels, names and buckets vary.

### E-019.3 — Histogram cost

The runner crossed 100/1,000/5,000 series with 1/10/50/100 buckets. At 5,000 mixed series, encoded output grew from
167,757 B at one bucket to 4,242,914 B at 100 buckets; retained construction allocation grew from 313,472 B to
1,779,552 B. A declaration with 101 buckets rejected before registry allocation and did not mutate state.

This confirms that both active histogram series and bucket count require independent bounds. A series limit alone does
not bound encoded size.

### E-019.4 — Snapshot strategy comparison

| Strategy                | Series | Operations | Mutation time | Encoding time |
|-------------------------|-------:|-----------:|--------------:|--------------:|
| Re-encode each mutation |  1,000 |      2,000 |      0.549 ms |  1,568.668 ms |
| Materialize on scrape   |  1,000 |      2,000 |      0.026 ms |      0.711 ms |
| Generation cache        |  1,000 |      2,000 |      0.026 ms |      0.704 ms |
| Copy on write           |  1,000 |      2,000 |     47.072 ms |      0.787 ms |
| Re-encode each mutation | 10,000 |        200 |      0.104 ms |  1,858.526 ms |
| Materialize on scrape   | 10,000 |      2,000 |      0.031 ms |      8.600 ms |
| Generation cache        | 10,000 |      2,000 |      0.027 ms |      8.504 ms |
| Copy on write           | 10,000 |        200 |     62.610 ms |      8.249 ms |

The different operation counts at 10,000 series intentionally cap the two pathological candidates; the raw TSV keeps
the denominator explicit. Encoding per mutation spent roughly 1.86 s for only 200 mutations at 10,000 series. Copy on
write spent 62.6 ms cloning for 200 mutations. On-scrape and generation-cache mutation cost remained negligible and
one materialization cost approximately 8.5–8.6 ms. The cache wins architecturally because an unchanged generation can serve the same
immutable bytes repeatedly; a materialize-on-every-scrape policy cannot reuse that work.

Core-like installation was measured separately from encoding and took 0.001–0.781 ms across the matrix. The larger
values remain allocation/scheduler observations. A malformed candidate was rejected while the prior active immutable
response remained unchanged.

### E-019.5 — Concurrent scrape under mutation

The writer committed one generation by atomically setting linked counter, gauge, histogram-count and histogram-sum
markers to the same value. Materialization ran through the selected generation cache and encoded the generation header
plus all four marker families. Four concurrent readers validated every field; one reader retained old byte slices for
200 microseconds while the writer committed and the cache published newer generations.

The run validated 142,901 complete responses, including 19 delayed-reader completions, across 10,899 committed
generations. Every response had `header == counter == gauge == histogram_count == histogram_sum`: zero mixed-generation
responses. Both cache paths were exercised (142,872 hits and 31 misses). This proves the tested cache path preserves one
complete linked registry generation rather than merely attaching a correct header to a potentially mixed body. It does
not imply that every intermediate generation is exposed.

### E-019.6 — Backpressure and overload

| Queue capacity | Attempted | Accepted | Rejected | Maximum depth |
|---------------:|----------:|---------:|---------:|--------------:|
|              1 |    10,000 |        1 |    9,999 |             1 |
|             16 |    10,000 |       16 |    9,984 |            16 |
|             64 |    10,000 |       64 |    9,936 |            64 |
|          1,024 |    10,000 |    1,024 |    8,976 |         1,024 |

Consumption was gated so the boundary is exact. Every admitted item remained present and every excess offer received a
visible rejection; memory did not grow beyond capacity. This matches ADR-017. It does not add an equal-share admission
guarantee.

### E-019.7 — Controlled resource exhaustion

- Normal policy boundary: 20,000 series and 100 buckets were accepted; 20,001 series and 101 buckets rejected normally.
- Fatal memory boundary: allocation of 128 MiB inside a 32 MiB cgroup exited 137 and Docker recorded
  `OOMKilled=true`. This is observably different from a normal policy rejection.
- FD boundary: the complete benchmark exited 0 with `nofile=64`; the stand does not require descriptors proportional to
  operations or cardinality.
- Main run: the retained 2-CPU/512-MiB container completed in 5,473 ms. Sampled memory peaked at approximately
  24.3 MiB. Coarse Docker sampling can miss short peaks, so it is not used as a hard bound.

## Additional Benchmarks Executed

No listed INV-019 variant was replaced by a future-work agreement. The run includes every candidate snapshot strategy,
all required publisher and cardinality classes, counter/gauge/histogram mixed traffic, four queue capacities, four
histogram bucket widths, concurrent mutation plus delayed scrape, the exact series and bucket boundary plus one,
immutable Core-like active-state preservation, a low-FD run, fixed CPU/memory limits and a fatal cgroup OOM. Raw results
retain positive and negative outcomes.

The superseded reference set with the weaker header-only E-019.5 assertion was removed; only the current full-body
linked-generation evidence set is retained.

## Provisional Acceptable Values and Policies

The architecture requires bounded, independently configurable limits, but this run does not contain a product memory
budget, maximum response budget, materialization-latency objective, queue-residence objective or agreed safety margin.
Consequently it cannot justify exact production defaults. The supported conclusions pending Ubuntu and ADR-019 are:

- snapshot strategy: generation-based immutable encoded cache;
- series cardinality: bounded and tested through 20,000; exact default and maximum are deferred;
- finite buckets per histogram: bounded and tested through 100; exact default and maximum are deferred;
- owner queue capacity: bounded and tested at 1, 16, 64 and 1,024; exact default and maximum are deferred;
- tested publisher envelope: 1–100 here and 1–128 in INV-017/018; no promise is inferred for publisher 129;
- successful ACK: only after owner commit, unchanged from ADR-017;
- overload: immediate observable rejection at the bounded admission edge; no silent drop;
- snapshot publication: coalesce mutations into the newest eligible complete generation; do not promise every
  intermediate generation;
- response ownership: immutable bytes remain valid until every concurrent scrape using them completes;
- limits fire before allocating the rejected series or bucket vector and preserve the previous committed generation;
- fatal cgroup OOM is a process/container failure, not a recoverable managed-operation rejection.

The 20,000/100/1,024 values are coverage endpoints, not proposed production ceilings or universal safe maxima. Exact
defaults require an explicit selection rule connecting representative workloads to memory and encoded-response budgets,
materialization and queue-residence targets, and a documented safety margin. Label lengths, family counts, scrape
frequency and host memory remain necessary inputs to that rule.

## Stand Fingerprint and Ubuntu Procedure

The portable fingerprint is:

```text
2a37bb25d9d3c09167d97d8861a42e4902e0874b903fd00f6896e4328ea97112
```

`export-stand.sh` packages the exact source, runner, verifier and expected fingerprint. On Ubuntu, unpack the archive,
run `./verify-fingerprint.sh`, then run the same `./run-bench.sh` command. Keep both result sets. Compare portable
assertion names/results and the fingerprint exactly; compare performance distributions as observations rather than
requiring equal timings. Architecture-specific image IDs must differ legitimately and are provenance only.

## Prototype Limits and Better Benchmarking

- Repeat at least 30 times on an idle native Ubuntu kernel with pinned CPUs, fixed governor and identical cgroup limits.
- Add end-to-end INV-018 Unix transport, framing, parse, enqueue-to-commit and commit-to-ACK timestamps.
- Replace Core-like byte installation with the production validator/atomic active-state implementation.
- Retain heap/CPU profiles, GC pauses and one-second cgroup `memory.current`/`memory.peak`; sample Docker stats more often.
- Sweep scrape frequencies and multiple simultaneous slow/aborting scrapers while rate-controlling publishers at
  50/80/100/120% of observed capacity.
- Add long labels/names, multiple families, skewed hot series and maximum bucket histograms; encoded bytes/series from
  this synthetic schema cannot stand in for those shapes.
- Run race detection and multi-hour soak tests at 10,000 and 20,000 series on native AMD64 and ARM64.
- Test connection/publisher FD limits through the selected Unix transport; `nofile=64` here proves the internal stand is
  bounded, not that 64 concurrent client sockets are sufficient.

## Conclusion

The reference evidence supports the hypothesis and provisionally selects generation-based immutable snapshot caching
with independent limits for series, histogram buckets and queued work. Re-encode-per-mutation and whole-registry
copy-on-write are rejected as tested. Normal limit rejection is safe and distinct from cgroup OOM. Exact production
defaults remain deferred because the required budget/latency/safety-margin selection inputs are not yet fixed.

INV-019 remains **in progress** until the identical fingerprint is run on Ubuntu and ADR-019 records the final limits.
