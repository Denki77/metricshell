# ISSUE-MA-009. Generation-based immutable materialization

**Статус:** Planned  
**Готовность:** Code-ready

**Эпик:** [EPIC-002 Managed Aggregation](../02-epics/EPIC-002-managed-aggregation.md)  
**Wave:** [Wave 4](../02-epics/EPIC-002-managed-aggregation.md#wave-4)

**ADR/INV:** ADR-019 / INV-019

## Нормативные входы

ADR-019 / INV-019, ADR-004, ADR-017.

## Зависимости

ISSUE-MA-003, ISSUE-MA-004 и ISSUE-MA-008.

## Scope

Реализовать immutable encoded snapshot cache, привязанный к complete registry generation, с reuse unchanged generation, stale marking после commit и safe concurrent/slow-reader lifetime.

## Вне scope

Core install, lifecycle freeze, гарантия публикации каждой intermediate generation.

## Конфигурация и наблюдаемые ошибки

Materialization принимает целую committed generation и не имеет отдельного публичного mode switch. Невозможное или неподдерживаемое значение registry либо encoding failure отклоняет candidate и сохраняет предыдущую immutable cache entry и Core state; partial bytes не публикуются. Concurrent requests для неизменной generation могут переиспользовать одно immutable representation, а commit лишь помечает его stale для будущих requests. Cache generation, hit/miss/rebuild/failure и coalescing наблюдаемы через bounded labels. Slow readers сохраняют выданные bytes и не могут продлить mutable registry ownership или удерживать неограниченное число generations сверх принятой resource policy.

## Критерии приёмки

- Каждый encoded body принадлежит одной registry-wide generation.
- Mixed-generation response отсутствует при concurrent mutation/readers.
- Unchanged generation reuse representation.
- New commit делает cache stale без изменения issued bytes.
- Slow readers безопасно удерживают immutable bytes.
- Intermediate generations могут coalesce.

## Обязательная матрица тестов

Linked cross-family consistency test; concurrent mutation/materialization; slow readers; cache hit/miss; stale rebuild; race detector; memory lifetime checks.

## Завершение

Завершено, когда все критерии и обязательные тесты проходят CI, а задача сохраняет ADR-016...ADR-020 и backward compatibility snapshot mode.
