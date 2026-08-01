# INV-002 — Жизненный цикл workload и семантика завершения

[English version](README.md)

**Статус:** завершено  
**Эталонные прогоны:** `results/20260721T200345Z`, `results/20260721T200406Z-extended`, `results/20260721T201256Z`,
`results/20260721T202227Z-extended`  
**Отчёт:** [report.md](report_ru.md)  
**Решение:** [ADR-002](../../docs-ru/06-architecture/adr/ADR-002.md)

## Вопрос

Каков точный жизненный цикл одного запуска MetricShell?

## Контекст

MetricShell должен сохранять однозначность результата workload, состояния application metrics и поведения при restart
контейнера. Внутренний retry превращает один запуск контейнера в несколько запусков приложения и требует решить,
сбрасываются ли counters между попытками или объединяются.

## Кандидаты

1. Выполнить ровно один workload и вернуть его результат.
2. Перезапускать workload внутри MetricShell.
3. Выполнить workload один раз и передать retry Docker, Compose или Kubernetes.

## Исходная гипотеза

MetricShell должен выполнять ровно один запуск workload. Restart policy должна оставаться вне MetricShell.

## Эксперименты

Docker-прототип сравнивает:

- успешное и ошибочное single execution;
- ошибку запуска workload;
- ограниченную post-exit доступность метрик;
- внутренний restart со сбросом и сохранением counter;
- ограничение числа внутренних restart;
- Docker `--restart=on-failure:2` вокруг single-execution MetricShell.

Каждый case фиксирует итоговый exit code, число executions и финальное значение `app_events_total`. См. `summary.tsv` и
`observations.tsv`.

## Результаты

Все core и extended expectations прошли на macOS/LinuxKit aarch64 и Ubuntu/LinuxKit x86_64. Все четыре прогона
использовали fingerprint `a8042a6b12b8d659f701125584223373a934186d45fdeae2e64f89f6362c05f2`.

Signal-to-exit p50/p95/p99: `0.594/1.194/2.010 ms` на macOS и `1.767/2.137/2.187 ms` на Ubuntu.

| Case                       | Executions | Exit | Последовательность counter | Результат |
|----------------------------|-----------:|-----:|----------------------------|-----------|
| Single success             |          1 |    0 | 5                          | pass      |
| Single failure             |          1 |   17 | 5                          | pass      |
| Internal retry, reset      |          2 |    0 | 5 → 2                      | pass      |
| Internal retry, preserve   |          2 |    0 | 5 → 7                      | pass      |
| Internal retry limit       |          3 |   17 | 1 → 2 → 3                  | pass      |
| Start failure              |          0 |  127 | 0                          | pass      |
| Docker external restart    | 2 процесса |    0 | 5 → 2                      | pass      |
| Post-exit window 2 секунды |          1 |   17 | 5, endpoint доступен       | pass      |

Обе внутренние counter policies некорректны: reset уменьшает counter внутри одного lifecycle MetricShell, а preserve
объединяет независимые executions. Внешний Docker restart создаёт два чистых single-execution lifecycle и сохраняет
наблюдаемый restart count runtime.

## Вывод

Гипотеза подтверждена. MetricShell выполняет workload ровно один раз; retry count и backoff не входят в его
configuration surface. Runtime restart создаёт новый metric-state epoch.

Kubernetes оценён структурно: Pod `restartPolicy: OnFailure` перезапускает контейнер, а Job controller применяет
собственную retry/backoff policy. Для заявления runtime parity нужен отдельный integration run.

## Допустимые значения lifecycle

- executions workload на процесс MetricShell: ровно `1`;
- internal retry count: `0`, production retry option отсутствует;
- post-exit duration: `0` либо явное конечное значение; `2s` подтверждено функционально, но не является sizing
  recommendation;
- результат workload сохраняется, включая signal-derived outcome;
- ошибка запуска должна отличаться (`127` в прототипе);
- internal failure MetricShell использует отдельный код;
- один процесс MetricShell соответствует одному metric-state epoch;
- внешний retry принадлежит Docker/Compose/Kubernetes и настраивается оператором.

## Запуск прототипа

```bash
./research/INV-002/run-bench.sh
./research/INV-002/run-extended-bench.sh
```

```bash
latest="$(cat research/INV-002/latest-results.txt)"
cat "$latest/summary.tsv"
cat "$latest/observations.tsv"
cat "$latest/environment.tsv"
cat "$latest/post-exit.metrics"
extended="$(cat research/INV-002/latest-extended-results.txt)"
cat "$extended/latency-stats.tsv"
cat "$extended/faults.tsv"
cat "$extended/post-exit-grid.tsv"
cat "$extended/restart-storm.tsv"
cat "$extended/coverage.tsv"
```

```bash
docker build -t metricshell-inv002:prototype research/INV-002/prototype
docker run --rm metricshell-inv002:prototype \
  --policy=single --post-exit=2s -- \
  /usr/local/bin/workload --state=/tmp/attempt --exits=17 --increments=5
```

Runner создаёт только контейнеры `inv002-*` и один temporary named volume, удаляет их после завершения и пишет новый UTC
result directory.

## Ограничения прототипа

- Research code, без production ingestion protocols и hardening.
- Измерены две LinuxKit-среды; native Linux, containerd/CRI-O и Kubernetes не выполнены.
- Synthetic counter проверяет lifecycle semantics, а не throughput ingestion.
- Диапазон `0–30s` подтверждает bounded behavior, но production budget определяет INV-003.
- TERM, KILL, container OOM, restart storms и concurrent restart scrapes покрыты; daemon restart, host reboot и
  disk-full не проверены.

## Дополнительные benchmarks

Extended run включает 30 повторов, p50/p95/p99, Compose restart, 150 scrape samples на границе restart, TERM/KILL/OOM,
post-exit `0,1,2,5,10,30s` и internal restart storms 10/100/1000 attempts.

## Результат решения

- Прототип: `prototype/`
- Runners: `run-bench.sh`, `run-extended-bench.sh`
- Core evidence: `results/20260721T200345Z/`
- Extended evidence: `results/20260721T200406Z-extended/`
- Ubuntu evidence: `results/20260721T201256Z/`, `results/20260721T202227Z-extended/`
- Подробный анализ: [report_ru.md](report_ru.md)
- Вход для ADR: ровно один execution workload, внешнее владение restart, один metric-state epoch на execution.
