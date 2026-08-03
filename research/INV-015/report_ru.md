# Отчёт INV-015 — Benchmarks и итоговое сравнение

**Статус:** завершено
**Даты прогонов:** 2026-08-02–2026-08-03
**Эталонные прогоны:** `results/20260802T180839Z`, `results/20260803T071831Z`
**Fingerprint:** `ffd49f1d5a23d5484a270329c6610d99196480228f4b84ad6b1aac10bbd9a807`
**Решение:** [ADR-015](../../docs-ru/06-architecture/adr/ADR-015.md)

## Цель и семантическая граница

Измерить повторяемые costs и выполнить correctness/failure matrix выбранной immutable complete-snapshot model.
Publication означает validation или generation одного полного candidate и atomic replacement active state. Operation
counts benchmark не являются increments application metrics; snapshot никогда не суммируются.

## Метод

Container выполняет warm-up и десять iterations для ingestion, cardinality, concurrent scrape, file detection и
initialization. Runner агрегирует p50/p95/p99, затем выполняет real HTTP/container startup, idle, replacement,
cardinality, concurrency, queue, drain, final-scrape, abort, timeout, bind и OOM scenarios. Raw и summarized data
хранятся отдельно от pass/fail assertions; timing observations не используются как assertion thresholds.

## Среды прогонов

| Среда                             | Дата       | Docker | Архитектура | Результат                  | Статус                |
|-----------------------------------|------------|-------:|-------------|----------------------------|-----------------------|
| Docker Desktop на macOS/LinuxKit  | 2026-08-02 | 29.6.2 | aarch64     | `results/20260802T180839Z` | 23/23 assertions pass |
| Docker Desktop на Ubuntu/LinuxKit | 2026-08-03 | 27.4.0 | x86_64      | `results/20260803T071831Z` | 23/23 assertions pass |

Оба прогона имеют одинаковый fingerprint; все 23 assertions прошли в каждой среде. Обе container-среды используют
LinuxKit. Это cross-architecture LinuxKit comparison, а не performance certification native Linux без LinuxKit.

## Результаты

### Cardinality и exposition

|  Series | Encoded bytes | macOS encode p50/p95 | Ubuntu encode p50/p95 | macOS HTTP scrape | Ubuntu HTTP scrape |
|--------:|--------------:|---------------------:|----------------------:|------------------:|-------------------:|
|     100 |         5 080 |     27,166/39,458 µs |      45,236/67,709 µs |         20,927 ms |          12,348 ms |
|   1 000 |        52 780 |   297,750/315,000 µs |    536,000/631,374 µs |         22,658 ms |          12,901 ms |
|  10 000 |       547 780 |       3,426/4,229 ms |        4,390/6,081 ms |         25,251 ms |          18,131 ms |
| 100 000 |     5 677 780 |     42,053/43,892 ms |      47,359/57,563 ms |         73,183 ms |          67,106 ms |

HTTP response добавляет небольшой prototype self-metric к encoded application body. Encoding cost масштабируется
прежде всего с cardinality. Architecture и host scheduling меняют timings, но не response sizes или correctness.

### Complete-snapshot ingestion

| Target rate | Accepted за 100 ms | macOS achieved p50 | Ubuntu achieved p50 | macOS latency p50/p95 | Ubuntu latency p50/p95 |
|------------:|-------------------:|-------------------:|--------------------:|----------------------:|-----------------------:|
|       100/s |                 10 |          111,070/s |           111,070/s |      26,750/32,875 µs |       28,522/59,661 µs |
|     1 000/s |                100 |        1 009,859/s |         1 009,812/s |      23,375/30,708 µs |       26,535/62,012 µs |
|    10 000/s |              1 000 |       10 007,639/s |        10 007,259/s |      23,084/30,541 µs |       26,431/63,583 µs |

Synthetic replacements со 100 series характеризуют in-process selected architecture. Они не измеряют production
transport и не разрешают cumulative metric semantics.

### Concurrent scrape

| Concurrent readers | macOS wall p50 | Ubuntu wall p50 |
|-------------------:|---------------:|----------------:|
|                  1 |       0,916 µs |        1,032 µs |
|                  2 |       0,625 µs |        1,761 µs |
|                  5 |       1,375 µs |        2,391 µs |
|                 10 |       1,875 µs |        4,977 µs |

In-process shape изолирует immutable pointer/body access. External ten-client HTTP wall observation составил
321,664 ms на macOS и 503,644 ms на Ubuntu. Concurrent publication сохранила один exact generation.

### File detection

| Mode    |    macOS p50/p95 |     Ubuntu p50/p95 |
|---------|-----------------:|-------------------:|
| polling |   1,311/1,363 ms | 237,989/299,719 µs |
| inotify | 54,541/90,750 µs | 277,413/479,544 µs |
| hybrid  | 36,167/52,958 µs | 275,151/487,869 µs |

