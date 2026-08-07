# ISSUE-027. Подсчёт complete responses и drain

**Статус:** Открыто
**Готовность:** Готово к разработке

**Эпик:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 5](../../02-epics/EPIC-001-core.md#wave-5)

Считать только полный successful write с non-cancelled context; bounded completion grace для accepted handlers.

## Контракт готовности к разработке

- **Нормативные входы:** ADR-011 и ADR-014, [Runtime State Machine](../../../04-specification/runtime-state-machine.md)
  и [Runtime Defaults](../../../04-specification/runtime-defaults-and-resource-limits.md).
- **Зависимости:** ISSUE-024 и ISSUE-026.
- **Объём / вне объёма:** Считать только полностью записанный eligible `/metrics` response frozen generation, затем
  drain already accepted responses внутри completion grace. Вне scope: учёт headers/probes.
- **Конфигурация и наблюдаемые отказы:** Cancelled, timed-out, partial, wrong-generation и probe responses дают
  not-counted outcomes; drain expiry bounded.
- **Критерии приёмки и обязательные тесты:** Full/partial/zero-byte writes; disconnect после headers/body; simultaneous
  threshold; wrong path/generation; completion-grace 0/boundary; race detector.
- **Условие завершения:** Готово, когда count/drain decisions принимаются в одной документированной write-completion
  point.
