# Managed Aggregation Architecture Research

This document defines the research topics for the Managed Aggregation extension.

## INV-016

Managed Registry Semantics

### Question

What exact registry, descriptor, operation and snapshot semantics should Managed Aggregation provide before concurrency or transport choices are considered?

### Context

The extension exists because simple and legacy clients should publish instrumentation operations without owning a complete Prometheus-compatible registry.

### Candidates

- explicit declaration plus typed operations;
- first-use implicit declaration;
- explicit protocol with convenience clients that may hide declaration.

### Topics

- descriptor identity;
- family lifetime;
- series identity;
- label canonicalization;
- counter initialization and delta rules;
- gauge SET and optional ADD/SUB;
- histogram bucket declaration and observation atomicity;
- conflicts;
- empty registry;
- deletion/staleness;
- operation/batch atomicity;
- registry epoch;
- complete Core snapshot generation.

### Initial Hypotheses

- descriptor semantics should be explicit even if convenience clients hide declaration;
- counters must preserve cumulative non-decreasing metric semantics within one registry epoch;
- the client operation model should support the broadest practical set of counter mutations whose semantics can be made deterministic;
- increment/add, absolute update, initialization and reset/epoch behavior must be compared rather than rejected in advance;
- absolute updates require explicit investigation under concurrent publishers;
- unsupported counter operations must be rejected only when research demonstrates semantic, correctness, compatibility or complexity problems;
- gauges require SET;
- histogram buckets are fixed before observations;
- one observation atomically updates count, sum and applicable cumulative buckets;
- identity reuses Core family-name plus canonical-label-set rules;
- conflicts reject without changing valid state;
- restart creates a new empty epoch;
- disconnect does not delete series;
- managed state becomes a complete Core snapshot.

### Evidence Required

- Prometheus/OpenMetrics model review;
- compatibility review against Application Snapshot Protocol;
- executable registry prototype;
- deterministic semantic tests;
- conflict/malformed tests;
- snapshot-generation tests;
- restart/new-epoch tests.

### Experiments

#### E-016.1 — Counter operational semantics

Compare:

- increment;
- add(delta);
- absolute update/set;
- explicit initialization;
- reset within the same epoch;
- reset/new epoch;
- zero, positive and negative values/deltas;
- NaN/Inf;
- overflow;
- repeated absolute values;
- decreasing absolute values.

Determine separately:

- which operations preserve counter semantics;
- which require epoch/reset semantics;
- which remain deterministic with multiple publishers;
- which can safely produce a Core-compatible complete snapshot.

Assertions:

- accepted increments produce exact cumulative value;
- rejected changes leave prior state unchanged;
- restart does not preserve state;
- every accepted operation preserves the selected counter invariants;
- rejected operations leave prior state unchanged;
- accepted operations have explicitly defined cumulative semantics;
- a decrease cannot silently occur without the selected reset/epoch semantics;
- restart/new epoch follows the selected lifecycle contract.

#### E-016.2 — Gauge semantics

Test SET across supported numeric classes. If ADD/SUB remains a candidate, evaluate it separately.

Assertions: accepted SET replaces the value; rejected input does not.

#### E-016.3 — Histogram semantics

Compare a non-negative-only candidate with signed-observation candidates. Test negative, zero, positive, NaN, +Inf and
-Inf observations; negative and mixed-sign bucket boundaries; sums becoming negative and crossing zero; and repeated
negative/positive observations. Materialize representative snapshots and record separately whether the prototype
candidate accepts them and whether the existing Core validator accepts them.

Assertions: one accepted observation updates count/sum/buckets exactly once; +Inf equals total count; no partial state is visible; changed bucket schema is rejected.

#### E-016.4 — Descriptor conflicts

Test same name/different type, HELP, label schema, buckets, derived-name collisions and reserved namespace.

Assertions: conflict rejects atomically; prior registry remains valid; generated Core snapshot remains valid.

#### E-016.5 — Label identity

Test reordered labels, missing/extra labels and duplicates.

Assertions: input order does not change identity; schema is enforced; duplicates reject.

#### E-016.6 — Batch semantics

Compare no-batch and all-or-nothing batch models.

If batching is retained: one invalid member rejects all; no partial state is observable.

#### E-016.7 — Snapshot materialization

After mutations, generate a complete snapshot and pass it through the existing Core validator.

Assertions: snapshot is self-contained, contains all active managed series, passes Core validation and does not depend on prior Core state.

### Evaluation Criteria

Semantic clarity, Prometheus compatibility, legacy-client simplicity, deterministic validation, minimal client state, protocol clarity and ability to generate complete Core snapshots.

