# Критерии приёмки

[English version](../../docs/03-requirements/acceptance-criteria.md)

> Статус: черновик для архитектурных исследований

## Трассируемость

| Требования    | Сценарии  |
|---------------|-----------|
| FR-001–FR-006 | AC-RUN-*  |
| FR-010–FR-016 | AC-ING-*  |
| FR-020–FR-026 | AC-MET-*  |
| FR-030–FR-034 | AC-EXP-*  |
| FR-040–FR-047 | AC-FIN-*  |
| FR-050–FR-052 | AC-OBS-*  |
| FR-060–FR-063 | AC-DIST-* |
| FR-070–FR-073 | AC-PORT-* |
| FR-080–FR-082 | AC-CONF-* |

## Жизненный цикл runtime

### AC-RUN-001 — Выполнение команды

При наличии валидного executable, аргументов и environment, когда запускается MetricShell, workload запускается
с указанными значениями.

### AC-RUN-002 — Сохранение success

Если workload завершается с `0`, то после выполнения настроенного shutdown behavior MetricShell завершается успешно,
если не произошёл независимый runtime failure.

### AC-RUN-003 — Сохранение failure

Если workload завершается с `17`, то после завершения MetricShell контейнер остаётся завершившимся с ошибкой и сохраняет
результат workload, если не произошёл документированный runtime failure с более высоким приоритетом.

### AC-RUN-004 — Ошибка запуска

Если executable невозможно запустить, MetricShell сообщает об ошибке запуска workload, не утверждает, что workload
выполнялся, и завершается с документированным runtime result.

### AC-RUN-005 — Пересылка сигнала

Если workload фиксирует signals, то при получении MetricShell настроенного termination signal workload либо выбранная
process group получает ожидаемый signal в пределах ограниченного интервала.

### AC-RUN-006 — Graceful shutdown

Если workload завершается в течение grace period, MetricShell не применяет forced termination до истечения deadline.

### AC-RUN-007 — Forced shutdown

Если workload игнорирует graceful termination, после истечения deadline применяется документированное forced
termination, а MetricShell завершается в пределах установленной границы.

### AC-RUN-008 — Reaping дочерних процессов

После завершения управляемых child processes не остаётся zombies, за которые отвечает MetricShell.

### AC-RUN-009 — Отсутствие бесконечного ожидания

Во всех modes и failure paths истечение всех настроенных deadlines приводит к terminal outcome.

## Ingestion transports

Один и тот же полный application snapshot ДОЛЖЕН передаваться через socket, file и local HTTP transports.

### AC-ING-001 — Эквивалентность file snapshot

Полный dataset с counters, gauges и histograms, переданный через file ingestion, публикуется в ожидаемом виде.

### AC-ING-002 — Эквивалентность socket snapshot

Тот же полный dataset, переданный как один framed socket snapshot, создаёт exposition, идентичный AC-ING-001.

### AC-ING-003 — Эквивалентность HTTP snapshot

Тот же полный dataset, переданный одним local HTTP request, создаёт exposition, идентичный AC-ING-001.

### AC-ING-004 — Атомарная видимость файла

Во время замены файла scrape видит либо предыдущее валидное состояние, либо новое валидное состояние, но не смешанное
или частичное.

### AC-ING-005 — Невалидный socket input

Невалидный socket input отклоняется атомарно, не изменяет последний принятый snapshot, диагностируется и не завершает
workload.

### AC-ING-006 — Невалидный файл

Невалидный file input отклоняется атомарно согласно документированной fallback policy и не изменяет последний принятый
snapshot.

### AC-ING-007 — Невалидный HTTP input

Невалидный HTTP input получает документированный atomic rejection и не изменяет последний принятый snapshot.

### AC-ING-008 — Capacity limit

Candidate, превышающий настроенный limit, отклоняется атомарно без изменения последнего принятого snapshot и
неконтролируемого роста memory.

### AC-ING-009 — Явный transport

Выбранный transport наблюдаем в effective configuration; отключённые transports не принимаются неявно.

