# ISSUE-006. Сохранение результата workload

**Статус:** Открыто
**Готовность:** Готово к разработке

**Эпик:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 1](../../02-epics/EPIC-001-core.md#wave-1)

**ADR/INV:** ADR-001, ADR-002 / INV-001, INV-002.  
**Результат:** после всех post-exit действий MetricShell возвращает исходный результат workload согласно policy.

**Acceptance criteria:**

- exit `0`, ненулевой exit и termination by signal различаются;
- final scrape timeout не подменяет workload result без явно выбранной policy;
- internal supervisor failure имеет отдельный диапазон exit codes;
- матрица exit propagation покрыта integration tests.

## Контракт готовности к разработке

- **Нормативные входы:** ADR-001, ADR-002 / INV-001,
  INV-002, [Configuration](../../../04-specification/configuration.md)
  и [Runtime State Machine](../../../04-specification/runtime-state-machine.md).
- **Зависимости:** ISSUE-002 и ISSUE-005.
- **Объём / вне объёма:** Сохранять exit 0, non-zero и signal outcomes через post-exit work. Вне scope: remapping
  результата запущенного workload в MetricShell-owned code.
- **Конфигурация и наблюдаемые отказы:** Pre-start failures используют закрытый exit registry MetricShell; post-start
  diagnostics фиксируют origin без замены полученного workload result.
- **Критерии приёмки и обязательные тесты:** Все byte-sized exits, включая collisions с registry; TERM/INT mapping;
  final-wait timeout; internal error до/после workload start.
- **Условие завершения:** Готово, когда exit propagation matrix является table-driven и проходит container integration
  tests.
