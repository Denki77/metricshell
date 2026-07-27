# INV-003 — Распределение времени shutdown

[English version](README.md)

**Статус:** завершено  
**Эталонные прогоны:** `results/20260723T150539Z`, `results/20260723T151454Z`  
**Подробный отчёт:** [report.md](report_ru.md)

## Вопрос

Какую часть внешнего shutdown grace period можно предоставить workload, а какую часть MetricShell должен
зарезервировать для себя?

## Контекст

MetricShell должен переслать termination signal, ограничить shutdown workload, при необходимости применить force-kill,
получить exit status, финализировать metrics/diagnostics, drain HTTP и выйти до SIGKILL runtime.

## Кандидаты

1. Fixed reserve: `workload_grace = total_grace - fixed_reserve`.
2. Percentage reserve: `workload_grace = total_grace × configured_ratio`.
3. Explicit workload timeout плюс explicit reserve MetricShell.
4. Absolute external deadline с динамическим remaining budget.

## Исходная гипотеза

Explicit workload timeout плюс runtime reserve проще анализировать, чем automatic inference.

## Эксперименты

Docker prototype проверяет windows `1, 5, 10, 30, 60s` и workloads, которые завершаются немедленно, за 100 ms до
budget, после budget или никогда. Он сравнивает fixed/percentage/explicit arithmetic, проверяет real absolute deadline,
отклоняет overcommit, повторяет shutdown 30 раз, тестирует HTTP admission/drain, независимый Docker deadline и
отсутствие
post-exit wait после external termination.

## Результаты

Все 20 grid cases, 210 assertions, 30 repetitions и дополнительные checks прошли на macOS/LinuxKit aarch64 и
Ubuntu/LinuxKit x86_64. Overdue/TERM-ignoring workloads завершены `137`, cooperative — `0`; `shutdown_complete` записан
до deadline.

| Window | Workload budget | Reserve | Just-before total | Forced total | Result |
|-------:|----------------:|--------:|------------------:|-------------:|--------|
|    1 s |          750 ms |  250 ms |        674.507 ms |   773.309 ms | pass   |
|    5 s |             4 s |     1 s |       3930.539 ms |  4023.113 ms | pass   |
|   10 s |             9 s |     1 s |       8935.097 ms |  9039.308 ms | pass   |
|   30 s |            28 s |     2 s |      27935.941 ms | 28062.445 ms | pass   |
|   60 s |            58 s |     2 s |      57941.380 ms | 58078.702 ms | pass   |

Все explicit assertions прошли: exit semantics, forced state, marker counts, budget expiry, total deadline, caps
finalization/HTTP и отсутствие раннего force-kill.

| Среда                  | Passed |          p50/p95/p99 |        Min/max |
|------------------------|-------:|---------------------:|---------------:|
| macOS/LinuxKit aarch64 |  30/30 | 6.825/7.710/7.986 ms | 5.609/9.092 ms |
| Ubuntu/LinuxKit x86_64 |  30/30 | 7.767/8.682/8.941 ms | 6.275/9.168 ms |

Fingerprint: `27e8a991546667f92abb5965c044b834c1583f710cbb060dfef81174b29bb53c`.
Signal-to-exit измеряется от TERM до полного shutdown, включая 5 ms synthetic finalization.

Absolute deadline: `4955 ms` full, `3930 ms` partially spent, `194 ms` near expiry, `0 ms` expired, `421 ms` reserve
exceeds remaining. Workload budget становится нулём, когда reserve не помещается.

100 ms scrape drained; 1500 ms scrape cap-нут примерно 700 ms; новый request после `shutdown_started` не принят
(`HTTP 000`). Internal totals 750 ms/4 s внутри external 1 s/5 s оставили margins `472.177 ms`/`1470.075 ms`.

## Вывод

Использовать explicit values, валидируя их против известного total/deadline. Fixed reserve допустим как derived default.
Percentage-only policy отклонена. Absolute deadline предпочтителен при надёжной передаче runtime.

## Допустимые значения и политики

- reject `workload_timeout + reserve > total_grace`;
- для 1 s: tested lower bound 250 ms reserve + 750 ms workload;
- для 5–10 s: reserve ≥ 1 s;
- для 30–60 s: reserve 2 s;
- HTTP drain отдельный и capped remaining deadline;
- после TERM/INT не запускать post-exit wait, только drain active requests;
- force-kill outcome `137`, finalization внутри reserve;
- values — starting points для LinuxKit, не universal guarantees.

## Запуск прототипа

```bash
./research/INV-003/run-bench.sh
```

```bash
latest="$(cat research/INV-003/latest-results.txt)"
cat "$latest/shutdown-grid.tsv"
cat "$latest/shutdown-grid-assertions.tsv"
cat "$latest/policy-comparison.tsv"
cat "$latest/absolute-deadline.tsv"
cat "$latest/latency-stats.tsv"
cat "$latest/http-drain.tsv"
cat "$latest/external-deadline.tsv"
cat "$latest/termination-policy.tsv"
cat "$latest/environment.tsv"
cat "$latest/coverage.tsv"
```

Ручной пример:

```bash
docker build -t metricshell-inv003:prototype research/INV-003/prototype
docker run --rm metricshell-inv003:prototype \
  --total-grace=10s --policy=explicit --workload-timeout=9s --reserve=1s -- \
  /usr/local/bin/workload --term-delay=-1ms
```

TERM: `docker stop --time 10 <container>`.
Deadline flags: `--shutdown-deadline-unix-ms=<timestamp>` или `--shutdown-deadline-file=<path>`; file читается при TERM,
runner передаёт его через `docker cp`.

## Ограничения прототипа

- Research code, synthetic finalization.
- LinuxKit aarch64/x86_64; native Linux, containerd/CRI-O, Kubernetes не проверены.
- Docker не сообщает stop timeout PID 1; deployment должен передать same total/deadline.
- CPU throttling, memory pressure, storage stalls, high concurrency, slow diagnostics не проверены.
- Timing — architecture evidence, не SLO.

## Дополнительные benchmarks

Покрыты 20-case grid, 210 assertions, derived arithmetic, 5 absolute-deadline states, overflow rejection,
30-run distributions, forced KILL, HTTP admission/drain, Docker 1s/5s deadlines и no post-exit wait.

Для stronger confidence: native Linux arm64/x86_64, CPU quotas, memory pressure, 100–1000 scrapes, slow storage,
containerd/CRI-O/Kubernetes, independent eBPF/`perf trace` timing.

## Результат решения

- Prototype: `prototype/`
- Runner: `run-bench.sh`
- macOS: `results/20260723T150539Z/`
- Ubuntu: `results/20260723T151454Z/`
- Report: [report_ru.md](report_ru.md)
- Decision: [ADR-003](../../docs-ru/06-architecture/adr/ADR-003.md)
