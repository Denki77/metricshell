# Отчёт INV-003 — Распределение времени shutdown

[English version](report.md)

**Статус:** завершено  
**Дата:** 2026-07-23  
**Docker Servers:** 29.4.3, 27.4.0  
**Платформы:** linux/aarch64, linux/x86_64  
**Эталонные прогоны:** `results/20260723T150539Z`, `results/20260723T151454Z`  
**Сводки:**  `results/20260723T150539Z/summary.tsv`, `results/20260723T151454Z/summary.tsv`

## Цель

Проверить explicit workload timeout + reserve MetricShell как детерминированную shutdown policy без external SIGKILL и
выбрать admissible starting budgets.

## Прототип

- `prototype/cmd/metricshell`: process group, TERM/INT forwarding, workload budget, forced kill, synthetic finalization,
  HTTP drain, preservation exit.
- `prototype/cmd/workload`: immediate/delayed/ignore TERM.
- `prototype/Dockerfile`: isolated image.
- `run-bench.sh`: build, assertions, raw logs/TSV, fingerprint.

Policies: explicit, fixed reserve, percentage, absolute deadline. Deadline читается из flag/file при TERM; budget
вычисляется по actual remaining time; runner использует `docker cp`; every phase capped by remaining time.

## Команды запуска

```bash
./research/INV-003/run-bench.sh
```

Same command on Ubuntu. Results in UTC directory; inspection/manual commands — в README_ru.

## Среды запуска

| Среда                            | Дата       | Docker | Platform      | Result set                 | Fingerprint                                                        |
|----------------------------------|------------|--------|---------------|----------------------------|--------------------------------------------------------------------|
| Docker Desktop/macOS LinuxKit VM | 2026-07-23 | 29.4.3 | linux/aarch64 | `results/20260723T150539Z` | `27e8a991546667f92abb5965c044b834c1583f710cbb060dfef81174b29bb53c` |
| Docker Ubuntu LinuxKit VM        | 2026-07-23 | 27.4.0 | linux/x86_64  | `results/20260723T151454Z` | same                                                               |

Evidence содержит repository SHA, benchmark SHA-256, image ID, server, kernel, arch, CPU, memory, storage driver.
Fingerprint определяет identity; different image IDs expected.

## Подтверждение между средами

| Metric            |                macOS |               Ubuntu |
|-------------------|---------------------:|---------------------:|
| Mandatory grid    |                20/20 |                20/20 |
| Assertions        |              210/210 |              210/210 |
| Repetitions       |                30/30 |                30/30 |
| p50/p95/p99       | 6.825/7.710/7.986 ms | 7.767/8.682/8.941 ms |
| min/max           |       5.609/9.092 ms |       6.275/9.168 ms |
| Absolute deadline |                  5/5 |                  5/5 |
| HTTP              |                  2/2 |                  2/2 |
| Docker deadline   |                  2/2 |                  2/2 |

Signal-to-exit = TERM reception → complete MetricShell shutdown, включая 5 ms finalization.

### Shutdown timing

| Window | macOS just-before/forced | Ubuntu just-before/forced |
|-------:|-------------------------:|--------------------------:|
|     1s |       674.507/773.309 ms |        673.360/772.163 ms |
|     5s |     3930.539/4023.113 ms |      3925.638/4026.071 ms |
|    10s |     8935.097/9039.308 ms |      8926.074/9029.936 ms |
|    30s |   27935.941/28062.445 ms |    27922.744/28021.638 ms |
|    60s |   57941.380/58078.702 ms |    57923.676/58026.519 ms |

### Absolute deadline

| Scenario        | macOS remaining/workload | Ubuntu remaining/workload |
|-----------------|-------------------------:|--------------------------:|
| Full            |             4955/3955 ms |              4737/3737 ms |
| Partially spent |                3930/2930 |                 3700/2700 |
| Nearly expired  |                    194/0 |                       0/0 |
| Expired         |                      0/0 |                       0/0 |
| Reserve exceeds |                    421/0 |                      89/0 |

### HTTP/external deadline

| Metric                 |      macOS |     Ubuntu |
|------------------------|-----------:|-----------:|
| Fitting drain          |  73.423 ms |   0.102 ms |
| Overlong drain         | 704.135 ms | 700.585 ms |
| New post-TERM admitted |      false |      false |
| Completion in 1s       |    527.823 |    525.371 |
| Margin in 1s           |    472.177 |    474.629 |
| Completion in 5s       |   3529.925 |   3523.575 |
| Margin in 5s           |   1470.075 |   1476.425 |

