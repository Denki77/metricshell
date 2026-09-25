# ISSUE-MA-016. Документация, examples и release readiness

**Статус:** Planned  
**Готовность:** Code-ready

**Эпик:** [EPIC-002 Managed Aggregation](../02-epics/EPIC-002-managed-aggregation.md)  
**Wave:** [Wave 7](../02-epics/EPIC-002-managed-aggregation.md#wave-7)

**ADR/INV:** ADR-016 / INV-016; ADR-017 / INV-017; ADR-018 / INV-018; ADR-019 / INV-019; ADR-020 / INV-020

## Нормативные входы

Требования Managed Aggregation; ADR-016 — ADR-020; Configuration, Runtime State Machine, Self-Metrics, Structured Logging, Defaults/Resource Limits, Docker/Compose docs.

## Зависимости

ISSUE-MA-015 и все предыдущие задачи Managed Aggregation.

## Scope

Завершить normative Managed Aggregation specification и обновить EN/RU implementation-facing specs, traceability, mode/protocol/socket security, limits, lifecycle, CLI/PHP examples, Docker/Compose/Job-shaped usage, troubleshooting и release notes.

## Вне scope

Новые architecture decisions, hybrid mode, distributed/persistent aggregation.

## Конфигурация и наблюдаемые ошибки

Для каждого публичного managed property документируются owner, syntax, источник default, valid domain, точка startup/runtime validation и observable failure. Примеры используют допустимые bounded values и показывают protocol/semantic/resource/overload/late/unknown outcomes без обещания безопасного retry или success до commit. Документация фиксирует, что rejected operations сохраняют committed registry/Core state, а final candidate validation/install может завершиться ошибкой при сохранении prior Core state. Нарушение EN/RU parity, broken links, недокументированная configuration или противоречащие ADR/specification примеры блокируют release.

## Критерии приёмки

- Behavioral draft promoted/replaced в accepted normative spec.
- Configuration/defaults/resource limits описывают все public managed properties.
- Self-metrics/logging specs содержат final managed registry/enums.
- Runtime lifecycle документирует ADR-020 composition.
- CLI/PHP/Docker/Compose examples verified по docs policy.
- Traceability связывает каждый FR-MA/NFR-MA с code/tests.
- Snapshot-mode compatibility/regression evidence зафиксированы.
- EN/RU parity checks проходят.

## Обязательная матрица тестов

Documentation link/parity checks; example automation; configuration completeness audit; traceability completeness audit; full CI и snapshot/managed regression suites.

## Завершение

Завершено, когда все критерии и обязательные тесты проходят CI, а задача сохраняет ADR-016...ADR-020 и backward compatibility snapshot mode.
