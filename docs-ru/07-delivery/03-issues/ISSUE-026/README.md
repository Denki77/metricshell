# ISSUE-026. State machine final scrape

**Статус:** Готово
**Готовность:** Готово к разработке

**Эпик:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 5](../../02-epics/EPIC-001-core.md#wave-5)

Default N=1 и finite timeout; modes immediate/duration/N; probes исключены; counter является saturating.

## Контракт готовности к разработке

- **Нормативные входы:** ADR-011 и ADR-012 / INV-011,
  INV-012, [Runtime State Machine](../../../04-specification/runtime-state-machine.md), [Runtime Defaults](../../../04-specification/runtime-defaults-and-resource-limits.md)
  и [Self-Metrics](../../../04-specification/self-metrics.md).
- **Зависимости:** ISSUE-007, ISSUE-010, ISSUE-023 и ISSUE-025.
- **Объём / вне объёма:** Реализовать immediate, duration и scrape-count final-wait modes с finite timeout и frozen
  generation. Вне scope: completion по probes/partial response.
- **Конфигурация и наблюдаемые отказы:** Создаётся ровно один closed completion reason; timeout/external termination
  bounded, а ISSUE-006 сохраняет workload result.
- **Критерии приёмки и обязательные тесты:** Каждый mode; N=1/N>1; timeout; no scraper; external signal; concurrent
  threshold responses; probes; stale-marker-aware verifier behavior.
- **Условие завершения:** Готово, когда state machine завершается ровно один раз при любом event ordering.

## Журнал выполнения

- 2026-09-02: переведена в `В работе`; реализованы validated immediate, duration и positive-N scrape modes, finite
  deadlines, matching frozen generation и atomic saturating threshold.
- 2026-09-02: переведена в `Тестирование`; defaults/ranges, immediate, zero/fixed duration, N=1/N>1, no-scraper
  timeout, wrong-generation, concurrent threshold и external-termination schedules прошли в Docker с race detector.
- 2026-09-02: переведена в `Готово`; natural CLI completion проходит нормативные lifecycle transitions, предыдущие
  container gates явно выбирают immediate mode, пройдены Docker vet/race/dependency/multi-architecture и EN/RU README
  checks.

## Свидетельства проверки

- `scrapes` остаётся production default с N=1 и finite timeout 60 секунд; значения вне каждого нормативного range
  отклоняются до workload start.
- Counts принимаются только пока state machine активна и только для её frozen generation, затем saturate на N.
- Immediate completion обходит `final_wait`; duration/scrape modes входят в него и завершаются exactly once без
  изменения resolved workload result.
