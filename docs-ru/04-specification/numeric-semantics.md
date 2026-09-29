# Числовая семантика

[English version](../../docs/04-specification/numeric-semantics.md)

> Статус: принятая нормативная спецификация
> Решения: ADR-004 и дополненная ADR-016

Семантика exposition и data model Prometheus является приоритетной. Абсолютные snapshot Core и материализация Managed
используют одинаковое представление; Managed operations добавляют только instrumentation-инварианты: неотрицательные
counter delta и атомарное histogram observation.

## Таблица аудита

| Метрика/тип             | Значение                                        | Семантика Prometheus                                                                               | Раньше в MetricShell                            | Требуемое/текущее действие                                      |
|-------------------------|-------------------------------------------------|----------------------------------------------------------------------------------------------------|-------------------------------------------------|-----------------------------------------------------------------|
| Counter total           | `0`, положительное finite                       | Валидно, монотонно не убывает (`OM`)                                                               | Принималось                                     | Принимать                                                       |
| Counter total           | `-0`                                            | Представимо на wire [P]; равно zero. `Counter.Add(-0)` принимает его как integer-zero no-op [CG-C] | Отклонялось                                     | Core сохраняет snapshot token; Managed normalize в `+0`         |
| Counter total           | `+Inf`                                          | Представимо [P], валидный terminal non-NaN total [OM]; `Counter.Add(+Inf)` принимает [CG-C]        | Отклонялось                                     | Принимать в Core, initialize и add; overflow даёт `+Inf` [CG-C] |
| Counter total           | `NaN`, отрицательное, `-Inf`                    | Представимо на wire [P], но нарушает typed counter semantics [OM]                                  | Отклонялось                                     | Атомарно отклонять                                              |
| Gauge                   | signed finite, `0`, `-0`, `NaN`, `+Inf`, `-Inf` | Валидные wire values (`P`, `OM`)                                                                   | Принималось                                     | Принимать и сохранять                                           |
| Histogram observation   | signed finite, `0`, `-0`                        | `Histogram.Observe` принимает [CG-H]; `_sum` может уменьшаться [PH]                                | Отрицательные отклонялись                       | Принимать                                                       |
| Histogram observation   | `NaN`                                           | Увеличивает count, делает sum `NaN`, не увеличивает configured buckets (`CG-H`, `NH`)              | Отклонялось                                     | Принимать; stored terminal `+Inf` следует exposition count      |
| Histogram observation   | `+Inf`, `-Inf`                                  | Принимается с IEEE-754 arithmetic (`CG-H`, `NH`)                                                   | Принималось только `+Inf`                       | Принимать                                                       |
| Histogram boundary      | signed finite, `0`, `-0`, `-Inf`                | Валидная ordered classic boundary (`P`, `CG-H`)                                                    | Отрицательные отклонялись                       | Принимать и сохранять                                           |
| Histogram boundary      | `+Inf`                                          | Обязательный terminal bucket [P]                                                                   | Принималось                                     | Требовать последней boundary                                    |
| Histogram boundary      | `NaN`                                           | Не имеет порядка и запрещена [OM]                                                                  | Отклонялось                                     | Атомарно отклонять                                              |
| Histogram count/buckets | unsigned cumulative integers                    | Cumulative; terminal `+Inf` равен count (`P`, `OM`)                                                | Принималось                                     | Сохранить                                                       |
| Histogram sum           | signed finite, `NaN`, `+Inf`, `-Inf`            | IEEE-754 sum (`CG-H`, `NH`); negative classic sum документирован (`PH`)                            | Остальные кроме non-negative/`+Inf` отклонялись | Принимать и сохранять                                           |

Три counter layer различаются. Core snapshot сохраняет token `-0` и принимает `+Inf`. Managed `counter_initialize`
принимает `-0`, но хранит canonical `+0`, и принимает `+Inf` как absolute non-NaN monotonic total. Managed
`counter_add` принимает `-0` как no-op и `+Inf`, как `client_golang`; finite overflow становится `+Inf`. Negative values,
`-Inf` и `NaN` отклоняются по typed counter/operation semantics, хотя generic text grammar может их передать.

Histogram observation атомарно обновляет count, все подходящие cumulative buckets и sum. В classic histogram
`client_golang` `NaN` не увеличивает configured buckets, а text encoder выводит terminal `+Inf` из total count.
MetricShell хранит terminal bucket явно, поэтому увеличивает только его; exposition эквивалентна. Sum следует IEEE-754.

## Правила форматов exposition

Core и Managed Registry сохраняют полное Prometheus-compatible состояние classic histogram. Затем exposition применяет
контракт выбранного формата:

- Prometheus text 0.0.4 всегда выводит `_sum`, включая negative, `NaN`, `+Inf` и `-Inf`.
- OpenMetrics text 1.0 выводит `_sum`, только если все thresholds неотрицательны, а sum не negative и не `NaN`.
  `+Inf` разрешён. При negative threshold или sum со значением negative, `NaN` либо `-Inf` `_sum` этого MetricPoint
  опускается. `_count` и все cumulative buckets, включая единственный terminal `+Inf`, сохраняются.

В OpenMetrics 1.0 Histogram Sum optional (`SHOULD`), но присутствующий Sum не может быть negative или `NaN` и обязан
отсутствовать при negative thresholds. Поэтому omission валидно представляет любое поддерживаемое Core состояние без
его изменения и без fallback. Encoding и проверка response-size завершаются до HTTP success headers, поэтому failure
не может создать partial response с Content-Type OpenMetrics 1.0.

## Источники и расхождение

- **[P]** [Prometheus exposition format](https://prometheus.io/docs/instrumenting/exposition_formats/) разрешает `NaN`, `+Inf`,
  `-Inf` и требует terminal classic `+Inf` bucket, равный count.
- **[PH]** [Руководство Prometheus по histogram](https://prometheus.io/docs/practices/histograms/#count-and-sum-of-observations)
  явно описывает отрицательные observations и уменьшение classic `_sum`.
- **[CG-C]** [`client_golang` `counter.Add`](https://github.com/prometheus/client_golang/blob/main/prometheus/counter.go)
  отклоняет только `v < 0`, обрабатывает `-0` как integer-zero no-op и выполняет binary64 addition.
- **[CG-H]** [`client_golang` `histogram.Observe`](https://github.com/prometheus/client_golang/blob/main/prometheus/histogram.go)
  для `NaN` не выбирает configured bucket, но добавляет значение к sum и увеличивает count.
- **[NH]** [Спецификация special observations](https://prometheus.io/docs/specs/native_histograms/#special-cases-of-observed-values)
  явно фиксирует: `NaN` увеличивает count, меняет sum по floating-point arithmetic и не входит в bucket.
- **[OM]** [OpenMetrics 1.0](https://github.com/prometheus/OpenMetrics/blob/v1.0.0/specification/OpenMetrics.md) требует
  special float values и non-NaN monotonic counters. Histogram Sum optional; присутствующий Sum не может быть negative
  или `NaN` и обязан отсутствовать при negative thresholds.

Malformed tokens, `NaN` boundaries, нарушенный порядок, decreasing bucket counts, count overflow и terminal bucket,
не равный count, отклоняются целиком без изменения Managed generation или последнего валидного Core snapshot.
