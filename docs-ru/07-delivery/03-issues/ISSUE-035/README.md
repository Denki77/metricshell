# ISSUE-035. Набор fault, soak и race tests

**Статус:** Готово
**Готовность:** Готово к разработке

**Эпик:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 6](../../02-epics/EPIC-001-core.md#wave-6)

Покрыть повторяющийся malformed input, slow clients, disconnects, saturation queue, bind failure, forced OOM, races signals и graceful drain.

## Контракт готовности к разработке

- **Нормативные входы:** ADR-001–ADR-015 и все accepted specifications,
  особенно [Runtime Defaults](../../../04-specification/runtime-defaults-and-resource-limits.md)
  и [Structured Logging](../../../04-specification/structured-logging.md).
- **Зависимости:** Implementation surfaces ISSUE-001–ISSUE-034.
- **Объём / вне объёма:** Автоматизировать fault injection, soak, fuzz и race coverage bounded runtime behavior. Вне
  scope: переопределение normative limits по benchmark results.
- **Конфигурация и наблюдаемые отказы:** Каждый injected failure проверяет exit/result origin, last-valid state, closed
  log/self-metric enums и bounded completion.
- **Критерии приёмки и обязательные тесты:** Malformed flood; slow clients; disconnects; saturation; bind/path failure;
  OOM container; signal/publication/scrape races; long reconciliation/drain.
- **Условие завершения:** Готово, когда production binary проходит suites документированной длительности с reproducible
  seeds и сохранёнными failure artifacts.

## Журнал поставки

- 2026-09-09: переведено в `In Progress`; добавлен production-binary fault fixture, который внутри Docker integration
  image покрывает HTTP malformed floods, slow client disconnects, queue saturation, scrape/publication races, bind/path
  startup failure и cgroup OOM.
- 2026-09-09: переведено в `Testing`; добавлен Makefile target `fault` как reproducible docker-only entrypoint suite и
  подключён к `ci`.
- 2026-09-09: переведено в `Done`; suite проверяет bounded recovery, overload logs, отсутствие workload start после
  startup bind failure и OOM exit behavior.

## Подтверждение проверки

- `go test ./internal/testfixture/faultsuite`
- `make fault IMAGE=metricshell-issue035-fault`
- `make test IMAGE=metricshell-issue035-fault`
