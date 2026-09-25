# ISSUE-001. Инициализация production-модуля Go и команд

**Статус:** Готово
**Готовность:** Готово к разработке

**Эпик:** [EPIC-001 Core](../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 1](../02-epics/EPIC-001-core.md#wave-1)

**ADR/INV:** основа проекта; prerequisite для ADR-001.
**Результат:** production package structure, `cmd/metricshell`, config bootstrap, version command, build/test targets.

**Acceptance criteria:**

- `metricshell --version` возвращает build identity;
- unit tests запускаются одной командой;
- binary собирается для Linux;
- research prototype code не импортируется production packages напрямую.

## Контракт готовности к разработке

- **Нормативные входы:** [Configuration](../../04-specification/configuration.md),
  [грамматика значений configuration](../../04-specification/configuration-value-grammar.md)
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

## Журнал выполнения

- 2026-08-11: задача переведена в `В работе`; реализация начата в отдельном worktree `ISSUE-001`.
- 2026-08-11: задача переведена в `Тестирование`; начата проверка в Docker.
- 2026-08-11: задача переведена в `Готово`; Docker CI, static binary, runtime image, dependency-boundary, build-metadata
  и README проверки пройдены.
- 2026-08-11: задача повторно переведена в `В работе`, чтобы ввести общий источник версии репозитория и исправить
  границу ответственности Make/Docker.
- 2026-08-11: задача переведена в `Тестирование`; проверяются Docker-only targets Make и общий источник версии.
- 2026-08-11: задача переведена в `Готово`; исправленная граница ответственности, общий `VERSION`, Docker CI и
  runtime-проверки нативной архитектуры пройдены.
- 2026-08-11: задача повторно переведена в `В работе` для удаления избыточного валидатора версии и его тестов.
- 2026-08-11: задача переведена в `Тестирование`; упрощённые проверки файла версии и наблюдаемый version output
  проверяются в Docker.
- 2026-08-11: задача переведена в `Готово`; Docker CI и runtime-проверка пройдены без отдельного валидатора версии и
  его внутренних тестов.
- 2026-08-12: задача повторно переведена в `В работе` для обновления production toolchain с Go 1.23 до Go 1.26.
- 2026-08-12: задача переведена в `Тестирование`; pinned Docker toolchain Go 1.26 проходит CI и runtime-проверку.
- 2026-08-12: задача переведена в `Готово`; Docker CI и arm64 runtime-проверка пройдены с Go 1.26.5.

## Свидетельства проверки

- `make ci` вызвал Docker target `test`; Dockerfile напрямую выполнил Go-проверки внутри pinned build layer Go 1.26
  Alpine.
- Makefile содержит только Docker-команды; Dockerfile не зависит от Make или вспомогательного CI script.
- Scratch runtime image вернул ожидаемый build identity по `--version`.
- Бинарники amd64 и arm64 проверены как stripped, statically linked Linux ELF executables.
- Английские и русские project/implementation README прошли проверку полноты.
- Корневой `VERSION` передал `0.1.0-dev`; timestamp сборки не встраивается.
- Финальный arm64 image сообщил `linux/arm64` и вернул ожидаемые version/revision при запуске.
- Version handling теперь проверяет только наблюдаемый вывод бинарника относительно одной непустой строки `VERSION`.
- Production module объявляет Go 1.26; pinned multi-platform Docker image разрешился в Go 1.26.5.