Prototype assertion success demonstrates internal consistency and preservation of the tested invariants for a candidate.
It does not by itself select that candidate as the product or architecture decision. Ordering, retry/idempotency,
external protocol, resource limits and lifecycle-dependent semantics remain deferred to INV-017 through INV-020.

### Decision Output

Managed-registry semantic ADR/specification, accepted descriptor/operation model, unsupported-operation list and semantic reference tests.

### Status

In progress.

---

## INV-017

Concurrent Publishers and Ordering

### Question

How should one managed registry safely accept operations from multiple threads/processes/connections of the same logical workload?

### Candidates

- single serialized mutation loop with bounded queue;
- global-lock shared registry;
- sharded locks;
- atomics plus per-family synchronization;
- copy-on-write transaction state.

### Initial Hypotheses

- a single-owner mutation loop is likely sufficient and easiest to prove;
- per-connection order should be preserved;
- cross-connection accepted mutations need one observable linearization order;
- increments must never be lost;
- concurrent gauge SET is valid only under a deterministic ordering rule;
- histogram observation is one atomic mutation;
- idempotency must be explicit, not assumed;
- unbounded queues are unacceptable.

### Experiments

#### E-017.1 — Concurrent counter increments

Use 1, 2, 8, 32 and 128 publishers.

Assertions: final value equals exact accepted sum; no accepted increment is lost/duplicated internally; snapshots remain valid.

#### E-017.2 — Concurrent gauge SET

Publish unique sequence values.

Assertions: acknowledgements map to a defined commit result; final value matches the defined linearization order; no torn value appears.

#### E-017.3 — Concurrent histograms

Assertions: final count/sum/buckets match accepted observations; no partial observation state appears.

#### E-017.4 — Descriptor creation race

Compatible/incompatible declarations race.

Assertions: compatible definitions converge; conflicts produce deterministic winner/rejection; mixed state never exists.

#### E-017.5 — Partial frame and crash

Kill writers before header completion, during payload, after send before ACK read and after ACK.

Assertions: incomplete frame never mutates; accepted complete frame mutates at most once internally; one broken connection does not corrupt others.

#### E-017.6 — Duplicate delivery

Resend after simulated ACK loss. Compare no retry guarantee, operation IDs/idempotency keys and explicit unknown-outcome semantics.

#### E-017.7 — Backpressure

Overload the consumer.

Assertions: memory stays bounded; reject/block behavior is deterministic; accepted registry state stays valid; exposition remains operational.

### Evaluation Criteria

Correctness, determinism, bounded memory, throughput, tail latency, fairness, failure recovery, PHP/shell compatibility and acknowledgement clarity.

### Decision Output

Concurrency model, ordering/linearization contract, idempotency policy and backpressure policy.

### Status

Planned.

---

## INV-018

Legacy Client and Transport Viability

### Question

Can PHP 5.4, shell and simple CLI workloads use Managed Aggregation without maintaining a complete registry, and which local transport/protocol is justified?

### Candidates

Transport: Unix stream socket with versioned operation protocol, local HTTP, reuse of another local adapter, or rejection if complexity remains too high.

Client API: small line/framed protocol, `metricshell metric ...` helper, PHP 5.4 client file, reference Go client.

### Initial Hypotheses

- Unix stream socket is the strongest initial candidate based on prior Core transport research;
- operation protocol should be versioned and separate from snapshot payload;
- PHP 5.4 viability must be demonstrated in a real runtime;
- shell should prefer a CLI helper over implementing framing itself;
- clients must not hold complete registry state.

### Experiments

#### E-018.1 — PHP 5.4 basic client

Implement increment/set/observe.

Assertions: no full registry, no complete Prometheus snapshot, operations visible through Core exposition, errors detectable.

#### E-018.2 — PHP 5.4 multiprocess

Several workers publish to one registry.

Assertions: no shared PHP registry required; concurrent metrics are correct; worker exit does not delete unrelated state.

#### E-018.3 — Shell/CLI

Test helper commands.

Assertions: shell does not build snapshot JSON; failure is visible; helper does not create another long-lived daemon.

#### E-018.4 — Startup race

Compare bounded retry, endpoint-before-workload-start and explicit readiness.

Assertions: no unbounded wait; behavior is deterministic; lifecycle contract is preserved.

#### E-018.5 — Reconnect/restart

Assertions: reconnect needs no registry reconstruction; new MetricShell epoch starts empty.

#### E-018.6 — Transport comparison

Compare dependency availability, client code size, security boundary, framing robustness, debugging and performance observations.

### Evaluation Criteria

Client simplicity, PHP 5.4 viability, shell viability, dependency count, deterministic errors, local security and operational clarity.

