# ISSUE-003. Управляемая process group/session

**Статус:** Открыто
**Готовность:** Готово к разработке

**Эпик:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 1](../../02-epics/EPIC-001-core.md#wave-1)

**ADR/INV:** ADR-001 / INV-001.  
**Результат:** workload и его потомки находятся в управляемой process group.

**Acceptance criteria:**

- сигналы адресуются всей workload process group;
- сторонние процессы контейнера не затрагиваются;
- тест содержит workload, создающий child и grandchild;
- group identity логируется без high-cardinality labels.

## Контракт готовности к разработке

- **Нормативные входы:** ADR-001 / INV-001, [Runtime State Machine](../../../04-specification/runtime-state-machine.md)
  и [Structured Logging](../../../04-specification/structured-logging.md).
- **Зависимости:** ISSUE-002.
- **Объём / вне объёма:** Создать и владеть process group/session workload, адресуя только это дерево. Вне scope:
  сторонние container processes и pod-wide signaling Kubernetes.
- **Конфигурация и наблюдаемые отказы:** Ошибки создания group или signaling дают sanitized structured errors; process
  identifiers не становятся metric labels.
- **Критерии приёмки и обязательные тесты:** Дерево child/grandchild; unrelated sibling process; быстрый exit при setup;
  доставка group signal; race detector и zombie check.
- **Условие завершения:** Готово, когда descendants управляются как одно дерево, а сторонние процессы не затрагиваются
  во всех integration fixtures.
