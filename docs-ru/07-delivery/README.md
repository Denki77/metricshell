# Delivery documentation

[English version](../../docs/07-delivery/README.md)

Раздел связывает принятые требования, исследования, ADR и спецификации с задачами реализации. Русская и английская
версии равноправны и обновляются синхронно.

## Статус проекта — production baseline v0.2.2

Core и optional Managed Registry реализованы. Production gate пройден; первый успешный GitHub Release опубликован
для Linux amd64/arm64. Текущая production baseline — `v0.2.2`; версия остаётся pre-1.0 без обещания неограниченной
совместимости API. Новые функциональные этапы не запланированы. Дальнейшие изменения ограничены дефектами,
безопасностью, совместимостью и сопровождением. Delivery-планы ниже сохраняют историю реализации и не задают новую roadmap.

- [Production-установка и проверка](production-deployment.md)
- [Итоговые release notes](releases/v0.2.2.md)
- [Аудит опубликованных артефактов и приёмки](../../docs/07-delivery/releases/v0.2.2-audit.md)
- [Действия владельца и будущие улучшения release infrastructure](../../docs/07-delivery/releases/v0.2.2-maintenance.md)

## Нормативная связь

Delivery-документы являются планами реализации. Принятые требования, ADR и спецификации задают наблюдаемое поведение.
Issue готова к разработке, только если содержит ссылки на эти входы, scope, acceptance criteria, обязательные тесты,
наблюдаемые ошибки, зависимости и условия завершения.

## Навигация

- [Трассировка requirements к ADR/specifications](01-traceability/requirements-research-adr-traceability.md)
- [EPIC-001: MetricShell Core](02-epics/EPIC-001-core.md)
- [ISSUE-001](03-issues/ISSUE-001.md)…[ISSUE-037](03-issues/ISSUE-037.md)