### Decision Output

Selected managed-operation transport/protocol direction and reference legacy-client contract.

### Status

Planned.

---

## INV-019

Managed Registry Performance, Snapshot Materialization and Resource Limits

### Question

Can the selected architecture sustain practical operation rates and cardinality while continuously producing consistent complete snapshots with bounded resources?

### Candidate Snapshot Strategies

- re-encode after every mutation;
- materialize on scrape;
- generation-based immutable snapshot cache;
- copy-on-write family/series state.

### Workloads

100/1,000/10,000+ series; 1/10/100+ publishers; counter/gauge/histogram/mixed; sustained mutation with scrape; overload.

### Initial Hypotheses

- full re-encode per operation will not scale well;
- immutable scrape-visible generations are desirable;
- bounded single-owner mutation plus snapshot caching may provide the best simplicity/performance balance;
- cardinality is a primary memory risk;
- histograms require bucket limits.

### Experiments

#### E-019.1 — Operation throughput

Measure accepted/rejected ops, p50/p95/p99 latency, CPU, RSS, allocations, queue depth and descriptors.

#### E-019.2 — Cardinality scaling

Measure memory/series, encoded bytes, mutation latency, materialization time and scrape latency.

Assertions: limits enforce safely; rejection does not corrupt state.

#### E-019.3 — Histogram cost

Vary buckets and active series.

Assertions: limits fire before unsafe allocation; rejection preserves state.

#### E-019.4 — Snapshot strategy comparison

Measure acceptance latency, generation eligibility, encoding, Core installation, scrape latency and memory duplication separately.

#### E-019.5 — Concurrent scrape under mutation

Assertions: each response is one consistent generation; no torn state; slow/failed scraper does not corrupt registry.

#### E-019.6 — Backpressure/overload

Assertions: memory bounded; behavior matches INV-017; no positively acknowledged operation is silently dropped.

#### E-019.7 — Controlled resource exhaustion

Use container memory/FD limits and distinguish normal limit rejection from fatal OS/container OOM.

### Evaluation Criteria

Correctness, bounded memory, sustainable throughput, tail latency, snapshot consistency, scrape responsiveness, complexity and reproducibility.

### Decision Output

Snapshot materialization strategy, managed limits, overload behavior, benchmark baseline and resource-limit specification updates.

### Status

Planned.

---

## INV-020

Managed Aggregation Lifecycle and Core Integration

### Question

What exact lifecycle connects managed ingestion, workload termination, registry freeze, final snapshot installation and the existing Core final-scrape lifecycle?

### Candidate Freeze Boundaries

- freeze immediately on observed workload exit;
- stop new connections, drain fully received work, then freeze;
- explicit client flush/close handshake;
- bounded hybrid drain.

### Initial Hypotheses

- registry has one explicit freeze transition;
- post-freeze operations reject deterministically;
- partial frames never become accepted;
- committed operations belong to final state even if ACK delivery fails;
- bounded drain may be preferable to immediate abort;
- final managed state becomes one complete Core snapshot before final-scrape waiting;
- application metrics freeze while self-metrics may continue;
- restart creates a new empty epoch.

### Experiments

#### E-020.1 — Normal exit

Assertions: freeze exactly once; final snapshot contains committed operations; late operations cannot change final state.

#### E-020.2 — Signal shutdown

Test partial frame, received/not validated, queued, committing, committed before ACK and acknowledged.

Each stage must map deterministically to accepted/rejected/unknown-to-client outcome.

#### E-020.3 — Late publisher

Assertions: rejected after freeze; final snapshot unchanged.

#### E-020.4 — Final snapshot failure

Inject conversion/install failure.

Assertions: failure is explicit; invalid snapshot is never installed; existing Core failure semantics remain authoritative.

#### E-020.5 — Final scrape

Assertions: frozen application state remains immutable; self-metrics may change; eligible scrape counting remains Core-defined; no TSDB-persistence claim.

#### E-020.6 — Restart/new epoch

Assertions: one new workload execution per MetricShell process; registry starts empty; prior state is not restored.

#### E-020.7 — Kubernetes Job/CronJob

Verify final managed snapshot exposure during bounded post-exit wait without Kubernetes API dependency.

### Evaluation Criteria

Deterministic freeze semantics, bounded shutdown, acknowledgement clarity, Core lifecycle compatibility, workload exit preservation and final-scrape correctness.

### Decision Output

Lifecycle ADR, freeze/in-flight semantics, lifecycle specification updates and proof that Managed Aggregation composes with Core.

### Status

Planned.

---
[Investigation overview](managed-aggregation-architecture-investigation.md) | [Documentation index](../README.md)