Inotify/hybrid существенно быстрее polling с периодом 1 ms на macOS/LinuxKit. Ubuntu/LinuxKit показал меньшую polling
latency в этой короткой выборке. Переносимый вывод — event-driven notification с periodic reconciliation, а не
универсальный ranking latency.

### Startup и idle

В каждой среде сохранено десять startup и десять idle samples. Readiness macOS находилась в диапазоне
194,877–514,710 ms, Ubuntu — 2 345,439–3 610,678 ms. Idle CPU равнялся 0,00% во всех samples. Memory оставалась примерно
2,93–3,23 MiB на macOS и 2,93–2,97 MiB на Ubuntu; open descriptors оставались равны 6.

### Drain, final scrape и failures

Docker stop дождался in-flight response длительностью 800 ms. Drain/stop занял 1,364 s на macOS и 1,464 s на Ubuntu.
Queue saturation дала явные 429. Malformed candidates вернули 400 с last-valid retention. Final-scrape cases
подтвердили, что health requests не считаются, один complete response освобождает wait, aborted large response не
считается, последующий complete response считается, а timeout завершается с нулём fabricated scrapes. Invalid bind дал
internal exit 70. Allocation 128 MiB при limit 32 MiB завершилась OOMKilled/137.

## Оценка

- Immutable pointer replacement сохранил single-generation scrape consistency в обеих средах.
- Cost complete-snapshot encoding и exposition масштабируется с cardinality и требует явных bounds.
- File transport должен использовать event notification с reconciliation; measured latency ordering зависит от среды.
- Backpressure, timeout, bind и cgroup failures observable и bounded.
- Final response считается после complete successful write, но это не доказывает TSDB persistence.
- Различия timings являются operational observations, а не correctness differences.

## Принятые архитектурные значения

- Сохранить atomic immutable complete-snapshot replacement.
- Сохранить payload, cardinality, concurrency, timeout и memory bounds из INV-014.
- Для Linux file transport использовать inotify с periodic polling/reconciliation fallback.
- Pre-encode и ограничивать крупные exposition responses до commit success.
- Использовать один eligible complete-response final scrape плюс finite timeout как default из INV-011.
- Публиковать явные self-metrics для malformed input, queue rejection, bind failure и resource exhaustion.
- Рассматривать 100k series как верхнюю research shape, а не default или SLO.
- До публикации performance targets требовать controlled release benchmarks с 30+ runs.

## Ограничения

- Обе container-среды используют LinuxKit; native Linux без LinuxKit не проверен.
- Selected architecture synthetic и ещё не является final production binary со всеми интегрированными transports.
- Десять iterations — architecture evidence, а не финальная statistical certification.
- Host timing включает Docker Desktop scheduling и command-line client overhead.
- Ingestion benchmark использует generated 100-series snapshots и не измеряет network transport saturation.
- Сравнивается только Linux inotify; kqueue и non-Linux notification mechanisms вне scope.
- In-process concurrent scrape изолирует pointer/body access от HTTP и network work.

## Дополнительные benchmarks

| Benchmark                                 | Статус    | Evidence/граница                      |
|-------------------------------------------|-----------|---------------------------------------|
| warm-up и десять iterations               | покрыто   | raw и summary TSV                     |
| ingestion 100/1k/10k complete snapshots/s | покрыто   | обе среды                             |
| cardinality 100/1k/10k/100k               | покрыто   | in-process и HTTP                     |
| concurrent scrape 1/2/5/10                | покрыто   | in-process и external ten-client case |
| polling/inotify/hybrid                    | покрыто   | environment-specific observations     |
| startup/idle CPU/memory/FD                | покрыто   | десять samples в каждой среде         |
| graceful drain                            | покрыто   | in-flight response 800 ms             |
| final scrape/abort/timeout                | покрыто   | выполнено локально в обоих прогонах   |
| malformed/bind/queue/OOM                  | покрыто   | выполнено локально в обоих прогонах   |
| Ubuntu с matching fingerprint             | покрыто   | 23/23 assertions, fingerprint совпал  |
| 30–100 pinned CPU runs и perf/eBPF        | follow-up | release performance certification     |
| production adapters side-by-side          | follow-up | одинаковые complete snapshots         |

## Вывод

INV-015 подтверждает viability выбранной complete-snapshot architecture в tested envelope обеих LinuxKit-сред.
Cardinality является dominant dimension encoding/exposition; предпочтителен event-driven file notification с
reconciliation; обязательны explicit backpressure, timeouts и cgroups. Measurements являются research baselines, а не
production SLO. Решение зафиксировано в [ADR-015](../../docs-ru/06-architecture/adr/ADR-015.md).

## Выход исследования

- macOS evidence: `results/20260802T180839Z/`
- Ubuntu evidence: `results/20260803T071831Z/`
- ADR: [ADR-015](../../docs-ru/06-architecture/adr/ADR-015.md)
