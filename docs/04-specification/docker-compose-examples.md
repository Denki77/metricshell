# Docker and Docker Compose Examples Specification

[Russian version](../../docs-ru/04-specification/docker-compose-examples.md)

> Status: Accepted normative specification
> Requirement: FR-071
> Acceptance criteria: AC-DIST-001–AC-DIST-004, AC-PORT-001, AC-PORT-002, AC-CONF-004
> Decisions: ADR-001, ADR-002, ADR-005–ADR-008, ADR-010–ADR-015

## Purpose

This specification defines the final minimum executable Docker and Docker Compose example set for the first stable
release. Examples are conformance assets, not illustrative snippets: CI must build and execute them.

## Required directory layout

```text
examples/
├── fixtures/
│   ├── application.prom
│   └── zero-series.snapshot
├── docker/
│   ├── standalone-copy/
│   ├── multistage-copy/
│   └── base-image/
├── compose/
│   ├── long-running/
│   ├── finite-workload/
│   └── transport-conformance/
└── test-examples.sh
```

Every example directory contains:

```text
README.md
Dockerfile              # where applicable
compose.yaml             # Compose examples only
prometheus.yml           # examples containing Prometheus
workload/                # minimal executable reference workload
expected/                # stable expected assertions or output
```

## Common requirements

All examples must:

- use exec-form ENTRYPOINT/CMD for MetricShell and the workload;
- run MetricShell as PID 1 unless the example explicitly demonstrates Docker `--init`;
- run as non-root;
- use a read-only root filesystem where the workload permits it;
- set `no-new-privileges` and drop all Linux capabilities;
- keep ingestion local to the application container;
- avoid `latest` tags;
- pin external images and the MetricShell artifact image by immutable digest in release fixtures;
- expose MetricShell version/revision;
- use the normative configuration defaults unless the example declares the tested override;
- include a health check that does not count as a final scrape;
- retain workload exit-code semantics;
- document build, run, verify and cleanup commands;
- run without Kubernetes APIs or service-account mounts.

The release registry is intentionally not selected by ADR-013. Examples accept these build inputs:

```text
METRICSHELL_VERSION
METRICSHELL_ARTIFACT_IMAGE
METRICSHELL_ARTIFACT_DIGEST
```

A development target may build the artifact image locally. A release target must fail when an immutable digest is not
provided.

## Shared fixture

`examples/fixtures/application.prom` contains one counter, one gauge and one classic histogram. The same logical
snapshot is used by file, Unix socket and local HTTP examples. `zero-series.snapshot` uses the explicit product encoding
for a valid zero-series publication; it is not an empty file or empty HTTP body.

## Example D-1: standalone copy

Path: `examples/docker/standalone-copy/`

Purpose:

- demonstrate verified static binary installation into an existing application image;
- run a long-lived workload using the default Unix socket transport;
- prove PID 1, signal forwarding and exit-code preservation.

Required behavior:

1. Copy the architecture-correct MetricShell binary and `SHA256SUMS` into the build context or download them in a
   checksum-verifying build stage.
2. Verify the checksum before installing `/usr/local/bin/metricshell`.
3. Start MetricShell as PID 1 with a minimal workload that publishes the shared fixture through the Unix socket.
4. Bind `/metrics` to the container interface while keeping the ingestion socket private.
5. `docker stop` must forward TERM with an external deadline greater than the configured 30-second internal grace.

Required commands:

```bash
docker build -t metricshell-example-standalone examples/docker/standalone-copy
docker run --rm -d --name metricshell-example-standalone -p 127.0.0.1:19100:9090 metricshell-example-standalone
curl --fail --retry 30 --retry-connrefused --retry-delay 1 http://127.0.0.1:19100/metrics
docker stop --time 32 metricshell-example-standalone
```

## Example D-2: pinned multi-stage copy

Path: `examples/docker/multistage-copy/`

Purpose:

- demonstrate `COPY --from=<artifact-image>@sha256:<digest>`;
- retain the application's selected base image;
- run a finite workload using atomic file ingestion and final scrape wait.

Required behavior:

- artifact source is immutable in the release fixture;
- the workload writes a same-directory temporary snapshot and atomically renames it;
- file reconciliation uses the 1-second default;
- natural workload completion freezes the snapshot and enters `scrapes` mode with `N=1`, timeout `60s`;
- the container exits with the workload result after one eligible completed response or timeout.

## Example D-3: optional base image

Path: `examples/docker/base-image/`

Purpose:

- demonstrate supported inheritance from the optional non-root MetricShell base image;
- install application dependencies and code after inheritance;
- use the stable local HTTP ingestion endpoint `/v1/metrics` on loopback.

Required behavior:

- the base image is digest-pinned for release testing;
- local ingestion never binds to wildcard or published host ports;
- the application submits the shared fixture and receives acknowledgement only after atomic installation;
- core conformance output matches D-1 and D-2.

