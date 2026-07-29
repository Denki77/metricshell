# Отчёт INV-004 — Владение состоянием метрик и семантика обновлений

[English version](report.md)

**Статус:** завершено  
**Дата прогонов:** 2026-07-23  
**Docker Servers:** 29.4.3, 27.4.0  
**Docker platforms:** `linux/aarch64`, `linux/x86_64`  
**Эталонные прогоны:** `results/20260723T073114Z`, `results/20260723T150118Z`  
**Сводки:** `results/20260723T073114Z/summary.tsv`, `results/20260723T150118Z/summary.tsv`

## Цель

Определить минимально достаточный metric-state contract для MetricShell после отделения transport/runtime
responsibilities от instrumentation и cross-producer aggregation, находящихся вне scope.

## Коррекция scope

Эксперимент намеренно сравнил более широкий набор semantic models, чем требуется продукту. Первоначальная интерпретация
предполагала, что MetricShell может принимать независимые operations, владеть per-producer contributions и агрегировать
их для exposition.

Project scope исключает локальную или распределённую агрегацию значений между producers и business metric design.
Functional requirements требуют consistent acceptance и exposition counters, gauges и histograms, но не требуют
`increment`, `set`, `observe`, producer epochs, sequence recovery или completeness classification. Эти concerns
принадлежат workload и его libraries, которые должны публиковать один полный, не содержащий конфликтов application
snapshot.

Prototype и result files не изменяются. Multi-owner и operation scenarios сохраняются как доказательство сложности,
которую исключает выбранный scope.

## Прототип

Прототип расположен в `research/INV-004`.

В состав прототипа входят:

* `prototype/cmd/inv004` — исполняемая семантическая модель и benchmark производительности и allocations;
* `prototype/Dockerfile` — воспроизводимый Linux build/runtime image, использующий `COPY ["cmd", "./cmd/"]`;
* `run-bench.sh` — полный набор семантических сценариев, scale matrix, repeated runs, assertions и сбор fingerprint
  среды;
* `results/<timestamp>` — TSV evidence и Docker build log.

## Команды запуска

Запуск полного набора исследований:

```bash
./research/INV-004/run-bench.sh
```

Просмотр результатов последнего прогона:

```bash
latest="$(cat research/INV-004/latest-results.txt)"
cat "$latest/summary.tsv"
cat "$latest/assertions.tsv"
cat "$latest/semantics.tsv"
cat "$latest/benchmark-stats.tsv"
cat "$latest/environment.tsv"
cat "$latest/coverage.tsv"
```

Одна и та же команда используется на macOS и Ubuntu.

Для увеличения числа repeated runs:

```bash
INV004_REPEAT_COUNT=100 ./research/INV-004/run-bench.sh
```

## Среды выполнения

| Среда                   | Дата       | Docker | Платформа        | Набор результатов          | Fingerprint                                                        | Результат                |
|-------------------------|------------|-------:|------------------|----------------------------|--------------------------------------------------------------------|--------------------------|
| Docker Desktop на macOS | 2026-07-23 | 29.4.3 | LinuxKit/aarch64 | `results/20260723T073114Z` | `e52784470ff33e35fb58ab142be26a345bb6a373bd2eac58666b269f56875fd6` | 34/34 assertions успешно |
| Ubuntu / LinuxKit       | 2026-07-23 | 27.4.0 | LinuxKit/x86_64  | `results/20260723T150118Z` | `e52784470ff33e35fb58ab142be26a345bb6a373bd2eac58666b269f56875fd6` | 34/34 assertions успешно |

Fingerprint рассчитывается только по исходному коду benchmark и содержимому runner с использованием относительных путей.

Поэтому он:

* не зависит от директории checkout;
* не зависит от repository HEAD;
* не меняется из-за правок документации;
* позволяет подтвердить идентичность запускаемого benchmark-кода между средами.

Environment metadata и идентификаторы image сохраняются отдельно в `environment.tsv`.

Оба прогона сформировали идентичные `semantics.tsv` и `assertions.tsv`.

Все 33 именованных семантических сценария дали ожидаемый результат. Дополнительный assertion проверил точное количество
сценариев. Итого в каждой среде успешно прошли 34 из 34 assertions.

## Результаты

### Семантические сценарии

