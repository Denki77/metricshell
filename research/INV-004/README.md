# INV-004 — Metric-state ownership and semantics

### Question

Does the workload send complete values, update operations, or both?

### Candidates

#### A. Complete registry snapshots

Producer publishes the current complete state.

#### B. Absolute series values

Producer submits the current value for individual series.

#### C. Operations

Producer submits increments, sets and observations.

#### D. Hybrid model

Different transports support different representations while preserving application-level semantics.

### Topics

- counters;
- gauges;
- histograms;
- duplicate series;
- type conflicts;
- multiple producers;
- ordering;
- lost updates;
- producer restarts;
- stale data;
- final application state.

### Initial Hypothesis

File ingestion naturally favors complete snapshots. Socket and local push may favor operations or absolute updates.
Equivalent client semantics do not require identical transport semantics.

### Evaluation Criteria

- correctness after dropped messages;
- recovery after MetricShell restart;
- client complexity;
- protocol complexity;
- throughput;
- memory;
- multi-producer behavior.
