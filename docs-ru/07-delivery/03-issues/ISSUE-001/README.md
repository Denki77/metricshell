# ISSUE-001. Инициализация production-модуля Go и команд

**Статус:** Открыто
**Готовность:** Готово к разработке

**Эпик:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 1](../../02-epics/EPIC-001-core.md#wave-1)

**ADR/INV:** основа проекта; prerequisite для ADR-001.
**Результат:** production package structure, `cmd/metricshell`, config bootstrap, version command, build/test targets.

**Acceptance criteria:**

- `metricshell --version` возвращает build identity;
- unit tests запускаются одной командой;
- binary собирается для Linux;
- research prototype code не импортируется production packages напрямую.

## Контракт готовности к разработке

- **Нормативные входы:
  ** [Configuration](../../../04-specification/configuration.md), [грамматика значений configuration](../../../04-specification/configuration-value-grammar.md)
  и prerequisites EPIC/ADR.
- **Зависимости:** Нет; задача создаёт production module и блокирует все implementation issues.
- **Объём / вне объёма:** Production module, command, version metadata и build/test targets. Вне scope: runtime behavior
  и повторное использование packages исследовательских прототипов.
- **Конфигурация и наблюдаемые отказы:** Некорректные build metadata завершают build ошибкой; невалидная startup
  configuration использует exit registry configuration и structured diagnostics.
- **Критерии приёмки и обязательные тесты:** `--version`, Linux amd64/arm64 build, unit target, dependency-boundary
  check и тест отсутствия production imports из `research/`.
- **Условие завершения:** Готово, когда clean CI воспроизводимо собирает и тестирует production command без пересборки
  или импортирования прототипов.