| Кандидат                   | Recovery после lost update    | Receiver restart   | Несколько producers              | Stale removal        | Scope interpretation                    |
|----------------------------|-------------------------------|--------------------|----------------------------------|----------------------|-----------------------------------------|
| Complete snapshot          | при следующем snapshot        | republish          | workload разрешает до публикации | да                   | выбранная minimum sufficient model      |
| Per-series absolute values | только при повторной отправке | partial republish  | неоднозначный last-writer-wins   | нет границы registry | недостаточный partial-state contract    |
| Operations only            | нет                           | нет                | counter increments коммутативны  | нет                  | вне scope и недостаточный durable truth |
| Hybrid                     | при reconciliation            | при reconciliation | owner-scoped                     | да                   | валидный aggregator design вне scope    |

Четыре строки в `semantics.tsv` намеренно имеют `result=fail`.

Это не ошибки runner, а falsifying counterexamples, демонстрирующие некорректность отдельных моделей:

* уменьшение абсолютного значения counter;
* конфликт абсолютных значений одного counter от нескольких producers;
* потеря operation;
* потеря состояния после restart receiver.

`assertions.tsv` проверяет каждый именованный сценарий отдельно и дополнительно проверяет точное количество сценариев.
Pass/fail больше не определяется только агрегированным числом успешных и неуспешных строк.

### Экспериментальная граница владения producer

Для экспериментального operation path используется полная граница:

```text
(producer_id, producer_epoch, sequence)
```

Эта граница является необходимым условием контрфактической operation/hybrid model, а не допустимым набором полей
production protocol MetricShell.

Правила:

* `producer_id` стабильно идентифицирует владельца состояния;
* `producer_epoch` изменяется при restart producer;
* `sequence` строго возрастает внутри пары `(producer_id, producer_epoch)`;
* старый epoch отклоняется;
* duplicate и older sequence не применяются;
* новый epoch, впервые обнаруженный через operation, помечается как incomplete и non-authoritative;
* operations нового epoch не могут изменять metric values, пока не принят его initial complete snapshot;
* initial authoritative snapshot открывает новое sequence space;
* sequence gap помечает owner как incomplete;
* последующий authoritative snapshot устраняет incomplete state.

### Семантика histograms

Histogram не моделируется скалярным значением.

Authoritative histogram state содержит:

* полный набор `bounds`;
* cumulative `buckets`;
* `count`;
* `sum`.

Проверяются следующие invariants:

* совместимость bucket boundaries;
* component-wise aggregation;
* `count` равен последнему cumulative bucket;
* cumulative buckets не уменьшаются внутри одного producer epoch;
* `count` не уменьшается внутри epoch;
* bucket schema не меняется внутри epoch;
* новый producer epoch может начинаться с reset histogram;
* несовместимые histogram schemas не агрегируются.

### Производительность representative cases

| Кандидат                         | Workload                                   | Throughput                                                 | Allocation/update |
|----------------------------------|--------------------------------------------|------------------------------------------------------------|------------------:|
| Snapshot                         | 1 producer, 100 series, 100 snapshots      | 15 500 snapshots/s в одном scale point; p50 — 28 108/s     |      25 890 B p50 |
| Operation fast path в hybrid     | 1 producer, 100 series, 100 000 operations | 3,91 млн ops/s в scale point; p50 — 4,48 млн ops/s         |           6 B p50 |
| Hybrid amortized, interval 1 000 | 1 producer, 100 series, 100 000 operations | 4,14 млн updates/s в scale point; p50 — 4,31 млн updates/s |          19 B p50 |

На максимальном tested scale point — 16 producers и 10 000 series:

* snapshots обрабатывали 249 complete updates/s и выделяли около 3,82 MB на update;
* operation fast path обрабатывал 4,16 млн updates/s при 34 B/update;
* фактический hybrid с reconciliation каждые 1 000 operations обрабатывал около 0,335 млн updates/s при 1 788 B/update;
* около 83,1% измеренного времени hybrid тратил на reconciliation.

Для 4 producers и 1 000 series:

| Reconciliation interval | Throughput         | Доля времени reconciliation |
|------------------------:|--------------------|----------------------------:|
|          100 operations | 0,48 млн updates/s |                       85,6% |
|        1 000 operations | 2,30 млн updates/s |                       35,7% |
|       10 000 operations | 2,05 млн updates/s |                        5,9% |

Это отдельные sensitivity observations.

