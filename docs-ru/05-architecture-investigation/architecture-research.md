# Архитектурные исследования

Текущий документ содержит список тем для исследования.

## INV-001

Модель процесса и PID 1

### Вопрос

Должен ли MetricShell запускаться непосредственно как PID 1, работать под init-процессом вроде Tini либо делегировать
управление процессами другому компоненту?

### Контекст

MetricShell должен запускать workload, принимать и пересылать signals, отслеживать завершение workload, сохранять exit
semantics, управлять ответственными descendants и при соответствующей настройке оставаться запущенным после завершения
workload.

### Кандидаты

#### A. MetricShell как PID 1

MetricShell самостоятельно реализует требуемое поведение init и supervisor.

#### B. Tini как PID 1

Tini запускает MetricShell, а MetricShell запускает workload.

#### C. Другой process supervisor

Примеры: dumb-init, s6 или supervisord.

### Исходные гипотезы

- MetricShell обязан владеть жизненным циклом workload даже при наличии Tini.
- Tini может уменьшить количество edge cases, связанных с PID 1, но добавляет ещё один binary и дополнительный signal
  layer.
- Корректная single-binary implementation может быть проще в эксплуатации.
- Основные риски корректности связаны с process group и orphaned descendants.

### Необходимые доказательства

- документация Linux по процессам и signals;
- документация Docker по init/process;
- анализ поведения и исходного кода Tini;
- прототип для каждого реалистичного process tree;
- тесты signals, descendants, zombies и exit codes.

### Эксперименты

#### E-001.1 — Пересылка signals

Запустить workload, записывающий TERM, INT и HUP. Проверить доставку для direct child, shell wrapper, child process
group и grandchild process.

#### E-001.2 — Reaping дочерних процессов

Запустить workloads, создающие короткоживущие children и double-fork descendants. Проверить наличие zombie processes.

#### E-001.3 — Exit status

Проверить exit `0`, exit `17`, TERM, KILL, ошибку запуска workload и внутреннюю ошибку MetricShell.

#### E-001.4 — Работа после завершения workload

Проверить, что MetricShell может продолжать отдавать метрики после завершения workload, сохраняя его исходный результат.

### Критерии оценки

- корректность;
- контроль process tree;
- целостность exit code;
- сложность реализации;
- количество внешних зависимостей;
- размер image;
- переносимость;
- эксплуатационная прозрачность.

### Открытые вопросы

- Следует ли MetricShell использовать `PR_SET_CHILD_SUBREAPER`?
- Создаётся ли отдельная process group для каждого workload?
- Поддерживаются ли daemonized descendants?
- Какое поведение ожидается для shell-form commands?
- Даёт ли Tini значимое повышение корректности после того, как MetricShell сам реализует lifecycle ownership?

### Статус

[Завершено](../../research/INV-001/report.md).

---

## INV-002

Жизненный цикл workload и семантика завершения

### Вопрос

Каков точный жизненный цикл одного запуска MetricShell?

### Исходная модель

```text
инициализация
    ↓
подготовка ingestion и exposition
    ↓
запуск workload
    ↓
выполнение и отдача метрик
    ↓
workload завершён либо начато termination
    ↓
финализация application metric state
    ↓
необязательная ограниченная доступность после завершения
    ↓
завершение с итоговым результатом
```

### Варианты политики

- ровно одно выполнение workload;
- необязательный restart workload;
- делегирование restart container runtime/orchestrator.

### Исходная гипотеза

MetricShell должен выполнять workload ровно один раз. Restart policy должна оставаться вне MetricShell.

### Необходимые доказательства

- анализ сложности;
- влияние на counter reset;
- поведение restart в Docker и Kubernetes;
- покрытие acceptance tests.

### Критерии оценки

- детерминированное поведение;
- минимальный scope supervisor;
- однозначность metric state;
- совместимость с restart policies orchestrator.

### Статус

[Завершено](../../research/INV-002/report.md).

---

## INV-003

