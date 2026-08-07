# Спецификация Docker и Docker Compose examples

[English version](../../docs/04-specification/docker-compose-examples.md)

> Статус: принятая нормативная спецификация
> Требование: FR-071
> Критерии приёмки: AC-DIST-001–AC-DIST-004, AC-PORT-001, AC-PORT-002, AC-CONF-004
> Решения: ADR-001, ADR-002, ADR-005–ADR-008, ADR-010–ADR-015

## Назначение

Документ определяет окончательный минимальный executable set Docker/Docker Compose examples первой stable release.
Examples являются conformance assets: CI обязана их собирать и запускать.

## Directory layout

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

Каждая example directory содержит:

```text
README.md
Dockerfile              # где применимо
compose.yaml             # только Compose examples
prometheus.yml           # examples с Prometheus
workload/                # минимальный executable reference workload
expected/                # стабильные expected assertions или output
```

## Общие требования

Все examples:

- используют exec-form ENTRYPOINT/CMD;
- запускают MetricShell как PID 1, кроме явно обозначенного `--init` example;
- работают non-root;
- используют read-only rootfs, `no-new-privileges`, `cap_drop: ALL`, когда workload допускает;
- сохраняют ingestion local внутри application container;
- не используют `latest`;
- pin external images и artifact image по digest в release fixtures;
- публикуют version/revision;
- используют normative defaults без скрытых overrides;
- имеют healthcheck, который не считается final scrape;
- сохраняют workload exit code;
- документируют build/run/verify/cleanup;
- не требуют Kubernetes APIs.

Registry не выбран ADR-013, поэтому examples принимают:

```text
METRICSHELL_VERSION
METRICSHELL_ARTIFACT_IMAGE
METRICSHELL_ARTIFACT_DIGEST
```

Development target может собрать local artifact image. Release target без immutable digest завершается ошибкой.

## Shared fixture

`application.prom` содержит counter, gauge и classic histogram и используется всеми transports.
`zero-series.snapshot` — явный valid product encoding, а не empty file/body.

## D-1: standalone copy

Path: `examples/docker/standalone-copy/`

Показывает checksum-verified static binary в существующем application image, long-running workload, default Unix socket,
PID 1, signal forwarding и exit preservation.

Назначение:

- показать verified installation static binary в существующий application image;
- запустить long-running workload через default Unix socket transport;
- доказать PID 1 behavior, signal forwarding и сохранение exit code.

Обязательное поведение:

1. Скопировать binary MetricShell и `SHA256SUMS` в build context либо скачать их в checksum-verifying build stage.
2. Проверить checksum до установки `/usr/local/bin/metricshell`.
3. Запустить MetricShell как PID 1 с минимальным workload, публикующим shared fixture через Unix socket.
4. Bind `/metrics` на container interface, оставив ingestion socket приватным.
5. `docker stop` обязан переслать TERM с external deadline больше настроенного internal grace `30s`.

Обязательные команды:

```bash
docker build -t metricshell-example-standalone examples/docker/standalone-copy
docker run --rm -d --name metricshell-example-standalone -p 127.0.0.1:19100:9090 metricshell-example-standalone
curl --fail --retry 30 --retry-connrefused --retry-delay 1 http://127.0.0.1:19100/metrics
docker stop --time 32 metricshell-example-standalone
```

## D-2: pinned multi-stage copy

Path: `examples/docker/multistage-copy/`

Показывает `COPY --from=<artifact-image>@sha256:<digest>`, application-selected base, finite workload, atomic file
publication, reconcile `1s`, final wait `scrapes`, `N=1`, timeout `60s` и сохранение workload result.

Обязательное поведение:

- artifact source в release fixture закреплён immutable digest;
- workload записывает temporary snapshot в том же directory и выполняет atomic rename;
- file reconciliation использует default `1s`;
- natural completion workload фиксирует snapshot и входит в mode `scrapes` с `N=1` и timeout `60s`;
- после одного eligible completed response или timeout container завершается с исходным workload result.

## D-3: optional base image

Path: `examples/docker/base-image/`

Показывает inheritance от optional non-root base image, установку dependencies/application code и local HTTP ingestion
`/v1/metrics` только на loopback. ACK выдаётся только после atomic installation. Exposition должна совпасть с D-1/D-2.

