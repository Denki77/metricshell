# Ограничения

[English version](../../docs/03-requirements/constraints.md)

## Значения по умолчанию

Нормативные значения определены
в [спецификации defaults и resource limits](../04-specification/runtime-defaults-and-resource-limits.md).

- Default final-wait mode после natural completion: `scrapes`.
- Default duration при явно выбранном `duration`: `30s`.
- Default timeout ожидания scrape: `60s`.
- Default required scrape count: `1`.
- Default external shutdown total grace: `30s`.
- Default workload shutdown timeout: `28s`.
- Default shutdown reserve MetricShell: `2s`.

## Ограничения

- Maximum configurable scrape wait: `1h`.
- Runtime никогда не ожидает бесконечно.
- Ошибка application не превращается в success из-за штатного завершения MetricShell.
- Final application snapshot immutable.
- Каждый candidate и response ограничен явными resource limits.
