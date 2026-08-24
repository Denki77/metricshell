# Production-реализация MetricShell

Этот каталог является production-модулем Go. Код из `research/` содержит исследовательские свидетельства и намеренно
не входит в модуль и граф зависимостей.

## Текущий объём

ISSUE-006 завершает supervisor foundation Wave 1. MetricShell сохраняет каждый byte-sized result primary workload,
включая значения, численно совпадающие с собственным registry `64` и `70-73`, и отображает signal termination как
`128+signal`. Structured lifecycle records и факт workload-started определяют origin результата. Primary result
разрешается один раз и сохраняется во время reaping adopted descendants. Final-wait modes, forced shutdown budgets и
полная lifecycle state machine остаются в последующих waves.

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

- `cmd/metricshell`: entrypoint production-бинарника.
- `internal/buildinfo`: build identity, передаваемый linker.
- `internal/cli`: bootstrap command surface.
- `internal/config`: bootstrap registry ошибок конфигурации.
- `internal/diagnostic`: упорядоченные structured lifecycle diagnostics для одной runtime identity.
- `internal/workload`: запуск в управляемой process group, signal forwarding, subreaper adoption и child reaping.
- `internal/testfixture`: бинарники только для real-container acceptance tests.
- `internal/dependencyboundary`: автоматический тест изоляции production от research.
- `../VERSION`: общая версия проекта на уровне репозитория.

## Нормативный контекст

Реализация следует ISSUE-006, EPIC-001, ADR-001, ADR-002, ADR-003, спецификациям Configuration, Runtime State Machine,
Self-Metrics и Structured Logging, ADR-013 о статической multi-architecture поставке и сквозному definition of done.
Result preservation не реализует lifecycle states из ISSUE-007, forced-kill policy из ISSUE-009 или final-wait modes из
ISSUE-026.

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
