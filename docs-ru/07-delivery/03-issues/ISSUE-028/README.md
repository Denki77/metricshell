# ISSUE-028. Наблюдаемость final wait

**Статус:** Открыто
**Готовность:** Готово к разработке

**Эпик:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 5](../../02-epics/EPIC-001-core.md#wave-5)

## Нормативные входы

- ADR-002, ADR-003, ADR-011, ADR-014.
- [Runtime state machine](../../../04-specification/runtime-state-machine.md).
- [Спецификация self-metrics](../../../04-specification/self-metrics.md).
- [Спецификация structured logging](../../../04-specification/structured-logging.md).

## Зависимости

ISSUE-007 — lifecycle, ISSUE-015 — self-metrics, ISSUE-026 — final-wait state machine, ISSUE-027 — подсчёт завершённых
responses.

## Scope

Реализовать полный final-wait registry self-metrics и structured events для start, counted/not-counted responses,
completion, state transitions, timeout, external termination и runtime failure. Экспонировать frozen snapshot generation,
mode, deadline, required/completed scrapes, bounded attempts/outcomes и terminal reason.

## Вне scope

Scraper identity, high-cardinality request/client labels, durable audit storage, подтверждение parsing со стороны
Prometheus и логирование application payload.

## Конфигурация и наблюдаемые ошибки

Использовать final_wait.mode/duration/timeout/required_scrapes/completion_grace и absolute shutdown deadline. Metrics и
logs используют закрытые registries mode/outcome/reason/error. Невалидная configuration завершается до запуска workload;
runtime failure выпускает runtime.failed и сохраняет bounded cleanup.

## Критерии приёмки

- Каждый public state transition только один раз выпускает runtime.state_changed и обновляет one-hot state metrics.
- Events start и completion выпускаются только один раз с mode/deadline и terminal reason.
- Пути counted, ineligible, cancelled, write-error, timeout и external-termination обновляют соответствующие metrics и
  log fields.
- Request/publication IDs появляются только там, где разрешено, и никогда не становятся metric labels.
- Rate limiting выпускает logging.suppression_summary и не подавляет terminal lifecycle events.
- Frozen generation identity остаётся стабильной на всём протяжении final_wait.

## Обязательная test matrix

Modes immediate/duration/scrapes; N=1 и N>1; timeout; concurrent responses на threshold; cancellation и partial write;
исключение probe/debug; races external signal; runtime failure; валидация log schema/type/enum; suppression windows;
cardinality assertions; race detector.

## Условие завершения

Задача завершена, когда каждый final-wait transition имеет соответствующие metrics и schema-valid events, проходят tests
паритета enums с self-metrics, terminal events выпускаются только один раз и отсутствуют unbounded или sensitive fields.
