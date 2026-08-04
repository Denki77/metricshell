# Завершение архитектуры MetricShell Core

[English version](../../docs/06-architecture/CORE_COMPLETION.md)

- **Статус:** завершено
- **Scope:** snapshot-семантика MetricShell Core
- **Расширения:** требуют отдельного нормативного контура

## 1. Назначение

Документ завершает этап определения архитектуры MetricShell Core в scope полных snapshots, принятом ADR-001…ADR-015.
Он не принимает и не проектирует заранее aggregation или другую модель владения application state.

## 2. Завершённый scope Core

MetricShell Core включает:

- владение workload как PID 1, передачу сигналов, reaping потомков и сохранение результата workload;
- bounded lifecycle, shutdown, finalization и final-scrape behavior;
- приём одного полного application snapshot за публикацию;
- whole-candidate validation и atomic replacement последнего валидного snapshot;
- семантически эквивалентный ingestion через file, Unix socket и local HTTP;
- Prometheus/OpenMetrics exposition одного immutable snapshot на scrape;
- non-root operation, resource limits, воспроизводимое распространение и release evidence.

Нормативное поведение задаётся принятыми требованиями, спецификациями и ADR-001…ADR-015.

## 3. Snapshot-контракт Core

Успешная публикация представляет одно полное application state одного запуска workload. Core валидирует candidate как
неделимую единицу, атомарно устанавливает его и сохраняет предыдущее валидное состояние при rejection.

Core не выполняет:

- merge независимых registries или snapshots;
- summation значений нескольких publishers;
- replay операций increment, set или observe;
- хранение per-producer contributions или истории snapshots;
- гарантированный scrape или durable storage каждой версии;
- сохранение application metrics между запусками процесса MetricShell.

Начальный zero-series snapshot и каждый последующий принятый zero-series snapshot являются валидными полными
состояниями.

## 4. Граница завершения

Завершение Core не включает managed aggregation, operation-level ingestion, cross-process registry ownership,
cross-container aggregation, remote write, durable metric storage или Prometheus HA deduplication.

Документ не фиксирует CLI mode, environment variable, публичный operation protocol, внутренний interface, package
layout, binary layout, image layout или deployment topology для будущей aggregation capability.

## 5. Правило расширения

Любая функция, изменяющая владение, объединение, восстановление application state или его построение из операций,
требует
собственной нормативной цепочки:

~~~text
requirements
-> investigation evidence
-> accepted ADR
-> public specification
-> delivery issues and acceptance tests
~~~

Расширение обязано явно определить совместимость со snapshot-семантикой Core. Оно может переиспользовать принятые
контракты Core, но это не означает предварительного одобрения конкретной структуры реализации.

## 6. Заявление о завершении

MetricShell Core архитектурно завершён только в принятом scope полных snapshots. Aggregation не входит в Core.
Будущая aggregation или operation-level publication не принята до завершения отдельного нормативного контура.
