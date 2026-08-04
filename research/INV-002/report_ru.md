# Отчёт INV-002 — Жизненный цикл workload и семантика завершения

[English version](report.md)

**Статус:** завершено  
**Дата прогонов:** 2026-07-21  
**Docker Servers:** 29.4.3, 27.4.0  
**Платформы:** `linux/aarch64`, `linux/x86_64`
**Эталонные прогоны:** `results/20260721T200345Z`, `results/20260721T200406Z-extended`,
`results/20260721T201256Z`, `results/20260721T202227Z-extended`  
**Сводки:** `results/20260721T200345Z/summary.tsv`, `results/20260721T201256Z/summary.tsv`,
`results/20260721T200406Z-extended/latency-stats.tsv`, `results/20260721T202227Z-extended/latency-stats.tsv`

## Цель

Проверить гипотезу INV-002: один процесс MetricShell должен выполнять только один workload, владеть одним однозначным
epoch состояния метрик, при необходимости публиковать final state в течение ограниченного времени и оставлять retry
policy Docker, Compose или Kubernetes.

## Прототип

Прототип расположен в `research/INV-002`.

- `prototype/cmd/metricshell` — Go supervisor с single-run и намеренно сравнительными internal-restart modes.
- `prototype/cmd/workload` — workload для exit codes, increments counter, duration и memory pressure.
- `prototype/Dockerfile` — image `metricshell-inv002:prototype`.
- `compose.yml` — restart scenario `on-failure:2`, принадлежащий Compose.
- `run-bench.sh` — core lifecycle и metric-state runner.
- `run-extended-bench.sh` — repetitions, Compose, scrapes, faults, post-exit grid и restart storms.
- `results/<timestamp>` — raw logs и TSV evidence.

Прототип запускает workload и ожидает его завершения, сохраняет outcome, сравнивает reset/preservation counters между
internal retries, публикует metrics в bounded post-exit interval и создаёт memory pressure для OOM injection.

## Команды запуска

```bash
./research/INV-002/run-bench.sh
./research/INV-002/run-extended-bench.sh
```

```bash
cat research/INV-002/latest-results.txt
cat research/INV-002/latest-extended-results.txt
cat "$(cat research/INV-002/latest-results.txt)/summary.tsv"
cat "$(cat research/INV-002/latest-extended-results.txt)/latency-stats.tsv"
cat "$(cat research/INV-002/latest-extended-results.txt)/faults.tsv"
cat "$(cat research/INV-002/latest-extended-results.txt)/post-exit-grid.tsv"
cat "$(cat research/INV-002/latest-extended-results.txt)/restart-storm.tsv"
cat "$(cat research/INV-002/latest-extended-results.txt)/coverage.tsv"
```

Ручной single execution:

```bash
docker build -t metricshell-inv002:prototype research/INV-002/prototype
docker run --rm metricshell-inv002:prototype \
  --policy=single --post-exit=2s -- \
  /usr/local/bin/workload --state=/tmp/attempt --exits=17 --increments=5
```

Ручной restart через Compose:

```bash
docker compose -p inv002 -f research/INV-002/compose.yml up --abort-on-container-exit
docker compose -p inv002 -f research/INV-002/compose.yml down -v
```

`INV002_REPEAT_COUNT=100` увеличивает число повторов с 30 до 100.

## Среды запуска

| Среда                    | Дата       | Docker | Platform         | Architecture | Набор результатов                   | Evidence                                                                 | Примечание                        |
|--------------------------|------------|-------:|------------------|--------------|-------------------------------------|--------------------------------------------------------------------------|-----------------------------------|
| Docker Desktop на macOS  | 2026-07-21 | 29.4.3 | LinuxKit 6.12.76 | aarch64      | `results/20260721T200345Z`          | [summary.tsv](results/20260721T200345Z/summary.tsv)                      | Core 8/8 pass.                    |
| Docker Desktop на macOS  | 2026-07-21 | 29.4.3 | LinuxKit 6.12.76 | aarch64      | `results/20260721T200406Z-extended` | [latency-stats.tsv](results/20260721T200406Z-extended/latency-stats.tsv) | Все extended assertions прошли.   |
| Docker Desktop на Ubuntu | 2026-07-21 | 27.4.0 | LinuxKit 6.10.14 | x86_64       | `results/20260721T201256Z`          | [summary.tsv](results/20260721T201256Z/summary.tsv)                      | Core 8/8 и все assertions прошли. |
| Docker Desktop на Ubuntu | 2026-07-21 | 27.4.0 | LinuxKit 6.10.14 | x86_64       | `results/20260721T202227Z-extended` | [latency-stats.tsv](results/20260721T202227Z-extended/latency-stats.tsv) | Все extended assertions прошли.   |

Обе среды используют LinuxKit. Это даёт cross-architecture confirmation, но не покрывает native Linux kernel без
LinuxKit. Kubernetes context существовал, но authentication отклонена из-за invalid OAuth refresh token. Gaps записаны
в `coverage.tsv`.

