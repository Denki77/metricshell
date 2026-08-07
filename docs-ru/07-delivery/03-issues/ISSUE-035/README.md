# ISSUE-035. Набор fault, soak и race tests

**Статус:** Открыто
**Готовность:** Готово к разработке

**Эпик:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 6](../../02-epics/EPIC-001-core.md#wave-6)

Покрыть повторяющийся malformed input, slow clients, disconnects, saturation queue, bind failure, forced OOM, races signals и graceful drain.

## Контракт готовности к разработке

- **Нормативные входы:** ADR-001–ADR-015 и все accepted specifications,
  особенно [Runtime Defaults](../../../04-specification/runtime-defaults-and-resource-limits.md)
  и [Structured Logging](../../../04-specification/structured-logging.md).
- **Зависимости:** Implementation surfaces ISSUE-001–ISSUE-034.
- **Объём / вне объёма:** Автоматизировать fault injection, soak, fuzz и race coverage bounded runtime behavior. Вне
  scope: переопределение normative limits по benchmark results.
- **Конфигурация и наблюдаемые отказы:** Каждый injected failure проверяет exit/result origin, last-valid state, closed
  log/self-metric enums и bounded completion.
- **Критерии приёмки и обязательные тесты:** Malformed flood; slow clients; disconnects; saturation; bind/path failure;
  OOM container; signal/publication/scrape races; long reconciliation/drain.
- **Условие завершения:** Готово, когда production binary проходит suites документированной длительности с reproducible
  seeds и сохранёнными failure artifacts.
