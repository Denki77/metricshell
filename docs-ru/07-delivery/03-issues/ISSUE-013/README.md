# ISSUE-013. Atomic holder последнего валидного state

**Статус:** Готово
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

## Журнал выполнения

- 2026-08-28: переведена в `В работе`; реализованы last-valid holder и linear installation boundary.
- 2026-08-28: переведена в `Тестирование`; replacement, omission, lifetime binding, freeze, overflow, ownership и
  concurrent reader/writer tests прошли race detector в Docker.
- 2026-08-28: переведена в `Готово`; readers загружают одну immutable old или new generation, пока writers атомарно
  устанавливают или отклоняют complete candidate.

## Свидетельства проверки

- Installation и freeze используют единый serialized boundary; active reads выполняют один atomic pointer load.
- Accepted replacements получают последовательные generations и удаляют omitted families/series без merge и history.
- Frozen, lifetime type-conflicting и generation-overflow candidates сохраняют content и generation.
- Empty families не создают lifetime type binding, что соответствует их zero-series normalization semantics.
- Возвращаемые active/validated representations остаются caller-owned copies; concurrent stress проходит race detector.
- `implementation/README.md` и `implementation/README_RU.md` проверены и обновлены синхронно.
