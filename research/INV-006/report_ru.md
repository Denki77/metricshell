# Отчёт INV-006 — Приём метрик через файл

[English version](report.md)

**Статус:** завершено  
**Дата прогона:** 2026-07-23  
**Docker Servers:** 29.4.3, 27.4.0  
**Docker platforms:** LinuxKit 6.12.76 `linux/aarch64`, LinuxKit 6.10.14 `linux/x86_64`  
**Эталонные прогоны:** `results/20260723T160155Z`, `results/20260723T161216Z`  
**Сводки:** `results/20260723T160155Z/summary.tsv`, `results/20260723T161216Z/summary.tsv`

## Цель

Проверить, позволяет ли directory-level inotify в сочетании с низкочастотным periodic reconciliation безопасно
обнаруживать изменения container-local файла с полным состоянием метрик, обеспечивая:

* меньшую latency по сравнению с polling;
* приемлемый idle overhead;
* восстановление после потерянных событий;
* восстановление после queue overflow;
* восстановление после invalidation watches;
* сходимость к последнему валидному полному состоянию.

## Прототип

Прототип расположен в `research/INV-006`.

В его состав входят:

* `prototype/cmd/inv006-bench` — Linux watcher, producer, correctness scenarios и инструменты измерений;
* `prototype/Dockerfile` — multi-stage Linux image;
* `run-bench.sh` — единый matrix runner для macOS и Ubuntu и сбор fingerprint;
* `results/<timestamp>` — raw per-case TSV и агрегированные evidence.

Реализация:

* подписывается на directory, содержащую snapshot;
* читает complete file snapshots;
* валидирует snapshot до замены last valid state;
* переустанавливает invalidated watches;
* выполняет reconciliation по фиксированному interval.

Runner не использует host bind mounts.

Каждый case записывает evidence внутри контейнера, после чего host извлекает его через `docker cp`.

Это исключает различия:

* Docker Desktop file sharing;
* host path handling;
* mount policy Docker daemon на Ubuntu.

## Команды запуска

```bash
./research/INV-006/run-bench.sh
```

Просмотр основных результатов:

```bash
cat research/INV-006/latest-results.txt
cat "$(cat research/INV-006/latest-results.txt)/summary.tsv"
cat "$(cat research/INV-006/latest-results.txt)/environment.tsv"
cat "$(cat research/INV-006/latest-results.txt)/correctness.tsv"
cat "$(cat research/INV-006/latest-results.txt)/performance.tsv"
cat "$(cat research/INV-006/latest-results.txt)/burst.tsv"
cat "$(cat research/INV-006/latest-results.txt)/overflow.tsv"
cat "$(cat research/INV-006/latest-results.txt)/lost-event.tsv"
cat "$(cat research/INV-006/latest-results.txt)/idle.tsv"
```

Ubuntu использует ту же команду.

Корректное cross-environment comparison требует одинакового:

```text
benchmark_code_fingerprint_sha256
```

## Среды выполнения

| Среда                    | Дата       | Docker | Kernel           | Architecture | Набор результатов          | Fingerprint                                                        |
|--------------------------|------------|-------:|------------------|--------------|----------------------------|--------------------------------------------------------------------|
| Docker Desktop на macOS  | 2026-07-23 | 29.4.3 | LinuxKit 6.12.76 | aarch64      | `results/20260723T160155Z` | `4238d6e1be961e2d864ccfc95c763c1c9a03365c74d9c868e38f1b0d96eb1580` |
| Docker Desktop на Ubuntu | 2026-07-23 | 27.4.0 | LinuxKit 6.10.14 | x86_64       | `results/20260723T161216Z` | `4238d6e1be961e2d864ccfc95c763c1c9a03365c74d9c868e38f1b0d96eb1580` |

Fingerprints идентичны.

Repository HEAD и architecture-specific image IDs различаются, что ожидаемо. Fingerprint напрямую рассчитывается
по содержимому prototype и runner.

Дополнительная metadata:

* macOS: `benchmark_scope_diff_clean=false`;
* Ubuntu: `benchmark_scope_diff_clean=true`;
* обе среды: `benchmark_scope_untracked_count=0`.

Обе container environments используют LinuxKit.

Совпадающие результаты на aarch64 и x86_64 подтверждают cross-architecture behavior внутри этих container environments,
но не подтверждают поведение native Linux без LinuxKit.

## Результаты

### Correctness

В каждой среде прошли все 162 correctness assertions.

Проверки выполнялись для:

* writable container layer;
* container-local tmpfs;
* Docker named volume;
* polling;
* inotify-only;
* hybrid inotify с reconciliation.

Проверенные scenarios:

* файл уже существует при startup;
* файл отсутствует при startup и появляется позже;
* atomic rename;
* 100 последовательных replacements;
* writer crash с оставшимся temporary file;
* invalid input;
* deletion snapshot;
* recreation directory;
* restart MetricShell.

