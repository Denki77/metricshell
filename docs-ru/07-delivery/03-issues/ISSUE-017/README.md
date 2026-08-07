# ISSUE-017. Protocol file publication

**Статус:** Открыто
**Готовность:** Готово к разработке

**Эпик:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 4](../../02-epics/EPIC-001-core.md#wave-4)

**ADR/INV:** ADR-006, ADR-015 / INV-006, INV-015.

**Acceptance criteria:** temp-write + fsync/close + atomic rename contract; partial file не активируется; inotify +
reconciliation fallback; delete/replace races покрыты.

## Контракт готовности к разработке

- **Нормативные входы:** ADR-006 и ADR-015 / INV-006,
  INV-015, [Application Snapshot Protocol](../../../04-specification/application-snapshot-protocol.md)
  и [Runtime Defaults](../../../04-specification/runtime-defaults-and-resource-limits.md).
- **Зависимости:** ISSUE-016.
- **Объём / вне объёма:** Читать только configured atomic-rename target, сочетать inotify с mandatory reconciliation и
  применять raw decoded input limits. Вне scope: partial/in-place writes.
- **Конфигурация и наблюдаемые отказы:** Absent, invalid, oversized и I/O states сохраняют last-valid snapshot и дают
  bounded file outcomes без logging paths/payloads.
- **Критерии приёмки и обязательные тесты:** Startup present/absent; atomic rename; in-place partial write;
  symlink/non-regular; overflow/invalidation/reinstall; whitespace-amplified file; periodic recovery.
- **Условие завершения:** Готово, когда потеря events и malformed files не приводят к partial activation или unbounded
  reads.
