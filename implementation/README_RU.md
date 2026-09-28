# Production-реализация MetricShell

Этот каталог является production-модулем Go. Код из `research/` содержит исследовательские свидетельства и намеренно
не входит в модуль и граф зависимостей.

## Текущий объём

Core до Wave 6 завершён и готов к эксплуатации в complete-snapshot profile. Publishers заменяют весь набор метрик
целиком; partial metric updates намеренно не входят в этот release.

ISSUE-011–ISSUE-015 реализуют immutable snapshot model, strict whole-candidate parser/validator, atomic last-valid
holder, exact generation-zero state и отдельный bounded self-metrics registry. ISSUE-016–ISSUE-022 реализуют единый
bounded ingestion core, atomic-file reconciliation, acknowledged Unix ingestion MSP/1 и его serialized client, bounded
loopback HTTP push, explicit mmap boundary и exhaustive cross-adapter conformance corpus. ISSUE-023–ISSUE-028
реализуют bounded exposition, response preparation, finalization ingestion barrier, final-wait state machine режимов
immediate/duration/scrape-count, complete-response drain и final-wait observability. ISSUE-029–ISSUE-031 добавляют
Kubernetes Job/CronJob examples, lifecycle controls и multi-replica Prometheus verification. ISSUE-032–ISSUE-037
добавляют static multi-architecture release artifacts, container hardening defaults, configurable capacity/time limits,
fault/soak/race gates, controlled release benchmarks и signed supply-chain evidence.

Managed Aggregation Wave 1 добавляет явную границу владения: snapshot остаётся режимом по умолчанию, а
`--mode=managed-registry` выбирает отдельный managed bootstrap. ISSUE-MA-002–ISSUE-MA-006 реализуют семантику метрик,
реестр одного запуска, ограниченный цикл с единственным владельцем, NDJSON protocol v1 и приватный Unix-сокет.
ISSUE-MA-007 добавляет клиент без локального состояния для `metricshell managed` и эталонный PHP 5.4-клиент. Они
различают accepted, rejected, overload, protocol, transport и unknown; unknown после полной отправки не повторяется
автоматически.
Protocol v1 принимает одну instrumentation operation в request; internal all-or-nothing primitive не является
публичной batch capability. ISSUE-MA-008 ограничивает managed families, active series, labels, histogram buckets и строки
descriptor/label. Policy-limit rejection отличается от queue overload и protocol rejection и сохраняет committed state
и generation.
ISSUE-MA-009 материализует одну полную registry generation в детерминированные immutable bytes протокола Application
Snapshot. Неизменившаяся generation переиспользует cache, concurrent misses объединяются, выданные reader bytes
изолированы, а неуспешный rebuild сохраняет предыдущую успешную entry.
ISSUE-MA-010 устанавливает materialized managed generations через тот же Core parser, admission barrier, atomic holder
и exposition source, что и snapshot transports. Ошибка conversion или Core validation сохраняет prior active state;
неизменившаяся managed generation не устанавливается повторно.
ISSUE-MA-011 закрывает managed socket и owner admission первым действием finalization. Только работа, admitted до этой
границы, может завершиться в существующем finalization/shutdown context; отдельный managed drain timeout не добавлен.
ISSUE-MA-012 ровно один раз замораживает drained managed registry, отклоняет каждого позднего publisher как `late` и
делает одну финальную попытку materialization/install через Core до существующих Core freeze и final-wait. Ошибка
финального candidate не повторяется и не заменяет ранее активный Core snapshot.
ISSUE-MA-013 добавляет self-metrics фиксированной кардинальности и structured events для operations, protocol, queue,
registry, materialization, freeze и final install. Управляемые приложением names, labels, paths, payloads и client
identity не попадают в observability labels или записи.
ISSUE-MA-014 добавляет production-container E2E gate: concurrent реальные Unix publishers, проверку commit при ACK
loss, protocol/semantic rejection, точную финальную exposition, final-scrape completion и два последовательных пустых
restart.
ISSUE-MA-015 объединяет реальные PHP 5.4 и process E2E gates с повторяемой race/resource matrix и controlled
owner/materialization benchmark. Raw results, точная конфигурация, revision и fingerprint Go OS/architecture/version
сохраняются в `dist/managed-validation`; timing evidence не выбирает defaults или SLA.
ISSUE-MA-016 публикует accepted bilingual Managed Aggregation contract, завершает configuration, lifecycle,
observability, client и release documentation и превращает EN/RU option completeness в Docker-tested release gate.
Review PR #38 закрывает live-visibility gap одним bounded coalescing publisher (`1s` default), сохраняет final freeze
authoritative, отображает cancellation после owner admission в `unknown` и проверяет non-zero exits, partial frames,
special numeric values, limit boundaries и oversized Core candidates.

Complete accepted application snapshots заменяют по одной immutable generation; live self-metrics используют
собственный fixed-cardinality state и не влияют на application identity. Production runtime создаёт один общий
`ingestion.Core`, направляет выбранный transport `file`, `unix` или `http` через него и финализируется через
`Core.CloseAndFreeze(ctx)` до того, как terminal exposition начинает наблюдать frozen snapshot.

