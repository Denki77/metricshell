# ISSUE-022. Cross-adapter conformance suite

**Статус:** Открыто
**Готовность:** Готово к разработке

**Эпик:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 4](../../02-epics/EPIC-001-core.md#wave-4)

Один набор complete snapshots и failure cases выполняется для file, socket и push; accepted state identity должна
совпадать.

## Контракт готовности к разработке

- **Нормативные входы:** ADR-004–ADR-008 и
  ADR-015, [Application Snapshot Protocol](../../../04-specification/application-snapshot-protocol.md), [Runtime Defaults](../../../04-specification/runtime-defaults-and-resource-limits.md)
  и [Self-Metrics](../../../04-specification/self-metrics.md).
- **Зависимости:** ISSUE-017, ISSUE-018 и ISSUE-020.
- **Объём / вне объёма:** Прогонять один immutable acceptance/rejection corpus через все три adapters. Вне scope:
  adapter-specific exceptions к semantic validation.
- **Конфигурация и наблюдаемые отказы:** Проверять exact canonical bytes, generation, active state, rejection reason,
  log fields и self-metric deltas; transport-only failures проверяются отдельно.
- **Критерии приёмки и обязательные тесты:** Каждый candidate reason; finite/special numbers; empty state; все limits;
  concurrency; timeout; disconnect; malformed transport и recovery.
- **Условие завершения:** Готово, когда добавление reason/enum требует одного shared fixture, а parity test обновляет
  все adapters.
