# ISSUE-031. Integration test нескольких реплик Prometheus

**Статус:** Открыто
**Готовность:** Готово к разработке

**Эпик:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 6](../../02-epics/EPIC-001-core.md#wave-6)

Каждая configured replica Prometheus проверяется независимо; aggregate HTTP count недостаточен.

## Контракт готовности к разработке

- **Нормативные входы:** ADR-012 /
  INV-012, [Docker and Compose Examples](../../../04-specification/docker-compose-examples.md)
  и [Self-Metrics](../../../04-specification/self-metrics.md).
- **Зависимости:** ISSUE-029 и ISSUE-030.
- **Объём / вне объёма:** Проверять каждую configured replica отдельно через Prometheus labels и historical queries. Вне
  scope: aggregate-only success.
- **Конфигурация и наблюдаемые отказы:** Missing, duplicate, stale-only или cross-replica samples дают replica-specific
  diagnostics с bounded query time.
- **Критерии приёмки и обязательные тесты:** Две и более replicas; одна missing publication; duplicate label; target
  disappearance/stale marker; delayed scrape; aggregate count false positive.
- **Условие завершения:** Готово, когда test падает при отсутствии expected final sample у любой named replica.
