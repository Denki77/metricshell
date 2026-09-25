# ISSUE-014. Начальное zero-series state

**Статус:** Готово
**Готовность:** Готово к разработке

**Эпик:** [EPIC-001 Core](../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 3](../02-epics/EPIC-001-core.md#wave-3)

**ADR/INV:** ADR-004, ADR-011.  
**Acceptance criteria:** система корректно экспонирует валидное начальное состояние до первой publication и после
workload без публикаций.

## Контракт готовности к разработке

- **Нормативные входы:**
  ADR-004, [Application Snapshot Protocol](../../04-specification/application-snapshot-protocol.md)
  и [Self-Metrics](../../04-specification/self-metrics.md).
- **Зависимости:** ISSUE-011 и ISSUE-013.
- **Объём / вне объёма:** Установить explicit generation-zero, zero-series application state до publication. Вне scope:
  трактовка empty transport input как empty snapshot.
- **Конфигурация и наблюдаемые отказы:** Empty payload отклоняется и наблюдаем; отсутствие publication при startup
  остаётся валидным self-metrics-only exposition.
- **Критерии приёмки и обязательные тесты:** Startup без publication; explicit zero-series publication; zero-byte
  file/body/socket transaction; assertions generation/self-metrics.
- **Условие завершения:** Готово, когда startup exposition валиден, а все empty-input cases сохраняют правильную
  generation.

## Журнал выполнения

- 2026-08-28: переведена в `В работе`; добавлены exact generation-zero application snapshot и production-конструктор
  holder.
- 2026-08-28: переведена в `Тестирование`; no-publication startup, explicit clearing publication, empty payload и
  ownership tests прошли race detector в Docker.
- 2026-08-28: переведена в `Готово`; startup имеет valid immutable zero-series state, а отсутствие input не считается
  publication.

## Свидетельства проверки

- `NewInitialHolder` до появления ingestion предоставляет exact canonical zero-series document generation `0`.
- Valid explicit zero-series candidate устанавливается атомарно, удаляет прежние application series и повышает generation.
- Zero-byte transport payload остаётся `empty_payload` и не меняет generation или canonical state.
- Каждый zero-state accessor возвращает caller-owned bytes; mutation не влияет на последующих readers.
- Projection generation-zero self-metrics покрывается complete registry в ISSUE-015.
- `implementation/README.md` и `implementation/README_RU.md` проверены и обновлены синхронно.
