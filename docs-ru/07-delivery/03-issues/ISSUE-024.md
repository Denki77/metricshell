# ISSUE-024. Pre-encoding response и limits

**Статус:** Готово
**Готовность:** Готово к разработке

**Эпик:** [EPIC-001 Core](../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 5](../02-epics/EPIC-001-core.md#wave-5)

Oversized response отклоняется до отправки success headers; gzip не меняет identity.

## Контракт готовности к разработке

- **Нормативные входы:** ADR-010 и
  ADR-014, [Runtime Defaults](../../04-specification/runtime-defaults-and-resource-limits.md), [Metric Filtering](../../04-specification/metrics-filtering.md)
  и [Self-Metrics](../../04-specification/self-metrics.md).
- **Зависимости:** ISSUE-013 и ISSUE-015; блокирует ISSUE-023.
- **Объём / вне объёма:** Pre-encode один complete identity response до success headers и применять
  response/concurrency/write limits. Вне scope: streaming partial success.
- **Конфигурация и наблюдаемые отказы:** Oversize/encode failure возвращает 503 до success headers и фиксирует closed
  exposition outcome; timeout/cancellation не считается final scrape.
- **Критерии приёмки и обязательные тесты:** At limit/limit+1; text/OpenMetrics; self-metrics-only; filter extremes;
  concurrent limit; gzip negotiation; short/slow/cancelled writes.
- **Условие завершения:** Готово, когда ни один failure path не экспонирует successful partial metric family и не
  превышает configured bounds.

## Журнал выполнения

- 2026-09-02: переведена в `В работе`; реализованы immutable application/self-metric pre-encoding для Prometheus и
  OpenMetrics, exact uncompressed response bounds, gzip preparation, write classification и non-blocking limiter.
- 2026-09-02: переведена в `Тестирование`; exact-limit, limit-minus-one, оба формата, self-metrics-only, gzip identity,
  saturation, short-write и cancellation cases прошли Docker race gate.
- 2026-09-02: переведена в `Готово`; пройдены Docker vet/race/dependency/multi-architecture checks и EN/RU README audit.

## Свидетельства проверки

- Response полностью находится в памяти и укладывается в uncompressed limit до commit status `200`.
- Gzip меняет только transport bytes; selected generation, format body и uncompressed byte accounting стабильны.
- Short writes, cancellation и invalid encoding не дают success outcome; concurrency exhaustion bounded и не ожидает.
