# Отчёт INV-001 — Модель процессов и работа в роли PID 1

[English version](report.md)

**Статус:** завершено  
**Даты прогонов:** 2026-07-17, 2026-07-18  
**Docker Server:** 29.4.3, 27.4.0  
**Платформы:** `linux/aarch64`, `linux/x86_64`  
**Эталонные прогоны:** `results/20260717T192610Z`, `results/20260718T085124Z`  
**Сводки:** `results/20260717T192610Z/summary.tsv`, `results/20260718T085124Z/summary.tsv`

## Цель

Проверить гипотезу INV-001: MetricShell способен корректно владеть жизненным циклом workload при работе как PID 1 или
под Docker init/Tini, если явно реализует запуск workload, пересылку сигналов, наблюдение exit status, reaping
descendants и работу после завершения workload.

## Прототип

Прототип расположен в `research/INV-001`.

- `prototype/cmd/metricshell` — прототип supervisor на Go.
- `prototype/cmd/workload-trap` — workload на Go, фиксирующий TERM/INT/HUP без промежуточного shell.
- `prototype/workloads/*.sh` — shell scenarios для wrappers, child spawning, double fork и проверки exit status.
- `prototype/Dockerfile` — image `metricshell-inv001:prototype`.
- `run-bench.sh` — сборка image и запуск scenarios.
- `results/<timestamp>` — raw logs, inspect JSON, exit files, aggregated events и TSV summaries.

Прототип запускает workload как child process, при необходимости создаёт process group, пересылает TERM/INT/HUP/QUIT,
включает `PR_SET_CHILD_SUBREAPER`, выполняет force-kill после grace period, сохраняет exit code, выполняет reaping
adopted descendants и оставляет `/metrics` доступным в ограниченном post-exit window.

## Команды запуска

```bash
./research/INV-001/run-bench.sh
```

```bash
cat research/INV-001/latest-results.txt
cat "$(cat research/INV-001/latest-results.txt)/summary.tsv"
cat "$(cat research/INV-001/latest-results.txt)/assertions.tsv"
cat "$(cat research/INV-001/latest-results.txt)/environment.tsv"
cat "$(cat research/INV-001/latest-results.txt)/signal-delivery.tsv"
cat "$(cat research/INV-001/latest-results.txt)/signal-to-exit-latency-stats.tsv"
cat "$(cat research/INV-001/latest-results.txt)/resources.tsv"
cat "$(cat research/INV-001/latest-results.txt)/scrapes.tsv"
cat "$(cat research/INV-001/latest-results.txt)/zombies.tsv"
```

```bash
docker build -t metricshell-inv001:prototype research/INV-001/prototype
docker run --rm metricshell-inv001:prototype --http=:9090 -- /usr/local/bin/workload-trap
```

```bash
docker run --rm --init metricshell-inv001:prototype --http=:9090 -- /usr/local/bin/workload-trap
```

```bash
docker run --rm metricshell-inv001:prototype --http=:9090 --process-group -- /usr/local/bin/workload-trap
```

```bash
docker run --rm --init metricshell-inv001:prototype --http=:9090 --subreaper -- /prototype/workloads/double-fork.sh
```

## Среды запуска

| Среда                                      | Дата       | Docker | Platform | Architecture | Набор результатов          | Сводка                                              |
|--------------------------------------------|------------|--------|----------|--------------|----------------------------|-----------------------------------------------------|
| Docker Desktop на macOS                    | 2026-07-17 | 29.4.3 | linux    | aarch64      | `results/20260717T192610Z` | [summary.tsv](results/20260717T192610Z/summary.tsv) |
| Docker Desktop на Ubuntu / LinuxKit x86_64 | 2026-07-18 | 27.4.0 | linux    | x86_64       | `results/20260718T085124Z` | [summary.tsv](results/20260718T085124Z/summary.tsv) |

