# ISSUE-007. State machine runtime lifecycle

**Статус:** Открыто
**Готовность:** Готово к разработке

**Эпик:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 2](../../02-epics/EPIC-001-core.md#wave-2)

**ADR/INV:** ADR-002 / INV-002.  
Реализовать точные публичные states из принятой
[Runtime State Machine](../../../04-specification/runtime-state-machine.md): `initializing`, `starting_workload`,
`running`, `stopping`, `finalizing`, `final_wait`, `failed`, `terminated`. Workload exit является event, а forced
termination — action, не дополнительными публичными states.

**Acceptance criteria:**

- переходы формально определены и тестируются table-driven tests;
- недопустимые переходы отклоняются;
- каждый state имеет readiness/health semantics;
- concurrent exit/signal/publication races проходят race detector.

## Контракт готовности к разработке

- **Нормативные входы:** ADR-002 / INV-002 и принятые Runtime State Machine, Self-Metrics и Structured Logging
  specifications.
- **Зависимости:** ISSUE-001; задача задаёт lifecycle semantics для ISSUE-008, ISSUE-010, ISSUE-025 и ISSUE-026.
- **Объём / вне объёма:** Реализовать только восемь public states, transitions, probes, logs и one-hot metric. Вне
  scope: дополнительные public states.
- **Конфигурация и наблюдаемые отказы:** Invalid transitions завершаются детерминированно, при terminal failure дают
  `runtime.failed` и не экспонируют две active state series.
- **Критерии приёмки и обязательные тесты:** Все valid/invalid transitions; concurrent exit/signal/publication; one-hot
  metric; health/readiness table; race detector.
- **Условие завершения:** Готово, когда одна transition table управляет runtime behavior, probes, logs и tests.
