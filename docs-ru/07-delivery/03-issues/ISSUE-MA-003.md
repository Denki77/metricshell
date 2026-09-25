# ISSUE-MA-003. Managed Registry и execution epoch

**Статус:** Planned  
**Готовность:** Code-ready

**Эпик:** [EPIC-002 Managed Aggregation](../02-epics/EPIC-002-managed-aggregation.md)  
**Wave:** [Wave 1](../02-epics/EPIC-002-managed-aggregation.md#wave-1)

**ADR/INV:** ADR-016 / INV-016; ADR-020 / INV-020

## Нормативные входы

ADR-016, ADR-020, требования Managed Aggregation.

## Зависимости

ISSUE-MA-002.

## Scope

Реализовать один in-memory Managed Registry на MetricShell/workload execution, family/series storage, committed generation tracking, deterministic mutation API и fresh empty epoch.

## Вне scope

Concurrency owner, transport, materialization cache, persistence/replay.

## Конфигурация и наблюдаемые ошибки

Создание registry не имеет отдельной публичной конфигурации помимо managed-mode boundary из ISSUE-MA-001. Невозможность создать in-memory registry — startup error до запуска workload. Каждый execution создаёт generation zero без families и series; persistence, replay или повторное использование предыдущего epoch являются недопустимыми состояниями. Runtime semantic rejection возвращает domain error из ISSUE-MA-002 и сохраняет текущую generation и весь committed registry. Успешная mutation увеличивает generation ровно один раз; создание и чтение registry её не меняют. Переходы generation и классы отклонений наблюдаемы без раскрытия application label values.

## Критерии приёмки

- Accepted mutation увеличивает generation ровно один раз.
- Rejected mutation сохраняет generation/state.
- Complete registry read соответствует одной generation.
- Новый execution стартует empty.
- Persistence/replay/recovery path отсутствует.
- Publisher disconnect не удаляет unrelated state.

## Обязательная матрица тестов

Registry unit tests для generation accounting, rejected mutations, empty epoch, restart/new instance, disconnect independence и complete-state reads.

## Завершение

Завершено, когда все критерии и обязательные тесты проходят CI, а задача сохраняет ADR-016...ADR-020 и backward compatibility snapshot mode.
