# ISSUE-MA-006. Unix socket server managed operations

**Статус:** Planned  
**Готовность:** Code-ready

**Эпик:** [EPIC-002 Managed Aggregation](../02-epics/EPIC-002-managed-aggregation.md)  
**Wave:** [Wave 3](../02-epics/EPIC-002-managed-aggregation.md#wave-3)

**ADR/INV:** ADR-018 / INV-018; ADR-017 / INV-017; ADR-020 / INV-020

## Нормативные входы

ADR-018, ADR-017, ADR-020, Configuration Specification.

## Зависимости

ISSUE-MA-005 и ISSUE-MA-004.

## Scope

Реализовать production Unix stream endpoint, filesystem lifecycle, configured permissions, bounded connection deadlines, protocol handoff к owner admission, startup readiness и close-admission behavior.

## Вне scope

Local HTTP/TCP managed transport, mandatory sidecar, remote transport.

## Конфигурация и наблюдаемые ошибки

Socket path, filesystem permissions и connection read/write deadlines относятся к managed-mode configuration. Пустой или непригодный path, недопустимые permissions, неположительные или непредставимые deadlines, bind conflict и небезопасный существующий filesystem object являются startup errors; readiness не публикуется, workload не запускается. Во время runtime истечение deadline, partial input, disconnect и closed admission дают разные transport outcomes и не обходят framing или owner admission. Cleanup может удалить только socket instance текущего execution. Ошибки bind/read/write/accept/cleanup, readiness и admission closure наблюдаемы с безопасным path и bounded error classes.

## Критерии приёмки

- Socket ready до workload publication.
- Concurrent local clients безопасно достигают owner.
- Slow/partial/disconnected client не повреждает registry и не потребляет unbounded resources.
- Permissions соответствуют accepted configuration, а не prototype values.
- Admission closure детерминированно reject new work.
- Listener cleanup deterministic.

## Обязательная матрица тестов

AF_UNIX E2E; concurrent clients; slow/partial deadlines; disconnect cases; path/permission/bind failures; startup readiness; admission closure; leak/race tests.

## Завершение

Завершено, когда все критерии и обязательные тесты проходят CI, а задача сохраняет ADR-016...ADR-020 и backward compatibility snapshot mode.
