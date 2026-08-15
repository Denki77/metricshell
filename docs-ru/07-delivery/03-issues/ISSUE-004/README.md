# ISSUE-004. Передача сигналов

**Статус:** Готово
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

## Журнал выполнения

- 2026-08-15: задача переведена в `В работе`; реализация начата в отдельном worktree `ISSUE-004`.
- 2026-08-15: задача переведена в `Тестирование`; Docker CI прошёл с race и signal-forwarding acceptance coverage.
- 2026-08-15: задача переведена в `Готово`; финальные Docker CI, arm64 runtime image и проверка полноты README EN/RU
  пройдены.
- 2026-08-15: задача возвращена в `В работе`: review выявил пробелы pre-start и error-path lifecycle.
- 2026-08-15: задача снова переведена в `Тестирование`; реализованы deterministic pre-start termination,
  гарантированный error-path Wait и нормативный ignored-signal logging.
- 2026-08-15: задача снова переведена в `Готово`; review fixes прошли финальные Docker CI, arm64 runtime image и
  проверки полноты README/specification.

## Свидетельства проверки

- `make ci` пройден с закреплённым по digest Go 1.26 builder и Docker daemon; `gofmt`, `go vet` и `go test -race`
  прошли.
- Docker доставил TERM и INT MetricShell как PID 1 контейнера; workload наблюдал каждый forwarded group signal.
- Signal fixture наблюдал TERM, INT, HUP, QUIT и повторный TERM. Каждый доставленный signal создал упорядоченную запись
  `workload.signal_forwarded` с PGID workload.
- Один runtime ID и монотонно растущий sequence сохраняются во всех lifecycle records. После TERM/INT последующие
  control-signal records остаются в `stopping`, а не возвращаются в `running`.
- Двадцать пять workload-exit/forwarding races завершились без зависаний и panic. Исчезнувшие process groups,
  unsupported signals и queued post-wait signals используют bounded ignored paths.
- Существующая immediate Docker startup-signal race остаётся покрытой, классификация startup failure сохранена.
- Queued pre-start TERM возвращает `143` без вызова workload start observer и без `workload.start_failed`; тест использует
  несуществующую команду и тем самым доказывает отсутствие exec attempt.
- Observer и forwarding failures завершают и ожидают реальный direct-child helper. После `Run` вызов `Wait4` возвращает
  `ECHILD`, а тест с неэффективным group kill покрывает bounded direct-process fallback.
- `workload.signal_ignored` и закрытые причины `target_exited|unsupported_signal` теперь нормативно описаны в обеих
  языковых версиях Structured Logging specification.
- Production `scratch` image собран для `linux/arm64`, его entrypoint-проверка `--help` прошла.
- `implementation/README.md` и `implementation/README_RU.md` проверены и обновлены синхронно. Descendant reaping
  остаётся в ISSUE-005; grace budgeting и forced cleanup — в ISSUE-009.
