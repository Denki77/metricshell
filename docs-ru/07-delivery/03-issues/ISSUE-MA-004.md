# ISSUE-MA-004. Bounded single-owner mutation loop

**Статус:** Planned  
**Готовность:** Code-ready

**Эпик:** [EPIC-002 Managed Aggregation](../02-epics/EPIC-002-managed-aggregation.md)  
**Wave:** [Wave 2](../02-epics/EPIC-002-managed-aggregation.md#wave-2)

**ADR/INV:** ADR-017 / INV-017

## Нормативные входы

ADR-017 / INV-017, ADR-016, требования Managed Aggregation.

## Зависимости

ISSUE-MA-003.

## Scope

Реализовать один bounded owner goroutine/event loop, который эксклюзивно применяет admitted mutations, задаёт registry-wide commit order, сохраняет per-connection receive order и выдаёт success только после commit.

## Вне scope

Wire transport, materialization cache и lifecycle freeze.

## Конфигурация и наблюдаемые ошибки

Capacity owner queue — проверяемое положительное bounded-свойство managed mode из resource-control contract; нулевое, отрицательное, переполненное или иное недопустимое значение является startup configuration error до запуска workload. Во время runtime полная queue отклоняет admission как overload до передачи владельцу и сохраняет registry state/generation. Cancellation или истечение deadline до admission — rejection; после admission owner выдаёт один authoritative committed-or-rejected result. Success разрешён только для successful committed result. Queue depth/capacity, overload, cancellation и owner failures наблюдаемы через bounded labels; accepted item не может потеряться без результата.

## Критерии приёмки

- Concurrent counter increments сохраняют exact accepted sum.
- Gauge SET final value следует commit order.
- Histogram observation atomic.
- Descriptor races deterministic.
- Queue bounded и overload observably rejects.
- Accepted work не теряется silently.
- Race detector проходит.

## Обязательная матрица тестов

Concurrency matrix 1/2/8/32/128 publishers; counter/gauge/histogram/descriptor races; queue boundaries; deadline/cancellation; race detector.

## Завершение

Завершено, когда все критерии и обязательные тесты проходят CI, а задача сохраняет ADR-016...ADR-020 и backward compatibility snapshot mode.
