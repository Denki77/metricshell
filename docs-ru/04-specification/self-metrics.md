# Спецификация self-metrics MetricShell

[English version](../../docs/04-specification/self-metrics.md)

> Статус: принятая нормативная спецификация
> Требование: FR-051
> Критерии приёмки: AC-EXP-003, AC-OBS-001–AC-OBS-003
> Решения: ADR-001–ADR-015

## Назначение

Спецификация определяет стабильный namespace self-metrics MetricShell, имена, типы, labels и правила обновления. Эти
метрики описывают только работу MetricShell и никогда не содержат labels или values application metrics.

## Общие правила

- Все имена начинаются с `metricshell_`.
- `metricshell_` зарезервирован; application candidate с этим prefix отклоняется как `reserved_name`.
- Counters имеют suffix `_total` и сбрасываются при запуске нового процесса MetricShell.
- Duration измеряется в seconds, size — в bytes, deadline — Unix timestamp seconds.
- Для каждой metric обязательны HELP и TYPE.
- Во время final wait application snapshot frozen, а self-metrics продолжают изменяться.
- Application filters не удаляют self-metrics.
- Labels используют только bounded enums ниже.
- Raw errors, paths, metric/label values, request/publication IDs, PIDs и client addresses запрещены в metric labels.

## Build и runtime

| Metric                               | Type    | Labels                | Семантика                                |
|--------------------------------------|---------|-----------------------|------------------------------------------|
| `metricshell_build_info`             | gauge   | `version`, `revision` | Одна series со значением `1`.            |
| `metricshell_uptime_seconds`         | gauge   | —                     | Время работы процесса.                   |
| `metricshell_runtime_state`          | gauge   | `state`               | One-hot vector всех известных states.    |
| `metricshell_runtime_failures_total` | counter | `reason`              | Unrecoverable failures по bounded class. |

States:

```text
initializing
starting_workload
running
stopping
finalizing
final_wait
failed
terminated
```

Runtime failure reasons:

```text
configuration
bind
workload_start
protocol
resource
internal
```

## Workload process

| Metric                                           | Type    | Labels             | Семантика                                |
|--------------------------------------------------|---------|--------------------|------------------------------------------|
| `metricshell_workload_running`                   | gauge   | —                  | `1` пока primary workload running.       |
| `metricshell_workload_process_id`                | gauge   | —                  | PID workload либо `0`.                   |
| `metricshell_workload_starts_total`              | counter | `outcome`          | Start attempts.                          |
| `metricshell_workload_exit_code`                 | gauge   | —                  | Resolved result; `-1` пока не определён. |
| `metricshell_workload_signals_forwarded_total`   | counter | `signal`, `target` | Пересланные signals.                     |
| `metricshell_workload_forced_terminations_total` | counter | —                  | Forced SIGKILL после grace.              |
| `metricshell_children_reaped_total`              | counter | `kind`             | Reaped direct/adopted children.          |

```text
outcome = started | start_failed
signal  = TERM | INT | HUP | QUIT | KILL
target  = process | process_group
kind    = direct | adopted
```

## Snapshot и ingestion

| Metric                                                 | Type    | Labels                 | Семантика                                                |
|--------------------------------------------------------|---------|------------------------|----------------------------------------------------------|
| `metricshell_snapshot_generation`                      | gauge   | —                      | Accepted generation; initial zero-series snapshot = `0`. |
| `metricshell_snapshot_series`                          | gauge   | —                      | Application series до filtering.                         |
| `metricshell_snapshot_bytes`                           | gauge   | —                      | Canonical uncompressed bytes.                            |
| `metricshell_snapshot_publications_total`              | counter | `transport`, `outcome` | Outcomes complete publications.                          |
| `metricshell_snapshot_rejections_total`                | counter | `transport`, `reason`  | Atomic rejections.                                       |
| `metricshell_ingestion_inflight`                       | gauge   | `transport`            | Current ingestion operations.                            |
| `metricshell_ingestion_connections`                    | gauge   | `transport`            | Active accepted connections.                             |
| `metricshell_ingestion_last_success_timestamp_seconds` | gauge   | `transport`            | Timestamp последнего accepted snapshot либо `0`.         |

```text
transport = file | unix | http
outcome   = accepted | rejected | busy | timeout | internal_error
```

Rejection reasons:

```text
malformed
numeric_invalid
schema_version
empty_payload
payload_limit
series_limit
label_limit
name_limit
policy
duplicate_series
type_conflict
metadata_conflict
histogram_invalid
reserved_name
frozen
internal
```

Reason — fixed whole-candidate registry из application snapshot protocol. `busy` и `timeout` являются publication
outcomes, а framing failures используют socket-frame registry ниже; ни одна категория не добавляется в этот label. Raw
parser text в labels запрещён.

## File/socket diagnostics

