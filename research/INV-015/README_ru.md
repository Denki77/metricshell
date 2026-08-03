# INV-015 — Benchmarks и итоговое сравнение

**Статус:** завершено
**macOS:** `results/20260802T180839Z`
**Ubuntu/LinuxKit:** `results/20260803T071831Z`
**Отчёт:** [report_ru.md](report_ru.md)
**Решение:** [ADR-015](../../docs-ru/06-architecture/adr/ADR-015.md)

## Вопрос и правила

Каковы costs, saturation и failures выбранной complete-snapshot architecture? По ADR-004 каждая publication — полный
snapshot и atomic replacement; benchmark counts не являются increments. Сохраняются warm-up, repetitions, raw data и
p50/p95/p99. Timing — observation, не pass threshold.

## Необходимые доказательства

Idle/startup, ingestion 100/1k/10k, cardinality до 100k, concurrency 1/2/5/10, polling/inotify/hybrid, shutdown/drain,
final/abort/timeout, malformed/bind/queue/OOM и Ubuntu repeat.

## Подтверждённый результат

В обеих средах пройдено 23/23 assertions с одинаковым fingerprint. Для каждого in-container shape выполнено 10
iterations. 100 concurrent publications оставили ровно 100
series одного generation. HTTP body вырос с 5 120 B при 100 до 5 677 820 B при 100k; host scrape — с 20.927 до 73.183
ms. Encode 100k p50/p95 42.053/43.892 ms.

File detection p50: polling 1.311 ms, inotify 54.541 µs, hybrid 36.167 µs. External 10 scrapes — 321.664 ms. Drain
in-flight 800 ms завершился за 1.364 s. Final/abort/timeout и все failure profiles прошли.

## Итоговая интерпретация

Immutable pointer упрощает consistency; cost растёт с cardinality; inotify/hybrid быстрее polling в LinuxKit;
final scrape считать только после full write; queue/parser/bind/cgroup failures должны быть bounded. Значения не SLO.

## Запуск

```bash
./research/INV-015/run-bench.sh
latest="$(cat research/INV-015/latest-results.txt)"
cat "$latest/assertions.tsv"
cat "$latest/benchmark-raw.tsv"
cat "$latest/benchmark-summary.tsv"
cat "$latest/startup.tsv"
cat "$latest/idle.tsv"
cat "$latest/cardinality-http.tsv"
```

Final и failure выполняются в самом INV-015, не delegated.

## Ограничения

Synthetic architecture, не production binary. Rate test использует 100-series snapshot и короткое окно. 10 iterations
достаточны для research, для SLO нужно 30+. Docker/curl overhead включён. Только Linux inotify. Обе container-среды
используют LinuxKit: macOS/LinuxKit aarch64 и Ubuntu/LinuxKit x86_64; native Linux без LinuxKit не проверен.

## Additional Benchmarks

| Пункт                                    | Статус         |
|------------------------------------------|----------------|
| warm-up/10 iterations                    | покрыто        |
| ingestion/p50-p99                        | покрыто        |
| cardinality 100–100k                     | покрыто        |
| concurrency 1/2/5/10                     | покрыто        |
| polling/inotify/hybrid                   | покрыто        |
| startup/idle/resources                   | покрыто        |
| drain/final/abort/timeout                | покрыто        |
| malformed/bind/queue/OOM                 | покрыто        |
| Ubuntu                                   | покрыто: 23/23 |
| 30+ pinned/perf/eBPF/production adapters | follow-up      |

## Выход

- Prototype: `prototype/`
- macOS evidence: `results/20260802T180839Z/`
- Ubuntu evidence: `results/20260803T071831Z/`
- ADR: [ADR-015](../../docs-ru/06-architecture/adr/ADR-015.md)
- Report: [report_ru.md](report_ru.md)
