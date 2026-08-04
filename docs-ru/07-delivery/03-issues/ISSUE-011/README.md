# ISSUE-011. Каноническая модель публикации

**Статус:** Открыто
**Готовность:** Готово к разработке

**Эпик:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 3](../../02-epics/EPIC-001-core.md#wave-3)

## Нормативные входы

- ADR-004 / INV-004.
- [Протокол application snapshot](../../../04-specification/application-snapshot-protocol.md).
- [Runtime defaults и resource limits](../../../04-specification/runtime-defaults-and-resource-limits.md).

## Зависимости

ISSUE-001 — bootstrap конфигурации. Эта задача блокирует ISSUE-012, ISSUE-013 и все ingestion adapters.

## Scope

Реализовать immutable-представления CandidateSnapshot, ValidatedSnapshot и ActiveSnapshot; диспетчеризацию по версии
схемы; base-family/encoded-sample naming; identity семейств/series; значения counter, gauge и classic histogram;
normalization empty families и явное zero-series state; binary64 canonicalization, ordering и generation metadata.

## Вне scope

Transport framing, кодирование exposition, filtering, aggregation, per-producer state, replay операций и история.

## Конфигурация и ошибки

Применять limits.snapshot_bytes, limits.series, limits.labels_per_series, byte limits имён метрик, labels и HELP, а
также зарезервированный namespace metricshell_. Возвращать только rejection codes из registry snapshot protocol;
ошибки не должны изменять active state.

## Критерии приёмки

- Каждый документ version 1 преобразуется в один immutable candidate либо в один детерминированный rejection.
- Canonical identity не зависит от порядка JSON members и входных series.
- Семантика counter, gauge, histogram и zero-series точно соответствует публичной schema.
- Duplicate identity, metadata/type conflict, невалидная histogram, неизвестные fields и resource limits приводят к
  atomic rejection.
- После создания объекта не сохраняются mutable input buffers или collections, принадлежащие caller.

## Обязательная test matrix

Golden JSON/canonical fixtures; zero-series и normalization empty families; float64 overflow/underflow/rounding и
boundary collisions; negative histogram boundaries/sums; family component collisions и histogram `le`; duplicate
families/series/labels; порядок buckets/cumulative counts; reserved names; limits на границе/boundary+1;
fuzzing; race-detector tests для concurrent readers.

## Условие завершения

Задача завершена, когда fixtures публичного protocol создают детерминированные immutable values, каждый rejection code
покрыт, downstream tests validator/state holder используют эти types, а документация и conformance corpus связаны
ссылками.