Распределение времени shutdown

### Вопрос

Какую часть внешнего shutdown grace period можно отдать workload, а какую MetricShell обязан зарезервировать для себя?

### Контекст

MetricShell требуется время, чтобы переслать termination signal, дождаться shutdown workload, при необходимости
принудительно завершить его, получить exit status, финализировать application metrics, закончить активные HTTP
responses, сбросить diagnostics и завершиться до внешнего SIGKILL.

### Варианты политики

#### A. Фиксированный резерв

```text
workload_grace = total_grace - fixed_reserve
```

#### B. Резерв в процентах

```text
workload_grace = total_grace × configured_ratio
```

#### C. Явные независимые значения

Оператор отдельно задаёт workload shutdown timeout и shutdown reserve MetricShell.

#### D. Абсолютный внешний deadline

MetricShell получает абсолютный deadline и динамически вычисляет оставшиеся budgets.

### Исходная гипотеза

Явный workload timeout вместе с runtime reserve проще для понимания, чем автоматический inference.

### Эксперименты

Проверить total shutdown windows 1, 5, 10, 30 и 60 секунд. Проверить workloads, завершающиеся сразу, непосредственно
перед deadline, после deadline и никогда.

Измерить:

- время, предоставленное workload;
- время финализации;
- время drain активных scrapes;
- полное shutdown time;
- корректность forced kill.

### Критерии оценки

- детерминированное завершение;
- понятность для оператора;
- безопасность при коротких deadlines;
- отсутствие случайного внешнего SIGKILL;
- достаточное время для HTTP shutdown;
- отсутствие неограниченного ожидания.

### Открытые вопросы

- Какой default reserve безопасен?
- Должен ли reserve быть фиксированным, процентным или комбинированным?
- Следует ли ждать post-exit scrape после начала внешнего termination?
- Как внешний deadline передаётся вне Kubernetes?
- Что происходит, если configured budgets превышают внешний grace period?

### Статус

[Завершено](../../research/INV-003/report.md).

---

## INV-004

Владение metric state и семантика

### Вопрос

Какое минимальное представление metric state требуется, если MetricShell транспортирует, валидирует и публикует
метрики, но не агрегирует значения между producers?

### Кандидаты

#### A. Полные snapshots registry

Producer публикует полное текущее состояние.

#### B. Абсолютные значения series

Producer передаёт текущее значение отдельных series.

#### C. Operations

Producer отправляет increments, sets и observations.

#### D. Гибридная модель

Разные transports поддерживают разные представления, сохраняя общую application-level semantics.

### Темы

- counters;
- gauges;
- histograms;
- duplicate series;
- type conflicts;
- несколько producers;
- ordering;
- lost updates;
- producer restarts;
- stale data;
- final application state.

### Исходная гипотеза

File ingestion естественно соответствует полным snapshots. Socket и local push могут использовать operations либо
absolute updates.

Эквивалентная client semantics не требует одинаковой transport semantics.

После сверки эксперимента с project scope эта гипотеза отклонена. Reconciliation требуется, если MetricShell принимает
изменяющие состояние operations. Producer ownership и aggregation policies требуются, если MetricShell объединяет
независимо принадлежащие registries. Обе возможности находятся вне scope MetricShell.

### Критерии оценки

- корректность после dropped messages;
- восстановление после restart MetricShell;
- сложность клиента;
- сложность protocol;
- throughput;
- memory;
- поведение при нескольких producers.

### Статус

Завершено. Выбрана модель одного полного, не содержащего конфликтов application snapshot для каждого transport.
MetricShell валидирует каждый candidate и атомарно заменяет последний валидный snapshot.

---

## INV-005

Сравнение ingestion transports

### Вопрос

Какие ingestion transports следует поддержать в первом стабильном релизе, а какие должны остаться optional adapters?

### Кандидаты

- file snapshot;
- Unix domain stream socket;
- Unix datagram socket;
- local HTTP;
- local gRPC;
- shared memory;
- memory-mapped file.

