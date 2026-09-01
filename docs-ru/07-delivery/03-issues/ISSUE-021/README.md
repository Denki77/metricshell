# ISSUE-021. Закрепление mmap как non-primary

**Статус:** Готово
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

## Журнал выполнения

- 2026-08-31: переведена в `В работе`; добавлены explicit rejection mmap/shared-memory CLI/environment surfaces и
  усилен production dependency boundary.
- 2026-08-31: переведена в `Тестирование`; negative configuration, public API/primitive, resolved dependency,
  module-license и research-unavailable Docker-context checks прошли в Docker race/build gate.
- 2026-08-31: переведена в `Готово`; production artifact собран для amd64/arm64 без research/shared-memory ABI, EN/RU
  issue и implementation README проверены синхронно.

## Свидетельства проверки

- mmap/shared-memory CLI spellings и reserved environment spellings отклоняются до workload start; три документированных
  stable transports остаются единственным архитектурным ingestion set.
- Production AST scan ломается на exported shared-memory API и direct mmap/shm primitives, resolved package/module
  scans отклоняют prototype dependencies.
- Каждый будущий external module должен иметь discoverable license file для прохождения того же boundary test.
- Docker tests утверждают отсутствие research tree в production build context, доказывая отсутствие accidental compile
  access к prototypes.
