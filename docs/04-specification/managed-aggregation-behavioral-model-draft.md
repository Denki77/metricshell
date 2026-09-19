# Managed Aggregation — Pre-ADR Behavioral Agreement

> **Status:** Non-normative input for architecture investigation
>
> **Purpose:** Record product intent and investigation boundaries without deciding semantics that must be proven by INV-016–INV-020.
>
> **Depends on:** Managed Aggregation scope/requirements extension and completed MetricShell Core contracts.
>
> **Does not supersede:** ADR-001–ADR-015 or accepted Core specifications.

## 1. Why this document exists

Managed Aggregation has requirements and a planned architecture-investigation cycle, but no accepted Managed Aggregation ADR yet. Therefore unresearched semantic choices must not be presented as an accepted normative protocol.

This document records **what we want to achieve** and **what the investigations must determine**. It is an input to INV-016–INV-020, not their conclusion. After investigations and ADRs are accepted, externally observable decisions are promoted into the normative specification.

## 2. Product intent: broad instrumentation support

Managed Aggregation exists so simple and legacy workloads do not have to own a complete metric registry.

The target is not an intentionally restricted `increment-only` API. We aim to support the useful instrumentation operations workloads reasonably need for supported metric families, provided deterministic, safe and Prometheus-compatible semantics can be defined.

Candidate operation space includes at least:

```text
counter:   increment / add / absolute update or initialization where semantically valid
gauge:     set / increment / decrement / add / subtract
histogram: observe and required declaration/configuration operations
registry:  declaration and lifecycle operations if research proves them necessary
batch:     multiple related operations if atomic batching is justified
```

These are **research candidates**, not accepted command names or protocol guarantees.

INV-016 must determine which operations are valid, what constraints apply, and which cannot be supported without violating the metric model or existing Core contracts.

Design preference:

> Support the broadest practical operation set. Exclude an operation only when research demonstrates a semantic, correctness, compatibility, safety, resource, or unjustifiable-complexity reason.

## 3. Why Counter must not be fixed to increment-only before research

A Prometheus counter represents cumulative state and normally does not decrease except for a reset. This constrains the **resulting metric semantics**, but does not prove that the Managed Aggregation client API must expose only `increment`.

A legacy workload may naturally know an absolute cumulative value maintained by the application, a delta to add, an initial value imported from application state, or a reset/new-lifecycle condition.

Whether Managed Aggregation should expose `increment`, `add`, an absolute counter update, explicit initialization, reset semantics, or a subset is an INV-016 question.

The investigation must distinguish:

1. **Metric semantic validity** — what state is valid for a counter.
2. **Client operation semantics** — how a client may express a state transition.
3. **Epoch/reset semantics** — when a decrease is a valid reset/new epoch rather than an invalid mutation.
4. **Concurrency semantics** — whether absolute updates remain deterministic with multiple publishers.
5. **Core compatibility** — whether the resulting complete snapshot remains valid under the accepted Core contract.

Until evidence exists, the specification must not state that counters are permanently `increment-only`.

## 4. What may be fixed before investigation

The following are already product/Core constraints:

- Managed Aggregation is optional.
- Snapshot mode remains the default.
- Managed Aggregation remains inside the same MetricShell binary/process model.
- One managed registry belongs to one logical workload execution.
- Independent applications do not share one managed registry.
- Managed state enters Core as a complete application snapshot.
- Existing Core snapshot semantics are not silently changed.
- Invalid/rejected managed input must not corrupt previously valid state.
- A new MetricShell execution starts a new managed-registry epoch by default.
- Persistence/replay across MetricShell restarts is outside the initial extension.
- Hybrid ownership by external complete snapshots and managed operations is outside the initial extension unless separately researched and approved.

These constrain research but do not decide the managed-registry protocol.

## 5. What remains open until investigation

Before the relevant INV and ADR are accepted, these remain unresolved:

- exact counter operations and initialization/reset behavior;
- exact gauge operations;
- histogram declaration/update semantics;
- descriptor declaration and mutation rules;
- series creation/deletion/staleness;
- publisher ownership and cleanup;
- batch support and atomicity;
- operation ordering and linearization;
- duplicate delivery and idempotency;
- acknowledgement semantics;
- transport and framing;
- snapshot materialization timing;
- relationship between operation acceptance and Core snapshot installation;
- freeze boundary and in-flight operation handling;
- concrete resource limits and overload behavior.

A draft may list hypotheses/candidates, but must not express them as accepted normative `MUST`/`SHALL` decisions.

## 6. Documentation flow

```text
03 Requirements
    ↓
04 Non-normative behavioral/specification draft
    ↓
05 Architecture Investigation (INV-016–INV-020)
    ↓ evidence
06 ADR(s)
    ↓ accepted decisions
04 Accepted normative specification
    ↓
07 Delivery / implementation
```

The pre-ADR document in `04-specification` is an **investigation input**, analogous to a behavioral model draft. It must not appear among accepted normative specifications until required ADRs are accepted.

## 7. Rule for updating the current draft

The Managed Aggregation draft should be revised so that:

1. established product/Core constraints remain normative references;
2. unresearched semantics are goals, candidates, hypotheses, or open questions;
3. `MUST`/`SHALL` does not prematurely select an operation model;
4. counter support is not artificially limited to `increment`;
5. research evaluates the broadest practical operation set;
6. unsupported operations require an explicit research-backed reason;
7. accepted INV/ADR conclusions are progressively promoted into the normative specification.

## 8. Completion rule

This is temporary architectural input. It has served its purpose when INV-016–INV-020 provide sufficient evidence, required ADRs are accepted, and the Managed Aggregation specification contains the resulting externally observable normative contract.

The final specification may be narrower than the candidate operation space above, but only because research establishes why a candidate is unsafe, ambiguous, incompatible, unjustifiably complex, or otherwise unsuitable—not because it was excluded before investigation.
