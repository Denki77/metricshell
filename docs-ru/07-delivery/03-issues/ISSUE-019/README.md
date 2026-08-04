# ISSUE-019. Сериализация writer официального client

**Статус:** Открыто
**Готовность:** Готово к разработке

**Эпик:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 4](../../02-epics/EPIC-001-core.md#wave-4)

**ADR/INV:** ADR-007, ADR-004.  
**Acceptance criteria:** один connection writer или mutex гарантирует contiguous frame; два application threads не могут
перемешать bytes.

## Контракт готовности к разработке

- **Нормативные входы:** ADR-007 и
  ADR-004, [Application Snapshot Protocol](../../../04-specification/application-snapshot-protocol.md).
- **Зависимости:** ISSUE-018.
- **Объём / вне объёма:** Создать official connection writer, сериализующий полные MSP/1 frames и correlating responses.
  Вне scope: retry policy после ambiguous disconnect.
- **Конфигурация и наблюдаемые отказы:** Short writes, closed connections и mismatched publication IDs дают typed client
  errors без interleaving bytes или раскрытия payload.
- **Критерии приёмки и обязательные тесты:** Concurrent goroutines на одном connection; forced short writes; server
  NACK/timeout; disconnect до ACK; response mismatch; race detector.
- **Условие завершения:** Готово, когда byte-level stress test доказывает contiguous/attributable для каждого emitted
  frame.
