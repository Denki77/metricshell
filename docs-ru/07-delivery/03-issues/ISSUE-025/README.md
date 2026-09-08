# ISSUE-025. Ingestion barrier finalization

**Статус:** Готово
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

## Журнал выполнения

- 2026-09-02: переведена в `В работе`; в Core добавлены linearized admission barrier, bounded finalization wait и
  exactly-once freeze final snapshot, общие для каждого transport.
- 2026-09-02: переведена в `Тестирование`; executing, queued, timeout, repeated-close и post-barrier file/Unix/HTTP
  schedules прошли под Docker race detector.
- 2026-09-02: переведена в `Готово`; frozen publications обновляют общие rejection metrics/diagnostics, пройдены Docker
  vet/race/dependency/multi-architecture checks и EN/RU README audit.

## Свидетельства проверки

- Admission и closure используют одну synchronization boundary, поэтому каждая publication детерминированно находится
  до или после barrier.
- Работа, admitted до closure, может установиться в пределах переданного context budget; его expiry замораживает
  предыдущую last-valid generation, а поздний install отклоняется с `frozen`.
- Каждый вызов видит одну final generation, а все три transport identities отклоняют post-barrier publications.
