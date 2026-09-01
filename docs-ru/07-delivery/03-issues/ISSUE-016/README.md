# ISSUE-016. Общий transport-independent ingestion interface

**Статус:** Готово
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

## Журнал выполнения

- 2026-08-29: переведена в `В работе`; реализованы общие registries transport/result/failure, bounded admission,
  отменяемая очередь и handoff complete candidate в atomic holder.
- 2026-08-29: переведена в `Тестирование`; acceptance, rejection, busy, timeout, frozen, internal, cancellation,
  queue-boundary, linearization и enum-parity tests прошли под race detector в Docker.
- 2026-08-29: переведена в `Готово`; production module прошёл Docker gate `make test`, оба implementation README
  проверены на полноту.

## Свидетельства проверки

- File, Unix и HTTP используют единый контракт `Publisher` и возвращают typed `Result`; candidate reasons остаются в
  snapshot registry, wire failures находятся в отдельном typed registry.
- Admission независимо ограничивает executing и pending work. Cancellation проверяется до parsing и повторно перед
  atomic installation, поэтому timed-out работа не изменяет active state.
- Общий metrics observer обновляет inflight, publication, rejection, last-success и active-snapshot metrics без
  attacker-controlled labels.
- Concurrent accepted candidates получают уникальные монотонные generations из одного holder.
