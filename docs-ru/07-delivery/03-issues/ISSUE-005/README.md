# ISSUE-005. Reaping дочерних процессов и обработка orphan

**Статус:** Открыто
**Готовность:** Готово к разработке

**Эпик:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 1](../../02-epics/EPIC-001-core.md#wave-1)

**ADR/INV:** ADR-001 / INV-001.  
**Результат:** MetricShell reap-ит завершившихся потомков и не оставляет zombies.

**Acceptance criteria:**

- orphaned descendants корректно reap-ятся PID 1;
- workload primary PID отслеживается отдельно;
- zombie count после stress test равен нулю;
- неожиданные child exits отражаются в diagnostics.

## Контракт готовности к разработке

- **Нормативные входы:** ADR-001 /
  INV-001, [Runtime State Machine](../../../04-specification/runtime-state-machine.md), [Self-Metrics](../../../04-specification/self-metrics.md)
  и [Structured Logging](../../../04-specification/structured-logging.md).
- **Зависимости:** ISSUE-002 и ISSUE-003.
- **Объём / вне объёма:** Reap direct/adopted children с отдельным tracking primary workload. Вне scope: supervision
  сторонних services.
- **Конфигурация и наблюдаемые отказы:** Unexpected child outcomes дают sanitized diagnostics; primary outcome остаётся
  authoritative; child PIDs исключены из metric labels.
- **Критерии приёмки и обязательные тесты:** Orphan adoption, double-fork, burst exits, порядок
  primary-before-child/child-before-primary, zero-zombie stress, race detector.
- **Условие завершения:** Готово, когда stress fixtures не оставляют zombies и дают ровно один primary workload result.
