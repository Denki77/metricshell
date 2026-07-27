# INV-006 — Приём метрик через файл

[English version](README.md)

Статус: завершено  
Эталонные прогоны: `results/20260723T160155Z`, `results/20260723T161216Z`  
Отчёт: [report_ru.md](report_ru.md)

## Вопрос

Как MetricShell должен безопасно обнаруживать и читать обновления файла внутри Linux container?

## Контекст

Файл метрик container-local. Основные filesystems: writable container layer и container-local tmpfs; Docker named volume
измеряется информационно. Host bind mounts намеренно исключены из supported contract.

## Кандидаты

- polling stat/read с фиксированным interval;
- directory-level inotify;
- directory-level inotify плюс periodic reconciliation;
- producer-triggered reload, отклонённый как дополнительный transport, не устраняющий reconciliation.

## Исходная гипотеза

Directory-level inotify плюс low-frequency reconciliation обеспечивает низкий idle overhead, event-speed detection и
recovery после missed events, queue overflow и replaced watches.

## Необходимые доказательства

Existing/absent/atomically replaced/invalid/deleted files, crash writer, repeated replacement, directory recreation,
restart process, реальный `IN_Q_OVERFLOW`, latency, idle CPU, high-frequency updates, file-size scaling, missed final
states, сравнение layer/tmpfs/volume и path-independent fingerprint.

## Эксперименты

`run-bench.sh` выполняет 162 correctness assertions, шесть controlled lost-event A/B cases, 135 performance rows, 12
bursts по 10,000 updates, шесть overflow cases по 20,000 updates и 18 idle measurements. Matrix охватывает 128 B, 4 KiB
и 1 MiB, по три repetitions.

## Результаты

| Среда  | Docker | Kernel           | Architecture | Result set         | Fingerprint                                                        |
|--------|-------:|------------------|--------------|--------------------|--------------------------------------------------------------------|
| macOS  | 29.4.3 | LinuxKit 6.12.76 | aarch64      | `20260723T160155Z` | `4238d6e1be961e2d864ccfc95c763c1c9a03365c74d9c868e38f1b0d96eb1580` |
| Ubuntu | 27.4.0 | LinuxKit 6.10.14 | x86_64       | `20260723T161216Z` | тот же                                                             |

Обе среды прошли 162/162 correctness, 6/6 lost-event A/B, 12/12 burst convergence и 6/6 overflow-pressure convergence.
Real overflow наблюдался в 4/6 cases. Inotify-only имел по одному missed intermediate version; hybrid — ни одного.

### p95 detection для 4 KiB

| Среда  | Filesystem | Poll 10ms | Poll 100ms |  inotify | Hybrid 100ms | Hybrid 1s |
|--------|------------|----------:|-----------:|---------:|-------------:|----------:|
| macOS  | Layer      | 11.670 ms | 103.560 ms | 0.879 ms |     0.825 ms |  1.001 ms |
| macOS  | tmpfs      | 11.570 ms | 103.353 ms | 0.604 ms |     0.489 ms |  0.707 ms |
| macOS  | Volume     | 11.786 ms | 103.679 ms | 0.608 ms |     0.755 ms |  0.646 ms |
| Ubuntu | Layer      | 10.810 ms | 100.778 ms | 1.202 ms |     0.988 ms |  0.785 ms |
| Ubuntu | tmpfs      | 10.709 ms | 100.914 ms | 0.896 ms |     0.835 ms |  1.083 ms |
| Ubuntu | Volume     | 10.793 ms | 100.889 ms | 0.927 ms |     0.848 ms |  0.843 ms |

Hybrid 1s idle CPU: 0.272–0.418% macOS и 0.402–0.438% Ubuntu. Poll 10ms: 2.593–3.014% и 4.496–5.085%.

INV-006 не имеет signal-to-exit: prototype не supervises workload. Relevant latency — producer timestamp → observed file
update.

## Оценка гипотезы

Подтверждена. Inotify даёт event-speed detection. В controlled lost-event tests inotify-only не восстанавливался без
следующего event на всех трёх filesystems, hybrid восстанавливался по reconciliation timer.

## Допустимые значения

- наблюдать directory, не inode файла;
- файл является complete registry state;
- producer пишет temporary file в той же directory и выполняет atomic rename;
- parse/validate до swap; invalid/partial/deleted inputs сохраняют last valid state;
- default reconciliation `1s`, tested range `100ms–1s`;
- tested file range `128 B–1 MiB`, не production maximum;
- successful reload определяется content/version, не только mtime;
- при `IN_Q_OVERFLOW`, invalidation и directory recreation reinstall watches и immediate reconcile.

## Запуск прототипа

```bash
./research/INV-006/run-bench.sh
```

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

```bash
docker build -t metricshell-inv006:prototype research/INV-006/prototype
docker run --name inv006-manual --tmpfs /data:rw,size=256m,mode=0755 \
  metricshell-inv006:prototype --mode=performance --strategy=hybrid \
  --interval=1s --updates=30 --file-bytes=4096 --output=/results/inv006-manual.tsv
docker cp inv006-manual:/results/inv006-manual.tsv .
docker rm inv006-manual
```

## Ограничения прототипа

- Research code.
- Только LinuxKit aarch64/x86_64; native Linux не проверен.
- Bind mounts исключены.
- Synthetic format и SHA-256 не моделируют production parser/cardinality.
- CPU включает producer и hashing.
- Overflow зависит от scheduling; portable criterion — final convergence.
- `INV006_REQUIRE_REAL_OVERFLOW=1` включает stricter local stress.
- fsync не выполняется; power-loss durability не доказана.

## Дополнительные benchmarks

Покрыты layer/tmpfs/volume, polling 10/100/1000ms, hybrid 100/1000ms, 128B/4KiB/1MiB, repetitions и percentiles, bursts,
overflow, controlled loss, idle CPU и environment fingerprint. Follow-up: realistic parser/cardinality, cgroup CPU/RSS,
larger files, native Linux, Kubernetes emptyDir и durability.

## Результат решения

- Прототип: `prototype/`
- Runner: `run-bench.sh`
- Evidence: `results/20260723T152957Z/`, `results/20260723T153335Z/`
- Отчёт: [report.md](report_ru.md)
- ADR: [ADR-006](../../docs-ru/06-architecture/adr/ADR-006.md)