Немонотонный результат для interval `10 000` показывает влияние host noise и не должен интерпретироваться как устойчивый
performance ordering.

Hybrid измеряется как amortized model.

### Cross-environment benchmark statistics

| Кандидат                         | Среда                  | Повторов | p50 updates/s | p95 updates/s | p99 updates/s | p50 bytes/update |
|----------------------------------|------------------------|---------:|--------------:|--------------:|--------------:|-----------------:|
| Snapshot                         | macOS/LinuxKit aarch64 |       30 |        28 108 |        36 507 |        37 232 |           25 890 |
| Snapshot                         | Ubuntu/LinuxKit x86_64 |       30 |         6 402 |        16 391 |        20 877 |           25 892 |
| Operation fast path              | macOS/LinuxKit aarch64 |       30 |     4 482 102 |     5 325 475 |     5 397 698 |                6 |
| Operation fast path              | Ubuntu/LinuxKit x86_64 |       30 |     2 208 350 |     2 987 775 |     3 274 232 |                6 |
| Hybrid amortized, interval 1 000 | macOS/LinuxKit aarch64 |       30 |     4 314 971 |     4 991 317 |     5 044 581 |               19 |
| Hybrid amortized, interval 1 000 | Ubuntu/LinuxKit x86_64 |       30 |     1 970 287 |     2 391 584 |     2 954 448 |               19 |

### Scale и sensitivity cases

| Сценарий                                         | macOS/LinuxKit                            | Ubuntu/LinuxKit                           |
|--------------------------------------------------|-------------------------------------------|-------------------------------------------|
| Snapshot, 16 producers / 10k series              | 249 updates/s; 3 822 541 B/update         | 216 updates/s; 3 822 694 B/update         |
| Operation fast path, 16 producers / 10k series   | 4,16 млн updates/s; 34 B/update           | 1,81 млн updates/s; 34 B/update           |
| Hybrid interval 1 000, 16 producers / 10k series | 0,335 млн updates/s; 83,1% reconciliation | 0,331 млн updates/s; 85,3% reconciliation |
| Hybrid interval 100, 4 producers / 1k series     | 0,476 млн updates/s; 85,6% reconciliation | 0,331 млн updates/s; 84,7% reconciliation |
| Hybrid interval 1 000, 4 producers / 1k series   | 2,30 млн updates/s; 35,7% reconciliation  | 1,15 млн updates/s; 34,5% reconciliation  |
| Hybrid interval 10 000, 4 producers / 1k series  | 2,05 млн updates/s; 5,9% reconciliation   | 2,10 млн updates/s; 7,2% reconciliation   |

Signal-to-exit latency не является метрикой INV-004.

Прототип INV-004 не запускает workload и не отправляет signals, поэтому не формирует signal-to-exit evidence. Эти
измерения относятся к INV-001 и INV-002.

Для INV-004 релевантны:

* throughput;
* allocation;
* стоимость reconciliation;
* корректность semantic state.

## Оценка по критериям

| Критерий                        | Snapshot                    | Absolute values          | Operations                                | Hybrid                                 |
|---------------------------------|-----------------------------|--------------------------|-------------------------------------------|----------------------------------------|
| Корректность после loss         | после следующего snapshot   | только для resent series | нет                                       | после reconciliation                   |
| Recovery после receiver restart | republish                   | partial republish        | нет                                       | republish                              |
| Client complexity               | средняя                     | низкая                   | средняя                                   | максимальная                           |
| Protocol complexity             | средняя                     | низкая                   | средняя                                   | максимальная                           |
| Throughput                      | зависит от размера registry | ожидаемо высокий         | высокий                                   | быстрый fast path + стоимость snapshot |
| Memory/network amplification    | максимальная                | низкая                   | низкая                                    | настраиваемая                          |
| Несколько producers             | owner-scoped                | неоднозначно             | безопасно для совместимых commutative ops | owner-scoped                           |

## Допустимые значения и protocol constraints

Принятая production boundary — один полный application snapshot.

