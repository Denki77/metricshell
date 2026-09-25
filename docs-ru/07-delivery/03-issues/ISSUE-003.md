# ISSUE-003. Управляемая process group/session

**Статус:** Готово
**Готовность:** Готово к разработке

**Эпик:** [EPIC-001 Core](../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 1](../02-epics/EPIC-001-core.md#wave-1)

**ADR/INV:** ADR-001 / INV-001.  
**Результат:** workload и его потомки находятся в управляемой process group.

**Acceptance criteria:**

- сигналы адресуются всей workload process group;
- сторонние процессы контейнера не затрагиваются;
- тест содержит workload, создающий child и grandchild;
- group identity логируется без high-cardinality labels.

## Контракт готовности к разработке

- **Нормативные входы:** ADR-001 / INV-001, [Runtime State Machine](../../04-specification/runtime-state-machine.md)
  и [Structured Logging](../../04-specification/structured-logging.md).
- **Зависимости:** ISSUE-002.
- **Объём / вне объёма:** Создать и владеть process group/session workload, адресуя только это дерево. Вне scope:
  сторонние container processes и pod-wide signaling Kubernetes.
- **Конфигурация и наблюдаемые отказы:** Ошибки создания group или signaling дают sanitized structured errors; process
  identifiers не становятся metric labels.
- **Критерии приёмки и обязательные тесты:** Дерево child/grandchild; unrelated sibling process; быстрый exit при setup;
  доставка group signal; race detector и zombie check.
- **Условие завершения:** Готово, когда descendants управляются как одно дерево, а сторонние процессы не затрагиваются
  во всех integration fixtures.

## Журнал выполнения

- 2026-08-15: задача переведена в `В работе`; реализация начата в отдельном worktree `ISSUE-003`.
- 2026-08-15: задача переведена в `Тестирование`; Docker CI прошёл с race detector и process-group acceptance fixture.
- 2026-08-15: задача переведена в `Готово`; финальные Docker CI, arm64 runtime image и проверка полноты README EN/RU
  пройдены.

## Свидетельства проверки

- `make ci` пройден с закреплённым по digest Go 1.26 builder и Docker daemon; `go vet` и `go test -race` прошли.
- Workload fixture увидел равенство своих PID и PGID. Его child и grandchild унаследовали этот PGID.
- Sibling, помещённый в другую process group, не получил сигнал, отправленный управляемой workload group; root, child и
  grandchild получили его в пределах ограниченного fixture deadline.
- Пройдены двадцать пять быстрых container-запусков workload. Process-tree fixture дождался потомков и после этого не
  обнаружил zombie processes.
- `workload.started` записывает PID и PGID workload как поля structured log. Ошибка запуска missing executable остаётся
  sanitized как `workload.start_failed` / `WORKLOAD_START_FAILED` без раскрытия аргументов workload.
- Production `scratch` image собран для `linux/arm64`, его entrypoint-проверка `--help` прошла.
- `implementation/README.md` и `implementation/README_RU.md` проверены и обновлены синхронно; external signal forwarding
  и descendant reaping явно остаются в ISSUE-004 и ISSUE-005.
