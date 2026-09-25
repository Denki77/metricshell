# ISSUE-MA-001. Конфигурация и bootstrap managed mode

**Статус:** Planned  
**Готовность:** Code-ready

**Эпик:** [EPIC-002 Managed Aggregation](../02-epics/EPIC-002-managed-aggregation.md)  
**Wave:** [Wave 1](../02-epics/EPIC-002-managed-aggregation.md#wave-1)

**ADR/INV:** ADR-016 / INV-016; ADR-020 / INV-020

## Нормативные входы

Требования Managed Aggregation, ADR-016, ADR-020, Configuration Specification, Configuration Value Grammar.

## Зависимости

Нет; задача начинает EPIC-002 после завершения Core.

## Scope

Добавить явный выбор режима managed registry при сохранении snapshot mode по умолчанию; выбирать managed bootstrap path только при соответствующей конфигурации; отклонять противоречивую или hybrid ownership configuration до запуска workload.

## Вне scope

Создание Managed Registry и пустой epoch, а также семантика registry (ISSUE-MA-003); transport server, materialization, Core bridge и реализация lifecycle drain/freeze.

## Конфигурация и наблюдаемые ошибки

Единственное публичное свойство этой задачи — selector режима. Отсутствующий selector или явный snapshot mode выбирает существующий bootstrap path; явный managed mode выбирает managed bootstrap boundary. Неизвестные значения, конфликтующие aliases и одновременный запрос snapshot- и managed-владения являются startup configuration errors и отклоняются до запуска workload. Эта задача не создаёт registry, epoch, socket или committed state, поэтому ошибка не может частично их инициализировать. Выбранный режим и безопасная причина отказа видны в существующей startup diagnostics.

## Критерии приёмки

- Default invocation остаётся snapshot mode и сохраняет поведение.
- Explicit snapshot mode эквивалентен default.
- Явный managed mode выбирает ровно один managed bootstrap path; пустой registry epoch создаёт ISSUE-MA-003.
- Invalid/hybrid ownership configuration детерминированно отклоняется до workload start, если ошибка статически известна.
- Snapshot-mode regression suite остаётся зелёным.

## Обязательная матрица тестов

Configuration table tests для default/snapshot/managed/unknown mode, precedence, contradictory options, bootstrap/no-listener assertions и snapshot regression suite.

## Завершение

Завершено, когда все критерии и обязательные тесты проходят CI, а задача сохраняет ADR-016...ADR-020 и backward compatibility snapshot mode.