## Example C-1: long-running Compose workload

Path: `examples/compose/long-running/`

Services:

```text
application
prometheus
```

Purpose:

- show a normal long-running workload and Prometheus pull scraping;
- use the default Unix socket ingestion transport;
- demonstrate that restart policy belongs to Compose, not MetricShell.

Required Compose properties:

- `application` is built from a documented example Dockerfile;
- Prometheus reaches only the `/metrics` endpoint through the Compose network;
- ingestion ports are not published;
- Prometheus configuration uses a finite scrape timeout below its interval;
- `restart: on-failure` or an explicit no-restart profile demonstrates external restart ownership;
- memory `64MiB`, PID limit `64`, `nofile 64/64`, non-root and least-privilege settings are present.

Verification:

```bash
docker compose up --build -d
curl --fail http://127.0.0.1:<prometheus-port>/-/ready
# query the shared application counter and metricshell_build_info
docker compose down --volumes --remove-orphans
```

## Example C-2: finite Compose workload

Path: `examples/compose/finite-workload/`

Services:

```text
finite-application
prometheus
verifier
```

Purpose:

- demonstrate a finite CLI workload;
- preserve both success and non-zero workload outcomes;
- verify one eligible final response plus finite timeout;
- prove health requests do not count.

Required scenarios:

1. Workload exits `0`; Prometheus scrapes final snapshot; container exits `0`.
2. Workload exits `17`; Prometheus scrapes final snapshot; container exits `17`.
3. Prometheus is disabled; timeout ends final wait; original workload result is preserved.
4. An external `docker compose stop` begins shutdown; no new post-exit wait starts.

The verifier must query and persist the final fixture while the target is still present in `final_wait`, then separately
inspect the application container exit code. If verification occurs after target disappearance, it must use a Prometheus
range query whose evaluation interval includes the last successful final scrape and ends before the stale marker. A
post-disappearance instant query is not evidence of final-sample delivery. The verifier must record the target
timestamp,
sample timestamp, query interval, and stale/not-found outcome as required by ADR-012.

## Example C-3: transport conformance

Path: `examples/compose/transport-conformance/`

Services or profiles:

```text
file
unix
http
verifier
```

Purpose:

- publish the same complete application snapshot through every stable transport;
- normalize `/metrics` responses by removing MetricShell self-metrics;
- require byte-equivalent canonical application exposition;
- verify malformed and zero-series cases through all transports.

Required checks:

- accepted fixture produces the same application families for file, Unix and HTTP;
- malformed input leaves the last valid snapshot unchanged;
- valid zero-series snapshot clears all application families;
- empty transport payload is rejected;
- disabled transports do not accept input;
- no cross-producer or cross-transport aggregation occurs.

## Top-level test runner

`examples/test-examples.sh` is non-interactive and returns non-zero on any failure. It must:

1. build all Docker examples;
2. execute their smoke/conformance checks;
3. execute all Compose profiles;
4. validate Prometheus exposition with official compatible tooling;
5. verify expected exit codes;
6. clean containers, networks and volumes in a trap;
7. print the immutable images and MetricShell revision used.

The runner supports:

```text
EXAMPLE_FILTER
KEEP_EXAMPLE_RESOURCES=1
METRICSHELL_ARTIFACT_IMAGE
METRICSHELL_ARTIFACT_DIGEST
```

## CI requirements

- Execute the full example runner on Ubuntu for every release candidate.
- Build both `linux/amd64` and `linux/arm64` images.
- Run at least the native architecture in CI; cross-architecture execution may use registered binfmt/QEMU.
- Pin Prometheus and helper images by digest.
- Archive logs and effective Compose configuration on failure.
- Do not treat example timings as SLOs.

## Documentation requirements

Every README must state:

- what requirement and ADRs it demonstrates;
- exact commands;
- expected metric names and exit result;
- local-only security boundary;
- cleanup steps;
- limitations, including LinuxKit/native-Linux evidence boundaries where relevant.

## Conformance requirements

The example set is complete only when AC-DIST-001–AC-DIST-004 and AC-PORT-001–AC-PORT-002 are mapped to automated
checks in `test-examples.sh` and all six example directories pass in release CI.

## References

- [Functional Requirements](../03-requirements/functional-requirements.md#fr-071--docker-examples)
- [Acceptance Criteria](../03-requirements/acceptance-criteria.md#distribution-and-portability)
- [ADR-005](../06-architecture/adr/ADR-005.md)
- [ADR-008](../06-architecture/adr/ADR-008.md)
- [ADR-011](../06-architecture/adr/ADR-011.md)
- [ADR-013](../06-architecture/adr/ADR-013.md)
- [Runtime Defaults and Resource Limits](runtime-defaults-and-resource-limits.md)