Ubuntu raw uses comma decimal due locale; presentation normalized. Pass/fail uses in-process marker and deadline order.
Static `native_ubuntu=prepared_not_run` superseded by actual Ubuntu environment/results.

## Результаты

| Total | Workload | Reserve | Immediate | Just before | After deadline |     Never |
|------:|---------:|--------:|----------:|------------:|---------------:|----------:|
|    1s |     .75s |    .25s |      pass |        pass |      pass/KILL | pass/KILL |
|    5s |       4s |      1s |      pass |        pass |      pass/KILL | pass/KILL |
|   10s |       9s |      1s |      pass |        pass |      pass/KILL | pass/KILL |
|   30s |      28s |      2s |      pass |        pass |      pass/KILL | pass/KILL |
|   60s |      58s |      2s |      pass |        pass |      pass/KILL | pass/KILL |

| Window | Immediate | Just before | After deadline |      Never |
|-------:|----------:|------------:|---------------:|-----------:|
|     1s |    22.484 |     674.507 |        773.309 | 777.234 ms |
|     5s |    20.624 |    3930.539 |       4023.113 |   4022.429 |
|    10s |    23.148 |    8935.097 |       9039.308 |   9028.187 |
|    30s |    24.815 |   27935.941 |      28062.445 |  28024.567 |
|    60s |    24.140 |   57941.380 |      58078.702 |  58050.850 |

Assertions enforce exit/forced state, exactly one marker, budget expiry count, before total grace, caps and no early
kill.
Policies fixed/percentage/explicit produced 4s workload for 5s example. Absolute pairs listed above. Overcommit exit
`64`.
30 immediate repetitions p50/p95/p99 `6.825/7.710/7.986`, min/max `5.609/9.092`.
Listener closes at shutdown; second request HTTP 000. 100 ms request drained; 1500 ms capped 700 ms.
Docker 1s/5s windows with internal 750ms/4s completed before external deadline; exactly one marker; no post_exit event.

## Оценка гипотез

### Explicit timeout/reserve понятнее

Подтверждено: visible phase ownership, pre-validation; fixed/percentage скрывают budget/margin; known total still
required.

### Completion до SIGKILL

Подтверждено: overdue workloads killed at budget, finalization in reserve, marker before Docker boundary.

### Reserve drains active HTTP

Подтверждено: fitting drained, overlong timed out, new post-exit wait rejected.

## Допустимые значения и политики

- `workload_timeout`, `shutdown_reserve` against total/deadline;
- reject invalid totals/phases/ratios/overcommit;
- 1s: 750ms+250ms; 5–10s: ≥1s reserve; 30–60s: 2s;
- fixed default acceptable after measurement; percentage-only rejected;
- absolute deadline preferred additional input;
- no silent clamp;
- stop admission, drain active only, cap by phase/remaining;
- post-exit wait only natural completion.

## Ограничения прототипа

Synthetic finalization; LinuxKit only; Docker timeout not signaled; CPU/memory/storage/cardinality/concurrency and other
runtimes unverified; drain does not prove Prometheus persistence.

## Дополнительное benchmark-покрытие

| Item                           | Status   | Evidence              |
|--------------------------------|----------|-----------------------|
| 1/5/10/30/60 × 4               | 20/20    | grid/logs             |
| Grid invariants                | all      | assertions            |
| Policy arithmetic              | 3/3      | comparison            |
| Absolute states                | 5/5      | deadline TSV          |
| Overflow                       | rejected | logs                  |
| 30 repetitions                 | pass     | latency TSV           |
| HTTP fits/overrun/new          | covered  | drain TSV             |
| Docker external                | 1s/5s    | external TSV          |
| Post-exit during TERM          | covered  | policy TSV            |
| Environment                    | covered  | environment TSV       |
| Ubuntu                         | covered  | result set            |
| Native Linux                   | not run  | unavailable           |
| Pressure/concurrency/slow disk | next     | production-like tests |
| containerd/CRI-O/K8s           | next     | integration           |

## Вывод

INV-003 подтверждён. Explicit timeout + reserve при known total/deadline и rejection overcommit — выбранный contract.
Fixed reserve допустим, percentage-only нет. Starting reserve: 1s для 5–10s, 2s для 30–60s, minimum tested 250ms
для total 1s. Stop admission, bounded active drain, no post-exit wait, deadline-aware phases. Remaining boundary:
native Linux и другие runtimes.
