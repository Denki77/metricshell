# ISSUE-002. Entrypoint PID 1 и разбор команды workload

**Статус:** Открыто
**Готовность:** Готово к разработке

**Эпик:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 1](../../02-epics/EPIC-001-core.md#wave-1)

**ADR/INV:** ADR-001 / INV-001.  
**Результат:** MetricShell работает как PID 1 и запускает переданную workload-команду без shell-интерпретации по
умолчанию.

**Acceptance criteria:**

- workload argv сохраняется без потери аргументов;
- ошибка запуска имеет отдельный internal exit code;
- MetricShell различает собственную startup failure и workload exit;
- integration test запускает MetricShell как PID 1 контейнера.

## Контракт готовности к разработке

- **Нормативные входы:** ADR-001 /
  INV-001, [Configuration](../../../04-specification/configuration.md), [Runtime State Machine](../../../04-specification/runtime-state-machine.md)
  и [Structured Logging](../../../04-specification/structured-logging.md).
- **Зависимости:** ISSUE-001.
- **Объём / вне объёма:** Entrypoint PID 1 и byte-preserving workload argv после `--`. Вне scope: shell parsing, если
  shell не передан как явный workload.
- **Конфигурация и наблюдаемые отказы:** Пустой argv даёт `configuration_invalid`; exec failure —
  `workload_start_failed`; workload exit не выдаётся как supervisor startup failure.
- **Критерии приёмки и обязательные тесты:** Container test с PID 1; spaces, empty arguments, Unicode и option-like
  argv; missing executable; exit 0/non-zero/signal; signal race при startup.
- **Условие завершения:** Готово, когда argv/outcome fixtures проходят в реальном container, а startup failures
  используют нормативные log и exit registries.
