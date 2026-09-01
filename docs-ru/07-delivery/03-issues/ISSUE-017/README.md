# ISSUE-017. Protocol file publication

**Статус:** Готово
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

## Журнал выполнения

- 2026-08-29: переведена в `В работе`; реализованы bounded target reads с `O_NOFOLLOW`, raw-content deduplication,
  directory inotify и обязательный periodic reconciliation.
- 2026-08-29: переведена в `Тестирование`; startup present/absent, rename, partial write, symlink, non-regular,
  oversized, unchanged и periodic-recovery tests прошли под Docker race gate.
- 2026-08-29: переведена в `Готово`; file ingestion прошёл `make test`, EN/RU issue и implementation README проверены
  синхронно.

## Свидетельства проверки

- Открывается только configured target; `O_NOFOLLOW` вместе с `fstat` отклоняет symlinks и non-regular files до чтения.
- `LimitReader` применяет decoded-byte bound до parsing, включая whitespace amplification.
- Inotify наблюдает containing directory для atomic replacements; overflow, invalidation и reinstallation имеют явные
  recovery paths, а finite periodic reconciliation остаётся authoritative после silent event loss.
- Absent, malformed и I/O states сохраняют last-valid holder generation; identical accepted file content не
  публикуется повторно при periodic reconciliation.
