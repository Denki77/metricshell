# ISSUE-015. Отдельный домен self-metrics

**Статус:** Открыто
**Готовность:** Готово к разработке

**Эпик:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 3](../../02-epics/EPIC-001-core.md#wave-3)

## Нормативные входы

- ADR-004, ADR-010, ADR-011, ADR-014.
- [Спецификация self-metrics](../../../04-specification/self-metrics.md).
- [Runtime state machine](../../../04-specification/runtime-state-machine.md).

## Зависимости

ISSUE-007 — lifecycle states, ISSUE-011 — canonical model, ISSUE-013 — active-state holder.

## Scope

Реализовать полный registry metricshell_, типы метрик, HELP/TYPE metadata, все закрытые label enums, one-hot series
состояний и modes, метрики generation/publication/ingestion/exposition/final-wait/shutdown и правила lifecycle
reset/update.

## Вне scope

Application labels или values, raw paths/IDs/error text, unbounded labels, фильтрация self-metrics механизмом application
filtering и любое влияние на application snapshot identity.

## Конфигурация и ошибки

В version 1 у self-metrics нет отдельного enable switch. Они подчиняются exposition.response_bytes и экспонируют bounded
классы внутренних сбоев. Конфликт при создании registry является internal_failure и предотвращает запуск workload.

## Критерии приёмки

- Каждая metric и label value из принятой specification существует с объявленным type и semantics.
- Полный one-hot набор states совпадает с runtime state machine; только одно состояние равно 1.
- Self-metrics остаются mutable, пока application snapshot заморожен.
- Rejection, replacement и filtering application candidate не добавляют, не удаляют и не переименовывают self-metric
  series.
- Строки, контролируемые атакующим, никогда не становятся labels.

## Обязательная test matrix

Golden exposition для каждого lifecycle state; все transport/outcome/reason enums; нулевые и ненулевые generations;
final-wait modes и terminal reasons; concurrent updates/scrapes под race detector; cardinality bound; reserved-name
rejection; невосприимчивость к filtering; process restart/reset.

## Условие завершения

Задача завершена, когда реализован весь нормативный registry, проходят golden outputs и enum exhaustiveness tests, а
structured logging использует те же state/mode/outcome/reason values.
