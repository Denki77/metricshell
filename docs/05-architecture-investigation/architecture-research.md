# Architecture Research

The current document shows the list of research topics.

## INV-001

Process and PID 1 Model

### Question

Should MetricShell run directly as PID 1, run under an init process such as Tini, or delegate process management to
another component?

### Context

MetricShell must start the workload, receive and forward signals, observe workload exit, preserve exit semantics, manage
responsible descendants and remain alive after workload completion when configured.

### Candidates

#### A. MetricShell as PID 1

MetricShell implements required init and supervisor behavior directly.

#### B. Tini as PID 1

Tini runs MetricShell, and MetricShell runs the workload.

#### C. Another process supervisor

Examples: dumb-init, s6 or supervisord.

### Initial Hypotheses

- MetricShell must own workload lifecycle even when Tini is present.
- Tini may reduce PID 1 edge cases but adds another binary and signal layer.
- A correct single-binary implementation may be simpler operationally.
- Process-group and orphaned-descendant behavior are the main correctness risks.

### Evidence Required

- Linux process and signal documentation;
- Docker init/process documentation;
- Tini behavior and source review;
- prototype for each viable process tree;
- tests for signals, descendants, zombies and exit codes.

### Experiments

#### E-001.1 — Signal forwarding

Run a workload that records TERM, INT and HUP signals. Verify delivery for a direct child, shell wrapper, child process
group and grandchild process.

#### E-001.2 — Child reaping

Run workloads that create short-lived children and double-forked descendants. Observe zombie processes.

#### E-001.3 — Exit status

Test exit `0`, exit `17`, TERM, KILL, workload start failure and MetricShell internal failure.

#### E-001.4 — Post-workload survival

Verify that MetricShell can continue serving metrics after workload exit while preserving the original workload result.

### Evaluation Criteria

- correctness;
- process-tree control;
- exit-code integrity;
- implementation complexity;
- external dependency count;
- image size;
- portability;
- operational clarity.

### Open Questions

- Should MetricShell use `PR_SET_CHILD_SUBREAPER`?
- Is one process group created for every workload?
- Are daemonized descendants supported?
- What behavior is expected for shell-form commands?
- Does Tini add meaningful correctness after MetricShell implements lifecycle ownership?

### Status

[Completed](../../research/INV-001/report.md).

---

## INV-002

Workload Lifecycle and Exit Semantics

### Question

What is the exact lifecycle of one MetricShell execution?

### Initial Model

```text
initialize
    ↓
prepare ingestion and exposition
    ↓
start workload
    ↓
run and expose metrics
    ↓
workload exits or termination begins
    ↓
finalize application metric state
    ↓
optional bounded post-exit availability
    ↓
terminate with resolved outcome
```

### Candidate Policies

- exactly one workload execution;
- optional workload restart;
- restart delegated to container runtime/orchestrator.

### Initial Hypothesis

MetricShell should run exactly one workload execution. Restart policy should remain outside MetricShell.

### Evidence Required

- complexity analysis;
- interaction with counter resets;
- Docker and Kubernetes restart behavior;
- acceptance-test coverage.

### Evaluation Criteria

- deterministic behavior;
- minimal supervisor scope;
- metric-state clarity;
- compatibility with orchestrator restart policies.

### Status

[Completed](../../research/INV-002/report.md).

---

## INV-003

Shutdown Time Budgeting

### Question

How much of the external shutdown grace period may be given to the workload, and how much must MetricShell reserve for
itself?

### Context

MetricShell needs time to forward termination, wait for workload shutdown, force termination when necessary, collect
exit status, finalize application metrics, finish active HTTP responses, flush diagnostics and terminate before external
SIGKILL.

### Candidate Policies

#### A. Fixed reserve

```text
workload_grace = total_grace - fixed_reserve
```

#### B. Percentage reserve

```text
workload_grace = total_grace × configured_ratio
```

#### C. Explicit independent values

The operator configures workload shutdown timeout and MetricShell shutdown reserve.

#### D. Absolute external deadline

