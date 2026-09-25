# ISSUE-005. Reaping дочерних процессов и обработка orphan

**Статус:** Готово
**Готовность:** Готово к разработке

**Эпик:** [EPIC-001 Core](../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 1](../02-epics/EPIC-001-core.md#wave-1)

**ADR/INV:** ADR-001 / INV-001.  
**Результат:** MetricShell reap-ит завершившихся потомков и не оставляет zombies.

**Acceptance criteria:**

- orphaned descendants корректно reap-ятся PID 1;
- workload primary PID отслеживается отдельно;
- zombie count после stress test равен нулю;
- неожиданные child exits отражаются в diagnostics.

## Контракт готовности к разработке

- **Нормативные входы:** ADR-001 /
  INV-001, [Runtime State Machine](../../04-specification/runtime-state-machine.md), [Self-Metrics](../../04-specification/self-metrics.md)
  и [Structured Logging](../../04-specification/structured-logging.md).
- **Зависимости:** ISSUE-002 и ISSUE-003.
- **Объём / вне объёма:** Reap direct/adopted children с отдельным tracking primary workload. Вне scope: supervision
  сторонних services.
- **Конфигурация и наблюдаемые отказы:** Unexpected child outcomes дают sanitized diagnostics; primary outcome остаётся
  authoritative; child PIDs исключены из metric labels.
- **Критерии приёмки и обязательные тесты:** Orphan adoption, double-fork, burst exits, порядок
  primary-before-child/child-before-primary, zero-zombie stress, race detector.
- **Условие завершения:** Готово, когда stress fixtures не оставляют zombies и дают ровно один primary workload result.

## Журнал выполнения

- 2026-08-15: задача переведена в `В работе`; реализация начата в отдельном worktree `ISSUE-005`.
- 2026-08-15: задача переведена в `Тестирование`; реализованы adoption через subreaper, единый владелец child reaping,
  отделение результата primary и Docker acceptance fixtures; Docker unit/race gate и integration suite прошли.
- 2026-08-15: задача переведена в `Готово`; финальные Docker CI, arm64 production image и проверка полноты README EN/RU
  пройдены.
- 2026-08-15: задача возвращена в `В работе`; review выявил пробелы обработки reuse primary PID и проверки subreaper
  под external init.
- 2026-08-15: задача снова переведена в `Тестирование`; reuse primary PID обрабатывается однократно, а orphan adoption
  проходит с внешним Docker init над MetricShell.
- 2026-08-15: задача снова переведена в `Готово`; review fixes прошли финальный Docker CI и проверки полноты README
  EN/RU.

## Свидетельства проверки

- `make ci` прошёл с закреплённым по digest Go 1.26 builder; пройдены `gofmt`, `go vet`, `go test -race`, проверки
  dependency boundary и все существующие container acceptance tests.
- MetricShell включает Linux child-subreaper mode до workload spawn и использует одного blocking-владельца `wait4`,
  поэтому direct и adopted children не могут быть получены конкурирующими waiters.
- Real-container fixtures покрывают double-spawn orphan adoption в обоих порядках primary-before-child и
  child-before-primary, сохраняя exit codes `21` и `22` как единственные authoritative primary results. Второй сценарий
  запускается с Docker `--init` и доказывает subreaper adoption, когда MetricShell не является PID 1.
- Burst fixture создаёт 64 adopted children, наблюдает 64 reap records с `kind=adopted` и один с `kind=direct`, а
  inspection `/proc` подтверждает ноль стабильных zombies.
- `workload.exited` публикуется ровно один раз с `forced=false`; `child.reaped` содержит только закрытый kind
  `direct|adopted` и не раскрывает child PID.
- Failures subreaper и неожиданные failures reaper используют sanitized `runtime.failed` / `INTERNAL_FAILURE`. Unit
  tests подтверждают отсутствие запуска workload при ошибке subreaper setup и обязательный reap direct child до
  возврата из error paths reaper, observer и forwarding.
- Детерминированный PID-reuse test передаёт primary PID, другой child и тот же числовой PID повторно; исходный primary
  result и единственное primary event сохраняются, а переиспользованный PID классифицируется как adopted.
- Production `scratch` image собран для `linux/arm64`, его entrypoint-проверка `--help` прошла.
- `implementation/README.md` и `implementation/README_RU.md` проверены и обновлены вместе. Grace budgeting и forced
  descendant cleanup явно остаются в ISSUE-009.
