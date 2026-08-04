# ISSUE-010. Контракт health и readiness

**Статус:** Открыто
**Готовность:** Готово к разработке

**Эпик:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 2](../../02-epics/EPIC-001-core.md#wave-2)

**ADR/INV:** ADR-002, ADR-011.  
**Acceptance criteria:**

- health отражает способность supervisor продолжать bounded operation;
- readiness различает startup, active exposition и intentionally-unready final phase;
- probes никогда не считаются final scrapes;
- endpoint states документированы.

## Контракт готовности к разработке

- **Нормативные входы:** ADR-002 и
  ADR-011, [Configuration](../../../04-specification/configuration.md), [Runtime State Machine](../../../04-specification/runtime-state-machine.md)
  и [Structured Logging](../../../04-specification/structured-logging.md).
- **Зависимости:** ISSUE-007; блокирует probe handling в ISSUE-023 и final-scrape logic в ISSUE-026.
- **Объём / вне объёма:** Реализовать fixed `/healthz` и `/readyz` semantics для каждого public state. Вне scope:
  configurable probe paths и учёт probes как scrapes.
- **Конфигурация и наблюдаемые отказы:** Probe responses bounded и выводятся из state; unavailable/failed states
  возвращают deterministic statuses без изменения lifecycle.
- **Критерии приёмки и обязательные тесты:** Таблица state-by-endpoint status; transition races; requests во время
  shutdown; method/path errors; доказательство, что probes не увеличивают final-scrape count.
- **Условие завершения:** Готово, когда specification table и HTTP integration fixtures совпадают для каждого state.