MetricShell receives an absolute deadline and computes remaining budgets dynamically.

### Initial Hypothesis

Explicit workload timeout plus runtime reserve is easier to reason about than automatic inference.

### Experiments

Test total shutdown windows of 1, 5, 10, 30 and 60 seconds. Test workloads that stop immediately, just before deadline,
after deadline and never.

Measure:

- workload time received;
- finalization time;
- active scrape drain time;
- total shutdown time;
- forced-kill correctness.

### Evaluation Criteria

- deterministic completion;
- operator clarity;
- safety under short deadlines;
- no accidental external SIGKILL;
- sufficient HTTP shutdown time;
- no unbounded waiting.

### Open Questions

- What default reserve is safe?
- Should reserve be fixed, percentage-based, or both?
- Should post-exit scrape waiting occur after external termination begins?
- How is the external deadline provided outside Kubernetes?
- What happens when configured budgets exceed the external grace period?

### Status

[Completed](../../research/INV-003/report.md).

---

## INV-004

Metric-State Ownership and Semantics

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

### Status

In progress.

---

## INV-005

Ingestion Transport Comparison

### Question

Which ingestion transports should be supported in the first stable release, and which should remain optional adapters?

### Candidates

- file snapshot;
- Unix domain stream socket;
- Unix datagram socket;
- local HTTP;
- local gRPC;
- shared memory;
- memory-mapped file.

### Evaluation Matrix

| Criterion              | File | Stream socket | Datagram | Local HTTP | gRPC | Shared memory | mmap |
|------------------------|-----:|--------------:|---------:|-----------:|-----:|--------------:|-----:|
| PHP integration effort |  TBD |           TBD |      TBD |        TBD |  TBD |           TBD |  TBD |
| Throughput             |  TBD |           TBD |      TBD |        TBD |  TBD |           TBD |  TBD |
| Latency                |  TBD |           TBD |      TBD |        TBD |  TBD |           TBD |  TBD |
| Recovery after loss    |  TBD |           TBD |      TBD |        TBD |  TBD |           TBD |  TBD |
| Multi-producer support |  TBD |           TBD |      TBD |        TBD |  TBD |           TBD |  TBD |
| Protocol complexity    |  TBD |           TBD |      TBD |        TBD |  TBD |           TBD |  TBD |
| Portability            |  TBD |           TBD |      TBD |        TBD |  TBD |           TBD |  TBD |
| Resource control       |  TBD |           TBD |      TBD |        TBD |  TBD |           TBD |  TBD |

### Required Outputs

- prototype for each serious candidate;
- benchmark results;
- PHP integration example;
- failure-mode tests;
- recommendation and rejected alternatives.

### Status

Not started.

---

## INV-006

File-Based Ingestion

### Question

How should MetricShell detect and read file updates safely inside the container?

### Assumption

The metrics file is stored inside the container filesystem or container-local tmpfs. Host bind mounts are not part of
the primary supported design.

### Candidates

#### A. Polling only

Periodically stat and read the file.

#### B. inotify only

Use Linux filesystem events.

#### C. inotify plus reconciliation

Use events for fast detection and periodic checks for recovery.

#### D. Producer-triggered reload

Producer writes the file and signals MetricShell through another local mechanism.

### Initial Hypothesis

Directory-level inotify plus low-frequency reconciliation provides low idle overhead and recovery from missed or
replaced watches.

### Required Correctness Cases

- initial file already exists;
- file does not yet exist;
- temporary file plus atomic rename;
- repeated replacement;
- writer crash before rename;
- invalid new file;
- file deletion;
- directory recreation;
- `IN_Q_OVERFLOW`;
- MetricShell restart.

### Experiments

Compare writable container layer and container-local tmpfs. Docker named volume may be tested for information only.

Measure update-detection latency, idle CPU, CPU at high update frequency, missed updates, recovery after watch
invalidation and atomic rename behavior.

### Evaluation Criteria

- correctness;
- low idle overhead;
- simplicity;
- recovery;
- Linux container compatibility;
- no dependency on host filesystem semantics.

