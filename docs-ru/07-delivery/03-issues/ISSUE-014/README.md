# ISSUE-014. Начальное zero-series state

**Статус:** Открыто
**Готовность:** Готово к разработке

**Эпик:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 3](../../02-epics/EPIC-001-core.md#wave-3)

**ADR/INV:** ADR-004, ADR-011.  
**Acceptance criteria:** система корректно экспонирует валидное начальное состояние до первой publication и после
workload без публикаций.

## Контракт готовности к разработке

- **Нормативные входы:**
  ADR-004, [Application Snapshot Protocol](../../../04-specification/application-snapshot-protocol.md)
  и [Self-Metrics](../../../04-specification/self-metrics.md).
- **Зависимости:** ISSUE-011 и ISSUE-013.
- **Объём / вне объёма:** Установить explicit generation-zero, zero-series application state до publication. Вне scope:
  трактовка empty transport input как empty snapshot.
- **Конфигурация и наблюдаемые отказы:** Empty payload отклоняется и наблюдаем; отсутствие publication при startup
  остаётся валидным self-metrics-only exposition.
- **Критерии приёмки и обязательные тесты:** Startup без publication; explicit zero-series publication; zero-byte
  file/body/socket transaction; assertions generation/self-metrics.
- **Условие завершения:** Готово, когда startup exposition валиден, а все empty-input cases сохраняют правильную
  generation.
