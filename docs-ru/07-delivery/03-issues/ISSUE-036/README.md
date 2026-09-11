# ISSUE-036. Controlled release benchmark suite

**Статус:** Готово
**Готовность:** Готово к разработке

**Эпик:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 6](../../02-epics/EPIC-001-core.md#wave-6)

**ADR/INV:** ADR-015 / INV-015.  
Не менее 30 repetitions, pinned CPU/resources, production binary/adapters; correctness оценивается отдельно от SLO thresholds.

## Контракт готовности к разработке

- **Нормативные входы:** ADR-015 /
  INV-015, [Runtime Defaults](../../../04-specification/runtime-defaults-and-resource-limits.md)
  и [Docker and Compose Examples](../../../04-specification/docker-compose-examples.md).
- **Зависимости:** ISSUE-032 и ISSUE-035.
- **Объём / вне объёма:** Benchmark production binary/adapters в controlled pinned environments минимум в 30
  repetitions, разделяя correctness и release thresholds. Вне scope: пересборка или использование research prototypes
  как release artifacts.
- **Конфигурация и наблюдаемые отказы:** Environment drift, недостаток repetitions, correctness failure или unstable
  variance инвалидируют run с machine-readable metadata.
- **Критерии приёмки и обязательные тесты:** File/socket/HTTP; idle/load; architecture matrix; pinned CPU/memory;
  warmup; 30+ samples; raw results, summary statistics, correctness gate.
- **Условие завершения:** Готово, когда independent runner воспроизводит suite и сравнивает distributions без пересборки
  прототипов.

## Журнал поставки

- 2026-09-09: переведено в `In Progress`; добавлен controlled release benchmark contract для file, Unix socket и HTTP
  adapters по idle/load profiles, 30 repetitions, warmups, architecture matrix и pinned resource metadata.
- 2026-09-09: переведено в `Testing`; добавлен Docker stage `benchmark-artifacts` и Makefile target `benchmark`, который
  строит production binary перед запуском benchmark suite и экспортирует raw results вместе с machine-readable metadata.
- 2026-09-09: переведено в `Done`; correctness assertions остаются внутри benchmark driver и отделены от любой
  интерпретации SLO thresholds.

## Подтверждение проверки

- `go test ./internal/benchrelease`
- `make benchmark IMAGE=metricshell-issue036-benchmark`
- `make test IMAGE=metricshell-issue036-benchmark`