Все reference runs использовали fingerprint
`a8042a6b12b8d659f701125584223373a934186d45fdeae2e64f89f6362c05f2`, охватывающий `prototype/`, оба runner и
`compose.yml`. macOS: `benchmark_scope_diff_clean=false`; Ubuntu: `true`; в обоих случаях untracked count `0`.
Identity устанавливается fingerprint, а не repository HEAD.

Все assertions прошли: core post-exit, external restart, container-OOM, 30 lifecycle repetitions, 30 signal-to-exit
repetitions, Compose restart, faults и полный post-exit grid.

## Результаты

### Core lifecycle cases

| Case                      | Ожидаемый exit | Фактический exit | Executions | Final counter | Результат |
|---------------------------|---------------:|-----------------:|-----------:|--------------:|-----------|
| `single_success`          |              0 |                0 |          1 |             5 | pass      |
| `single_failure`          |             17 |               17 |          1 |             5 | pass      |
| `internal_reset`          |              0 |                0 |          2 |             2 | pass      |
| `internal_preserve`       |              0 |                0 |          2 |             7 | pass      |
| `restart_limit`           |             17 |               17 |          3 |             3 | pass      |
| `start_failure`           |            127 |              127 |          0 |             0 | pass      |
| `external_docker_restart` |              0 |                0 | 2 процесса |             2 | pass      |
| `post_exit_endpoint`      |             17 |               17 |          1 |             5 | pass      |

`post_exit_endpoint` подтверждён шестью assertions: `docker wait=17`, ровно один `attempt_started` и
`lifecycle_finalized`, counter `5`, доступный final state `/metrics` и internal elapsed `2005.003 ms` для `2s`.

### Повторы и end-to-end latency

| Среда                  | Count | Passed |         p50 |         p95 |         p99 |         Min |         Max |
|------------------------|------:|-------:|------------:|------------:|------------:|------------:|------------:|
| macOS/LinuxKit aarch64 |    30 |     30 |  285.287 ms |  392.271 ms |  410.242 ms |  222.859 ms |  660.293 ms |
| Ubuntu/LinuxKit x86_64 |    30 |     30 | 5142.922 ms | 6166.546 ms | 6280.791 ms | 4274.080 ms | 6547.394 ms |

Это end-to-end `docker run`, включая startup container, Docker CLI/daemon и 1 ms workload, а не только supervisor.

### Signal-to-exit latency

Исправленный runner измеряет `signal_forwarded -> workload exit observed` внутри MetricShell:

| Среда                  | Count | Passed |      p50 |      p95 |      p99 |      Min |      Max |
|------------------------|------:|-------:|---------:|---------:|---------:|---------:|---------:|
| macOS/LinuxKit aarch64 |    30 |     30 | 0.594 ms | 1.194 ms | 2.010 ms | 0.330 ms | 2.194 ms |
| Ubuntu/LinuxKit x86_64 |    30 |     30 | 1.767 ms | 2.137 ms | 2.187 ms | 0.612 ms | 2.398 ms |

### Cross-environment confirmation

| Метрика                             | macOS/LinuxKit aarch64 | Ubuntu/LinuxKit x86_64 |
|-------------------------------------|-----------------------:|-----------------------:|
| Core cases                          |                    8/8 |                    8/8 |
| Core post-exit assertions           |                    6/6 |                    6/6 |
| Lifecycle repetitions               |                  30/30 |                  30/30 |
| Signal-to-exit repetitions          |                  30/30 |                  30/30 |
| Configured 2s internal elapsed      |            2005.003 ms |            2001.432 ms |
| Compose restart count / lifecycles  |                  1 / 2 |                  1 / 2 |
| Restart scrape invariant assertions |                    5/5 |                    5/5 |
| Restart scrapes HTTP 200 / gaps     |                5 / 145 |                6 / 144 |
| Первый / второй epoch observed      |           true / false |           true / false |
| TERM / KILL / container-OOM exit    |        143 / 137 / 137 |        143 / 137 / 137 |
| Container-OOM assertions            |                    4/4 |                    4/4 |
| Post-exit grid cases                |                    6/6 |                    6/6 |
| 1000-attempt storm duration         |            1137.820 ms |            6263.921 ms |
| 1000-attempt storm log size         |              175,585 B |              194,501 B |

Timing-dependent scrape visibility является observation и не определяет pass/fail.

### Runtime restart behavior

Compose `on-failure:2` сделал один restart: первый lifecycle завершился `17`, второй `0`, container restart count `1`.
Оба процесса MetricShell выполнили workload один раз.

| Scrape samples | HTTP 200 | Gaps | Counter 5 | Counter 2 | Completed lifecycles |
|---------------:|---------:|-----:|----------:|----------:|---------------------:|
|            150 |        5 |  145 |         5 |         0 |                    2 |

Второй epoch не был scraped, endpoint имел gap. Runtime restart гарантирует lifecycle ownership, но не continuity или
final scrape. Portable invariants находятся в `restart-scrape-assertions.tsv`, observations — отдельно.

### Fault injection