### AC-ING-010 — Изоляция failure transport

Failure выбранного metrics transport по умолчанию не завершает workload.

### AC-ING-011 — Пустой body не является пустым snapshot

Пустой HTTP body отклоняется как malformed transport payload и не изменяет последний принятый snapshot. Синтаксически
валидный полный snapshot с нулём metric families или series принимается как zero-series snapshot.

### AC-ING-012 — Acknowledgement и порядок принятия

Успешный acknowledgement socket или local HTTP возвращается только после validation и atomic installation candidate.
Одновременно принятые candidates получают один линейный порядок, и последний candidate в этом порядке становится active
state. Timestamps producers не изменяют этот порядок.

## Модель метрик

### AC-MET-001 — Counter

Структурно валидный counter в одном snapshot публикуется с корректным Prometheus representation. MetricShell не
сравнивает последовательные snapshots для проверки business-level monotonicity counter.

### AC-MET-002 — Gauge

Валидный gauge может увеличиваться и уменьшаться.

### AC-MET-003 — Histogram

Валидный classic histogram snapshot содержит упорядоченные boundaries, cumulative bucket values, bucket `+Inf`,
равный `count`, и структурно валидный numeric `sum`; такой snapshot публикуется корректно.

### AC-MET-004 — Невалидные имена и labels

Candidate snapshot с любым невалидным metric или label name отклоняется атомарно. Последний принятый snapshot остаётся
неизменным.

### AC-MET-005 — Duplicate series

Candidate snapshot, содержащий одну series более одного раза, отклоняется атомарно и не изменяет active state.

### AC-MET-006 — Конфликт привязки типа

Type conflict внутри candidate snapshot или с установленной привязкой name-to-type metric family отклоняется атомарно.
Отсутствие всех series family и последующее повторное использование её имени с другим type остаётся конфликтом внутри
того же workload execution. Изменение только HELP не нарушает type binding, если candidate остаётся внутренне
согласованным.

### AC-MET-007 — Limits series и labels

Candidate, превышающий настроенные limits series или labels, отклоняется атомарно, последний принятый snapshot не
изменяется, а diagnostic signals публикуются.

### AC-MET-008 — Payload limit

Oversized input отклоняется без неограниченного allocation.

### AC-MET-009 — Сохранение последнего валидного snapshot

Если после принятого snapshot поступает malformed или conflicting candidate snapshot, exposition продолжает без
изменений публиковать ранее принятый snapshot.

### AC-MET-010 — Отсутствие агрегации между producers

Если принятый полный application snapshot содержит series со значением `2`, а следующий принятый полный application
snapshot содержит ту же series со значением `3`, exposition содержит `3`. MetricShell никогда не выводит `5` и не
хранит producer-scoped contributions. Компоненты workload должны координироваться до публикации.

### AC-MET-011 — Zero-series snapshot

Если после snapshot с application series принимается корректно закодированный zero-series snapshot, все application
series удаляются, а self-metrics MetricShell остаются доступны.

## Exposition

### AC-EXP-001 — Разбираемый response

Успешный metrics request возвращает заявленный compatible content type и payload, разбираемый совместимыми
Prometheus tools.

### AC-EXP-002 — Согласованный concurrent scrape

Конкурентные ingestion и scrape никогда не создают torn либо синтаксически невалидный response.

### AC-EXP-003 — Состав response

Response содержит active application snapshot — первоначально zero-series либо впоследствии принятый — и
документированные self-metrics MetricShell, но не содержит не относящиеся к workload host-wide metrics.

### AC-EXP-004 — Filtering

Настроенная фильтрация family/prefix детерминирована.

### AC-EXP-005 — Concurrent clients

Concurrent scrapes безопасны и ограничены по ресурсам.

### AC-EXP-006 — Exposition failure

Если валидный response сформировать невозможно, используется документированное HTTP и diagnostic behavior.

### AC-EXP-007 — Bind failure

Если обязательный endpoint невозможно bind до запуска workload, startup завершается ошибкой, если явно не выбран
документированный degraded mode.

