# ISSUE-029. Интеграция Kubernetes Job

**Статус:** Открыто
**Готовность:** Готово к разработке

**Эпик:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 6](../../02-epics/EPIC-001-core.md#wave-6)

**ADR/INV:** ADR-012 / INV-012.  
Добавить examples PodMonitor/direct discovery, bounded final window и документированную readiness policy.

## Контракт готовности к разработке

- **Нормативные входы:** ADR-012 /
  INV-012, [Docker and Compose Examples](../../../04-specification/docker-compose-examples.md), [Runtime State Machine](../../../04-specification/runtime-state-machine.md)
  и [Configuration](../../../04-specification/configuration.md).
- **Зависимости:** ISSUE-010, ISSUE-026, ISSUE-032 и ISSUE-033.
- **Объём / вне объёма:** Предоставить executable Kubernetes Job discovery/PodMonitor examples с explicit ports, probes,
  final window и Prometheus verification. Вне scope: обязательность одного operator.
- **Конфигурация и наблюдаемые отказы:** Manifest/config errors ломают static verification; runtime verifier отличает
  final sample от последующего stale marker и сообщает missing targets.
- **Критерии приёмки и обязательные тесты:** Schema/kubeconform; direct discovery и PodMonitor; finite job; final sample
  query в recorded time; target disappearance; readiness transitions.
- **Условие завершения:** Готово, когда examples запускаются из clean manifests и проверяют final sample без active
  target.
