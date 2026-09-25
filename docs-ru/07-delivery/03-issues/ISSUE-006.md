# ISSUE-006. Сохранение результата workload

**Статус:** Готово
**Готовность:** Готово к разработке

**Эпик:** [EPIC-001 Core](../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 1](../02-epics/EPIC-001-core.md#wave-1)

**ADR/INV:** ADR-001, ADR-002 / INV-001, INV-002.  
**Результат:** после всех post-exit действий MetricShell возвращает исходный результат workload согласно policy.

**Acceptance criteria:**

- exit `0`, ненулевой exit и termination by signal различаются;
- final scrape timeout не подменяет workload result без явно выбранной policy;
- internal supervisor failure имеет отдельный диапазон exit codes;
- матрица exit propagation покрыта integration tests.

## Контракт готовности к разработке

- **Нормативные входы:** ADR-001, ADR-002 / INV-001,
  INV-002, [Configuration](../../04-specification/configuration.md)
  и [Runtime State Machine](../../04-specification/runtime-state-machine.md).
- **Зависимости:** ISSUE-002 и ISSUE-005.
- **Объём / вне объёма:** Сохранять exit 0, non-zero и signal outcomes через post-exit work. Вне scope: remapping
  результата запущенного workload в MetricShell-owned code.
- **Конфигурация и наблюдаемые отказы:** Pre-start failures используют закрытый exit registry MetricShell; post-start
  diagnostics фиксируют origin без замены полученного workload result.
- **Критерии приёмки и обязательные тесты:** Все byte-sized exits, включая collisions с registry; TERM/INT mapping;
  final-wait timeout; internal error до/после workload start.
- **Условие завершения:** Готово, когда exit propagation matrix является table-driven и проходит container integration
  tests.

## Журнал выполнения

- 2026-08-24: задача переведена в `В работе`; реализация начата в отдельном worktree `ISSUE-006`, основанном на merged
  ISSUE-005.
- 2026-08-24: задача переведена в `Тестирование`; table-driven unit matrix и Docker-contained result matrix `0-255`
  плюс TERM/INT прошли вместе со всеми предыдущими integration checks Wave 1.
- 2026-08-24: задача переведена в `Готово`; явный Docker gate Wave 1, arm64 production image и проверки полноты README
  EN/RU прошли.

## Свидетельства проверки

- `make wave1` прошёл в Docker Desktop/LinuxKit aarch64 с закреплённым по digest Go 1.26 builder. Gate включает `gofmt`,
  `go vet`, `go test -race`, dependency-boundary checks, обе статические Linux-архитектуры и все real-container
  acceptance fixtures, накопленные ISSUE-001—ISSUE-006.
- Table-driven unit matrix разрешает все exits `0-255`, `SIGINT -> 130` и `SIGTERM -> 143` как результаты started
  workload.
- Один Docker-contained verifier запустил 256 независимых lifecycle MetricShell/workload плюс INT и TERM. Каждый
  lifecycle выполнил workload ровно один раз, выпустил ровно один `workload.started` и один соответствующий
  `workload.exited` и вернул ожидаемый result.
- Тот же verifier запускает реальный artifact с отклонённой bootstrap configuration и требует exit `64`, пустой stdout
  и ровно одну запись `configuration.rejected`/`CONFIG_INVALID`. Assert остаётся внутри Docker, поэтому exit Docker CLI
  нельзя принять за результат MetricShell.
- Workload exits, численно равные registry MetricShell (`64`, `70`, `71`, `72`, `73`), остались workload results и не
  создали `runtime.failed` или `workload.start_failed`; lifecycle records сохраняют origin.
- Существующая unit failure injection различает internal failure до workload start и после start, но до primary
  resolution. Оба используют MetricShell-owned path `70` и сохраняют корректный факт `Started`.
- Primary result хранится отдельно во время reaping post-exit adopted children. Normal post-exit completion, включая
  нормативный будущий final-wait timeout, не имеет пути для его замены. Реализация и timing фактических final-wait modes
  остаются в ISSUE-026.
- Audit Wave 1 подтвердил статусы `Готово` для ISSUE-001—ISSUE-006 в обеих языковых версиях и способность MetricShell
  работать basic PID 1 entrypoint при отключённом metrics ingestion: argv preservation, process-group ownership, signal
  forwarding, subreaping под external init, zero-zombie burst handling и result preservation проходят в Docker.
- Production `scratch` image собран для `linux/arm64`, его entrypoint-проверка `--help` прошла.
- `implementation/README.md` и `implementation/README_RU.md` проверены и обновлены вместе.
