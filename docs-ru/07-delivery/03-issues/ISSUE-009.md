# ISSUE-009. Эскалация termination

**Статус:** Готово
**Готовность:** Готово к разработке

**Эпик:** [EPIC-001 Core](../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 2](../02-epics/EPIC-001-core.md#wave-2)

**ADR/INV:** ADR-003 / INV-003.  
**Результат:** graceful signal -> bounded wait -> forced kill process group.

**Acceptance criteria:**

- cooperative workload завершается graceully;
- ignoring workload принудительно завершается в budget;
- descendants не переживают MetricShell;
- escalation не стирает сохранённый workload result policy.

## Контракт готовности к разработке

- **Нормативные входы:** ADR-003 /
  INV-003, [Runtime State Machine](../../04-specification/runtime-state-machine.md), [Runtime Defaults](../../04-specification/runtime-defaults-and-resource-limits.md)
  и [Structured Logging](../../04-specification/structured-logging.md).
- **Зависимости:** ISSUE-003, ISSUE-004 и ISSUE-008.
- **Объём / вне объёма:** Послать graceful signal, ждать внутри derived budget, затем force-kill owned process group.
  Вне scope: unbounded retries.
- **Конфигурация и наблюдаемые отказы:** Escalation создаёт нормативные forwarding/forced events; missing processes
  обрабатываются idempotently; workload result следует ISSUE-006.
- **Критерии приёмки и обязательные тесты:** Cooperative, ignoring и fork-after-signal workloads; zero remaining budget;
  repeated signal; disappearing group; отсутствие surviving descendants.
- **Условие завершения:** Готово, когда каждый termination path завершается внутри budget без оставшихся descendants.

## Журнал выполнения

- 2026-08-24: задача переведена в `В работе`; реализованы budget-aware escalation controller и shutdown diagnostics.
- 2026-08-24: задача переведена в `Тестирование`; в Docker прошли race-enabled unit tests и real-container сценарии
  cooperative, ignoring, fork-after-signal, repeated signal, zero budget и disappearing group.
- 2026-08-24: задача переведена в `Готово`; termination завершается одним authoritative result после reaping всех
  managed children, а forced cleanup выполняется idempotently и в границах принятого shutdown plan.

## Свидетельства проверки

- Первый TERM/INT фиксирует один absolute shutdown deadline, пересылает graceful signal и запускает только workload
  grace timer; повторный termination signal может немедленно форсировать owned process group.
- По истечении grace SIGKILL посылается всей process group и даёт exit `137`; уже исчезнувшая group обрабатывается
  idempotently без ложного forced-termination event.
- Container fixtures доказывают cooperative exit, forced exit для ignoring workload и reaping descendant, созданного
  после graceful signal.
- `shutdown.started`, `shutdown.forced`, `shutdown.completed` и поле `forced` результата workload соответствуют
  registry structured events.
- `implementation/README.md` и `implementation/README_RU.md` проверены на полноту и обновлены синхронно.
