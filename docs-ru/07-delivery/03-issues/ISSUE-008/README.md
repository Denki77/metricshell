# ISSUE-008. Модель shutdown budget

**Статус:** Готово
**Готовность:** Готово к разработке

**Эпик:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 2](../../02-epics/EPIC-001-core.md#wave-2)

**ADR/INV:** ADR-003 / INV-003.  
**Результат:** единый budget разбивается между forwarding, workload grace, final scrape, server drain и forced cleanup.

**Acceptance criteria:**

- никакая фаза не может ждать бесконечно;
- remaining budget передаётся следующей фазе;
- конфигурация валидируется до запуска workload;
- exhausted budget имеет явную reason и diagnostics.

## Контракт готовности к разработке

- **Нормативные входы:** ADR-003 /
  INV-003, [Configuration](../../../04-specification/configuration.md), [грамматика значений configuration](../../../04-specification/configuration-value-grammar.md), [Runtime Defaults](../../../04-specification/runtime-defaults-and-resource-limits.md)
  и [Runtime State Machine](../../../04-specification/runtime-state-machine.md).
- **Зависимости:** ISSUE-007.
- **Объём / вне объёма:** Получить один monotonic absolute deadline и распределить workload timeout, reserve,
  finalization и drain внутри него. Вне scope: продление external deadline.
- **Конфигурация и наблюдаемые отказы:** Невалидные cross-field budgets отклоняются до workload start; exhaustion имеет
  closed completion reason и structured remaining time.
- **Критерии приёмки и обязательные тесты:** Каждая duration boundary; equality/overflow timeout плюс reserve;
  already-expired deadline; clock advancement; cancellation каждой phase.
- **Условие завершения:** Готово, когда ни одна phase не превышает absolute deadline, а все validation/error paths
  наблюдаемы.

## Журнал выполнения

- 2026-08-24: задача переведена в `В работе`; реализованы shutdown configuration и deadline-derived phase model.
- 2026-08-24: задача переведена в `Тестирование`; grammar, range, equality/overflow, expiry, clock advancement и
  phase-cancellation tests прошли под race detector в Docker.
- 2026-08-24: задача переведена в `Готово`; invalid budgets отклоняются до spawn, каждый phase context ограничен deadline.

## Свидетельства проверки

- Defaults равны `30s` total, `28s` workload и `2s` reserve; CLI имеет приоритет над environment, environment — над defaults.
- Duration parser реализует нормативную integer/unit grammar и отклоняет signs, fractions, compounds, leading zeroes и
  overflow до range validation.
- Resolved plan удерживает один absolute deadline, задаёт нулевой workload grace, когда reserve не помещается, и никогда
  не продлевает external deadline.
- Signal forwarding, workload grace, finalization, HTTP drain и forced cleanup получают cancellable contexts,
  ограниченные phase boundary и общим deadline.
- CLI-level test подтверждает возврат `64`/`configuration.rejected` для overcommitted budget без workload event.
- `implementation/README.md` и `implementation/README_RU.md` проверены и обновлены вместе.
