# ISSUE-030. Lifecycle controls Kubernetes

**Статус:** Открыто
**Готовность:** Готово к разработке

**Эпик:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 6](../../02-epics/EPIC-001-core.md#wave-6)

`activeDeadlineSeconds`, `ttlSecondsAfterFinished`, termination grace, CronJob `Forbid` examples and tests.

## Контракт готовности к разработке

- **Нормативные входы:** ADR-003 и
  ADR-012, [Runtime Defaults](../../../04-specification/runtime-defaults-and-resource-limits.md)
  и [Docker and Compose Examples](../../../04-specification/docker-compose-examples.md).
- **Зависимости:** ISSUE-008, ISSUE-009 и ISSUE-029.
- **Объём / вне объёма:** Определить Job/CronJob deadlines, TTL, restart/concurrency policy и termination grace с
  измеримым margin над internal grace. Вне scope: cluster-specific admission policy.
- **Конфигурация и наблюдаемые отказы:** Static checks отклоняют external grace не больше internal total; forced
  deadline/scheduling outcomes видны в Kubernetes status/logs.
- **Критерии приёмки и обязательные тесты:** Deadline до/после workload; baseline 32s external против 30s internal; TTL
  cleanup; CronJob Forbid; restart Never; forced termination.
- **Условие завершения:** Готово, когда manifests кодируют каждый lifecycle bound, а conformance tests измеряют safety
  margin.
