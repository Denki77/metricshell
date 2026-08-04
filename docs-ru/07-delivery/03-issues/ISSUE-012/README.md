# ISSUE-012. Parser и validator полного candidate

**Статус:** Открыто
**Готовность:** Готово к разработке

**Эпик:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 3](../../02-epics/EPIC-001-core.md#wave-3)

**ADR/INV:** ADR-004, ADR-010, ADR-014 / INV-004, INV-010, INV-014.

**Acceptance criteria:** syntax, metadata conflicts, duplicate series, histogram consistency, payload/cardinality/label
policies проверяются до activation.

Parsing и rejection codes соответствуют принятой спецификации
[Application Snapshot Protocol](../../../04-specification/application-snapshot-protocol.md).

## Контракт готовности к разработке

- **Нормативные входы:** ADR-004 и ADR-014 /
  INV-004, [Application Snapshot Protocol](../../../04-specification/application-snapshot-protocol.md)
  и [Runtime Defaults](../../../04-specification/runtime-defaults-and-resource-limits.md).
- **Зависимости:** ISSUE-011; блокирует ISSUE-013 и все adapters.
- **Объём / вне объёма:** Parse, validate, canonicalize и вернуть только общий candidate result. Вне scope:
  transport-specific parsing и изменение active state.
- **Конфигурация и наблюдаемые отказы:** Использовать closed rejection mapping, включая `schema_version` и `policy`;
  decoded/canonical limits независимы, last-valid state не меняется.
- **Критерии приёмки и обязательные тесты:** Общий corpus для каждого reason; binary64 range, canonical rendering,
  rounding collisions и special floats; histogram sign rules; base/component-name и `le` collisions; normalization
  empty families; decoded amplification; точные limits; malformed/fuzz input; race-safe concurrent validation.
- **Условие завершения:** Готово, когда один validator и corpus без изменений используются file, socket и HTTP adapters.
