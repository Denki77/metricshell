# ISSUE-MA-007. CLI helper и PHP 5.4 reference client

**Статус:** Planned  
**Готовность:** Code-ready

**Эпик:** [EPIC-002 Managed Aggregation](../02-epics/EPIC-002-managed-aggregation.md)  
**Wave:** [Wave 3](../02-epics/EPIC-002-managed-aggregation.md#wave-3)

**ADR/INV:** ADR-018 / INV-018

## Нормативные входы

ADR-018 / INV-018, требования Managed Aggregation.

## Зависимости

ISSUE-MA-006.

## Scope

Предоставить shell-friendly CLI operations и минимальный PHP 5.4-compatible stateless reference client для protocol v1 и documented safe retry boundaries.

## Вне scope

Persistent local agent, exactly-once retry guarantees, modern Prometheus client dependency.

## Конфигурация и наблюдаемые ошибки

Конфигурация CLI/client ограничена socket endpoint и bounded client deadline. Отсутствующий или недопустимый endpoint/deadline является локальной invocation error до подключения. Недопустимые аргументы операции отклоняются локально без отправки frame. Server outcomes `rejected`, `overload` и protocol errors имеют разные machine-readable non-zero results; ошибки connect/read/write являются transport outcomes. EOF или timeout после отправки полного request, но до response, означает `unknown`, поскольку commit мог состояться; helper не должен рекомендовать blind retry. Diagnostics не содержит credentials и неограниченных application payloads.

## Критерии приёмки

- Shell использует operations без построения snapshots.
- PHP 5.4 client не хранит registry state.
- Accepted/rejected/connect/protocol/unknown outcomes machine-detectable.
- Reconnect не требует registry reconstruction.
- Unknown outcome не retry blindly.

## Обязательная матрица тестов

Real PHP 5.4 container tests; shell CLI tests; accepted/rejected/connect/protocol/unknown cases; reconnect; quoting/labels/numeric edges.

## Завершение

Завершено, когда все критерии и обязательные тесты проходят CI, а задача сохраняет ADR-016...ADR-020 и backward compatibility snapshot mode.
