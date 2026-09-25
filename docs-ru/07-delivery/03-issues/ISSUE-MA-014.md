# ISSUE-MA-014. E2E suite concurrency, shutdown, restart и failures

**Статус:** Planned  
**Готовность:** Code-ready

**Эпик:** [EPIC-002 Managed Aggregation](../02-epics/EPIC-002-managed-aggregation.md)  
**Wave:** [Wave 6](../02-epics/EPIC-002-managed-aggregation.md#wave-6)

**ADR/INV:** ADR-016 / INV-016; ADR-017 / INV-017; ADR-018 / INV-018; ADR-019 / INV-019; ADR-020 / INV-020

## Нормативные входы

ADR-016 — ADR-020 и все accepted Managed Aggregation specifications.

## Зависимости

ISSUE-MA-012 и ISSUE-MA-013.

## Scope

Создать production-process E2E/race coverage всего path: real Unix clients, owner queue, registry, materialization, Core install/exposition, workload lifecycle, overload, failure injection и restart.

## Вне scope

Performance certification/default selection; research prototype execution вместо production tests.

## Конфигурация и наблюдаемые ошибки

Suite проверяет принятые minimum/equal/over-limit values и invalid startup configuration для каждого managed property из ISSUE-MA-001 и ISSUE-MA-004...013. Runtime assertions различают protocol, semantic, resource, overload, late, timeout и unknown outcomes и после каждого rejection проверяют committed registry generation и active Core state. Failure injection охватывает bind/read/write, owner, materialization, validation/install, diagnostics и budget exhaustion. Тесты должны обнаруживать silent drops, success без commit, mutation после freeze, mixed generations, unbounded waits и resource leaks и фиксировать bounded observable outcome каждой ошибки.

## Критерии приёмки

- Multiple publishers сохраняют exact accepted state и один commit order.
- ACK-loss сохраняет committed mutation и unknown client outcome.
- Malformed/partial/disconnected/overload сохраняют valid Core state.
- Natural/nonzero exit, SIGTERM, budget exhaustion и late publishers соответствуют ADR-020.
- Restart стартует empty epoch.
- Final scrape показывает frozen generation с existing eligibility rules.
- Race detector проходит.

## Обязательная матрица тестов

Production scenario matrix из INV-017/018/020; repeated races; process-level containers; race detector; leak/deadlock timeouts.

## Завершение

Завершено, когда все критерии и обязательные тесты проходят CI, а задача сохраняет ADR-016...ADR-020 и backward compatibility snapshot mode.