### Матрица оценки

| Критерий                       | File | Stream socket | Datagram | Local HTTP | gRPC | Shared memory | mmap |
|--------------------------------|-----:|--------------:|---------:|-----------:|-----:|--------------:|-----:|
| Сложность PHP integration      |  TBD |           TBD |      TBD |        TBD |  TBD |           TBD |  TBD |
| Throughput                     |  TBD |           TBD |      TBD |        TBD |  TBD |           TBD |  TBD |
| Latency                        |  TBD |           TBD |      TBD |        TBD |  TBD |           TBD |  TBD |
| Восстановление после loss      |  TBD |           TBD |      TBD |        TBD |  TBD |           TBD |  TBD |
| Поддержка нескольких producers |  TBD |           TBD |      TBD |        TBD |  TBD |           TBD |  TBD |
| Сложность protocol             |  TBD |           TBD |      TBD |        TBD |  TBD |           TBD |  TBD |
| Переносимость                  |  TBD |           TBD |      TBD |        TBD |  TBD |           TBD |  TBD |
| Контроль resources             |  TBD |           TBD |      TBD |        TBD |  TBD |           TBD |  TBD |

### Обязательные результаты

- прототип для каждого серьёзного кандидата;
- benchmark results;
- пример PHP integration;
- failure-mode tests;
- рекомендация и отклонённые альтернативы.

### Статус

Завершено.

---

## INV-006

File-based ingestion

### Вопрос

Как MetricShell должен безопасно обнаруживать и читать обновления файла внутри container?

### Допущение

Metrics file хранится в container filesystem либо container-local tmpfs. Host bind mounts не входят в основной supported
design.

### Кандидаты

#### A. Только polling

Периодически выполнять `stat` и читать файл.

#### B. Только inotify

Использовать filesystem events Linux.

#### C. inotify плюс reconciliation

Использовать events для быстрого обнаружения и periodic checks для восстановления.

#### D. Reload по сигналу producer

Producer записывает файл и уведомляет MetricShell через другой local mechanism.

### Исходная гипотеза

Directory-level inotify с low-frequency reconciliation обеспечивает низкий idle overhead и восстановление после missed
либо replaced watches.

### Обязательные correctness cases

- initial file уже существует;
- файла ещё нет;
- temporary file плюс atomic rename;
- repeated replacement;
- crash writer до rename;
- invalid new file;
- удаление файла;
- recreation directory;
- `IN_Q_OVERFLOW`;
- restart MetricShell.

### Эксперименты

Сравнить writable container layer и container-local tmpfs. Docker named volume можно проверить только для информации.

Измерить update-detection latency, idle CPU, CPU при высокой частоте updates, missed updates, восстановление после watch
invalidation и поведение atomic rename.

### Критерии оценки

- корректность;
- низкий idle overhead;
- простота;
- восстановление;
- совместимость с Linux container;
- отсутствие зависимости от semantics host filesystem.

### Открытые вопросы

- Следить за directory или file?
- Какой reconciliation interval приемлем?
- Какой используется file format?
- Является ли файл полным registry state?
- Следует ли сохранять last valid state при invalid update?
- Каковы максимальные размеры файла и количество series?

### Статус

Завершено.

---

## INV-007

Socket-based ingestion

### Вопрос

Какая модель local socket и какой protocol лучше всего подходят MetricShell?

### Кандидаты

- Unix stream socket;
- Unix datagram socket;
- framed binary protocol;
- line-based text protocol;
- существующий protocol вроде StatsD;
- custom versioned protocol.

### Темы

- delivery guarantees;
- ordering;
- reconnect;
- producer identity;
- несколько producers;
- backpressure;
- message size;
- socket permissions;
- workload startup race;
- shutdown MetricShell.

### Эксперименты

Проверить одного producer, нескольких producers, burst traffic, slow reader, disconnect во время message, restart
MetricShell, malformed frames, maximum payload и file-descriptor exhaustion.

### Статус

Завершено.

