# ISSUE-004. Передача сигналов

**Статус:** Открыто
**Готовность:** Готово к разработке

**Эпик:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 1](../../02-epics/EPIC-001-core.md#wave-1)

**ADR/INV:** ADR-001, ADR-003 / INV-001, INV-003.  
**Результат:** SIGTERM, SIGINT, SIGHUP и выбранные operational signals корректно пересылаются workload group.

**Acceptance criteria:**

- сигнал, полученный PID 1, наблюдается workload;
- повторный signal имеет определённую policy;
- signal во время startup и post-exit не вызывает race/panic;
- e2e tests проверяют TERM и INT.

## Контракт готовности к разработке

- **Нормативные входы:** ADR-001, ADR-003 / INV-001,
  INV-003, [Runtime State Machine](../../../04-specification/runtime-state-machine.md)
  и [Structured Logging](../../../04-specification/structured-logging.md).
- **Зависимости:** ISSUE-002 и ISSUE-003.
- **Объём / вне объёма:** Пересылать TERM, INT, HUP и документированные operational signals с deterministic
  repeated-signal behavior. Вне scope: workload-specific signal policy.
- **Конфигурация и наблюдаемые отказы:** Forwarding и ignored late signals наблюдаемы; unsupported signal или
  исчезнувшая target group не вызывают panic и используют bounded error path.
- **Критерии приёмки и обязательные тесты:** TERM/INT/HUP; repeated signals; signal до exec, во время exit и после reap;
  исчезновение process group; race detector.
- **Условие завершения:** Готово, когда каждый supported signal имеет deterministic state-dependent outcome и
  integration coverage.