Invalid, partial и deleted inputs сохраняли last valid state.

После recreation directory и restart MetricShell watcher сходился к текущему валидному snapshot в обеих средах.

### Cross-environment confirmation

| Метрика                                             | macOS/LinuxKit aarch64 | Ubuntu/LinuxKit x86_64 |
|-----------------------------------------------------|-----------------------:|-----------------------:|
| Correctness assertions                              |                162/162 |                162/162 |
| Controlled lost-event A/B assertions                |                    6/6 |                    6/6 |
| Сходимость burst из 10 000 updates                  |                  12/12 |                  12/12 |
| Сходимость после overflow pressure                  |                    6/6 |                    6/6 |
| Наблюдения реального overflow                       |                    4/6 |                    4/6 |
| Performance rows с пропущенной intermediate version |                      1 |                      1 |
| Hybrid performance rows с пропуском                 |                      0 |                      0 |

Все portable assertions прошли в обеих средах.

Факт возникновения реального overflow и количество пропущенных intermediate complete-state versions являются
наблюдениями, а не portable pass/fail criteria.

### Detection latency

В таблице приведено среднее значение соответствующего percentile по трём repetitions для файлов размером 4 KiB.

| Среда  | Filesystem | Strategy    |       p50 |        p95 |        p99 |
|--------|------------|-------------|----------:|-----------:|-----------:|
| macOS  | Layer      | poll 10 ms  |  9,974 ms |  11,670 ms |  11,926 ms |
| macOS  | Layer      | poll 100 ms | 99,915 ms | 103,560 ms | 103,859 ms |
| macOS  | Layer      | inotify     |  0,231 ms |   0,879 ms |   1,016 ms |
| macOS  | Layer      | hybrid 1 s  |  0,265 ms |   1,001 ms |   2,614 ms |
| macOS  | tmpfs      | inotify     |  0,109 ms |   0,604 ms |   0,758 ms |
| macOS  | tmpfs      | hybrid 1 s  |  0,109 ms |   0,707 ms |   0,775 ms |
| macOS  | Volume     | inotify     |  0,169 ms |   0,608 ms |   0,747 ms |
| macOS  | Volume     | hybrid 1 s  |  0,171 ms |   0,646 ms |   0,846 ms |
| Ubuntu | Layer      | poll 10 ms  |  9,966 ms |  10,810 ms |  11,098 ms |
| Ubuntu | Layer      | poll 100 ms | 99,972 ms | 100,778 ms | 101,183 ms |
| Ubuntu | Layer      | inotify     |  0,745 ms |   1,202 ms |   1,334 ms |
| Ubuntu | Layer      | hybrid 1 s  |  0,479 ms |   0,785 ms |   1,240 ms |
| Ubuntu | tmpfs      | inotify     |  0,466 ms |   0,896 ms |   1,044 ms |
| Ubuntu | tmpfs      | hybrid 1 s  |  0,479 ms |   1,083 ms |   1,190 ms |
| Ubuntu | Volume     | inotify     |  0,378 ms |   0,927 ms |   0,990 ms |
| Ubuntu | Volume     | hybrid 1 s  |  0,467 ms |   0,843 ms |   1,202 ms |

Обычная detection latency определяется event path.

Изменение hybrid reconciliation interval с `100 ms` до `1 s` не привело к существенному ухудшению нормальной
event-driven detection latency.

Во всей paced matrix:

* на macOS один `volume/inotify-only` case с файлом 1 MiB пропустил одну intermediate version;
* на Ubuntu один `volume/inotify-only` case с файлом 4 KiB пропустил одну intermediate version;
* ни один hybrid performance row не пропустил update в обеих средах.

Это соответствует complete-state contract:

* intermediate versions могут coalesce;
* наблюдение каждой версии не гарантируется;
* watcher должен сходиться к latest complete version.

INV-006 не формирует signal-to-exit statistics, поскольку прототип не supervises workload и не отправляет ему signals.

Релевантная latency для INV-006 — время от producer timestamp до обнаружения file update.

### Idle overhead

| Среда  | Filesystem | Poll 10 ms | Poll 100 ms | Poll 1 s | inotify | Hybrid 100 ms | Hybrid 1 s |
|--------|------------|-----------:|------------:|---------:|--------:|--------------:|-----------:|
| macOS  | Layer      |     2,969% |      0,480% |   0,060% |  0,189% |        0,427% |     0,272% |
| macOS  | tmpfs      |     3,014% |      0,481% |   0,358% |  0,231% |        0,433% |     0,418% |
| macOS  | Volume     |     2,593% |      0,361% |   0,087% |  0,245% |        0,553% |     0,286% |
| Ubuntu | Layer      |     4,780% |      0,568% |   0,075% |  0,364% |        0,774% |     0,438% |
| Ubuntu | tmpfs      |     5,085% |      0,490% |   0,077% |  0,359% |        0,745% |     0,421% |
| Ubuntu | Volume     |     4,496% |      0,454% |   0,077% |  0,359% |        0,817% |     0,402% |

