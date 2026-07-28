# INV-001 — Модель процессов и работа в роли PID 1

[English version](README.md)

**Статус:** завершено  
**Эталонные прогоны:** `results/20260717T192610Z`, `results/20260718T085124Z`  
**Подробный отчёт:** [report.md](report_ru.md)

## Вопрос

Должен ли MetricShell работать непосредственно как PID 1, запускаться под init-процессом, например Tini, либо передать
управление процессами другому компоненту?

## Контекст

MetricShell должен запускать workload, принимать и пересылать сигналы, наблюдать завершение workload, сохранять
семантику exit, управлять descendants в своей зоне ответственности и, если это настроено, оставаться активным после
завершения workload.

## Кандидаты

### A. MetricShell как PID 1

MetricShell самостоятельно реализует необходимое поведение init-процесса и supervisor.

### B. Tini как PID 1

Tini запускает MetricShell, а MetricShell запускает workload.

### C. Другой supervisor процессов

Примеры: dumb-init, s6 или supervisord.

## Исходные гипотезы

- MetricShell должен владеть жизненным циклом workload даже при наличии Tini.
- Tini может уменьшить число edge cases PID 1, но добавляет ещё один binary и слой обработки сигналов.
- Корректная single-binary реализация может быть проще в эксплуатации.
- Основные риски корректности связаны с process group и orphan descendants.

## Необходимые доказательства

- документация Linux по процессам и сигналам;
- документация Docker по init и process model;
- анализ поведения и исходного кода Tini;
- прототип для каждого жизнеспособного дерева процессов;
- тесты сигналов, descendants, zombie processes и exit codes.

## Эксперименты

### E-001.1 — Пересылка сигналов

Реализовано в `run-bench.sh` для direct child, process group, shell script, wrapper `/bin/sh -c` и вариантов с Docker
init/Tini.

### E-001.2 — Reaping дочерних процессов

Реализовано через создание короткоживущих child processes и сценарии daemonization посредством double fork. Когда
MetricShell владеет orphan descendant, прототип фиксирует событие `descendant_reaped`.

### E-001.3 — Exit status

Проверены exit `0`, exit `17`, TERM, KILL, ошибка запуска workload и имитируемый внутренний сбой MetricShell.

### E-001.4 — Работа после завершения workload

Проверено с `--post-exit=3s`: endpoint `/metrics` остаётся доступным и публикует сохранённый exit code workload.

## Критерии оценки

- корректность;
- контроль дерева процессов;
- целостность exit code;
- сложность реализации;
- число внешних зависимостей;
- размер image;
- переносимость;
- эксплуатационная прозрачность.

## Открытые вопросы

- Следует ли использовать `PR_SET_CHILD_SUBREAPER`?
  - Да, когда MetricShell не является PID 1, но всё ещё должен владеть daemonized descendants или наблюдать их.
- Следует ли создавать отдельную process group для каждого workload?
  - Рекомендуется, если нужна group-wide доставка сигналов; shell-form exit semantics должны быть определены явно.
- Поддерживаются ли daemonized descendants?
  - Да, когда MetricShell является PID 1 либо настроен как subreaper под другим init-процессом.
- Какое поведение ожидается от shell-form commands?
  - После TERM shell wrapper может вернуть `143`, даже если descendants получили сигнал; это следует документировать и
    тестировать.
- Даёт ли Tini дополнительную корректность после реализации lifecycle ownership в MetricShell?
  - Он может выполнять общие обязанности namespace init, но не заменяет владение lifecycle со стороны MetricShell.

## Результаты

| Среда                                      | Дата       | Набор результатов          | Сводка                                              | Fingerprint benchmark                                              |
|--------------------------------------------|------------|----------------------------|-----------------------------------------------------|--------------------------------------------------------------------|
| Docker Desktop на macOS / LinuxKit aarch64 | 2026-07-17 | `results/20260717T192610Z` | [summary.tsv](results/20260717T192610Z/summary.tsv) | `35dc9c63a0a9f6dedf56a1c6c80b582919d5961b8f233c49ef1aed55652b71fb` |
| Docker Desktop на Ubuntu / LinuxKit x86_64 | 2026-07-18 | `results/20260718T085124Z` | [summary.tsv](results/20260718T085124Z/summary.tsv) | `35dc9c63a0a9f6dedf56a1c6c80b582919d5961b8f233c49ef1aed55652b71fb` |

