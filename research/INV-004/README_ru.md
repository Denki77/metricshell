# INV-004 — Владение состоянием метрик и семантика обновлений

[English version](README.md)

**Статус:** завершено  
**Эталонные прогоны:** `results/20260723T073114Z`, `results/20260723T150118Z`  
**Подробный отчёт:** [report.md](report_ru.md)

## Вопрос

Должен ли workload передавать complete registry snapshots, absolute values отдельных series, update operations либо их
комбинацию?

## Контекст

Представление определяет владельца истинного metric state, возможность восстановления после loss/restart и семантику
нескольких producers, обновляющих одну exported family.

## Кандидаты

- complete per-producer snapshots;
- absolute per-series values;
- operations (`increment`, `set`, `observe`);
- hybrid: operations как fast path плюс authoritative periodic/final snapshots.

## Исходная гипотеза

File ingestion естественно соответствует snapshots; socket и local push могут использовать operations или absolute
updates. Эквивалентная client semantics не требует одинаковой transport semantics.

## Эксперименты

Прототип выполняет 33 deterministic scenarios для counters, gauges, histograms, duplicates, type conflicts, multiple
producers, ordering, dropped updates, producer/receiver restarts, stale data и final reconciliation. Затем запускается
scale matrix 1/4/16 producers × 1/100/1000/10000 series и 30 representative repetitions.

## Результаты

Оба прогона дали 33 scenarios, 29 подтверждённых invariants, четыре ожидаемых counterexamples, 34/34 assertions и 129
benchmark rows. Fingerprint: `e52784470ff33e35fb58ab142be26a345bb6a373bd2eac58666b269f56875fd6`.

| Среда                  | Result set         | Assertions | Snapshot p50 | Operation p50 | Hybrid p50 |
|------------------------|--------------------|-----------:|-------------:|--------------:|-----------:|
| macOS/LinuxKit aarch64 | `20260723T073114Z` |      34/34 |     28,108/s |       4.48M/s |    4.31M/s |
| Ubuntu/LinuxKit x86_64 | `20260723T150118Z` |      34/34 |      6,402/s |       2.21M/s |    1.97M/s |

- Complete per-producer snapshots восстанавливают dropped updates и restarts, удаляют stale series и детерминированно
  агрегируют owners.
- Absolute counters допускают decrease `10→7`; last-writer-wins не выражает aggregate нескольких producers.
- Operations idempotent при sequence, но dropped increment даёт `2` вместо `3`, а receiver restart — `0` вместо `5`.
- Rejected type conflict не потребляет sequence; gap делает owner incomplete до snapshot reconciliation.
- Boundary: `(producer_id, producer_epoch, sequence)`; old epochs отклоняются, new epoch требует initial snapshot.
- Histograms содержат bounds, cumulative buckets, count и sum; совместимые states агрегируются component-wise.
- Duplicate gauges без policy отклоняются; `sum` проверен отдельно.
- Hybrid восстанавливает loss/restart и удаляет stale series.
- На 16 producers/10k series snapshot выделял около 3.82 MB/update; fast path — 34 B/update; hybrid interval 1000 —
  около 1788 B/update.

## Вывод

Выбрать hybrid model: **полные versioned per-producer snapshots являются authoritative state**. Operations допускаются
только как performance adapter между snapshots и не могут быть единственной durable truth.

## Допустимые semantic values

- stable producer ID;
- monotonically changed producer epoch;
- strictly increasing sequence внутри `(producer, epoch)`;
- initial snapshot gate для нового epoch;
- complete snapshot одного producer, а не global snapshot произвольного producer;
- отсутствующие series удаляют contribution данного owner;
- counter — non-negative cumulative, без decrease внутри epoch;
- gauge — absolute set;
- histogram — cumulative buckets/count/sum с immutable schema внутри epoch;
- compatible counters/histograms суммируются, gauge aggregation требует explicit policy;
- conflicts отклоняются и диагностируются;
- operations optional, sequenced, deduplicated; reconciliation после gap/restart и для final state;
- interval не фиксируется INV-004.

## Запуск прототипа

```bash
./research/INV-004/run-bench.sh
```

```bash
latest="$(cat research/INV-004/latest-results.txt)"
cat "$latest/summary.tsv"
cat "$latest/assertions.tsv"
cat "$latest/semantics.tsv"
cat "$latest/benchmark-stats.tsv"
cat "$latest/environment.tsv"
cat "$latest/coverage.tsv"
```

```bash
docker build -t metricshell-inv004:prototype research/INV-004/prototype
docker run --rm metricshell-inv004:prototype --mode=scenarios
```

```bash
docker run --rm metricshell-inv004:prototype \
  --mode=benchmark --candidate=hybrid_amortized --producers=16 --series=10000 --updates=100000 \
  --reconciliation-interval=1000
```

Для 100 repetitions: `INV004_REPEAT_COUNT=100`.

## Cross-environment fingerprint

Runner хэширует normalized relative paths и contents `prototype/` и `run-bench.sh`, не включая host path, HEAD,
timestamps и results. Оба прогона дали одинаковый fingerprint.

## Ограничения прототипа

- In-memory semantic model, без production parser, persistence и transport.
- Measurements включают Go map/string allocation и Docker startup per sample; это evidence, не SLO.
- Crash-safe persistence, encoding, authentication и hostile cardinality отложены.
- Обе среды LinuxKit; native Linux и Kubernetes не проверены.
- `container_go_version` содержит help banner и исключён из comparisons.
- Gauge aggregation намеренно запрещена без explicit policy.

## Дополнительные benchmarks

Покрыты all candidates, scale/producers grid, 30-run distributions, allocations, loss/reorder/duplicate, restarts, stale
deletion, conflicts, histogram semantics, gap state, transactional rejection и intervals 100/1000/10000.

## Результат решения

- Прототип: `prototype/`
- Runner: `run-bench.sh`
- Evidence: `results/20260723T073114Z/`, `results/20260723T150118Z/`
- Отчёт: [report_ru.md](report_ru.md)
- Решение: [ADR-004](../../docs-ru/06-architecture/adr/ADR-004.md)
