# ISSUE-018. Framed protocol Unix socket

**Статус:** Открыто
**Готовность:** Готово к разработке

**Эпик:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 4](../../02-epics/EPIC-001-core.md#wave-4)

**ADR/INV:** ADR-007 / INV-007.

**Acceptance criteria:** versioned frame header; length bound; full-read deadline; truncated/interleaved/oversized frame
rejection; separate connections may validate concurrently; activation remains linearized.

## Контракт готовности к разработке

- **Нормативные входы:** ADR-007 /
  INV-007, [Application Snapshot Protocol](../../../04-specification/application-snapshot-protocol.md), [Configuration](../../../04-specification/configuration.md)
  и [Runtime Defaults](../../../04-specification/runtime-defaults-and-resource-limits.md).
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
