# INV-004 — Владение Metric State и семантика

[English version](README.md)

**Статус:** завершено; вывод пересмотрен после scope review

**Reference runs:** `results/20260723T073114Z`, `results/20260723T150118Z`

**Отчёт:** [report_ru.md](report_ru.md)

**Решение:** [ADR-004](../../docs-ru/06-architecture/adr/ADR-004.md)

## Вопрос

Какое представление metric state является минимально достаточным contract для runtime-wrapper, который транспортирует,
валидирует и публикует метрики, но не агрегирует значения между producers?

## Коррекция scope

Первоначально исследование рассматривало независимые operations и registries нескольких producers как возможные
product requirements. Это расширяло MetricShell до локального metrics aggregator с producer identity, epoch, sequence,
reconciliation, completeness и type-specific aggregation policies.

Project scope явно исключает локальную или распределённую агрегацию значений между producers и business metric design.
Functional requirements требуют сохранять типы и consistent state, но не требуют от MetricShell реализовывать
`increment`, `set` или `observe`. Workload и его libraries отвечают за один полный, не содержащий конфликтов application
snapshot.

Prototype и raw evidence сохраняются. Они показывают две независимые стоимости. Reconciliation требуется, если
MetricShell принимает изменяющие состояние operations. Producer ownership и aggregation policies требуются, если
MetricShell объединяет независимо принадлежащие registries. Обе возможности находятся вне scope MetricShell.

## Проверенные кандидаты

- complete snapshots;
- absolute values отдельных series;
- operations (`increment`, `set`, `observe`);
- hybrid operations и authoritative snapshot reconciliation.

## Эксперименты

Docker prototype выполняет 33 детерминированных scenarios для counters, gauges, histograms, duplicate series, type
conflicts, нескольких producers, ordering, dropped updates, producer/receiver restarts, stale data и reconciliation.
Он также измеряет candidates при 1/4/16 producers и 1/100/1 000/10 000 series и выполняет 30 repetitions.

Эти scenarios намеренно исследуют superset project scope. Multi-producer aggregation и operation ownership являются
доказательством сложности контрфактической модели, а не требованиями production protocol.

## Результаты

Оба reference runs записали 33 scenarios, 29 подтверждённых invariants и четыре ожидаемых counterexamples, без
per-scenario failures и со 129 benchmark rows. Общий fingerprint:
`e52784470ff33e35fb58ab142be26a345bb6a373bd2eac58666b269f56875fd6`.

| Environment       | Result set                 | Docker platform | Assertions | Snapshot p50 | Operation p50 | Hybrid p50 |
|-------------------|----------------------------|-----------------|-----------:|-------------:|--------------:|-----------:|
| macOS / LinuxKit  | `results/20260723T073114Z` | `linux/aarch64` | 34/34 pass |     28 108/s |       4,48M/s |    4,31M/s |
| Ubuntu / LinuxKit | `results/20260723T150118Z` | `linux/x86_64`  | 34/34 pass |      6 402/s |       2,21M/s |    1,97M/s |

Для исправленного scope существенны следующие результаты:

- complete snapshots восстанавливают полное состояние после dropped или superseded промежуточной публикации;
- отсутствие series в новом complete snapshot детерминированно удаляет stale series;
- operations не восстанавливают dropped increment или потерянный in-memory receiver state;
- hybrid recovery работает, но требует sequencing, gap detection, authoritative snapshots и reconciliation;
- автоматическая работа с несколькими owners требует aggregation counter/histogram и collision policy для gauge;
- стоимость snapshot зависит от cardinality registry: synthetic case 16 producers / 10 000 series выделял около 3,82 MB
  на complete update;
- при 4 producers / 1 000 series hybrid reconciliation занимал 85,6%, 35,7% и 5,9% времени для intervals 100, 1 000 и
  10 000.

Performance numbers сравнивают research models и не являются production SLO. Operation path быстрее, потому что
выполняет меньше работы на update; hybrid сохраняет стоимость snapshot для исправления operation loss.

## Вывод

