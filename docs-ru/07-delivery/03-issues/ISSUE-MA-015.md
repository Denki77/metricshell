# ISSUE-MA-015. Legacy, performance, resource и production validation

**Статус:** Planned  
**Готовность:** Code-ready

**Эпик:** [EPIC-002 Managed Aggregation](../02-epics/EPIC-002-managed-aggregation.md)  
**Wave:** [Wave 7](../02-epics/EPIC-002-managed-aggregation.md#wave-7)

**ADR/INV:** ADR-018 / INV-018; ADR-019 / INV-019; ADR-020 / INV-020

## Нормативные входы

Evidence discipline INV-018/019/020, ADR-018/019/020 и Core release benchmark policy.

## Зависимости

ISSUE-MA-014.

## Scope

Проверить production implementation реальными PHP 5.4 и shell clients и повторить key cardinality, histogram, owner-queue, concurrent scrape/mutation, slow client/scraper, memory pressure, shutdown/freeze и restart scenarios с reproducible evidence.

## Вне scope

Изменение architecture только по timing; превращение tested endpoints в product defaults/SLA без accepted rule.

## Конфигурация и наблюдаемые ошибки

Validation выполняется только с явно записанной production-like configuration; research endpoints являются test inputs, а не неявными defaults. Недопустимая harness configuration или отсутствие provenance останавливает validation run до принятия результатов. Под нагрузкой каждый настроенный frame/queue/series/histogram/deadline/finalization bound должен выдавать предписанный observable rejection или lifecycle outcome, а rejected work — сохранять проверяемые registry generation/Core state. Fatal OOM/process loss записывается отдельно от managed policy rejection. Timing/resource measurements включают environment fingerprint, configuration, outcome counts и raw evidence и сами по себе не устанавливают default или SLA.

## Критерии приёмки

- PHP 5.4 и shell проходят production compatibility.
- Configured bounds соблюдаются под load.
- Concurrent mutation/scrape не возвращает mixed generation.
- Slow clients/scrapers и memory pressure не обходят bounded policies.
- Shutdown/freeze/restart соответствует ADR-020.
- Raw evidence и provenance сохраняются.
- Timing number не становится default/SLA без explicit accepted selection.

## Обязательная матрица тестов

Production matrices из INV-018/019/020; multiple architectures/environments где практично; provenance/fingerprint; controlled resource pressure; correctness assertions.

## Завершение

Завершено, когда все критерии и обязательные тесты проходят CI, а задача сохраняет ADR-016...ADR-020 и backward compatibility snapshot mode.
