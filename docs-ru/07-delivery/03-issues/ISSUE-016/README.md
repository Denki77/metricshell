# ISSUE-016. Общий transport-independent ingestion interface

**Статус:** Открыто
**Готовность:** Готово к разработке

**Эпик:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 4](../../02-epics/EPIC-001-core.md#wave-4)

**ADR/INV:** ADR-005 / INV-005.  
Единые result types, error taxonomy, admission hooks, deadlines и candidate handoff.

## Контракт готовности к разработке

- **Нормативные входы:** ADR-005 /
  INV-005, [Application Snapshot Protocol](../../../04-specification/application-snapshot-protocol.md), [Configuration](../../../04-specification/configuration.md)
  и [Runtime Defaults](../../../04-specification/runtime-defaults-and-resource-limits.md).
- **Зависимости:** ISSUE-012 и ISSUE-013; блокирует ISSUE-017, ISSUE-018 и ISSUE-020.
- **Объём / вне объёма:** Определить общие admission, deadlines, candidate handoff, result и error taxonomy. Вне scope:
  wire framing adapters.
- **Конфигурация и наблюдаемые отказы:** Candidate reasons, publication outcomes и transport failures являются разными
  typed enums и map-ятся в общие logs/self-metrics.
- **Критерии приёмки и обязательные тесты:** Contract tests с fake adapters для
  accepted/rejected/busy/timeout/frozen/internal; cancellation; queue boundaries; enum-parity compile/test check.
- **Условие завершения:** Готово, когда каждый adapter реализует interface без ad hoc translation semantic rejection.