Оба прогона использовали fingerprint `35dc9c63a0a9f6dedf56a1c6c80b582919d5961b8f233c49ef1aed55652b71fb`, вычисляемый
только по `prototype/` и `run-bench.sh`.

## Подтверждение в разных средах

| Метрика                                | macOS/LinuxKit aarch64 | Ubuntu/LinuxKit x86_64 |
|----------------------------------------|-----------------------:|-----------------------:|
| Успешные summary cases                 |                     52 |                     52 |
| Успешные assertions                    |                    141 |                    141 |
| Наблюдения доставки сигналов           |                     42 |                     42 |
| Агрегированные structured events       |                    597 |                    598 |
| Успешные post-exit HTTP scrapes        |                      5 |                      5 |
| Samples без zombie                     |                     25 |                     27 |
| `repeat_signal_direct_pg` p50/p95/p99  |   0.434/0.581/0.625 ms |   0.468/1.894/2.155 ms |
| Forced shutdown signal-to-exit latency |             506.627 ms |             502.317 ms |

## Результаты

| Case                              | Модель                               | Ожидалось | Фактически | Результат |
|-----------------------------------|--------------------------------------|----------:|-----------:|-----------|
| `signal_direct_pid1`              | PID 1, direct child                  |         0 |          0 | pass      |
| `signal_direct_pg`                | PID 1, direct child, process group   |         0 |          0 | pass      |
| `signal_direct_pg_init`           | Docker init/Tini, process group      |         0 |          0 | pass      |
| `signal_direct_pg_init_subreaper` | Docker init/Tini + subreaper         |         0 |          0 | pass      |
| `signal_shell_script_pg`          | shell script workload, process group |       143 |        143 | pass      |
| `signal_shell_no_pg`              | `/bin/sh -c`, без process group      |       143 |        143 | pass      |
| `signal_shell_pg`                 | `/bin/sh -c`, process group          |       143 |        143 | pass      |
| `signal_shell_pg_init`            | Docker init/Tini + `/bin/sh -c`      |       143 |        143 | pass      |
| `signal_bash_pg`                  | `/bin/bash -c`, process group        |       143 |        143 | pass      |
| `reap_short_children_200`         | 200 short-lived children             |         0 |          0 | pass      |
| `child_churn_1000`                | 1k short-lived children              |         0 |          0 | pass      |
| `child_churn_10000`               | 10k short-lived children             |         0 |          0 | pass      |
| `double_fork_pid1_no_subreaper`   | PID 1, daemonized grandchild         |         0 |          0 | pass      |
| `double_fork_no_subreaper_init`   | Docker init/Tini, без subreaper      |         0 |          0 | pass      |
| `double_fork_subreaper_init`      | Docker init/Tini + subreaper         |         0 |          0 | pass      |
| `exit_zero`                       | workload exit 0                      |         0 |          0 | pass      |
| `exit_17`                         | workload exit 17                     |        17 |         17 | pass      |
| `sigkill`                         | workload получает KILL               |       137 |        137 | pass      |
| `shutdown_grace_forced_kill`      | TERM игнорируется, KILL после 500 ms |       137 |        137 | pass      |
| `start_failure`                   | ошибка запуска workload              |       127 |        127 | pass      |
| `internal_failure`                | внутренний сбой MetricShell          |        70 |         70 | pass      |
| `post_exit_survival`              | post-exit metrics с исходным exit 17 |        17 |         17 | pass      |

Основные наблюдения:

- `summary.tsv` выводит `pass` только после прохождения всех assertions case.
- Signal forwarding работает как по child PID, так и через process group.
- Получение сигналов записывается в `signal-delivery.tsv`, а не выводится косвенно из exit code.
- `assertions.tsv` проверяет descendants, double-fork reaping и `/metrics`.
- Shell-form commands могут завершаться с `143`, несмотря на доставку TERM descendants.
- Как PID 1 MetricShell выполнял reaping daemonized descendant с исходным exit `23`.
- Под Docker init/Tini без subreaper descendant принадлежал init; с subreaper — MetricShell.
- TERM-игнорирующий workload был force-killed примерно через 500 ms с exit `137`.
- Child churn 200/1k/10k прошёл; в успешных 10k samples zombies не обнаружены.
- RSS/HWM находился в диапазоне `8296–8380 KiB` на macOS и `8728–10740 KiB` на Ubuntu.
- Все пять внешних post-exit scrapes вернули HTTP `200` и `metricshell_workload_exit_code 17`.

