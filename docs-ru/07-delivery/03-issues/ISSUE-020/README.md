# ISSUE-020. Local push HTTP adapter

**Статус:** Готово
**Готовность:** Готово к разработке

**Эпик:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 4](../../02-epics/EPIC-001-core.md#wave-4)

**ADR/INV:** ADR-008 / INV-008.

**Acceptance criteria:** local-only endpoint; same whole-candidate contract and error taxonomy; bounded
body/time/concurrency; adapter не получает особую state semantics.

## Контракт готовности к разработке

- **Нормативные входы:** ADR-008 /
  INV-008, [Application Snapshot Protocol](../../../04-specification/application-snapshot-protocol.md), [Configuration](../../../04-specification/configuration.md)
  и [Runtime Defaults](../../../04-specification/runtime-defaults-and-resource-limits.md).
- **Зависимости:** ISSUE-016.
- **Объём / вне объёма:** Реализовать loopback-only POST `/v1/metrics`, identity/gzip decoding, shared validation и
  точный HTTP mapping. Вне scope: remote authentication/non-loopback bind.
- **Конфигурация и наблюдаемые отказы:** Wire, decoded и canonical limits применяются независимо; rejection bodies
  используют closed codes; ACK не предшествует installation.
- **Критерии приёмки и обязательные тесты:** Bind validation; methods/media types/encodings; gzip bomb; каждая
  status/code row; slow read/write; busy/timeout; concurrent ordering.
- **Условие завершения:** Готово, когда HTTP проходит shared corpus и совпадает с file/socket по state, generation,
  reason и observability.

## Журнал выполнения

- 2026-08-31: переведена в `В работе`; реализованы loopback-only versioned HTTP handler, bounded identity/gzip decoding,
  exact response mapping и finite server timeouts.
- 2026-08-31: переведена в `Тестирование`; bind, method, path, media type, encoding, wire/decoded limits, gzip bomb,
  каждый candidate status row, busy, timeout, cancellation и concurrent-order tests прошли под Docker race detector.
- 2026-08-31: переведена в `Готово`; adapter прошёл `make test`, EN/RU issue и implementation README проверены
  синхронно.

## Свидетельства проверки

- Только `POST /v1/metrics` принимает `application/json` или version-1 media type MetricShell с identity/gzip encoding;
  остальные method/media/encoding classes имеют closed bounded response codes.
- Wire bytes ограничиваются до decompression, decoded bytes — во время decompression, canonical bytes — общим parser.
  Compressed amplification candidate не приводит к unbounded allocation.
- Каждый candidate reason использует нормативную HTTP status table; busy и timeout остаются publication outcomes со
  статусами 429 и 408. ACK записывается только из post-install accepted result общего Core.
- Listener configuration отклоняет empty, wildcard и non-loopback hosts и сохраняет finite header/read/write/idle
  timeouts и header-byte bound.
- Concurrent requests получают уникальные generations из того же atomic holder, что file и Unix ingestion.
