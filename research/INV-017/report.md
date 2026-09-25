# INV-017 Report — Concurrent Publishers and Ordering

Status: completed

Run date: 2026-09-24

Run: `results/20260924T194912Z`

Reference evidence: `results/20260924T194912Z/reference`

Extended evidence: `results/20260924T194912Z/extended`

Ubuntu confirmation run: `results/20260924T195921Z`

Reference environment: Docker Desktop 29.8.0, LinuxKit 7.0.12, linux/aarch64, 6 CPUs, 8.32 GB daemon memory

Confirmation environment: Docker Desktop 27.4.0, LinuxKit 6.10.14, linux/x86_64, 6 CPUs, 8.06 GB daemon memory

Result: both environments passed reference 136/136, extended 311/311 and race detector PASS/0 reported races

Decision: [ADR-017](../../docs/06-architecture/adr/ADR-017.md)

## Goal and Evidence Rule

INV-017 tests concurrent mutation, ordering, declaration races, incomplete delivery, duplicate delivery and overload on
the ADR-016 managed-registry semantic boundary. Assertions are portable correctness evidence. Throughput, latency,
fairness sample counts and overload acceptance counts are environment-sensitive observations.

The macOS and Ubuntu runs share the exact benchmark fingerprint. Portable assertions select the architecture through
ADR-017; throughput, latency, snapshot sample counts and scheduler-sensitive fairness observations remain
environment-specific measurements.

## Candidates

| Candidate              | Prototype synchronization                        | Main property tested                  | Current disposition               |
|------------------------|--------------------------------------------------|---------------------------------------|-----------------------------------|
| Serialized loop        | one owner, bounded channel                       | direct commit order and backpressure  | selected                          |
| Global lock            | one mutex around mutable state                   | simplest shared-state baseline        | viable fallback                   |
| Sharded locks          | 16 counter shards plus metadata lock             | reduced counter contention            | reject as tested: mixed snapshots |
| Atomics + family locks | atomic counter/gauge, histogram/descriptor locks | family-local synchronization          | reject as tested: mixed snapshots |
| Copy-on-write          | mutex plus state clone per mutation              | immutable candidate publication shape | viable fallback; copying cost     |

All candidates passed operation-local invariants. E-017.8 then rejected the tested sharded and atomic-family snapshot
strategies: family-local synchronization did not yield a complete registry state from one generation. Adding a global
snapshot publication barrier could repair them, but that is a materially different synchronization design and removes
their demonstrated simplicity advantage. The serialized loop owns operation order and snapshot order directly.

## Experiments and Results

### E-017.1 — Concurrent counter increments

Each candidate ran 1, 2, 8, 32 and 128 publishers. The reference used 1,000 increments per publisher and three
repetitions; the extended run used 10,000 and ten. All 325 final-sum assertions across both runs matched the exact
accepted count; no increment was lost or duplicated internally.

Mean observed throughput (ops/s):

| Candidate            | 1 publisher | 8 publishers | 32 publishers | 128 publishers |
|----------------------|------------:|-------------:|--------------:|---------------:|
| Serialized loop      |   1,905,917 |    1,416,254 |     1,257,207 |      1,303,295 |
| Global lock          |  10,671,420 |    4,619,731 |     4,024,751 |      4,144,925 |
| Sharded locks        |   9,360,621 |    5,100,969 |     4,753,507 |      4,591,531 |
| Atomics/family locks |  10,161,991 |    6,659,281 |     6,765,537 |      6,794,707 |
| Copy-on-write        |   7,123,604 |    4,739,041 |     3,548,050 |      3,124,867 |

These are in-container microbenchmarks, not end-to-end client throughput or a production capacity commitment.

### E-017.2 — Concurrent gauge SET

Thirty-two publishers sent 100 unique values each. For every candidate, every accepted SET received a commit number and
the final gauge equaled the value attached to the highest committed SET. No torn value was observed. This supports SET
under a defined commit order. It does not select concurrent gauge ADD/SUB as an external operation.

### E-017.3 — Concurrent histograms and snapshots

Thirty-two publishers produced 3,200 observations per candidate. Final count and `+Inf` bucket both equaled 3,200.
While writers were active, the extended run sampled 189–1,914 snapshots per candidate; every sample preserved
cumulative bucket ordering and `+Inf == count`. No partial observation was visible.

### E-017.4 — Descriptor creation race

Sixty-four compatible/incompatible declarations raced for one name. Exactly one complete descriptor shape won;
compatible repeats converged and incompatible declarations rejected. No mixed descriptor was observed. The specific
winner is scheduler-dependent, but the winner rule is deterministic: first accepted declaration fixes the descriptor.

