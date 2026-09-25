# ISSUE-MA-008. Resource controls Managed Aggregation

**Статус:** Planned  
**Готовность:** Code-ready

**Эпик:** [EPIC-002 Managed Aggregation](../02-epics/EPIC-002-managed-aggregation.md)  
**Wave:** [Wave 4](../02-epics/EPIC-002-managed-aggregation.md#wave-4)

**ADR/INV:** ADR-019 / INV-019; ADR-017 / INV-017; ADR-018 / INV-018

## Нормативные входы

ADR-019 / INV-019, ADR-017, ADR-018, Configuration, Runtime Defaults and Resource Limits.

## Зависимости

ISSUE-MA-004, ISSUE-MA-005 и ISSUE-MA-006.

## Scope

Реализовать configurable independent bounds для active series, histogram buckets, owner queue, protocol frame и других accepted managed dimensions; reject до mutation/unsafe allocation.

## Вне scope

Использование research coverage endpoints как defaults; маскировка fatal OOM как normal rejection.

## Конфигурация и наблюдаемые ошибки

Независимые limits охватывают active families/series, labels, histogram buckets, размеры batch/operation, capacity owner queue и protocol frame согласно принятому configuration contract. Отсутствующие значения используют только принятые documented defaults; нулевые, отрицательные, переполненные, внутренне противоречивые или неподдерживаемые значения отклоняются до запуска workload. Во время runtime каждый limit проверяется до unsafe allocation и mutation. Исчерпание limit — policy rejection, отличный от queue overload, protocol rejection и fatal OOM; весь committed registry и generation сохраняются. Имя limit, его bound и число rejection наблюдаемы без application-controlled labels.

## Критерии приёмки

- Каждый bound применяется независимо.
- At/under limit succeeds; limit+1 rejects.
- Rejected operation сохраняет committed generation/state.
- Queue overload отличается от policy rejection.
- Fatal OOM остаётся process/container failure.
- Ни одно число INV-019 не становится default без accepted rule.

## Обязательная матрица тестов

Below/equal/above matrix для всех limits; combined limits; configuration validation; queue overload; generation preservation; memory-pressure distinction; snapshot regression.

## Завершение

Завершено, когда все критерии и обязательные тесты проходят CI, а задача сохраняет ADR-016...ADR-020 и backward compatibility snapshot mode.
