# ISSUE-033. Defaults container hardening

**Статус:** Открыто
**Готовность:** Готово к разработке

**Эпик:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 6](../../02-epics/EPIC-001-core.md#wave-6)

**ADR/INV:** ADR-014 / INV-014.  
Обеспечить non-root, read-only rootfs, dropped capabilities, no-new-privileges, private runtime paths и loopback/local socket.

## Контракт готовности к разработке

- **Нормативные входы:** ADR-007, ADR-008 и
  ADR-013, [Configuration](../../../04-specification/configuration.md), [Runtime Defaults](../../../04-specification/runtime-defaults-and-resource-limits.md)
  и [Docker and Compose Examples](../../../04-specification/docker-compose-examples.md).
- **Зависимости:** ISSUE-018, ISSUE-020 и ISSUE-032.
- **Объём / вне объёма:** Поставлять non-root, read-only-rootfs, dropped-capability, no-new-privileges examples с
  private runtime paths и socket group ownership. Вне scope: privileged fallback.
- **Конфигурация и наблюдаемые отказы:** Permission/bind/path failures используют normative startup errors; socket mode
  ровно 0660, HTTP ingestion остаётся loopback-only.
- **Критерии приёмки и обязательные тесты:** Arbitrary non-root UID; shared producer GID; read-only rootfs; no
  capabilities; unwritable/missing runtime dir; socket mode/ownership assertions.
- **Условие завершения:** Готово, когда hardened examples работают без privilege, а второй UID в configured group может
  публиковать.