| Metric                                     | Type    | Labels               | Семантика                                                    |
|--------------------------------------------|---------|----------------------|--------------------------------------------------------------|
| `metricshell_file_reconciliations_total`   | counter | `trigger`, `outcome` | Попытки reconciliation состояния файла.                      |
| `metricshell_file_watch_events_total`      | counter | `event`              | Overflow, invalidation и успешная переустановка watch.       |
| `metricshell_socket_transactions_inflight` | gauge   | —                    | Число удерживаемых bounded multipart candidate transactions. |
| `metricshell_socket_frames_rejected_total` | counter | `reason`             | Отклонённые frames, которые не установили snapshot.          |

```text
trigger = startup | event | periodic | overflow | watch_reinstall
outcome = accepted | unchanged | absent | invalid | error
event   = overflow | invalidated | reinstalled
reason  = malformed | protocol_version | frame_limit | part_limit | duplicate_part | missing_part | transaction_invalid | transaction_expired
```

## Filtering

| Metric                        | Type  | Labels    | Семантика                                                       |
|-------------------------------|-------|-----------|-----------------------------------------------------------------|
| `metricshell_filter_rules`    | gauge | `kind`    | Число effective normalized правил include/exclude.              |
| `metricshell_filter_families` | gauge | `outcome` | Число включённых/исключённых families текущего exposition view. |

```text
kind    = include | exclude
outcome = included | excluded
```

## Exposition

| Metric                                  | Type    | Labels              | Семантика                                    |
|-----------------------------------------|---------|---------------------|----------------------------------------------|
| `metricshell_exposition_requests_total` | counter | `format`, `outcome` | Result `/metrics` после завершения handler.  |
| `metricshell_exposition_inflight`       | gauge   | —                   | Active handlers.                             |
| `metricshell_exposition_response_bytes` | gauge   | `format`            | Последний полный uncompressed response size. |

```text
format  = prometheus | openmetrics
outcome = success | write_error | response_limit | encoding_error | timeout
```

`success` увеличивается только после полного write без cancelled context.

## Final wait

| Metric                                              | Type    | Labels    | Семантика                                            |
|-----------------------------------------------------|---------|-----------|------------------------------------------------------|
| `metricshell_final_wait_active`                     | gauge   | —         | `1` во время final wait после natural completion.    |
| `metricshell_final_wait_mode_info`                  | gauge   | `mode`    | Constant `1` для effective final-wait mode.          |
| `metricshell_final_wait_required_scrapes`           | gauge   | —         | Effective required scrape count.                     |
| `metricshell_final_wait_completed_scrapes`          | gauge   | —         | Saturating count eligible completed responses.       |
| `metricshell_final_scrape_attempts_total`           | counter | `outcome` | Попытки scrape final state после завершения handler. |
| `metricshell_final_wait_completions_total`          | counter | `reason`  | Terminal reason final wait.                          |
| `metricshell_final_wait_deadline_timestamp_seconds` | gauge   | —         | Absolute timeout deadline или `0` вне timed wait.    |

```text
mode    = immediate | duration | scrapes
outcome = completed | ineligible | write_error | cancelled
reason  = immediate | duration_elapsed | required_scrapes | timeout | external_termination | runtime_failure
```

Health/readiness/debug endpoints не увеличивают final scrape attempts.

## Shutdown

| Metric                                            | Type      | Labels  | Семантика                                                        |
|---------------------------------------------------|-----------|---------|------------------------------------------------------------------|
| `metricshell_shutdown_active`                     | gauge     | —       | `1` после начала external termination и до process exit.         |
| `metricshell_shutdown_deadline_timestamp_seconds` | gauge     | —       | Effective absolute deadline либо `0`, если неизвестен/неактивен. |
| `metricshell_shutdown_phase_duration_seconds`     | histogram | `phase` | Duration завершённых shutdown phases.                            |

Допустимые phases:

```text
workload_wait
forced_termination
finalization
http_drain
total
```

Начальные histogram buckets:

```text
0.001, 0.005, 0.01, 0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10, 30, 60
```

## Cardinality и lifecycle

- Все label enums закрыты, кроме build `version` и `revision`.
- State/mode metrics публикуют полный enum set, одна series равна `1`.
- Generation увеличивается только после atomic installation, включая valid zero-series snapshot.
- Rejection не меняет generation/series/bytes.
- Все metrics process-local и сбрасываются при новом execution.

## Compatibility

Rename/remove metric или label — breaking change. Новый enum value — объявляемый compatibility change. Experimental
metrics используют `metricshell_experimental_`.

## Conformance

Тесты проверяют HELP/TYPE, exact names, bounded labels, reset, one-hot, generation, last-valid retention, live
final-wait
self-metrics, immunity to filtering и parsing в Prometheus/OpenMetrics.

## Ссылки

- [Функциональные требования](../03-requirements/functional-requirements.md)
- [ADR-010](../06-architecture/adr/ADR-010.md)
- [ADR-011](../06-architecture/adr/ADR-011.md)
- [ADR-014](../06-architecture/adr/ADR-014.md)
- [Спецификация фильтрации](metrics-filtering.md)
