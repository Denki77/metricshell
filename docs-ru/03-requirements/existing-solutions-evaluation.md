# Существующие решения: критерии оценки и официальные источники

[English version](../../docs/03-requirements/existing-solutions-evaluation.md)

> Статус: исследовательская база для архитектурного исследования

## Назначение

Документ определяет правила сравнения MetricShell с существующими подходами.

Цель — не доказать универсальное превосходство. Требуется определить, даёт ли переиспользуемый runtime wrapper внутри
container согласованную комбинацию компромиссов, которую существующие подходы не предоставляют одновременно.

## Официальные свидетельства

Prometheus прямо указывает, что Pushgateway рекомендуется только для ограниченных сценариев, главным образом для
service-level batch jobs. При использовании в качестве общей замены обычного pull scraping документированы проблемы
lifecycle, stale series, bottleneck, single point of failure и потеря метрики up.

- Prometheus — When to use the Pushgateway
  [https://prometheus.io/docs/practices/pushing/](https://prometheus.io/docs/practices/pushing/)

Руководство Prometheus по exporters рекомендует Pushgateway для service-level batch metrics. Для instance-level batch
metrics оно прямо отмечает отсутствие однозначного паттерна и перечисляет Node Exporter textfile collector, in-memory
state либо реализацию аналогичного механизма.

- Prometheus — Writing exporters
  [https://prometheus.io/docs/instrumenting/writing_exporters/](https://prometheus.io/docs/instrumenting/writing_exporters/)

То же руководство рекомендует, чтобы exporter обычно наблюдал один экземпляр application и работал рядом с ним. Это
поддерживает исследование локальной per-workload exporter/runtime model, но само по себе не доказывает дизайн
MetricShell.

Docker официально поддерживает wrapper scripts и process managers, когда container должен запускать несколько связанных
процессов, и указывает, что main process отвечает за управление child processes.

- Docker — Run multiple processes in a container
  [https://docs.docker.com/engine/containers/multi-service_container/](https://docs.docker.com/engine/containers/multi-service_container/)

Kubernetes поддерживает native lifecycle sidecar containers, включая Jobs. Это допустимая альтернатива, но она остаётся
Kubernetes-specific multi-container deployment model с отдельными lifecycle и resource accounting.

- Kubernetes — Sidecar Containers
  [https://kubernetes.io/docs/concepts/workloads/pods/sidecar-containers/](https://kubernetes.io/docs/concepts/workloads/pods/sidecar-containers/)

## Правила доказательств

Каждое сравнение ОБЯЗАНО различать:

- факт из официальной документации;
- измерение прототипа;
- предположение проекта;
- вывод проекта.

Подход нельзя называть «ненужным», «сложным», «лучшим» или «худшим» без указания workload и критерия оценки.

## Критерии сравнения

### C-01 — Pull semantics Prometheus

- Связан ли endpoint с одним экземпляром workload?
- Завершает ли исчезновение target его scrape lifecycle естественным образом?
- Сохраняется ли стандартная семантика up?

### C-02 — Связь lifecycle

- Связаны ли metrics с экземпляром workload/container?
- Могут ли stale series сохраниться после его исчезновения?
- Кто выполняет cleanup?

### C-03 — Доступность final state конечного workload

- Остаются ли final metrics доступными для scrape после process exit?
- Ограничена ли доступность по времени?
- Можно ли согласовать завершение с одним или несколькими scrapes?

### C-04 — Обязательная инфраструктура

- Требуется ли gateway, collector, node agent, controller или datastore?
- Появляется ли новый общий failure domain?

### C-05 — Deployment topology

- Требуется ли дополнительный container, shared volume, Pod mutation, node daemon или service?
- Можно ли внедрить решение только на этапе сборки image?

### C-06 — Переносимость окружения

- Работает ли один artifact в Docker, Compose, Kubernetes, CI и локальном окружении?
- Требуются ли Kubernetes APIs или Pod lifecycle features?

### C-07 — Интеграция application

- Должно ли application поднимать HTTP?
- Должно ли оно знать адрес центрального service?
- Можно ли использовать простой file или local IPC?

### C-08 — Корректность процессов

- Кто отвечает за PID 1, signals, process groups, child reaping, exit codes и deadlines?
- Может ли telemetry handling изменить workload outcome?

### C-09 — Изоляция и обновления

- Можно ли обновить metrics component независимо?
- Какая process, filesystem, network и resource isolation существует?
- Какая coupling принимается?

### C-10 — Эксплуатационная нагрузка

- Что platform team должна разворачивать, защищать, обнаруживать, наблюдать, обновлять и очищать?
- Configuration задаётся per app, node, cluster или глобально?

### C-11 — Владение metric state

- Кто владеет counters, gauges, histograms, aggregation, expiry и resets?
- Переживает ли state restart?
- Является ли persistence преимуществом или риском stale data?

### C-12 — Поведение при отказе

- Что происходит при failure обработки metrics?
- Может ли workload продолжить работу?
- Наблюдаема ли потеря?
- Есть ли центральный bottleneck или single point of failure?

### C-13 — Resource и cardinality controls

- Ограничены ли series, labels, payloads, connections, queues и memory?
- Наблюдаемы ли rejections?

### C-14 — Подтверждение final scrape

- Может ли решение знать, что final response был отправлен?
- Может ли оно отличить Prometheus от probes или ручных requests?
- Гарантируется только доставка response или durable storage?

### C-15 — Совместимость со стандартами

- Какие форматы Prometheus/OpenMetrics поддерживаются?
- Сохраняются ли metric types и labels?
- Можно ли переиспользовать существующие discovery и scrape configuration?

## Сравниваемые подходы

1. Прямая instrumentation application со встроенным HTTP.
2. Prometheus Pushgateway.
3. Node Exporter textfile collector.
4. Отдельный exporter process в том же container.
5. Kubernetes sidecar exporter.
6. OpenTelemetry SDK и Collector.
7. Локальный exporter в стиле StatsD.
8. Runtime-wrapper model MetricShell.

## Начальная исследовательская гипотеза

| Критерий                        | Embedded endpoint     | Pushgateway            | Textfile collector       | Kubernetes sidecar     | Цель MetricShell    |
|---------------------------------|-----------------------|------------------------|--------------------------|------------------------|---------------------|
| Per-instance pull               | Сильная               | Косвенная              | Через node target        | Сильная                | Сильная             |
| Доступность final finite-job    | Пока app обслуживает  | Сильная                | File сохраняется         | Зависит от дизайна     | Явные bounded modes |
| Нужен shared component          | Нет                   | Да                     | Node exporter            | Нет central component  | Нет                 |
| Переносимость в обычный Docker  | Сильная               | Нужен доступ к gateway | Нужна collector topology | Слабая как K8s pattern | Сильная             |
| Application поднимает HTTP      | Да                    | Нет                    | Нет                      | Нет                    | Нет                 |
| Внедрение только через image    | Требует изменения app | Нет                    | Обычно нет               | Нет                    | Целевая capability  |
| Независимое обновление exporter | Связано с app         | Центральное            | На уровне node           | Сильное                | Связано с image     |
| Cleanup экземпляра              | Естественный          | Требует управления     | Требует владения file    | Вместе с Pod           | Вместе с container  |
| Process management включён      | Зависит от app        | Нет                    | Нет                      | Нет                    | Целевая capability  |

Таблица является гипотезой, а не выводом. Архитектурное исследование и прототипы ОБЯЗАНЫ её проверить.

## Проверка необходимости

MetricShell обоснован только если исследование докажет:

1. Существующие подходы не дают одновременно следующую комбинацию:
  1.1. per-instance pull endpoint;
  1.2. отсутствие обязательного central component;
  1.3. integration на уровне image;
  1.4. независимость от orchestrator;
  1.5. управляемый lifecycle workload;
  1.6. bounded availability final state.

2. Для хотя бы одного точно определённого класса workloads объединённый runtime проще ближайшей альтернативы.
3. Риски process wrapper можно ограничить и протестировать.
4. Runtime overhead приемлем.
5. Socket, file и local push могут иметь общую согласованную семантику без неподдерживаемой protocol surface.
6. Проект явно документирует сценарии, где предпочтительно другое решение.

Если эти условия не доказаны, scope MetricShell следует сузить или перепозиционировать, а не обосновывать одной
амбициозностью.
