# Архитектурное исследование

> Статус: выполняется  
> Назначение: оценить архитектурные варианты до создания ADR и начала реализации  
> Область: runtime MetricShell, жизненный цикл процессов, ingestion метрик, exposition и координация shutdown

## 1. Назначение

Этот документ фиксирует ход архитектурного исследования MetricShell. Он не является архитектурной спецификацией и не
утверждает решение заранее.

Исследование должно выявить значимые архитектурные вопросы, описать реалистичные варианты, сформулировать опровержимые
гипотезы, определить воспроизводимые эксперименты, собрать доказательства, оценить компромиссы и создавать ADR только
после того, как доказательств станет достаточно.

## 2. Метод исследования

Каждый пункт исследования проходит одну и ту же последовательность:

1. **Вопрос** — один конкретный архитектурный вопрос.
2. **Контекст** — почему он важен и какие требования затрагивает.
3. **Кандидаты** — реалистичные альтернативы, включая текущий предпочтительный вариант.
4. **Гипотезы** — ожидаемые результаты, сформулированные до эксперимента.
5. **Необходимые доказательства** — официальная документация, прототип, интеграционный тест, fault injection, benchmark
   или эксплуатационное сравнение.
6. **Эксперимент** — воспроизводимая среда, workload, входные данные, измерения и количество повторений.
7. **Критерии оценки** — определяются до получения результатов.
8. **Результаты** — сырые наблюдения, измерения и ошибки.
9. **Вывод** — принять, отклонить, оставить fallback-вариантом, отложить либо признать доказательства недостаточными.
10. **Результат решения** — ADR, обновлённое требование, benchmark result, запись об отклонённой альтернативе либо новый
    пункт исследования.

Хорошая гипотеза допускает опровержение:

> Directory-level inotify с периодической reconciliation обнаруживает атомарную замену файла при меньшем idle CPU, чем
> polling-only, и при этом восстанавливается после потерянных уведомлений.

Плохая гипотеза субъективна:

> inotify лучше.

## 3. Уровни доказательств

Предпочтительный порядок:

1. спецификация или официальная документация;
2. воспроизводимый интеграционный тест;
3. воспроизводимый benchmark;
4. наблюдение за прототипом;
5. анализ исходного кода;
6. опыт команды;
7. мнение.

Доказательства нижних уровней полезны для формирования гипотез, но не должны быть единственным основанием для значимого
архитектурного решения.

## 4. Согласованное направление проектирования

Следующее направление принято и не пересматривается без новых противоречащих доказательств.

MetricShell:

- добавляется в container image приложения;
- запускает и контролирует одно выполнение workload;
- предоставляет Prometheus-compatible endpoint;
- принимает application metrics локально;
- после завершения workload может оставаться доступным ограниченное время;
- сохраняет результат workload, если сам MetricShell не завершился с ошибкой;
- не требует центрального metrics gateway;
- не отправляет application metrics в Prometheus;
- не зависит от Kubernetes API;
- поддерживает использование в Docker, Docker Compose и Kubernetes;
- оставляет restart policy container runtime или orchestrator.

Архитектурное исследование может уточнять, как именно реализуются эти возможности.

## 5. Порядок исследований