## Оценка гипотез

### MetricShell должен владеть lifecycle workload даже при наличии Tini

Подтверждено. Tini выполняет generic init duties, но не сохраняет MetricShell-level knowledge о workload и descendants и
не заменяет запуск, ожидание, signal forwarding, фиксацию exit status и post-exit behavior.

### Tini уменьшает edge cases PID 1, но добавляет binary и signal layer

Подтверждено. Под Docker init MetricShell работает не как PID 1 и теряет ownership orphan descendants без
`PR_SET_CHILD_SUBREAPER`.

### Single-binary implementation может быть проще

Подтверждено в границах прототипа: жёстких блокеров для MetricShell как PID 1 не обнаружено.

### Process group и orphan descendants — основные риски

Подтверждено. Главный риск — shell exit semantics; под внешним init для daemonized descendants требуется subreaper.

## Допустимые значения и политики

- MetricShell по умолчанию работает как PID 1.
- Запускается только один workload как direct child.
- Process group создаётся при необходимости group-wide signal delivery.
- TERM/INT/HUP/QUIT пересылаются group или direct child PID.
- Для non-cooperative workload обязателен bounded force-kill fallback.
- Exit `0` и `N` сохраняются; signal `S` преобразуется в `128 + S`; ошибка запуска — `127`; internal failure — отдельный
  код.
- Post-workload survival допускается только на конечное время.
- Под external init при ownership daemonized descendants включается `PR_SET_CHILD_SUBREAPER`.
- Shell-form commands поддерживаются с явным caveat по exit `143`.

## Ограничения прототипа

- Research code, не production implementation.
- Обе среды используют LinuxKit; native Linux и Kubernetes не проверены.
- Docker `--init` используется как Tini-compatible layer.
- Timing — архитектурное evidence, а не SLO.
- Zombie state sampled через `/proc`; короткие windows могли быть пропущены.
- HTTP проверялся только простым post-exit scrape.

## Дополнительные benchmarks

| Проверка                       | Статус                                | Evidence                              |
|--------------------------------|---------------------------------------|---------------------------------------|
| 30 повторов p50/p95/p99        | Покрыто для `repeat_signal_direct_pg` | `signal-to-exit-latency*.tsv`         |
| Получение сигналов descendants | Покрыто                               | `signal-delivery.tsv`, `events.jsonl` |
| Assertions сверх exit code     | Покрыто                               | `assertions.tsv`, `summary.tsv`       |
| CPU/RSS                        | Покрыто                               | `resources.tsv`, `environment.tsv`    |
| Child churn 1k/10k             | Покрыто; 100k opt-in                  | `summary.tsv`, `zombies.tsv`          |
| Shutdown grace и forced KILL   | Покрыто                               | logs, latency stats                   |
| PID 1 / `--init` / subreaper   | Покрыто                               | `summary.tsv`                         |
| Shell variants                 | Покрыто                               | `summary.tsv`, `signal-delivery.tsv`  |
| External post-exit scrape      | Покрыто                               | `scrapes.tsv`                         |
| Native Linux/Kubernetes        | Не выполнено                          | требуется другая среда                |

```bash
INV001_RUN_HEAVY=1 ./research/INV-001/run-bench.sh
```

## Вывод

Гипотеза INV-001 подтверждена. Рекомендуемое направление — single-binary MetricShell как PID 1 по умолчанию, с optional
compatibility mode под Tini и subreaper, когда требуется владение daemonized descendants. Process group полезна для
signal coverage, но shell-form commands требуют отдельной exit-semantics policy.
