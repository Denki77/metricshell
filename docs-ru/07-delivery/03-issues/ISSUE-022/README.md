# ISSUE-022. Cross-adapter conformance suite

**Статус:** Готово
**Готовность:** Готово к разработке

**Эпик:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 4](../../02-epics/EPIC-001-core.md#wave-4)

Один набор complete snapshots и failure cases выполняется для file, socket и push; accepted state identity должна
совпадать.

## Контракт готовности к разработке

- **Нормативные входы:** ADR-004–ADR-008 и
  ADR-015, [Application Snapshot Protocol](../../../04-specification/application-snapshot-protocol.md), [Runtime Defaults](../../../04-specification/runtime-defaults-and-resource-limits.md)
  и [Self-Metrics](../../../04-specification/self-metrics.md).
- **Зависимости:** ISSUE-017, ISSUE-018 и ISSUE-020.
- **Объём / вне объёма:** Прогонять один immutable acceptance/rejection corpus через все три adapters. Вне scope:
  adapter-specific exceptions к semantic validation.
- **Конфигурация и наблюдаемые отказы:** Проверять exact canonical bytes, generation, active state, rejection reason,
  log fields и self-metric deltas; transport-only failures проверяются отдельно.
- **Критерии приёмки и обязательные тесты:** Каждый candidate reason; finite/special numbers; empty state; все limits;
  concurrency; timeout; disconnect; malformed transport и recovery.
- **Условие завершения:** Готово, когда добавление reason/enum требует одного shared fixture, а parity test обновляет
  все adapters.

## Журнал выполнения

- 2026-09-01: переведена в `В работе`; реализован единый semantic corpus поверх real file reconciliation, MSP/1 commit
  и HTTP request paths, а также common ingestion diagnostics.
- 2026-09-01: переведена в `Тестирование`; каждый candidate/state rejection reason, finite/special values, zero state,
  resource limits, replacement, admission, transport failure и recovery cases прошли под Docker race tests.
- 2026-09-01: переведена в `Готово`; полный Docker gate `make wave4` прошёл, все EN/RU implementation и issue README
  проверены синхронно.

## Свидетельства проверки

- Immutable corpus exhaustive относительно `snapshot.RejectionReasons`; добавление reason ломает coverage до обновления
  единственного shared fixture.
- File, Unix и HTTP дают одинаковые outcome/reason, canonical bytes, active generation и last-valid retention для одного
  complete candidate. Accepted counter/gauge/histogram values включают finite и все поддерживаемые special gauges.
- Каждый adapter обновляет одинаковые bounded publication/rejection metric labels и пишет нормативный accepted/rejected
  structured event с transport, generation/bytes/series или reason fields.
- Complete replacement удаляет omitted state. Malformed file content, wrong MSP version и malformed HTTP gzip не
  изменяют state; каждый adapter принимает следующую valid publication.
- Busy и timeout admission outcomes используют общие counters; busy пишет bounded event `ingestion.overloaded`.
  Socket disconnect/expiry и client timeout остаются в transport-specific suites.
- `make wave4` — explicit alias полного Docker CI gate, поэтому CI выполняет conformance suite, race detector, vet,
  dependency boundary, multi-architecture builds и real-container lifecycle acceptance tests.