### E-017.5 — Partial frame and crash points

The internal length-prefixed adapter was cut before every header boundary and inside the payload. Only the complete
12-byte frame mutated state. A complete send followed by simulated ACK loss mutated exactly once internally. The sender
cannot infer that outcome without a receipt; this is the required unknown-outcome case, not permission to retry blindly.

### E-017.6 — Duplicate delivery

- Without an idempotency key, replay is another valid increment and the value becomes two.
- With a publisher-scoped operation key retained in a dedupe table, replay returns the stored outcome and applies once.
- If ACK is lost and no reusable key is available, the result is explicitly unknown; automatic blind retry is unsafe.

Retryable mutations require stable publisher/session identity plus operation identity sufficient for deduplication.
Wire representation, ID format/width, retention limits and reconnect/session semantics remain INV-018/019 scope.

### E-017.7 — Backpressure

The serialized candidate was overloaded through a non-blocking enqueue policy:

| Queue capacity | Attempted | Accepted | Rejected |
|---------------:|----------:|---------:|---------:|
|              1 |    10,000 |        1 |    9,999 |
|             16 |    10,000 |       16 |    9,984 |
|             64 |    10,000 |       64 |    9,936 |
|          1,024 |    10,000 |    1,024 |    8,976 |

The gated overload run proves the exact capacity bound: before consumption starts, exactly `capacity` entries are
accepted and every remaining attempt rejects without mutation. The acceptable policy is `accepted` or immediate
`overloaded` at the enqueue boundary. Blocking may be offered only with an explicit client deadline; an unbounded queue
is rejected.

## Additional Benchmarks Executed

The reference and extended runners executed the full candidate/publisher/repetition cross-product, per-connection monotonic-order checks,
concurrent snapshot sampling, queue capacities 1/16/64/1,024, incomplete header/payload cuts, ACK-loss replay policies,
compatible/incompatible declaration races, a 9,600-operation mixed-family workload, every 0–12 byte frame cut and a
32-publisher overload-fairness burst. No listed INV-017 experiment was replaced by a future-work agreement.

An independent `go run -race` Docker execution (100 operations/publisher, one repetition) completed with no reported
data races; its outcome is retained in `race/race-detector.tsv`. This is useful fault detection, not a proof of race freedom.

### E-017.8 — Registry-wide snapshot linearizability

ADR-004 defines a complete candidate as application state at one publication point, and ADR-016 requires managed state
to cross that existing complete-snapshot boundary. Therefore a Managed Registry complete snapshot must correspond to
one registry-wide linearization point. Internal consistency of one histogram family is necessary but insufficient.

The prototype applied 100,000 linked mutations that set counter and gauge markers to the same generation while reading
complete snapshots concurrently:

| Candidate            | Snapshots | Mixed generations | Linearizable as tested |
|----------------------|----------:|------------------:|------------------------|
| Serialized loop      |    88,581 |                 0 | yes                    |
| Global lock          |    12,533 |                 0 | yes                    |
| Sharded locks        |   195,198 |           143,720 | no                     |
| Atomics/family locks |   566,968 |           516,340 | no                     |
| Copy-on-write        |    11,135 |                 0 | yes                    |

This answers the architectural question: yes, one registry-wide point is required by composition with the accepted Core
boundary. ADR-004 and ADR-016 are unchanged. The negative result applies to the concrete candidates tested, not to every
possible sharded or atomic implementation.

### E-017.9 — Admission fairness contract

FR-MA-006 requires multiple cooperating publishers. FR-MA-014, NFR-MA-005 and NFR-PERF-005 require bounded queued work
and bounded overload behavior. NFR-MA-004 requires deterministic semantics for the accepted order. None requires equal
admission share or protection from starvation under overload, and guaranteed history/delivery of every event is
explicitly out of scope.

The overload burst retained the negative evidence: 64 of 32,000 offers were admitted, minimum per publisher was 0,
maximum was 64 and Jain index was `0.031250`. Consequently the research does not add a scheduler solely to improve that
observation. The selected contract is:

> MetricShell guarantees bounded admission and deterministic processing of accepted mutations. MetricShell does not
> guarantee equal admission share between concurrent publishers under overload. A publisher may receive overload
> rejection while another publisher continues to receive admission.

Consequences: memory does not grow without bound; acceptance/rejection is visible; accepted operations retain commit
semantics; starvation and equal share are not service guarantees. Jain index remains an observation, not a requirement.

## Acceptable Values and Policies