| Case                    | Expected | Actual | Result |
|-------------------------|---------:|-------:|--------|
| TERM                    |      143 |    143 | pass   |
| KILL                    |      137 |    137 | pass   |
| Container OOM at 32 MiB |      137 |    137 | pass   |

Docker зафиксировал `OOMKilled=true`. MetricShell записал `attempt_exited` и `lifecycle_finalized` с `137`; exact kernel
OOM victim отдельно не установлена.

### Post-exit grid

| Configured | Internal elapsed | Host lifecycle | Exit | Result |
|-----------:|-----------------:|---------------:|-----:|--------|
|        0 s |         0.001 ms |     508.319 ms |   17 | pass   |
|        1 s |      1007.201 ms |    1517.810 ms |   17 | pass   |
|        2 s |      2004.732 ms |    2444.365 ms |   17 | pass   |
|        5 s |      5008.071 ms |    5511.979 ms |   17 | pass   |
|       10 s |     10004.263 ms |   10544.773 ms |   17 | pass   |
|       30 s |     30023.144 ms |   30540.120 ms |   17 | pass   |

Все durations сохранили `17`; pass/fail использует in-process timer.

### Restart storms

| Internal attempts |    Duration | Exit |  Log size | Result |
|------------------:|------------:|-----:|----------:|--------|
|                10 |  267.706 ms |   17 |   1,783 B | pass   |
|               100 |  295.248 ms |   17 |  17,914 B | pass   |
|              1000 | 1137.820 ms |   17 | 175,585 B | pass   |

Internal retry линейно увеличивает supervisor work и logs. External Docker storms не расширялись до 100/1000 из-за
runtime restart delay/backoff.

## Оценка гипотез

### Только один workload execution

Подтверждено: сохранены success, failure, TERM, KILL, container-OOM и start failure; 30 repetitions прошли.

### Restart policy вне MetricShell

Подтверждено: Docker/Compose делают visible restart, а каждый процесс MetricShell выполняет один workload. Internal
retry
добавляет eligibility, limits, backoff, cancellation и aggregation outcome.

### Один process — один metric-state epoch

Подтверждено: reset дал `5 -> 2`, preserve — `5 -> 7`; external restart создал epochs `5 -> 2`.

### Bounded post-exit availability

Подтверждено для `0, 1, 2, 5, 10, 30s`, но это не выбирает default и не гарантирует final scrape.

## Допустимые значения и политики

- executions workload на process MetricShell: `1`;
- internal retry count: `0`;
- metric state: один epoch на process;
- post-exit duration: `0` или finite; verified `0–30s`;
- post-exit default — INV-003;
- exit `N` сохраняется; signal `S` → `128+S`; TERM `143`, KILL/OOM `137`;
- start failure: distinct, prototype `127`;
- internal failure: distinct MetricShell-owned code;
- retry/backoff: deployment policy;
- не обещать endpoint continuity/final scrape через restart.

## Ограничения прототипа

- Research code, не production.
- LinuxKit aarch64/x86_64; native Linux не проверен.
- Kubernetes не выполнялся из-за invalid OAuth token.
- Docker daemon restart и disk-full не выполнялись по причинам безопасности.
- Network partition неприменим.
- Synthetic counter не измеряет ingestion throughput.
- Storms слишком коротки для reliable CPU/RSS point sampling.
- Scrape cadence фактически 90–100 ms из-за запуска curl.

## Дополнительное benchmark-покрытие

| Benchmark item                  | Status                         | Evidence                               |
|---------------------------------|--------------------------------|----------------------------------------|
| 30 repetitions p50/p95/p99      | Covered                        | `repetitions.tsv`, `latency-stats.tsv` |
| 30 signal-to-exit               | Covered both                   | latency TSV                            |
| Docker/Compose external restart | Covered                        | core/compose logs                      |
| Scrapes around restart          | Covered, observations separate | restart scrape TSV                     |
| TERM/KILL/OOM                   | Covered                        | faults/OOM assertions                  |
| Post-exit 0/1/2/5/10/30         | Covered                        | grid TSV                               |
| Storms 10/100/1000              | Covered rejected model         | storm TSV/logs                         |
| Environment metadata            | Covered                        | `environment.tsv`                      |
| Native Linux                    | Not run                        | LinuxKit only                          |
| Kubernetes                      | Not run                        | OAuth invalid                          |
| Docker daemon restart           | Not run                        | unsafe                                 |
| Disk-full                       | Not run                        | unsafe/out of scope                    |
| Network partition               | Not applicable                 | no remote dependency                   |

Machine-readable status — в `coverage.tsv` обоих extended sets.

## Вывод

INV-002 подтверждён matching-fingerprint runs:

- один workload на process;
- один metric epoch на process;
- internal retries отклонены;
- Docker/Compose владеют restarts;
- post-exit 0–30 s функционально работает, sizing — INV-003;
- restart не гарантирует scrape continuity или final epoch collection.

Рекомендуется хранить retry/backoff в deployment configuration и не предполагать успешный scrape на restart boundary.
