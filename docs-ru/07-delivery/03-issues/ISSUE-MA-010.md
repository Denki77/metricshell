# ISSUE-MA-010. Core candidate bridge и atomic installation

**Статус:** Planned  
**Готовность:** Code-ready

**Эпик:** [EPIC-002 Managed Aggregation](../02-epics/EPIC-002-managed-aggregation.md)  
**Wave:** [Wave 5](../02-epics/EPIC-002-managed-aggregation.md#wave-5)

**ADR/INV:** ADR-019 / INV-019; ADR-020 / INV-020; ADR-004 / INV-004

## Нормативные входы

ADR-004, ADR-019, ADR-020, Application Snapshot Protocol.

## Зависимости

ISSUE-MA-009.

## Scope

Преобразовать одну complete managed generation в existing Core candidate representation и переиспользовать production whole-candidate validation, atomic active-state install и exposition path.

## Вне scope

Managed-specific Core store/path, direct registry exposition, изменённые Core semantics.

## Конфигурация и наблюдаемые ошибки

Bridge не добавляет managed-specific Core configuration или альтернативный store. Ошибки conversion и существующие Core validation/installation errors отклоняют candidate целиком; никакие partial family/series не становятся active, а previous Core state остаётся authoritative. Active state меняется только после успешного завершения существующего atomic Core install. Candidate generation, conversion result, validation rejection и install result наблюдаемы через bounded internal diagnostics без второго managed exposition path. Существующие Core size/parse constraints применяются без ослабления.

## Критерии приёмки

- Managed generation создаёт self-contained complete candidate.
- Candidate использует тот же validator, что snapshot mode.
- Successful install использует тот же atomic Core state holder.
- Invalid candidate сохраняет prior active state.
- Exposition не имеет separate managed path.
- ADR-004 semantics не меняются.

## Обязательная матрица тестов

Managed-to-Core conformance corpus; valid/invalid conversion; validator failures; atomic replacement; concurrent scrapes; equivalent snapshot exposition comparison.

## Завершение

Завершено, когда все критерии и обязательные тесты проходят CI, а задача сохраняет ADR-016...ADR-020 и backward compatibility snapshot mode.
