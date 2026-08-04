# ISSUE-036. Controlled release benchmark suite

**Статус:** Открыто
**Готовность:** Готово к разработке

**Эпик:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 6](../../02-epics/EPIC-001-core.md#wave-6)

**ADR/INV:** ADR-015 / INV-015.  
Не менее 30 repetitions, pinned CPU/resources, production binary/adapters; correctness оценивается отдельно от SLO thresholds.

## Контракт готовности к разработке

- **Нормативные входы:** ADR-015 /
  INV-015, [Runtime Defaults](../../../04-specification/runtime-defaults-and-resource-limits.md)
  и [Docker and Compose Examples](../../../04-specification/docker-compose-examples.md).
- **Зависимости:** ISSUE-032 и ISSUE-035.
- **Объём / вне объёма:** Benchmark production binary/adapters в controlled pinned environments минимум в 30
  repetitions, разделяя correctness и release thresholds. Вне scope: пересборка или использование research prototypes
  как release artifacts.
- **Конфигурация и наблюдаемые отказы:** Environment drift, недостаток repetitions, correctness failure или unstable
  variance инвалидируют run с machine-readable metadata.
- **Критерии приёмки и обязательные тесты:** File/socket/HTTP; idle/load; architecture matrix; pinned CPU/memory;
  warmup; 30+ samples; raw results, summary statistics, correctness gate.
- **Условие завершения:** Готово, когда independent runner воспроизводит suite и сравнивает distributions без пересборки
  прототипов.
