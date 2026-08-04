# ISSUE-021. Закрепление mmap как non-primary

**Статус:** Открыто
**Готовность:** Готово к разработке

**Эпик:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 4](../../02-epics/EPIC-001-core.md#wave-4)

**ADR/INV:** ADR-009 / INV-009.  
**Результат:** core не зависит от shared-memory ABI; mmap не нужен для первого production release.

## Контракт готовности к разработке

- **Нормативные входы:** ADR-009 / INV-009 и [Configuration](../../../04-specification/configuration.md).
- **Зависимости:** ISSUE-001.
- **Объём / вне объёма:** Зафиксировать отсутствие mmap option, shared-memory ABI и production dependency в Core. Вне
  scope: будущие experimental research.
- **Конфигурация и наблюдаемые отказы:** Любой undocumented mmap/shared-memory option отклоняется как unknown;
  dependency checks ломают CI при production import prototype mmap code.
- **Критерии приёмки и обязательные тесты:** CLI/environment negative tests; public API scan; dependency/license scan;
  clean build при недоступном research tree.
- **Условие завершения:** Готово, когда release artifacts и public packages не содержат shared-memory contract.