1. [Модель процесса и PID 1](architecture-research.md#inv-001)
2. [Жизненный цикл workload и семантика завершения](architecture-research.md#inv-002)
3. [Распределение shutdown budget](architecture-research.md#inv-003)
4. [Владение состоянием метрик и семантика](architecture-research.md#inv-004)
5. [Сравнение ingestion transports](architecture-research.md#inv-005)
6. [File-based ingestion](architecture-research.md#inv-006)
7. [Socket-based ingestion](architecture-research.md#inv-007)
8. [Local push ingestion](architecture-research.md#inv-008)
9. [Пригодность shared memory и mmap](architecture-research.md#inv-009)
10. [Prometheus exposition](architecture-research.md#inv-010)
11. [Финальное состояние и семантика подсчёта scrape](architecture-research.md#inv-011)
12. [Пригодность Kubernetes Job/CronJob](architecture-research.md#inv-012)
13. [Модели распространения](architecture-research.md#inv-013)
14. [Безопасность и resource limits](architecture-research.md#inv-014)
15. [Benchmarks и итоговое сравнение](architecture-research.md#inv-015)

---

## 6. Отслеживание исследований

| ID      | Тема                                                            | Статус     | Доказательства                                 | Решение                                      |
|---------|-----------------------------------------------------------------|------------|------------------------------------------------|----------------------------------------------|
| INV-001 | [PID 1 и модель процесса](architecture-research.md#inv-001)     | Завершено  | [INV-001](../../research/INV-001/README_ru.md) | [ADR-001](../06-architecture/adr/ADR-001.md) |
| INV-002 | [Жизненный цикл workload](architecture-research.md#inv-002)     | Завершено  | [INV-002](../../research/INV-002/README_ru.md) | [ADR-002](../06-architecture/adr/ADR-002.md) |
| INV-003 | [Shutdown budgeting](architecture-research.md#inv-003)          | Завершено  | [INV-003](../../research/INV-003/README_ru.md) | [ADR-003](../06-architecture/adr/ADR-003.md) |
| INV-004 | [Семантика metric state](architecture-research.md#inv-004)      | Завершено  | [INV-004](../../research/INV-004/README_ru.md) | [ADR-004](../06-architecture/adr/ADR-004.md) |
| INV-005 | [Сравнение transports](architecture-research.md#inv-005)        | Завершено  | [INV-005](../../research/INV-005/README_ru.md) | [ADR-005](../06-architecture/adr/ADR-005.md) |
| INV-006 | [File ingestion](architecture-research.md#inv-006)              | Завершено  | [INV-006](../../research/INV-006/README_ru.md) | [ADR-006](../06-architecture/adr/ADR-006.md) |
| INV-007 | [Socket ingestion](architecture-research.md#inv-007)            | Завершено  | [INV-007](../../research/INV-007/README_ru.md) | [ADR-007](../06-architecture/adr/ADR-007.md) |
| INV-008 | [Local push](architecture-research.md#inv-008)                  | Завершено  | [INV-008](../../research/INV-008/README_ru.md) | [ADR-008](../06-architecture/adr/ADR-008.md) |
| INV-009 | [Shared memory/mmap](architecture-research.md#inv-009)          | В процессе | —                                              | —                                            |
| INV-010 | [Exposition](architecture-research.md#inv-010)                  | Не начато  | —                                              | —                                            |
| INV-011 | [Семантика финального scrape](architecture-research.md#inv-011) | Не начато  | —                                              | —                                            |
| INV-012 | [Пригодность Kubernetes](architecture-research.md#inv-012)      | Не начато  | —                                              | —                                            |
| INV-013 | [Распространение](architecture-research.md#inv-013)             | Не начато  | —                                              | —                                            |
| INV-014 | [Безопасность и лимиты](architecture-research.md#inv-014)       | Не начато  | —                                              | —                                            |
| INV-015 | [Benchmarks](architecture-research.md#inv-015)                  | Не начато  | —                                              | —                                            |

## 7. Критерии завершения

Архитектурное исследование достаточно полно для начала production implementation, когда:

- по всем высокорисковым вопросам есть выводы, подтверждённые доказательствами;
- для выбранных альтернатив созданы ADR;
- отклонённые альтернативы задокументированы;
- behavioral model обновлена;
- state machine отражает выбранную lifecycle semantics;
- для protocols подготовлены draft specifications;
- определены benchmark targets;
- обновлены acceptance criteria;
- не осталось нерешённых вопросов, способных опровергнуть базовую runtime model.

Во время исследования допустимы implementation spikes, но до фиксации решений они не должны считаться production
architecture.
