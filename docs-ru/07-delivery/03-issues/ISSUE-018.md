# ISSUE-018. Framed protocol Unix socket

**Статус:** Готово
**Готовность:** Готово к разработке

**Эпик:** [EPIC-001 Core](../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 4](../02-epics/EPIC-001-core.md#wave-4)

**ADR/INV:** ADR-007 / INV-007.

**Acceptance criteria:** versioned frame header; length bound; full-read deadline; truncated/interleaved/oversized frame
rejection; separate connections may validate concurrently; activation remains linearized.

## Контракт готовности к разработке

- **Нормативные входы:** ADR-007 /
  INV-007, [Application Snapshot Protocol](../../04-specification/application-snapshot-protocol.md), [Configuration](../../04-specification/configuration.md)
  и [Runtime Defaults](../../04-specification/runtime-defaults-and-resource-limits.md).
- **Зависимости:** ISSUE-016.
- **Объём / вне объёма:** Реализовать MSP/1 BEGIN/PART/COMMIT, `BEGIN` и indexed frame ACK, unpadded base64url, bounded
  transactions и linearized commit. Вне scope: alternate framing/shared memory.
- **Конфигурация и наблюдаемые отказы:** Socket mode равен 0660; protocol/frame/transaction failures отделены от
  candidate reasons; каждый timeout/NACK наблюдаем.
- **Критерии приёмки и обязательные тесты:** Valid one/multipart; padded/invalid base64; duplicate/out-of-order/missing
  part; declared-size mismatch; точная capacity formula и передача default 1MiB; rejection capacity ниже snapshot limit;
  все limits; expiry/disconnect; concurrent commits.
- **Условие завершения:** Готово, когда protocol golden transcripts и cross-adapter corpus проходят с точными
  frames/enums.

## Журнал выполнения

- 2026-08-29: переведена в `В работе`; реализованы bounded MSP/1 line framing, multipart transaction storage,
  ответы FRAME_ACCEPTED/ACK/NACK и Unix listener с mode 0660.
- 2026-08-29: переведена в `Тестирование`; golden one/multipart transcripts, base64, ordering, duplication,
  missing-part, size/capacity, expiry, frame drain, default 1MiB и concurrent-commit tests прошли под Docker race tests.
- 2026-08-29: переведена в `Готово`; socket adapter прошёл `make test`, EN/RU issue и implementation README проверены
  синхронно.

## Свидетельства проверки

- Каждая line bounded, oversized input дренируется до следующего frame; декодируется только unpadded RFC 4648
  base64url.
- Transactions резервируют bounded shared slots, применяют canonical IDs/indexes/sizes, strict part order, exact
  decoded size и finite expiry; disconnect освобождает все reservations.
- ACK формируется только из accepted result общего Core после installation и содержит назначенную generation.
  Candidate reasons, admission outcomes и wire failures остаются в разных registries.
- Exact conservative capacity formula отклоняет configuration ниже canonical snapshot limit; default 8KiB × 256
  передаёт полный decoded candidate размером 1MiB.
- Real AF_UNIX listener test проверяет mode 0660 и end-to-end installation; concurrent commits получают уникальные
  linear generations.