## Требования

- Docker с запущенным daemon. Сборка использует закреплённый по digest Go 1.26 toolchain внутри Docker; Go toolchain на
  host не требуется и не используется workflow проекта.

## Команды

Запуск того же Docker-only gate, который используется в CI. Make только передаёт revision репозитория и вызывает Docker:

```sh
make ci
```

Запуск явного exit gate Wave 1, который сейчас совпадает с полным Docker gate:

```sh
make wave1
```

Запуск exit gate Wave 2 для lifecycle, shutdown, escalation и probes:

```sh
make wave2
```

Запуск exit gate Wave 3 для transport-independent snapshot и self-metrics state core:

```sh
make wave3
```

Запуск exit gate Wave 4 для common ingestion и cross-adapter conformance:

```sh
make wave4
```

Запуск exit gate Wave 5 для exposition и final wait:

```sh
make wave5
```

Запуск managed production-process E2E или полного gate Wave 6:

```sh
make managed-e2e
make wave6
```

Создание воспроизводимого managed production-validation evidence:

```sh
make managed-validation
```

Запуск exit gate Wave 6 для Kubernetes, hardening, release, benchmark и supply-chain:

```sh
make wave6
```

Запуск fault-injection gate:

```sh
make fault
```

Экспорт controlled benchmark artifacts:

```sh
make benchmark
```

Сборка static multi-architecture release artifacts:

```sh
make release
```

Экспорт signed supply-chain evidence с release signing keys, переданными как BuildKit secrets:

```sh
make supply-chain \
  RELEASE_SIGNING_KEY_FILE=/path/to/private.hex \
  RELEASE_SIGNING_PUBLIC_KEY_FILE=/path/to/public.hex
```

Docker-contained exit verifier проверяет на реальном artifact отклонённую bootstrap configuration (`64` и единственную
запись `configuration.rejected`), все workload exits `0-255` и отображения INT/TERM. Product exit codes наблюдаются
внутри контейнера, поэтому сбой Docker CLI нельзя принять за результат MetricShell.

Сборка минимального runtime image для выбранной Linux-архитектуры:

```sh
make build PLATFORM=linux/amd64 IMAGE=metricshell
```

Makefile только вызывает Docker targets; `make ci` запускает build/unit checks, real-container acceptance fixtures,
fault injection, controlled benchmark artifact generation и supply-chain verification с ephemeral CI signing key вне
repository. Release target `supply-chain` требует внешние signing-key и public-key files, переданные как BuildKit
secrets. Dockerfile не зависит от Make; host-команды Go и вспомогательные shell orchestration scripts не используются.

Supply-chain artifacts используют `SHA256SUMS` как signed manifest. Он покрывает release binaries, `go.mod`, raw
`MODULES.jsonl` / `GOVULNCHECK.json` inputs и каждый generated evidence file. SBOM verification сверяет module
components с подписанным module graph, включая module versions и sums.

Kubernetes examples находятся в `examples/kubernetes/` и покрывают direct Job discovery, lifecycle controls, CronJob
concurrency policy, PodMonitor integration и multi-replica Prometheus scrape validation. Docker hardening examples
находятся в `examples/docker/` и описывают non-root, read-only, no-new-privileges и tmpfs runtime defaults.

Запуск workload без shell interpretation:

```sh
docker run --rm metricshell:local -- /path/to/workload "argument with spaces"
```

Каждый token после первого standalone `--` передаётся напрямую как workload argv. Для shell behavior надо явно
запустить shell, например `-- /bin/sh -c 'command'`.

## Build identity

Корневой файл [`VERSION`](../VERSION) — единственный источник версии всего проекта. Сборка требует ровно одну непустую
строку. `REVISION` — commit исходников, передаваемый Makefile. Timestamp сборки не встраивается: архитектура его не
требует, а воспроизводимость от него ухудшается. При одинаковых исходниках, toolchain, архитектуре и identity build flags
исключают локальные пути, VCS probing и случайный build ID.

```text
metricshell version=0.1.2 revision=0123456
```

## Структура пакетов

- `client`: публичный official connection writer MSP/1 с complete-publication serialization и typed errors.
- `cmd/metricshell`: entrypoint production-бинарника.
- `internal/benchrelease`: controlled release benchmark contract и artifact checks.
- `internal/buildinfo`: build identity, передаваемый linker.
- `internal/cli`: bootstrap command surface.
- `internal/config`: валидированная workload, shutdown и exposition bootstrap configuration.
- `internal/conformance`: общий file/Unix/HTTP corpus semantics, state и observability.
- `internal/diagnostic`: упорядоченные structured lifecycle diagnostics для одной runtime identity.
- `internal/exposition`: immutable application/self-metric encoding, response bounds, compression, write outcomes и
  final-response drain.
- `internal/finalwait`: validated natural-completion policies, frozen-generation threshold и terminal decisions.
- `internal/hardening`: проверки container-hardening example и runtime defaults.
- `internal/ingestion`: transport-independent admission, cancellation, finalization barrier, result taxonomy и
  complete-candidate handoff.
