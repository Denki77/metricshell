# Отчёт INV-015 — Benchmarks и итоговое сравнение

**Статус:** в процессе
**Дата:** 2026-08-02
**Прогон:** `results/20260802T180839Z`
**Платформа:** Docker 29.6.2, LinuxKit aarch64, 12 CPU, 16 GiB
**Fingerprint:** `ffd49f1d5a23d5484a270329c6610d99196480228f4b84ad6b1aac10bbd9a807`

## Цель и метод

Измерить selected immutable complete-snapshot model. Container делает warm-up и 10 iterations ingestion, cardinality,
concurrency, detection, initialization. Runner добавляет real HTTP startup/idle/replacement/cardinality/queue/drain,
final/abort/timeout/bind/OOM и агрегирует p50/p95/p99.

## Результаты

Все 23 assertions прошли.

|  Series |     Bytes |     Encode p50/p95 | HTTP scrape observation |
|--------:|----------:|-------------------:|------------------------:|
|     100 |     5 080 |   27.166/39.458 µs |               20.927 ms |
|   1 000 |    52 780 | 297.750/315.000 µs |               22.658 ms |
|  10 000 |   547 780 |     3.426/4.229 ms |               25.251 ms |
| 100 000 | 5 677 780 |   42.053/43.892 ms |               73.183 ms |

Detection p50/p95: polling 1.311/1.363 ms, inotify 54.541/90.750 µs, hybrid 36.167/52.958 µs. 100 concurrent
publications оставили один exact generation. 10 external scrapes — 321.664 ms. Queue дала 429. Malformed сохранил last
valid. Drain дождался response. Final one/abort/timeout, bind 70 и OOM 137 прошли.

## Оценка и policy

Cost масштабируется с cardinality; нужны bounds. Atomic immutable replacement сохраняет consistency. Event-driven file
detection предпочтительнее polling с fallback. Backpressure/cgroup observable и bounded. Final count после full write.
Не сравниваются разные semantics: все publications полные snapshot’ы.

Предварительно сохраняются ADR replacement, limits INV-014, inotify+fallback, bounded/pre-encoded exposition, N=1 final
scrape+timeout и explicit failure metrics. 100k — research shape, не default.

## Ограничения и Additional

10 iterations/Docker Desktop не performance certification. Production transports ещё не интегрированы. Ubuntu, 30+
pinned runs, perf/eBPF, allocations и production adapter comparison остаются. Все локальные final/failure cases реально
выполнены.

## Вывод

Model viable в tested envelope; dominant dimension — cardinality, Linux file detection должен быть event-driven,
backpressure/timeouts/cgroups обязательны. Статус остаётся in progress до Ubuntu и production integration.

## Выход

- Evidence: `results/20260802T180839Z/`
- Ubuntu/ADR: ожидаются