## Поведение конечных workloads

### AC-FIN-001 — Формирование финального состояния

После завершения workload и до того, как final scrape может быть засчитан, MetricShell формирует ровно одно финальное
наблюдаемое application state.

### AC-FIN-002 — Стабильное финальное состояние

Повторные post-exit scrapes возвращают стабильные значения application metrics.

### AC-FIN-003 — Immediate mode

Immediate mode не добавляет намеренного ожидания после завершения workload.

### AC-FIN-004 — Fixed-duration mode

При configured duration `D` endpoint остаётся доступным приблизительно в течение `D`, с учётом scheduling tolerance
и внешнего termination.

### AC-FIN-005 — Scrapes не продлевают delay

В fixed-duration mode scrapes не продлевают deadline, если это явно не настроено как отдельная возможность.

### AC-FIN-006 — Один final scrape

При required count `1` один допустимый полностью переданный response удовлетворяет scrape condition.

### AC-FIN-007 — N final scrapes

При required count `N` меньше `N` допустимых responses не удовлетворяют condition; ровно `N` — удовлетворяют.

### AC-FIN-008 — Timeout

Если threshold не достигнут, configured timeout завершает ожидание и по умолчанию не изменяет результат workload.

### AC-FIN-009 — Health request исключён

Health/readiness requests никогда не увеличивают final-scrape count.

### AC-FIN-010 — Неуспешный response исключён

Request, который не получил полный final response успешно, не засчитывается.

### AC-FIN-011 — Scrape до финализации исключён

Scrape до формирования final state не засчитывается.

### AC-FIN-012 — Concurrent final scrapes

Конкурентные допустимые requests учитываются атомарно согласно документированной policy и без races.

### AC-FIN-013 — Приоритет external termination

External shutdown deadline завершает post-exit waiting, и MetricShell завершается в пределах доступного grace.

### AC-FIN-014 — Отсутствие гарантии durability

Документация и diagnostics не утверждают, что выданный response доказывает сохранение в TSDB или remote-write.

### AC-FIN-015 — Отсутствие application publication

Если workload завершается без единого принятого application snapshot, финальное application state содержит ноль
application series, а документированные self-metrics MetricShell остаются доступны.

## Наблюдаемость

### AC-OBS-001

Операторы могут различать working runtime, running workload, completed workload во время ожидания runtime,
forced termination и runtime failure.

### AC-OBS-002

Во время post-exit waiting logs или self-metrics показывают активный mode, оставшееся condition и deadline.

### AC-OBS-003

Rejected input, endpoint failures и forced termination наблюдаемы.

## Поставка и переносимость

### AC-DIST-001

Документированный multi-stage Dockerfile собирает и запускает reference application без дополнительного контейнера.

### AC-DIST-002

Reference application наследуется от поддерживаемого MetricShell base image, устанавливает dependencies и code
и успешно запускается.

### AC-DIST-003

Standalone-copy и base-image builds проходят один core conformance suite.

### AC-DIST-004

Reference images работают как non-root user и публикуют build version/revision.

### AC-PORT-001

Reference long-running и finite workloads проходят Docker end-to-end tests.

### AC-PORT-002

Prometheus в Docker Compose успешно scrapes reference workload.

### AC-PORT-003

Kubernetes long-running workload публикует метрики и корректно завершается.

### AC-PORT-004

Kubernetes finite Job-style workload выполняет configured final availability и завершается.

### AC-PORT-005

Один и тот же executable работает в Docker без доступа к Kubernetes API, service account или Kubernetes-specific mounts.

## Конфигурация

### AC-CONF-001

Валидная configuration запускается и публикует effective non-secret values.

### AC-CONF-002

Malformed или negative durations и invalid scrape counts отклоняются до запуска workload.

### AC-CONF-003

Противоречивые lifecycle или transport options отклоняются с actionable error.

### AC-CONF-004

Пропущенные optional values разрешаются в документированные детерминированные defaults.

### AC-CONF-005

Secrets скрываются в обычных logs и diagnostics.
