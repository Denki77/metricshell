# ISSUE-013. Atomic holder последнего валидного state

**Статус:** Открыто
**Готовность:** Готово к разработке

**Эпик:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 3](../../02-epics/EPIC-001-core.md#wave-3)

**ADR/INV:** ADR-004, ADR-010 / INV-004, INV-010.

**Acceptance criteria:** concurrent readers видят только одну generation; rejection сохраняет last-valid; omitted series
исчезают при replacement.

## Контракт готовности к разработке

- **Нормативные входы:** ADR-004 и
  ADR-014, [Application Snapshot Protocol](../../../04-specification/application-snapshot-protocol.md)
  и [Runtime State Machine](../../../04-specification/runtime-state-machine.md).
- **Зависимости:** ISSUE-011 и ISSUE-012.
- **Объём / вне объёма:** Атомарно устанавливать immutable validated snapshots с одной monotonic generation. Вне scope:
  merge, history, replay и per-producer state.
- **Конфигурация и наблюдаемые отказы:** Rejected/frozen candidates сохраняют предыдущие pointer/generation; internal
  swap failures используют closed internal reason и structured event.
- **Критерии приёмки и обязательные тесты:** Concurrent readers/writers; удаление omitted series; zero-series
  replacement; rejection retention; generation ordering; race detector и allocation ownership.
- **Условие завершения:** Готово, когда под stress readers видят только полную старую или полную новую generation.
