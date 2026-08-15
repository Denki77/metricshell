# Production-реализация MetricShell

Этот каталог является production-модулем Go. Код из `research/` содержит исследовательские свидетельства и намеренно
не входит в модуль и граф зависимостей.

## Текущий объём

ISSUE-005 включает Linux child-subreaper mode до запуска workload и использует единственного владельца `wait4` для
primary и всех adopted descendants. PID и process-compatible result primary остаются authoritative и публикуются ровно
один раз; каждый reaped process классифицируется как `direct` или `adopted` без публикации child PID. MetricShell ждёт,
пока все adopted children будут reaped. Forced shutdown budgets и post-exit lifecycle остаются в последующих задачах.

## Требования

- Docker с запущенным daemon. Сборка использует закреплённый по digest Go 1.26 toolchain внутри Docker; Go toolchain на
  host не требуется и не используется workflow проекта.

## Команды

Запуск того же Docker-only gate, который используется в CI. Make только передаёт revision репозитория и вызывает Docker:

```sh
make ci
```

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

Реализация следует ISSUE-005, EPIC-001, ADR-001, ADR-003, спецификациям Runtime State Machine, Self-Metrics и Structured
Logging, ADR-013 о статической multi-architecture поставке и сквозному definition of done. Child reaping не добавляет
grace-budget или forced-kill policy из ISSUE-009.

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

Для ISSUE-005 `make ci` дополнительно проверяет orphan adoption через double spawn, включая внешний Docker init над
MetricShell, оба порядка завершения primary-before-child и child-before-primary, reuse primary PID, ровно один primary
result, reaping diagnostics `direct|adopted`, burst из 64 завершающихся children с нулём стабильных zombies, sanitized
failures subreaper/reaper, гарантированный cleanup direct child и race detector. Существующее signal-forwarding
acceptance coverage остаётся активным.