Base image закрепляется digest в release tests. Ingestion не публикуется на wildcard или host port. Application
отправляет
shared fixture и обязано получить ACK только после atomic installation.

## C-1: long-running Compose

Path: `examples/compose/long-running/`

Services:

```text
application
prometheus
```

Показывает long-running workload, Prometheus pull, default Unix socket и ownership restart policy со стороны Compose.
Ingestion port не публикуется. Reference profile: memory `64MiB`, PID `64`, nofile `64/64`, non-root, least privilege.

Compose обязан содержать services `application` и `prometheus`, documented Dockerfile, scrape timeout меньше interval и
`restart: on-failure` либо отдельный no-restart profile. Prometheus получает доступ только к `/metrics` через Compose
network.

Проверка:

```bash
docker compose up --build -d
curl --fail http://127.0.0.1:<prometheus-port>/-/ready
# запросить shared application counter и metricshell_build_info
docker compose down --volumes --remove-orphans
```

## C-2: finite Compose workload

Path: `examples/compose/finite-workload/`

Services: `finite-application`, `prometheus`, `verifier`.

```text
finite-application
prometheus
verifier
```

Обязательные scenarios:

1. workload exit `0`, final snapshot scraped, container exit `0`;
2. workload exit `17`, final snapshot scraped, container exit `17`;
3. Prometheus disabled, timeout сохраняет исходный workload result;
4. external compose stop не запускает новый post-exit wait.

Verifier query-ит и сохраняет final fixture, пока target присутствует в `final_wait`, а затем отдельно проверяет
container exit code. После исчезновения target используется Prometheus range query: interval включает последний
successful final scrape и заканчивается до stale marker. Instant query после исчезновения target не доказывает доставку
final sample. По ADR-012 verifier сохраняет target timestamp, sample timestamp, query interval и stale/not-found
outcome.

## C-3: transport conformance

Path: `examples/compose/transport-conformance/`

Profiles/services: `file`, `unix`, `http`, `verifier`.

```text
file
unix
http
verifier
```

Один complete snapshot публикуется всеми stable transports. После удаления self-metrics canonical application exposition
должна быть byte-equivalent.

Checks:

- accepted fixture одинаков для file/unix/http;
- malformed input сохраняет last-valid snapshot;
- valid zero-series очищает application families;
- empty payload отклоняется;
- disabled transport не принимает input;
- cross-producer/cross-transport aggregation отсутствует.

## Top-level runner

`examples/test-examples.sh` non-interactive и возвращает non-zero при любом failure. Он обязан:

1. собрать все Docker examples;
2. выполнить их smoke/conformance checks;
3. выполнить все Compose profiles;
4. проверить Prometheus exposition официальным совместимым tooling;
5. проверить ожидаемые exit codes;
6. очистить containers, networks и volumes через trap;
7. напечатать использованные immutable images и revision MetricShell.

Поддерживаемые env:

```text
EXAMPLE_FILTER
KEEP_EXAMPLE_RESOURCES=1
METRICSHELL_ARTIFACT_IMAGE
METRICSHELL_ARTIFACT_DIGEST
```

## CI

- Full runner на Ubuntu для каждого release candidate.
- Build `linux/amd64` и `linux/arm64`.
- Native run обязателен; cross-arch допускает pinned binfmt/QEMU.
- Prometheus/helper images pinned by digest.
- При failure архивируются logs и effective Compose config.
- Timing examples не считается SLO.

## Требования к документации

Каждый README обязан указывать:

- демонстрируемые requirement и ADR;
- точные команды;
- ожидаемые metric names и exit result;
- local-only security boundary;
- шаги cleanup;
- ограничения, включая границы доказательств LinuxKit/native Linux, где они применимы.

## Требования соответствия

Set complete только когда AC-DIST-001–004 и AC-PORT-001–002 связаны с automated checks и все шесть directories проходят
release CI.

## Ссылки

- [Функциональные требования](../03-requirements/functional-requirements.md)
- [Критерии приёмки](../03-requirements/acceptance-criteria.md)
- [ADR-005](../06-architecture/adr/ADR-005.md)
- [ADR-008](../06-architecture/adr/ADR-008.md)
- [ADR-011](../06-architecture/adr/ADR-011.md)
- [ADR-013](../06-architecture/adr/ADR-013.md)
- [Defaults и limits](runtime-defaults-and-resource-limits.md)