### Open Questions

- Watch directory or file?
- What reconciliation interval is acceptable?
- What file format is used?
- Is the file complete registry state?
- Should invalid updates retain the last valid state?
- What are maximum file and series sizes?

### Status

Not started.

---

## INV-007

Socket-Based Ingestion

### Question

Which local socket model and protocol best fit MetricShell?

### Candidates

- Unix stream socket;
- Unix datagram socket;
- framed binary protocol;
- line-based text protocol;
- existing protocol such as StatsD;
- custom versioned protocol.

### Topics

- delivery guarantees;
- ordering;
- reconnect;
- producer identity;
- multiple producers;
- backpressure;
- message size;
- socket permissions;
- workload startup race;
- MetricShell shutdown.

### Experiments

Test a single producer, many producers, burst traffic, slow reader, disconnect during message, MetricShell restart,
malformed frames, maximum payload and file-descriptor exhaustion.

### Status

Not started.

---

## INV-008

Local Push Ingestion

### Question

Does a local HTTP or gRPC ingestion API provide enough value beyond the socket adapter to justify implementation and
maintenance?

### Clarification

This concerns only local producer-to-MetricShell ingestion. Pushing metrics from MetricShell to Prometheus, Pushgateway
or a central collector remains out of scope.

### Candidates

- no local push API;
- local HTTP API;
- local gRPC API.

### Initial Hypothesis

Local HTTP may simplify integration for languages with mature HTTP clients, but may duplicate socket capabilities and
increase attack surface.

### Evaluation Criteria

- client implementation effort;
- protocol versioning;
- performance;
- resource usage;
- endpoint exposure risk;
- debugging convenience;
- duplication with socket transport.

### Status

Not started.

---

## INV-009

Shared Memory and mmap Adapter

### Question

Can shared memory or mmap provide a useful high-performance adapter without making clients unsafe or platform-specific?

### Candidates

- POSIX shared memory;
- anonymous shared mapping inherited by child;
- memory-mapped file;
- shared ring buffer;
- no shared-memory adapter.

### Initial Hypothesis

Shared memory may offer the best raw performance but may not justify client complexity, especially for PHP.

### Experiments

- Go producer prototype;
- PHP FFI or extension prototype;
- one producer;
- multiple producers;
- process crash during write;
- ring-buffer overflow;
- schema upgrade;
- MetricShell restart;
- memory-limit enforcement.

### Evaluation Criteria

- actual performance gain over socket;
- client safety;
- PHP adoption cost;
- synchronization complexity;
- versioning;
- failure recovery;
- portability.

### Status

Not started.

---

## INV-010

Prometheus Exposition

### Question

Which exposition formats and consistency guarantees should MetricShell provide?

### Topics

- Prometheus text format;
- OpenMetrics;
- content negotiation;
- HELP and TYPE;
- histograms;
- optional timestamps;
- concurrent scrape;
- partial failure;
- response-size limits;
- slow clients;
- runtime self-metrics.

### Initial Hypothesis

MetricShell should use an existing Prometheus client library or parser/encoder rather than implement exposition
manually.

### Experiments

Validate output with Prometheus tooling and test concurrent ingestion and scrape, large registry, malformed internal
state, disconnected scraper, slow scraper and multiple concurrent scrapers.

### Status

Not started.

---

## INV-011

Final Application State and Scrape Counting

### Question

What remains immutable after workload exit, and when is a final scrape considered complete?

### Proposed Separation

#### Application metrics

Final application metric state becomes immutable after workload finalization.

#### MetricShell self-metrics

Runtime self-metrics may continue changing while MetricShell waits and shuts down.

### Candidate Exit Modes

- immediate;
- fixed duration;
- wait for one eligible scrape;
- wait for configured N eligible scrapes.

### Initial Hypotheses

- default required scrape count should be `1`;
- `N > 1` should be optional;
- a scrape counts only after the complete response is written successfully;
- health and readiness requests never count;
- serving a response does not prove TSDB persistence.

### Research Questions