---

## INV-008

Local Push ingestion

### Вопрос

Даёт ли local HTTP или gRPC ingestion API достаточно преимуществ по сравнению с socket adapter, чтобы оправдать
реализацию и сопровождение?

### Уточнение

Речь идёт только о local producer-to-MetricShell ingestion. Отправка метрик из MetricShell в Prometheus, Pushgateway
либо central collector остаётся вне scope.

### Кандидаты

- не добавлять local push API;
- local HTTP API;
- local gRPC API.

### Исходная гипотеза

Local HTTP может упростить integration для языков с mature HTTP clients, но способен дублировать socket capabilities и
увеличивать attack surface.

### Критерии оценки

- сложность client implementation;
- versioning protocol;
- performance;
- resource usage;
- риск exposure endpoint;
- удобство debugging;
- дублирование socket transport.

### Статус

Завершено.

---

## INV-009

Shared Memory и mmap adapter

### Вопрос

Могут ли shared memory или mmap предоставить полезный high-performance adapter без небезопасности для клиентов и
platform-specific complexity?

### Кандидаты

- POSIX shared memory;
- anonymous shared mapping, наследуемый child process;
- memory-mapped file;
- shared ring buffer;
- отсутствие shared-memory adapter.

### Исходная гипотеза

Shared memory может обеспечить максимальную raw performance, но не оправдать client complexity, особенно для PHP.

### Эксперименты

- прототип producer на Go;
- прототип через PHP FFI либо extension;
- один producer;
- несколько producers;
- crash процесса во время write;
- overflow ring buffer;
- schema upgrade;
- restart MetricShell;
- enforcement memory limit.

### Критерии оценки

- фактический performance gain относительно socket;
- безопасность клиента;
- стоимость внедрения для PHP;
- сложность synchronization;
- versioning;
- failure recovery;
- переносимость.

### Статус

Не начато.

---

## INV-010

Prometheus exposition

### Вопрос

Какие exposition formats и consistency guarantees должен предоставлять MetricShell?

### Темы

- Prometheus text format;
- OpenMetrics;
- content negotiation;
- HELP и TYPE;
- histograms;
- optional timestamps;
- concurrent scrape;
- partial failure;
- response-size limits;
- slow clients;
- runtime self-metrics.

### Исходная гипотеза

MetricShell должен использовать существующую Prometheus client library либо parser/encoder, а не реализовывать
exposition вручную.

### Эксперименты

Проверить output средствами Prometheus и протестировать concurrent ingestion и scrape, большой registry, malformed
internal state, disconnected scraper, slow scraper и несколько concurrent scrapers.

### Статус

Не начато.

---

## INV-011

Финальное состояние application metrics и подсчёт scrape

### Вопрос

Что становится immutable после завершения workload и когда final scrape считается завершённым?

### Предлагаемое разделение

#### Application metrics

После финализации workload состояние application metrics становится immutable.

#### Self-metrics MetricShell

Runtime self-metrics могут продолжать изменяться, пока MetricShell ожидает и завершается.

### Варианты exit mode

- немедленно;
- фиксированная длительность;
- ожидать один eligible scrape;
- ожидать настроенное количество N eligible scrapes.

### Исходные гипотезы

- default required scrape count должен быть `1`;
- `N > 1` должно быть optional;
- scrape засчитывается только после успешной полной записи response;
- health и readiness requests не засчитываются;
- отдача response не доказывает сохранение данных в TSDB.

### Вопросы исследования

- Засчитывается ли ручной `curl`?
- Нужна ли authentication scraper?
- Считаются ли concurrent scrapes независимо?
- Требуется ли уникальность scraper?
- Засчитывается ли aborted connection?
- Влияют ли изменения runtime self-metrics на identity final state?
- Что происходит после истечения timeout?
- Application metrics замораживаются до или после остановки ingestion?

### Статус

Не начато.

---

## INV-012

Пригодность Kubernetes Job и CronJob

### Вопрос