* MetricShell структурно валидирует candidate целиком до state mutation.
* Валидный candidate атомарно заменяет предыдущий валидный snapshot.
* Корректно закодированный zero-series snapshot очищает application series; отсутствующий или пустой payload malformed.
* Missing series удаляются через отсутствие в новом принятом snapshot.
* Duplicate series, type conflicts и внутренние metadata conflicts candidate отклоняют candidate целиком.
* Установленная привязка name-to-type family сохраняется после omission до следующего workload execution.
* Прочая metadata проверяется на внутреннюю согласованность в candidate и не фиксируется на весь execution.
* Classic histogram требует ordered cumulative buckets, `+Inf`, равный `count`, и numeric `sum`.
* Counters и histograms не валидируются по business monotonicity между snapshots.
* MetricShell не принимает instrumentation operations и не агрегирует значения между producers.
* Success acknowledgement socket/HTTP подтверждает atomic installation в одном linear acceptance order; producer
  timestamps не переупорядочивают candidates.
* Final application state — последний валидный принятый полный snapshot либо первоначальный zero-series snapshot.

`producer_id`, `producer_epoch`, `sequence`, gaps, completeness state и reconciliation intervals относятся только к
отклонённой operation/hybrid aggregator model.

## Дополнительные benchmarks и coverage

Runner выполняет полный первоначальный набор semantic и synthetic benchmarks:

* все модели-кандидаты;
* scale matrix;
* producer matrix;
* 30-run distributions;
* allocation measurements;
* потерянные updates;
* reordered updates;
* duplicate updates;
* producer restart;
* receiver restart;
* stale deletion;
* type conflicts;
* duplicate ownership;
* counter semantics;
* gauge semantics;
* полную histogram semantics;
* explicit gap state;
* transactional conflict rejection;
* переходы producer epoch;
* snapshot monotonicity;
* component-wise histogram aggregation;
* gauge/type/histogram ownership conflicts;
* hybrid reconciliation intervals `100`, `1 000` и `10 000`.

`semantics.tsv` и `assertions.tsv` содержат отдельные проверяемые contracts.

`coverage.tsv` содержит high-level coverage groups.

Этот набор намеренно шире текущего product scope. Сохранение broad runner оставляет falsifying и complexity evidence,
не превращая исследованные механизмы в implementation requirements.

Для более надёжного performance sizing рекомендуется:

* использовать тихий native Linux host;
* выполнить 100 repetitions;
* зафиксировать CPU и memory limits;
* добавить реальные transport encodings;
* отдельно измерить scaling числа histogram buckets;
* добавить crash-safe persistence;
* проверить disk-full;
* проверить hostile cardinality;
* измерить serialization, syscalls, network и filesystem overhead.

Эти аспекты не являются скрытыми допущениями INV-004. Они намеренно перенесены в INV-005–INV-009, поскольку
transport-free semantic model не может измерить их корректно.

## Ограничения

* Используется research-only in-memory model на Go.
* Production protocol отсутствует.
* Durable storage отсутствует.
* Обе измеренные среды используют LinuxKit:

  * macOS/LinuxKit `linux/aarch64`;
  * Ubuntu/LinuxKit `linux/x86_64`.
* Native non-LinuxKit Linux не проверен.
* containerd и CRI-O не проверены.
* Kubernetes не проверен.
* В обоих `environment.tsv` поле `container_go_version` содержит help banner прототипа вместо версии Go toolchain.
* Некорректное информационное поле `container_go_version` не используется в correctness или performance comparison.
* Идентичность исходного кода подтверждается matching benchmark fingerprint.
* Docker image tags в будущем могут указывать на другие base image digests.
* Для точного повторения следует сравнивать fingerprint и image provenance либо сохранять построенный image.
* Synthetic throughput изолирует структуры данных ownership model.
* Измерения не включают serialization.
* Измерения не включают transport syscalls.
* Измерения не включают network или filesystem transport.

## Вывод

Matching-fingerprint evidence поддерживает более узкий вывод, чем первоначальный report. Reconciliation требуется, если
MetricShell принимает изменяющие состояние operations. Producer ownership и aggregation policies требуются, если
MetricShell объединяет независимо принадлежащие registries. Обе возможности находятся вне scope MetricShell.

Выбирается один полный, не содержащий конфликтов application snapshot для file, Unix stream и local HTTP. Каждый
transport выполняет одну application operation: structural validation и atomic replacement последнего валидного
snapshot. MetricShell сохраняет Prometheus types, но не применяет instrumentation operations, не проверяет business
monotonicity между snapshots и не агрегирует конфликтующие producer values.

Решение зафиксировано в [ADR-004](../../docs-ru/06-architecture/adr/ADR-004.md).
