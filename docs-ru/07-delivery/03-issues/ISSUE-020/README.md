# ISSUE-020. Local push HTTP adapter

**Статус:** Открыто
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
