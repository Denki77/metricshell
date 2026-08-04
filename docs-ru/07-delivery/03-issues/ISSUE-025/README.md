# ISSUE-025. Ingestion barrier finalization

**Статус:** Открыто
**Готовность:** Готово к разработке

**Эпик:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 5](../../02-epics/EPIC-001-core.md#wave-5)

**ADR/INV:** ADR-011 / INV-011.  
При workload exit новые publications закрываются до final wait; accepted in-flight publication имеет явно выбранную
ordering policy.

## Контракт готовности к разработке

- **Нормативные входы:** ADR-011 / INV-011, [Runtime State Machine](../../../04-specification/runtime-state-machine.md)
  и [Application Snapshot Protocol](../../../04-specification/application-snapshot-protocol.md).
- **Зависимости:** ISSUE-007, ISSUE-013 и ISSUE-016.
- **Объём / вне объёма:** Закрыть admission при finalization, определить ordering already admitted candidates, freeze
  одну final generation и отклонять late publications. Вне scope: merge late data.
- **Конфигурация и наблюдаемые отказы:** `frozen` использует shared mapping; in-flight acceptance linearized до/после
  barrier и наблюдаем в logs/metrics.
- **Критерии приёмки и обязательные тесты:** Publication до/на/после barrier; queued/validating candidate; все adapters;
  concurrent workload exit; generation freeze; race detector.
- **Условие завершения:** Готово, когда каждый schedule даёт одну deterministic frozen generation без post-barrier
  mutation.