Пятисекундные process CPU samples показывают ожидаемый trade-off:

* медленный polling потребляет меньше ресурсов, но медленно обнаруживает изменения;
* polling каждые 10 ms заметно увеличивает idle CPU;
* hybrid с reconciliation раз в 1 s сохраняет event-driven latency;
* idle CPU hybrid 1 s составил `0,272–0,418%` на macOS/LinuxKit;
* idle CPU hybrid 1 s составил `0,402–0,438%` на Ubuntu/LinuxKit.

Это короткие container-process measurements, а не production resource guarantees.

### Burst и overflow recovery

Все 12 bursts по 10 000 updates сошлись к final complete-state version в каждой среде.

При forced reader pause и 20 000 atomic replacements:

* все шесть cases восстановили final version в каждой среде;
* kernel сообщил `IN_Q_OVERFLOW` в четырёх cases из шести в каждой среде;
* два остальных cases завершились без зарегистрированного overflow.

Overflow event не является failure.

Он означает, что история событий неполна и watcher должен немедленно выполнить full reconciliation.

Отсутствие фактического overflow также не является failure, поскольку его возникновение зависит от:

* kernel scheduling;
* скорости filesystem;
* скорости producer и reader.

Для более строгого local stress criterion можно использовать:

```bash
INV006_REQUIRE_REAL_OVERFLOW=1 ./research/INV-006/run-bench.sh
```

В этом режиме требуется хотя бы один фактический overflow для каждой inotify strategy.

Portable assertion — сходимость к final state во всех pressure cases.

### Controlled lost-event boundary

Все шесть A/B assertions прошли в обеих средах.

| Filesystem | inotify-only после потерянного события | Hybrid после потерянного события |
|------------|----------------------------------------|----------------------------------|
| Layer      | не восстановился                       | восстановился                    |
| tmpfs      | не восстановился                       | восстановился                    |
| Volume     | не восстановился                       | восстановился                    |

Watcher намеренно прочитал одно update event, но не выполнил reconciliation.

После этого новых events не поступало.

Результат:

* inotify-only не имел механизма обнаружения текущего snapshot;
* hybrid обнаружил snapshot по reconciliation timer `250 ms`;
* результат повторился на всех трёх filesystems;
* результат повторился в обеих environments.

Это прямое экспериментальное доказательство необходимости periodic reconciliation, а не только теоретический
failure-model analysis.

### Размер файла

Matrix включала:

* 128 B;
* 4 KiB;
* 1 MiB.

Event detection оставался быстрым.

Однако active tests для 1 MiB становились hashing/I/O bound и регулярно потребляли примерно одно CPU core во время
создания и валидации updates.

Это подтверждает feasibility, но не устанавливает:

* production maximum file size;
* production maximum series count;
* production parser limits.

## Оценка гипотез

### Directory-level inotify обеспечивает быструю normal detection

Подтверждено.

В обеих средах p95 для hybrid/inotify на файлах 4 KiB находился в диапазоне `0,604–1,202 ms`.

Для сравнения:

* polling 10 ms: `10,709–11,786 ms`;
* polling 100 ms: `100,778–103,679 ms`.

### Reconciliation необходим для корректности

Подтверждено:

* controlled A/B test;
* failure-model analysis;
* recovery tests;
* наблюдения queue overflow;
* invalidation watches;
* coalescing и loss events.

Inotify-only не восстановился после контролируемой потери события.

Hybrid восстановился по periodic reconciliation interval.

### Reconciliation interval 1 s приемлем

Подтверждено.

Он:

* ограничивает recovery после потерянного event примерно одним interval;
* не ухудшает обычную event-driven latency;
* уменьшает steady-state reads и CPU по сравнению с interval `100 ms`.

Interval `100 ms` увеличивал idle reads и CPU, но не давал существенного улучшения normal detection latency.

### Container-local filesystems ведут себя согласованно

Подтверждено для:

* writable container layer;
* container-local tmpfs;
* Docker named volume.

Результаты подтверждены в обеих LinuxKit environments.

Host bind mounts исключены из supported contract.

## Допустимые значения и политики

