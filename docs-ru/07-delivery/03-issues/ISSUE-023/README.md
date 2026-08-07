# ISSUE-023. Сервер Prometheus exposition

**Статус:** Открыто
**Готовность:** Готово к разработке

**Эпик:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 5](../../02-epics/EPIC-001-core.md#wave-5)

## Нормативные входы

- ADR-004, ADR-010, ADR-014 / INV-010.
- [Спецификация filtering метрик](../../../04-specification/metrics-filtering.md).
- [Спецификация self-metrics](../../../04-specification/self-metrics.md).
- [Спецификация configuration](../../../04-specification/configuration.md).
- [Runtime defaults и resource limits](../../../04-specification/runtime-defaults-and-resource-limits.md).

## Зависимости

ISSUE-013 — active snapshot, ISSUE-015 — self-metrics, ISSUE-024 — bounded pre-encoding.

## Scope

Создать listener по configured exposition address; реализовать GET /metrics с baseline Prometheus text 0.0.4 и
content negotiation OpenMetrics 1.0; выбирать одну immutable application generation на request; применять принятую
family filtering; добавлять полный домен self-metrics; экспонировать фиксированные health/readiness paths согласно
lifecycle contract.

## Вне scope

Per-request filter parameters, filters по labels/values, aggregation, host-wide collectors, remote write и durable
доставка scrape.

## Конфигурация и наблюдаемые ошибки

Реализовать exposition.listen, limits размера response, concurrency и write timeout, правила metrics include/exclude и
фиксированные paths. Unsupported methods/media negotiation возвращают документированные 4xx; saturation — 503; encoding
или response-limit failure происходит до success headers; bind failure завершается с endpoint_bind_failed;
cancelled/partial writes не считаются success.

## Критерии приёмки

- Parsers Prometheus и OpenMetrics принимают successful responses с корректными content type и EOF.
- Request наблюдает одну application generation и отдельно согласованный self-metric view.
- Include/exclude precedence, family atomicity и reserved self-metrics соответствуют filtering specification.
- Response limits проверяются до отправки status 200.
- Probe/debug requests не попадают в scrape counters или final-scrape eligibility.
- Concurrent handlers остаются в configured bounds и drain-ятся в shutdown budget.

## Обязательная test matrix

Оба формата и варианты Accept; format-specific counter HELP/TYPE/sample names; component-name collisions; finite,
`NaN`, `+Inf` и `-Inf` gauge values; валидные non-negative histograms; exposition empty families/zero-series; каждое
filtering rule/precedence case; наличие
self-metrics; concurrent replacement во время scrape; slow/cancelled clients; encoding и size failures; saturation; bind
failure; health/readiness в каждом runtime state; graceful drain; race detector.

## Условие завершения

Задача завершена, когда проходят parser-based golden tests, filtering conformance, bounds/failure tests, lifecycle
probes и concurrent snapshot-selection tests, а все наблюдаемые ошибки документированы.
