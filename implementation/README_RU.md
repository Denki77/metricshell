# Production-реализация MetricShell

Этот каталог является production-модулем Go. Код из `research/` содержит исследовательские свидетельства и намеренно
не входит в модуль и граф зависимостей.

## Текущий объём

Wave 5 выполняется. ISSUE-023–ISSUE-026 реализуют bounded exposition, response preparation, finalization ingestion
barrier и final-wait state machine режимов immediate/duration/scrape-count. Wave 4
ISSUE-016–ISSUE-022 реализует единый bounded ingestion core, atomic-file reconciliation,
acknowledged Unix ingestion MSP/1 и его serialized client, bounded loopback HTTP push, explicit mmap boundary и
exhaustive cross-adapter conformance corpus.
ISSUE-011–ISSUE-015 реализуют immutable snapshot model, strict whole-candidate parser/validator, atomic
last-valid holder, exact generation-zero state и отдельный bounded self-metrics registry. Complete accepted application
snapshots заменяют по одной immutable generation; live self-metrics используют собственный fixed-cardinality state и не
влияют на application identity.
Production runtime создаёт один общий `ingestion.Core`, направляет выбранный transport `file`, `unix` или `http` через
него и финализируется через `Core.CloseAndFreeze(ctx)` до того, как terminal exposition начинает наблюдать frozen
snapshot.

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

Docker-contained exit verifier проверяет на реальном artifact отклонённую bootstrap configuration (`64` и единственную
запись `configuration.rejected`), все workload exits `0-255` и отображения INT/TERM. Product exit codes наблюдаются
внутри контейнера, поэтому сбой Docker CLI нельзя принять за результат MetricShell.

Сборка минимального runtime image для выбранной Linux-архитектуры:

```sh
make build PLATFORM=linux/amd64 IMAGE=metricshell
```

Makefile только вызывает Docker targets; `make ci` запускает build/unit checks и real-container acceptance fixtures.
Dockerfile не зависит от Make; host-команды Go и вспомогательные shell orchestration scripts не используются.

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
metricshell version=0.1.0-dev revision=0123456
```

## Структура пакетов

- `client`: публичный official connection writer MSP/1 с complete-publication serialization и typed errors.
- `cmd/metricshell`: entrypoint production-бинарника.
- `internal/buildinfo`: build identity, передаваемый linker.
- `internal/cli`: bootstrap command surface.
- `internal/config`: валидированная workload, shutdown и exposition bootstrap configuration.
- `internal/conformance`: общий file/Unix/HTTP corpus semantics, state и observability.
- `internal/diagnostic`: упорядоченные structured lifecycle diagnostics для одной runtime identity.
- `internal/exposition`: immutable application/self-metric encoding, response bounds, compression и write outcomes.
- `internal/finalwait`: validated natural-completion policies, frozen-generation threshold и terminal decisions.
- `internal/ingestion`: transport-independent admission, cancellation, finalization barrier, result taxonomy и
  complete-candidate handoff.
- `internal/httpingest`: loopback-only POST adapter с независимыми wire/decoded limits и exact HTTP mapping.
- `internal/fileingest`: bounded no-follow file reconciliation и Linux directory-inotify recovery.
- `internal/socketingest`: bounded MSP/1 transactions, exact ACK/NACK framing и Unix listener с mode 0660.
- `internal/lifecycle`: synchronized public runtime state и transitions.
- `internal/probe`: bounded HTTP health/readiness responses, зависящие только от lifecycle state.
- `internal/shutdown`: валидированные shutdown budgets, deadlines, phase contexts и completion reasons.
- `internal/selfmetric`: bounded self-metric registry, immutable scrape views и text encoding Prometheus/OpenMetrics.
- `internal/snapshot`: immutable application snapshot model, parser, canonicalization, atomic holder, limits и rejection registry.
- `internal/workload`: запуск в управляемой process group, signal forwarding, subreaper adoption и child reaping.
- `internal/testfixture`: бинарники только для real-container acceptance tests.
- `internal/dependencyboundary`: проверки production/research, public API, primitives, modules и licenses boundaries.
- `../VERSION`: общая версия проекта на уровне репозитория.

## Нормативный контекст

Реализация следует ISSUE-011–ISSUE-022, EPIC-001, ADR-001–ADR-011, ADR-014–ADR-015, спецификациям Application Snapshot Protocol,
Configuration, Runtime State Machine, Self-Metrics и Structured Logging, ADR-013 о статической multi-architecture
поставке и сквозному definition of done. Complete-response drain и final-wait observability остаются в
ISSUE-027–ISSUE-028.

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