Основные результаты:

- MetricShell как PID 1 корректно пересылал сигналы, сохранял exit codes, выполнял reaping orphan descendant после
  double fork и публиковал post-exit metrics.
- Все assertions на уровне test cases прошли в обеих средах.
- Docker init/Tini без `PR_SET_CHILD_SUBREAPER` выполнял reaping daemonized descendants вне видимости MetricShell.
- С `PR_SET_CHILD_SUBREAPER` MetricShell мог выполнять reaping daemonized descendants самостоятельно.
- Process-group signaling достигал shell descendants, но shell wrappers могли завершаться с `143`.
- Для 30 прогонов `repeat_signal_direct_pg` p50/p95/p99 составили `0.434/0.581/0.625 ms` на macOS и
  `0.468/1.894/2.155 ms` на Ubuntu.
- Расширенный benchmark включал явные события доставки сигналов, CPU/RSS samples, child churn 1k/10k, zombie scans,
  forced shutdown grace, внешние post-exit scrapes и metadata среды.

## Вывод

Для следующего этапа принимается направление «MetricShell по умолчанию работает как PID 1». Tini/Docker init сохраняется
как compatibility mode, но не заменяет lifecycle ownership MetricShell. Если MetricShell работает под init-процессом и
должен управлять daemonized descendants, следует включать `PR_SET_CHILD_SUBREAPER`.

Process groups полезны для доставки сигналов descendants, но поведение shell-form commands должно быть зафиксировано
отдельным контрактом.

## Запуск прототипа

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

При сравнении сред следует использовать `benchmark_code_fingerprint_sha256` из `environment.tsv`. `repository_head_sha`
сохраняется только как контекст. `summary.tsv` сообщает `pass` только после успеха всех assertions соответствующего
case.

Ручной запуск как PID 1:

```bash
docker build -t metricshell-inv001:prototype research/INV-001/prototype
docker run --rm metricshell-inv001:prototype --http=:9090 -- /usr/local/bin/workload-trap
```

Ручной запуск под Docker init/Tini:

```bash
docker run --rm --init metricshell-inv001:prototype --http=:9090 -- /usr/local/bin/workload-trap
```

Ручной запуск в subreaper mode:

```bash
docker run --rm --init metricshell-inv001:prototype --http=:9090 --subreaper -- /prototype/workloads/double-fork.sh
```

## Ограничения прототипа

- Это research code, а не production MetricShell.
- Обе среды используют LinuxKit; native Linux без LinuxKit и Kubernetes runtime не проверены.
- Для Tini-compatible model использован Docker `--init`; отдельные init binaries не сравнивались.
- Timing values являются архитектурным evidence, а не performance guarantees.
- Поведение shell зависит от фактического shell и wrapper workload.

## Дополнительные benchmarks

Покрыты 30 повторов signal-to-exit, smoke samples остальных signal cases, явная доставка сигналов direct workload/shell
parent/grandchild, assertions для exit/readiness/reaping/post-exit metrics, raw events, environment metadata, CPU/RSS,
child churn 1k/10k, shutdown grace, сравнение PID 1/`--init`/subreaper и внешние post-exit scrapes.

Остаются открытыми heavy child churn 100k, отдельные Tini/dumb-init/s6 и повторы на native Linux и Kubernetes
Job/CronJob.

## Результат решения

- Прототип: `prototype/`
- Runner: `run-bench.sh`
- Evidence: `results/20260717T192610Z/`, `results/20260718T085124Z/`
- Отчёт: [report.md](report_ru.md)
- Вход для ADR: MetricShell по умолчанию работает как PID 1; compatibility mode под init требует subreaper для владения
  descendants.