Reconciliation требуется, если MetricShell принимает изменяющие состояние operations. Producer ownership и aggregation
policies требуются, если MetricShell объединяет независимо принадлежащие registries. Обе возможности находятся вне scope
MetricShell.

Поэтому выбрана production model:

```text
workload/library
    -> один полный, не содержащий конфликтов application snapshot
    -> file | Unix stream | local HTTP
    -> structural validation
    -> atomic last-valid replacement
    -> Prometheus exposition
    -> frozen final snapshot
```

Все transports передают эквивалентные complete snapshots. MetricShell сохраняет representation counter, gauge и
histogram, но не применяет `increment`, `set` или `observe`, не контролирует business monotonicity между snapshots, не
отслеживает producer epochs, не восстанавливает missing operations и не агрегирует конфликтующие series.

## Допустимые семантические значения

- authoritative unit: один полный application snapshot;
- producer coordination: принадлежит workload или его libraries и находится вне MetricShell;
- initial state: zero-series application snapshot;
- валидный zero-series snapshot: очищает все active application series;
- отсутствующий или пустой transport payload: malformed input, а не zero-series snapshot;
- series отсутствует в новом принятом snapshot: она отсутствует в новом active state;
- counter: структурно валидное Prometheus representation внутри одного snapshot;
- gauge: absolute value внутри одного snapshot;
- histogram: упорядоченные boundaries, cumulative buckets, обязательный `+Inf`, равный `count`, и numeric `sum`;
- duplicate series, type conflict или внутренний metadata conflict candidate: атомарно отклонить candidate целиком;
- установленная привязка name-to-type family: сохраняется после omission до следующего workload execution;
- прочая metadata: проверяется на внутреннюю согласованность в candidate, но не фиксируется на весь execution;
- invalid candidate: сохранить предыдущий валидный snapshot;
- success acknowledgement socket/HTTP: candidate атомарно установлен в одном линейном acceptance order;
- producer timestamp: не является источником ordering или freshness;
- cross-snapshot monotonicity counter/histogram: MetricShell не проверяет;
- cross-producer aggregation: не выполняется;
- final state: последний валидный принятый полный application snapshot либо первоначальный zero-series snapshot, если
  публикаций не было.

## Запуск prototype

Из корня repository на macOS или Ubuntu с Docker:

```bash
./research/INV-004/run-bench.sh
```

Просмотр evidence:

```bash
latest="$(cat research/INV-004/latest-results.txt)"
cat "$latest/summary.tsv"
cat "$latest/assertions.tsv"
cat "$latest/semantics.tsv"
cat "$latest/benchmark-stats.tsv"
cat "$latest/environment.tsv"
cat "$latest/coverage.tsv"
```

Prototype сохраняет operation и hybrid benchmark modes, потому что raw experiments остаются доказательством границы
scope.

## Cross-environment fingerprint

Runner хеширует normalized relative names и содержимое `prototype/` и `run-bench.sh`; host path, repository HEAD,
timestamps и results не входят. Оба run дали fingerprint
`e52784470ff33e35fb58ab142be26a345bb6a373bd2eac58666b269f56875fd6`.

## Ограничения prototype

- Это in-memory semantic model, а не production parsing, persistence или transport implementation.
- Measurements включают Go map/string allocation и Docker startup per sample; это comparative evidence.
- Оба environments используют LinuxKit. Native non-LinuxKit Linux, containerd/CRI-O и Kubernetes не проверены.
- Поле `container_go_version` содержит help banner prototype и исключено из сравнения evidence.
- Ветки producer ownership и aggregation в prototype превышают текущий product scope.

## Результат решения

- Prototype и runner сохранены без изменения как research evidence.
- Raw evidence: `results/20260723T073114Z/`, `results/20260723T150118Z/`.
- Подробный отчёт: [report_ru.md](report_ru.md).
- Решение: [ADR-004](../../docs-ru/06-architecture/adr/ADR-004.md) — complete application snapshots, atomic last-valid
  replacement и отсутствие cross-producer aggregation.
