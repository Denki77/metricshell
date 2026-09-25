# ISSUE-MA-013. Managed self-metrics и structured diagnostics

**Статус:** Planned  
**Готовность:** Code-ready

**Эпик:** [EPIC-002 Managed Aggregation](../02-epics/EPIC-002-managed-aggregation.md)  
**Wave:** [Wave 6](../02-epics/EPIC-002-managed-aggregation.md#wave-6)

**ADR/INV:** ADR-017 / INV-017; ADR-019 / INV-019; ADR-020 / INV-020

## Нормативные входы

NFR-MA-006, Self-Metrics Specification, Structured Logging Specification, ADR-017, ADR-019, ADR-020.

## Зависимости

ISSUE-MA-008 и ISSUE-MA-012.

## Scope

Добавить bounded managed-mode self-metrics/logs для accepted/rejected operations, rejection class, overload, queue depth/capacity, active series, generations, materialization/cache, protocol/server failures, freeze и finalization.

## Вне scope

Arbitrary application metric names/labels или per-client identifiers в self-metric labels.

## Конфигурация и наблюдаемые ошибки

Application-controlled metric name, label value, client identity, socket path или raw payload не могут быть self-metric labels. Опциональные diagnostic level/sink следуют существующей logging configuration; недопустимый enum/sink отклоняется до запуска workload. Неизвестные rejection/result enum values являются implementation errors и не сводятся к success. Metrics/logs различают protocol, semantic, resource, overload, late, unknown-client-outcome, materialization и final-install results; ошибка emission diagnostics не меняет registry/Core state или result операции. Cardinality bounds, redaction и стабильные enum schemas являются проверяемыми contracts.

## Критерии приёмки

- Semantic/protocol/overload/resource rejection различимы.
- Queue/resource state observable без application cardinality leakage.
- Materialization/cache/lifecycle transitions имеют stable metrics/logs.
- Redaction rules сохраняются.
- Snapshot-mode observability не меняется случайно.

## Обязательная матрица тестов

Metric registry golden tests; bounded-label audit; log schema/enums; rejection/lifecycle paths; redaction; snapshot regression.

## Завершение

Завершено, когда все критерии и обязательные тесты проходят CI, а задача сохраняет ADR-016...ADR-020 и backward compatibility snapshot mode.