* Watcher подписывается на directory, а не только на inode файла.
* После relevant event выполняется immediate reconciliation.
* Periodic reconciliation обязателен.
* Default reconciliation interval: `1s`.
* Протестированный допустимый диапазон: `100ms–1s`.
* File представляет complete registry state.
* File не является append-only event log.
* Producer записывает temporary file в той же directory.
* Producer выполняет atomic rename temporary file поверх snapshot.
* Snapshot полностью parses и validates до замены active state.
* Invalid input сохраняет last valid state.
* Partial input сохраняет last valid state.
* Deleted snapshot сохраняет last valid state.
* При watch invalidation выполняются reinstall и immediate reconcile.
* При `IN_Q_OVERFLOW` выполняются immediate full reconcile и reinstall watches при необходимости.
* Протестированный диапазон file size: `128 B–1 MiB`.
* Production maximum определяется отдельно.
* Intermediate versions могут coalesce.
* Гарантируется final-state convergence, а не наблюдение каждой intermediate version.
* Primary storage:

  * writable container layer;
  * container-local tmpfs.
* Docker named volume рассматривается как informational tested option.
* Host bind mounts не поддерживаются основным contract.

## Ограничения прототипа

* Обе evidence environments используют LinuxKit.
* Native non-LinuxKit Linux не проверен.
* Synthetic file format не моделирует production parser.
* SHA-256 validation не моделирует стоимость production series registry.
* Active CPU включает writer, filesystem и hashing в одном процессе.
* Percentiles используют:

  * 30 updates на repetition для небольших файлов;
  * 5 updates на repetition для файлов 1 MiB.
* Пятисекундные idle samples чувствительны к scheduler noise.
* Фактическое возникновение overflow зависит от scheduling и filesystem speed.
* Тест использует atomic rename, но не использует fsync.
* Power-loss durability не подтверждена.
* Host bind mounts не тестировались.
* Native Linux не тестировался.
* Kubernetes `emptyDir` не тестировался.
* Network filesystem не тестировалась.

## Дополнительные benchmarks

| Benchmark item                           | Статус                                    | Evidence                                         |
|------------------------------------------|-------------------------------------------|--------------------------------------------------|
| Обязательные correctness cases           | Покрыто, 162/162 в обеих средах           | `correctness.tsv`                                |
| Layer, tmpfs, named volume               | Покрыто                                   | aggregate TSV                                    |
| Polling 10/100/1000 ms                   | Покрыто                                   | `correctness.tsv`, `performance.tsv`, `idle.tsv` |
| Hybrid reconcile 100/1000 ms             | Покрыто                                   | те же файлы                                      |
| Files 128 B, 4 KiB, 1 MiB                | Покрыто                                   | `performance.tsv`                                |
| Три repetitions и percentiles            | Покрыто                                   | `performance.tsv`                                |
| Bursts 10 000 updates                    | Покрыто, 12/12 convergence в обеих средах | `burst.tsv`                                      |
| Controlled lost-event A/B                | Покрыто, 6/6 в обеих средах               | `lost-event.tsv`                                 |
| Реальный queue overflow                  | Наблюдался в 4/6 cases в обеих средах     | `overflow.tsv`                                   |
| Final recovery после overflow pressure   | Покрыто, 6/6 convergence в обеих средах   | `overflow.tsv`                                   |
| Idle CPU                                 | Покрыто                                   | `idle.tsv`                                       |
| Environment и fingerprint                | Покрыто                                   | `environment.tsv`                                |
| Ubuntu matching-fingerprint confirmation | Покрыто                                   | `results/20260723T161216Z`                       |
| Native Linux без LinuxKit                | Не выполнено                              | требуется отдельная среда                        |
| Kubernetes emptyDir                      | Не выполнено                              | требуется отдельный integration run              |
| fsync crash durability                   | Не выполнено                              | отдельное persistence requirement                |

Рекомендуемые follow-up исследования:

* 30–100 repetitions на выделенном idle runner;
* realistic parser и cardinality payloads после выбора file format;
* cgroup CPU и RSS sampling в течение 5–15 минут steady state;
* большие files и series counts для выбора production limits;
* native Linux;
* Kubernetes `emptyDir` на disk и Memory;
* отдельное исследование fsync/fdatasync durability, если crash persistence станет требованием.

## Вывод

Гипотеза подтверждена matching-fingerprint прогонами Docker Desktop/LinuxKit на macOS aarch64 и Ubuntu x86_64.

Directory-level inotify в сочетании с reconciliation раз в `1s` обеспечивает выбранный баланс между:

* normal detection latency;
* idle overhead;
* bounded recovery после потерянных events;
* recovery после overflow;
* recovery после invalidated watches.

Все portable assertions прошли в обеих средах.

Timing distributions различались, но следующие свойства оставались согласованными:

* correctness;
* recovery behavior;
* ranking кандидатов;
* необходимость periodic reconciliation;
* преимущество hybrid над inotify-only для bounded recovery.

Решение зафиксировано в [ADR-006](../../docs-ru/06-architecture/adr/ADR-006.md).