- Correctness envelope demonstrated: 1–128 concurrent publishers and 1,000 operations per publisher.
- Reference bounded queue: 64 entries; demonstrated capacities: 1, 16, 64 and 1,024. No production default is selected.
- Mutation acceptance point: successful enqueue followed by owner-loop commit; ACK is emitted only after commit.
- Order: preserve each connection's receive order and assign one registry-wide monotonically increasing commit number.
- Gauge SET: final value is the value at the highest committed SET.
- Histogram observation: count, sum and every applicable cumulative bucket are one mutation.
- Descriptor race: first accepted descriptor wins; exact repeats succeed; conflicts reject without state change.
- Retry: only with an idempotency key; otherwise lost ACK produces an explicit unknown outcome.
- Complete snapshots: one registry-wide committed generation is mandatory; family-local consistency alone is invalid.
- Overload: bounded queue with explicit visible rejection; optional blocking requires a deadline. Equal-share admission
  and starvation protection are not guaranteed by current requirements.

The 1–128 range is tested coverage, not a promise to support exactly 128 connections or reject 129. Queue capacity 64
is a research setting, not a production default.

## Stand Fingerprint and Ubuntu Procedure

The benchmark fingerprint covers every prototype file, `run-bench.sh` and `run-research.sh`:

```text
22bc1820e394d9e7331be2857ca2298376b0d410ce52728833f1ca2f04792697
```

`export-stand.sh` packages that scope plus the expected fingerprint and verifier. Ubuntu uses the identical
`./run-research.sh` command, which stores reference, extended and race-detector evidence under one timestamped directory
without deleting any result set. On failure it retains `failure.tsv` and phase logs and prints diagnostic log tails.
Evidence cleanup remains an explicit maintainer action, so Ubuntu confirmation cannot delete the macOS result set. The
image ID is intentionally not the portable identity because ARM64 and
AMD64 images contain different machine code; source/runner fingerprint equality is the valid cross-platform identity.

## Prototype Limits and Better Benchmarking

- Run `INV017_OPS_PER_PUBLISHER=10000 INV017_REPETITIONS=30` on an otherwise idle host with fixed Docker CPU/memory
  limits; retain raw TSV rather than only percentiles.
- Repeat the Go race-detector at extended scale, add sanitizer images, repeated descriptor/fault races, and native Linux
  in addition to LinuxKit.
- Measure allocations, RSS, GC pauses, queue residency, enqueue-to-commit and commit-to-ACK latency separately.
- Add mixed counter/gauge/histogram families, label cardinality, histogram bucket counts and concurrent Core snapshot
  materialization; INV-019 owns production limits.
- Add sustained, rate-controlled publishers and Jain fairness observations at 50/80/100/120% offered load if future
  requirements introduce an admission-fairness guarantee.
- Fault a real local transport with independent processes at each byte boundary and connection teardown. The present
  adapter proves mutation gating, not a transport selection; INV-018 owns that choice.
- Test idempotency-table eviction, publisher restart/session rollover, key collision and memory bounds before selecting a
  retention window.

## Portable Confirmation

| Evidence                                     |   macOS/LinuxKit ARM64 | Ubuntu/LinuxKit x86_64 | Portable result        |
|----------------------------------------------|-----------------------:|-----------------------:|------------------------|
| Fingerprint                                  |       `22bc1820…92697` |       `22bc1820…92697` | exact match            |
| Reference assertions                         |                136/136 |                136/136 | match                  |
| Extended assertions                          |                311/311 |                311/311 | match                  |
| Race detector                                |               PASS / 0 |               PASS / 0 | match                  |
| Serialized/global-lock/COW mixed generations |                      0 |                      0 | linearizable as tested |
| Sharded/atomic-family mixed generations      |               observed |               observed | rejected as tested     |
| Overload accepted/min/max/Jain               | 64 / 0 / 64 / 0.031250 | 64 / 0 / 64 / 0.031250 | same negative witness  |

Snapshot sample counts and performance distributions differed across environments, as expected. They are observations,
not portable acceptance criteria, and do not affect the selected correctness model.

## Conclusion

The matching-fingerprint evidence selects a bounded single-owner serialized mutation loop. It supplies one
registry-wide commit order, per-connection FIFO into the owner, atomic histogram mutation, registry-wide snapshot
linearizability and an explicit overload boundary. Selection is based on correctness and proof simplicity, not measured
throughput. The no-fairness contract closes the shared-queue trade-off without inventing a product requirement. The
tested sharded and atomic-family snapshot designs are rejected because they emitted mixed generations; this does not
claim every possible sharded/atomic design is impossible. Global-lock and copy-on-write remain viable but unselected
alternatives. Performance/resource limits remain INV-019 scope.

INV-017 is **completed** and its decision is recorded by ADR-017.
