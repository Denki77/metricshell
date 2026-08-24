# ISSUE-007. State machine runtime lifecycle

**Статус:** Готово
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

## Журнал выполнения

- 2026-08-24: задача переведена в `В работе`; принятая таблица states/events отображена в отдельный lifecycle package.
- 2026-08-24: задача переведена в `Тестирование`; valid/invalid transitions, one-hot, probe table, concurrent
  exit/termination и Docker supervisor tests прошли под race detector.
- 2026-08-24: задача переведена в `Готово`; runtime behavior выпускает каждый effective public transition из общей
  state machine.

## Свидетельства проверки

- Закрытые registry из восьми states и двенадцати events представлены typed constants и одной нормативной transition
  table; каждая пара `(state, event)` имеет ровно один target.
- Invalid transitions не меняют state и возвращают deterministic errors; state-dependent concurrent events разрешаются
  под lock машины.
- Spawn timing и final-wait policy используют contextual events, поэтому runtime только передаёт events и не выбирает
  target вне machine. Tests выполняют каждую нормативную строку через `TransitionEvent()` и отклоняют ambiguous tables.
- Данные `metricshell_runtime_state` представлены полным one-hot vector ровно с одним active state.
- Runtime logs содержат `runtime.initializing` и ровно одну запись `runtime.state_changed` после каждого effective
  transition, включая initial state без `previous_state`.
- `make ci IMAGE=metricshell-wave2` прошёл полностью через Docker, включая `go test -race` и все fixtures Wave 1.
- `implementation/README.md` и `implementation/README_RU.md` проверены и обновлены вместе.
