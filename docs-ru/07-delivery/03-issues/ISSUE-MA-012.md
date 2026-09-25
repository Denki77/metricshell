# ISSUE-MA-012. Freeze, final snapshot и final-scrape integration

**Статус:** Planned  
**Готовность:** Code-ready

**Эпик:** [EPIC-002 Managed Aggregation](../02-epics/EPIC-002-managed-aggregation.md)  
**Wave:** [Wave 5](../02-epics/EPIC-002-managed-aggregation.md#wave-5)

**ADR/INV:** ADR-020 / INV-020; ADR-019 / INV-019; ADR-004 / INV-004; ADR-011 / INV-011

## Нормативные входы

ADR-020, ADR-019, ADR-004, Runtime State Machine и existing final-scrape specification.

## Зависимости

ISSUE-MA-011 и ISSUE-MA-010.

## Scope

Реализовать один logical freeze на managed epoch, отклонять publishers после freeze, выбрать frozen generation как authoritative final generation, выполнить одну final candidate/install attempt и делегировать ожидание существующим Core final-scrape modes.

## Вне scope

Новый managed final wait, Kubernetes API coordination, persistence/replay.

## Конфигурация и наблюдаемые ошибки

Finalization использует только существующие Core final-scrape mode/duration/count и shutdown budget; недопустимые lifecycle values остаются startup configuration errors. Один успешный freeze фиксирует одну authoritative final registry generation. Для неё выполняется ровно одна final materialization/candidate/install attempt. Conversion, validation или install могут завершиться ошибкой; ошибка наблюдаема, не вызывает retry или выбор другой generation и сохраняет ранее active Core state. После freeze любой publisher отклоняется как late и не меняет registry. Freeze winner/generation, late rejection, candidate outcome и retained-Core-state outcome наблюдаемы через bounded labels.

## Критерии приёмки

- Concurrent freeze attempts дают одного logical winner.
- Application registry/generation immutable после freeze.
- Late publishers reject deterministic.
- Один freeze выбирает одну authoritative final generation и запускает одну final candidate/install attempt.
- Если final materialization, validation или installation завершается ошибкой, final generation не устанавливается, а previous Core state остаётся active.
- Failed final candidate сохраняет prior Core state.
- Core final-scrape eligibility/counting не меняется; self-metrics могут меняться.
- New execution стартует fresh empty epoch.

## Обязательная матрица тестов

Freeze races; late publisher storm; conversion/validation/install failures; natural/nonzero/signal exits; immediate/duration/scrapes; eligible/ineligible scrape matrix; restart/new epoch.

## Завершение

Завершено, когда все критерии и обязательные тесты проходят CI, а задача сохраняет ADR-016...ADR-020 и backward compatibility snapshot mode.