- Does a manual `curl` count?
- Is scraper authentication needed?
- Are concurrent scrapes counted independently?
- Is scraper uniqueness required?
- Does an aborted connection count?
- Do runtime self-metric changes affect final-state identity?
- What happens when timeout expires?
- Are application metrics frozen before or after ingestion shutdown?

### Status

Not started.

---

## INV-012

Kubernetes Job and CronJob Viability

### Question

Can Prometheus continue discovering and scraping the MetricShell endpoint while a completed workload is held alive
inside a still-running Job Pod?

### Why This Is Critical

MetricShell may correctly wait for a scrape, but the wait is useless if the target is removed from Prometheus discovery
before the final scrape occurs.

### Experiments

Test PodMonitor, ServiceMonitor, direct Pod discovery, readiness true and false during post-exit wait, Job, CronJob,
termination, `activeDeadlineSeconds`, `ttlSecondsAfterFinished` and overlapping schedules.

Measure target discovery duration, last successful scrape, Job completion delay, scheduler overlap impact and behavior
with two Prometheus instances.

### Decision Criteria

- final scrape succeeds reliably;
- no Kubernetes-specific code in MetricShell core;
- documented deployment configuration is sufficient;
- Job remains operationally understandable.

### Status

Not started.

---

## INV-013

Distribution Models

### Question

How should MetricShell be added to application images?

### Candidates

- copy standalone static binary;
- multi-stage Dockerfile;
- MetricShell base image;
- language-specific convenience images.

### Evaluation Criteria

- image size;
- libc compatibility;
- amd64 and arm64;
- non-root operation;
- version pinning;
- reproducibility;
- supply-chain verification;
- application freedom to install dependencies.

### Status

Not started.

---

## INV-014

Security and Resource Limits

### Topics

- non-root execution;
- socket and file permissions;
- local push binding;
- metrics endpoint binding;
- input validation;
- series and label limits;
- payload size;
- concurrent clients;
- slow clients;
- file-descriptor limits;
- memory limits;
- secrets in labels;
- malicious producer behavior.

### Required Output

- threat model;
- default security posture;
- resource-limit configuration;
- failure policy;
- security acceptance tests.

### Status

Not started.

---

## INV-015

Benchmark Plan

### Purpose

Benchmarks compare candidates and validate non-functional requirements. They must not be used to manufacture a preferred
conclusion.

### Reference Environment

Record CPU, allocated cores, RAM, kernel, container runtime, Docker version, Go version, PHP version, image, CPU/memory
limits, host load and commit SHA.

### Workloads

#### B-001 — Idle runtime

Measure CPU and RSS with no updates and a normal scrape interval.

#### B-002 — Ingestion throughput

For each transport test 100, 1,000 and 10,000 updates per second, then increase until saturation.

Measure accepted/rejected updates, p50/p95/p99 ingestion latency, CPU, RSS, allocations and open descriptors.

#### B-003 — Registry cardinality

Test 100, 1,000, 10,000 and, where practical, 100,000 series.

Measure memory, scrape size, scrape latency and ingestion latency.

#### B-004 — Concurrent scrape

Test 1, 2, 5 and 10 concurrent scrapers.

#### B-005 — File detection

Compare polling, inotify and hybrid mode.

#### B-006 — Startup

Measure MetricShell initialization and time until workload start.

#### B-007 — Shutdown

Measure graceful shutdown, forced shutdown, finalization and HTTP drain.

#### B-008 — Final scrape wait

Test zero scrapes, one scrape, N scrapes, concurrent scrapes, aborted response and timeout.

#### B-009 — Failure injection

Test malformed input, transport disconnect, endpoint bind failure, queue overflow and resource exhaustion.

### Benchmark Rules

- warm up before measurement;
- run multiple iterations;
- report median and dispersion;
- retain raw results;
- pin environment and commit;
- compare equivalent semantic workloads;
- do not compare debug builds with optimized builds;
- distinguish throughput from end-to-end latency;
- document discarded runs and reasons.

### Status

Not started.
