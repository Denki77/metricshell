# ISSUE-MA-002. Domain model и descriptor semantics Managed Aggregation

**Статус:** Planned  
**Готовность:** Code-ready

**Эпик:** [EPIC-002 Managed Aggregation](../02-epics/EPIC-002-managed-aggregation.md)  
**Wave:** [Wave 1](../02-epics/EPIC-002-managed-aggregation.md#wave-1)

**ADR/INV:** ADR-016 / INV-016

## Нормативные входы

ADR-016 / INV-016, требования Managed Aggregation, managed behavioral model.

## Зависимости

ISSUE-MA-001.

## Scope

Реализовать production domain types для descriptors, family/series identity, canonical labels, counter/gauge/classic-histogram semantics, descriptor conflict rules, reserved names и batch semantics ADR-016.

## Вне scope

Owner queue, socket protocol/server, Core materialization/installation, lifecycle.

## Конфигурация и наблюдаемые ошибки

Эта задача принимает descriptors и операции, а не startup properties. Недопустимые имена метрик, неподдерживаемые типы или операции, конфликты canonical labels, недопустимые числовые operands, нестрого возрастающие histogram buckets, reserved-name collisions и конфликты descriptor/type/HELP/label/bucket являются semantic runtime rejections. Отклонённая операция или batch возвращает стабильный класс ошибки и не меняет descriptors, значения series и generation. Весь batch проверяется до mutation; ни один его префикс не может быть committed. Capacity policy относится к ISSUE-MA-008, framing errors — к ISSUE-MA-005.

## Критерии приёмки

- Descriptor identity детерминирован и порядок labels не меняет series identity.
- Counter/gauge operations реализуют semantics ADR-016.
- Histogram observation атомарно обновляет count, sum и applicable buckets.
- Descriptor/type/HELP/label/bucket conflicts reject без partial mutation.
- Supported batches all-or-nothing.

## Обязательная матрица тестов

Table-driven unit tests для numeric edge cases, NaN/Inf, labels, descriptor conflicts, histogram boundaries, batches и сохранения state после rejection.

## Завершение

Завершено, когда все критерии и обязательные тесты проходят CI, а задача сохраняет ADR-016...ADR-020 и backward compatibility snapshot mode.
