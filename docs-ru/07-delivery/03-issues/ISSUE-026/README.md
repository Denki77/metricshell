# ISSUE-026. State machine final scrape

**Статус:** Открыто
**Готовность:** Готово к разработке

**Эпик:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 5](../../02-epics/EPIC-001-core.md#wave-5)

Default N=1 и finite timeout; modes immediate/duration/N; probes исключены; counter является saturating.

## Контракт готовности к разработке

- **Нормативные входы:** ADR-011 и ADR-012 / INV-011,
  INV-012, [Runtime State Machine](../../../04-specification/runtime-state-machine.md), [Runtime Defaults](../../../04-specification/runtime-defaults-and-resource-limits.md)
  и [Self-Metrics](../../../04-specification/self-metrics.md).
- **Зависимости:** ISSUE-007, ISSUE-010, ISSUE-023 и ISSUE-025.
- **Объём / вне объёма:** Реализовать immediate, duration и scrape-count final-wait modes с finite timeout и frozen
  generation. Вне scope: completion по probes/partial response.
- **Конфигурация и наблюдаемые отказы:** Создаётся ровно один closed completion reason; timeout/external termination
  bounded, а ISSUE-006 сохраняет workload result.
- **Критерии приёмки и обязательные тесты:** Каждый mode; N=1/N>1; timeout; no scraper; external signal; concurrent
  threshold responses; probes; stale-marker-aware verifier behavior.
- **Условие завершения:** Готово, когда state machine завершается ровно один раз при любом event ordering.