- `internal/httpingest`: loopback-only POST adapter с независимыми wire/decoded limits и exact HTTP mapping.
- `internal/fileingest`: bounded no-follow file reconciliation и Linux directory-inotify recovery.
- `internal/kubeexamples`: проверка Kubernetes Job, CronJob, lifecycle и Prometheus examples.
- `internal/managed`: descriptor-driven semantics, execution-scoped registry, bounded single-owner mutation loop и
  generation/commit accounting.
- `internal/managedclient`: stateless managed-operation client, result taxonomy и shell-friendly command surface.
- `internal/managedmaterialize`: generation-keyed immutable cache полного snapshot encoding и reader ownership.
- `internal/managedobserve`: bounded-проекция managed self-metrics и редактированные lifecycle diagnostics.
- `internal/managedpublish`: single-goroutine bounded periodic publication и finalization join barrier.
- `internal/managedbridge`: передача полного managed candidate в существующий Core validation/atomic install path.
- `internal/managedfinalize`: single-winner freeze реестра и координация ровно одной финальной managed install.
- `internal/managedprotocol`: versioned bounded NDJSON framing и отображение domain/result.
- `internal/managedserver`: permission-aware managed Unix listener, bounded connections/deadlines и admission closure.
- `internal/socketingest`: bounded MSP/1 transactions, exact ACK/NACK framing и Unix listener с mode 0660.
- `internal/lifecycle`: synchronized public runtime state и transitions.
- `internal/probe`: bounded HTTP health/readiness responses, зависящие только от lifecycle state.
- `internal/promverify`: helpers для multi-replica Prometheus scrape и label-cardinality verification.
- `internal/releaseverify`: проверки release, Dockerfile, pinned images и supply-chain contract.
- `internal/shutdown`: валидированные shutdown budgets, deadlines, phase contexts и completion reasons.
- `internal/selfmetric`: bounded self-metric registry, immutable scrape views и text encoding Prometheus/OpenMetrics.
- `internal/snapshot`: immutable application snapshot model, parser, canonicalization, atomic holder, limits и rejection registry.
- `internal/supplychain`: signed release evidence, SBOM, provenance и vulnerability verification.
- `internal/workload`: запуск в управляемой process group, signal forwarding, subreaper adoption и child reaping.
- `internal/testfixture`: бинарники только для real-container acceptance, fault, release и supply-chain tests.
- `examples/clients/php54`: PHP 5.4 managed-operation reference client без зависимостей и документация.
- `internal/dependencyboundary`: проверки production/research, public API, primitives, modules и licenses boundaries.
- `../VERSION`: общая версия проекта на уровне репозитория.

## Нормативный контекст

Реализация следует ISSUE-011–ISSUE-037, EPIC-001, ADR-001–ADR-015, спецификациям Application Snapshot Protocol,
Configuration, Runtime State Machine, Self-Metrics и Structured Logging, ADR-012 Kubernetes viability, ADR-013
static multi-architecture distribution, ADR-014 security and limits, ADR-015 final benchmark policy и сквозному
definition of done до Wave 6 включительно.

## Инженерный контракт для следующих задач

- Принятые requirements, specifications и ADR определяют наблюдаемое поведение; delivery issues определяют scope
  реализации и обязательные acceptance tests. Узкая задача не должна неявно реализовывать runtime-поведение следующих.
- Production-код находится в этом модуле. Исследовательские прототипы служат только свидетельствами и не могут
  становиться production-импортами или копируемыми архитектурными shortcuts.
- Core принимает один complete candidate snapshot, целиком его проверяет и атомарно заменяет last valid state. Core не
  агрегирует и не объединяет состояния producers, не воспроизводит операции и не хранит историю snapshots.
- Resources, queues, payloads, concurrency и waits должны иметь явные границы. Ошибки используют нормативные registry
  exit codes и structured diagnostics без публикации аргументов workload, значений environment или payload data.
- Docker является единственной средой сборки и тестирования; Go запускается внутри pinned build layer, а Make остаётся
  внешним интерфейсом Docker-команд. Production Linux artifacts собираются без CGO для amd64 и arm64, когда применимо.
- Каждая задача завершается поддерживаемыми автоматическими тестами. Concurrency-код также проходит race detector, а
  релевантное container-поведение проверяется с запущенным Docker daemon.
- Английская и русская документация изменяются синхронно. Каждая задача проходит статусы `В работе`, `Тестирование` и
  `Готово`, фиксирует test evidence и завершается проверкой полноты затронутых README.

Для ISSUE-006 `make wave1` сохраняет все предыдущие проверки PID 1, argv, process group, signal forwarding и descendant
reaping. Docker-contained result verifier дополнительно запускает 256 независимых lifecycle MetricShell для exits
`0-255` и cases TERM/INT, доказывает одно выполнение workload и один authoritative result на lifecycle и отклоняет
ошибочную классификацию registry collisions как failures MetricShell. Pre-result internal failures остаются покрыты до
и после workload start; post-exit descendant work доказывает сохранение resolved workload result до завершения.
