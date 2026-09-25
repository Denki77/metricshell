# ISSUE-MA-011. Runtime admission barrier и bounded drain

**Статус:** Planned  
**Готовность:** Code-ready

**Эпик:** [EPIC-002 Managed Aggregation](../02-epics/EPIC-002-managed-aggregation.md)  
**Wave:** [Wave 5](../02-epics/EPIC-002-managed-aggregation.md#wave-5)

**ADR/INV:** ADR-020 / INV-020; ADR-003 / INV-003; ADR-017 / INV-017

## Нормативные входы

ADR-020 / INV-020, ADR-003, ADR-017, Runtime State Machine.

## Зависимости

ISSUE-MA-006 и ISSUE-MA-010.

## Scope

Интегрировать close-admission с natural workload exit и external termination; только already complete/validated/admitted work может завершиться в remaining existing shutdown/finalization budget.

## Вне scope

Новый drain timeout, unbounded drain, mandatory client flush/close handshake, final freeze implementation.

## Конфигурация и наблюдаемые ошибки

Задача переиспользует существующий shutdown/finalization budget и не добавляет второй drain timeout. Недопустимая существующая lifecycle duration/count configuration остаётся startup error по Runtime State Machine. После закрытия admission новые, partial, validated-but-not-admitted и queued-without-owner-acceptance операции получают late/closed rejection и не меняют state. Уже admitted owner-операция может завершиться только в оставшемся budget; его истечение даёт наблюдаемый bounded-drain failure/abandonment outcome без ложного success для uncommitted operation. Причина admission close, admitted/in-flight counts, drain completion и budget exhaustion наблюдаемы с bounded labels.

## Критерии приёмки

- New operations reject после closure.
- Partial и received-not-admitted work не commit после closure.
- Admitted/committing work может завершиться только пока есть budget.
- Budget exhaustion детерминированно завершает drain.
- Shutdown остаётся в ADR-003 budget.
- ACK semantics соответствуют ADR-017.

## Обязательная матрица тестов

Natural exit/SIGTERM на каждом receive/validate/queue/commit/ACK stage; zero/small/expiring budgets; slow admitted work; late connection storm.

## Завершение

Завершено, когда все критерии и обязательные тесты проходят CI, а задача сохраняет ADR-016...ADR-020 и backward compatibility snapshot mode.