Может ли Prometheus продолжать обнаруживать и scrape endpoint MetricShell, пока завершённый workload удерживается внутри
всё ещё работающего Job Pod?

### Почему это критично

MetricShell может корректно ждать scrape, но ожидание бесполезно, если target удаляется из Prometheus discovery до final
scrape.

### Эксперименты

Проверить PodMonitor, ServiceMonitor, direct Pod discovery, readiness true и false во время post-exit wait, Job,
CronJob, termination, `activeDeadlineSeconds`, `ttlSecondsAfterFinished` и overlapping schedules.

Измерить длительность target discovery, последний успешный scrape, задержку завершения Job, влияние scheduler overlap и
поведение с двумя экземплярами Prometheus.

### Критерии решения

- final scrape выполняется надёжно;
- в core MetricShell отсутствует Kubernetes-specific code;
- достаточно документированной deployment configuration;
- Job остаётся понятным в эксплуатации.

### Статус

Не начато.

---

## INV-013

Модели распространения

### Вопрос

Как MetricShell должен добавляться в application images?

### Кандидаты

- копирование standalone static binary;
- multi-stage Dockerfile;
- base image MetricShell;
- language-specific convenience images.

### Критерии оценки

- размер image;
- совместимость libc;
- amd64 и arm64;
- запуск не от root;
- version pinning;
- воспроизводимость;
- supply-chain verification;
- свобода приложения устанавливать зависимости.

### Статус

Не начато.

---

## INV-014

Безопасность и resource limits

### Темы

- запуск не от root;
- socket и file permissions;
- binding local push;
- binding metrics endpoint;
- input validation;
- лимиты series и labels;
- payload size;
- concurrent clients;
- slow clients;
- file-descriptor limits;
- memory limits;
- secrets в labels;
- malicious producer behavior.

### Обязательные результаты

- threat model;
- default security posture;
- конфигурация resource limits;
- failure policy;
- security acceptance tests.

### Статус

Не начато.

---

## INV-015

План benchmark

### Назначение

Benchmarks сравнивают кандидатов и подтверждают non-functional requirements. Их нельзя использовать для искусственного
обоснования заранее выбранного решения.

### Эталонная среда

Фиксировать CPU, выделенные cores, RAM, kernel, container runtime, Docker version, Go version, PHP version, image,
CPU/memory limits, host load и commit SHA.

### Workloads

#### B-001 — Idle runtime

Измерить CPU и RSS без updates при обычном scrape interval.

#### B-002 — Ingestion throughput

Для каждого transport проверить 100, 1 000 и 10 000 updates/s, затем повышать нагрузку до saturation.

Измерить accepted/rejected updates, p50/p95/p99 ingestion latency, CPU, RSS, allocations и open descriptors.

#### B-003 — Registry cardinality

Проверить 100, 1 000, 10 000 и, где возможно, 100 000 series.

Измерить memory, scrape size, scrape latency и ingestion latency.

#### B-004 — Concurrent scrape

Проверить 1, 2, 5 и 10 concurrent scrapers.

#### B-005 — File detection

Сравнить polling, inotify и hybrid mode.

#### B-006 — Startup

Измерить инициализацию MetricShell и время до запуска workload.

#### B-007 — Shutdown

Измерить graceful shutdown, forced shutdown, finalization и HTTP drain.

#### B-008 — Ожидание final scrape

Проверить zero scrapes, one scrape, N scrapes, concurrent scrapes, aborted response и timeout.

#### B-009 — Failure injection

Проверить malformed input, transport disconnect, endpoint bind failure, queue overflow и resource exhaustion.

### Правила benchmark

- выполнять warm-up до измерения;
- запускать несколько iterations;
- публиковать median и dispersion;
- сохранять raw results;
- фиксировать environment и commit;
- сравнивать эквивалентные semantic workloads;
- не сравнивать debug builds с optimized builds;
- разделять throughput и end-to-end latency;
- документировать отброшенные runs и причины.

### Статус

Не начато.
