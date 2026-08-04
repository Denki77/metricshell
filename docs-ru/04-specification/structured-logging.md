# Спецификация structured logs

[English version](../../docs/04-specification/structured-logging.md)

> Статус: принятая нормативная спецификация
> Требование: FR-052
> Критерии приёмки: AC-OBS-001–AC-OBS-003, AC-CONF-005
> Решения: ADR-001–ADR-015

## Назначение

Документ определяет stable schema и обязательный event registry для логов MetricShell. Stdout/stderr workload передаются
без изменений и не считаются соответствующими этой schema.

## Encoding и output

- Один UTF-8 JSON object на строку (JSON Lines).
- Логи MetricShell всегда пишутся в stderr в version 1.
- ANSI color и multiline records запрещены.
- Timestamp — UTC RFC3339Nano.
- Field names — lowercase `snake_case`.
- `schema_version` первоначально равен `"1"`.

`log.level=info` эмитит все registry events уровней info, warn и error. `log.level=debug` дополнительно эмитит все debug
events и не подавляет более высокие levels. В version 1 нет значений `warn`, `error`, `off` и настройки destination,
поэтому обязательные lifecycle/rejection records нельзя отключить.

## Обязательные base fields

| Field            | Type    | Значение                                                                                              |
|------------------|---------|-------------------------------------------------------------------------------------------------------|
| `timestamp`      | string  | UTC RFC3339Nano.                                                                                      |
| `sequence`       | integer | Monotonic process-local sequence.                                                                     |
| `schema_version` | string  | Версия schema.                                                                                        |
| `level`          | string  | `debug`, `info`, `warn`, `error`.                                                                     |
| `event`          | string  | Stable event identifier.                                                                              |
| `component`      | string  | `runtime`, `workload`, `ingestion`, `file`, `socket`, `http`, `exposition`, `final_wait`, `shutdown`. |
| `runtime_id`     | string  | ID одного процесса MetricShell.                                                                       |
| `state`          | string  | Effective runtime state.                                                                              |
| `message`        | string  | Краткое сообщение без raw payload.                                                                    |

Внутри процесса authoritative order задаёт `sequence`, а не только timestamp.

## Common optional fields

| Field                 | Type    | Назначение                                                                                   |
|-----------------------|---------|----------------------------------------------------------------------------------------------|
| `duration_ms`         | number  | Duration завершённой операции.                                                               |
| `deadline`            | string  | Absolute deadline в UTC RFC3339Nano.                                                         |
| `remaining_ms`        | integer | Оставшееся bounded time в момент оценки.                                                     |
| `pid`                 | integer | PID MetricShell.                                                                             |
| `workload_pid`        | integer | PID primary workload.                                                                        |
| `workload_pgid`       | integer | Process-group ID workload.                                                                   |
| `exit_code`           | integer | Разрешённый process-compatible result.                                                       |
| `signal`              | string  | Stable signal name, например `TERM`.                                                         |
| `forced`              | boolean | Применялось ли forced termination.                                                           |
| `transport`           | string  | `file`, `unix` или `http`.                                                                   |
| `snapshot_generation` | integer | Accepted internal generation.                                                                |
| `snapshot_bytes`      | integer | Candidate/canonical bytes, если их безопасно записать.                                       |
| `series`              | integer | Число application series в candidate/active snapshot.                                        |
| `reason`              | string  | Bounded event-specific reason enum.                                                          |
| `previous_state`      | string  | Public state до transition.                                                                  |
| `mode`                | string  | Final-wait mode из registry self-metrics.                                                    |
| `kind`                | string  | Child kind из registry self-metrics.                                                         |
| `trigger`             | string  | Trigger file reconciliation.                                                                 |
| `outcome`             | string  | Event-specific outcome из registry self-metrics.                                             |
| `watch_event`         | string  | File-watch event из registry self-metrics.                                                   |
| `suppressed_event`    | string  | Event identifier в suppression summary.                                                      |
| `suppressed_count`    | integer | Число suppressed records в окне.                                                             |
| `window_ms`           | integer | Duration suppression window.                                                                 |
| `http_status`         | integer | HTTP status, созданный MetricShell.                                                          |
| `publication_id`      | string  | Socket correlation ID; только debug и bounded length.                                        |
| `request_id`          | string  | Созданный MetricShell request ID; только debug.                                              |
| `error_code`          | string  | Stable machine-readable internal code.                                                       |
| `error_message`       | string  | Sanitized summary ошибки.                                                                    |
| `selectors`           | array   | Нормализованные filtering selectors; только в `configuration.validated` при явном включении. |

Consumers обязаны игнорировать неизвестные fields. Type существующего field не меняется в schema `1`.

## Обязательный event registry

| Event                         | Min level | Required fields                                                               | Когда                               |
|-------------------------------|-----------|-------------------------------------------------------------------------------|-------------------------------------|
| `runtime.initializing`        | info      | `pid`                                                                         | Один раз после logger init.         |
| `runtime.state_changed`       | info      | `previous_state`, кроме initial transition                                    | Каждый public state transition.     |
| `configuration.validated`     | info      | —                                                                             | До workload start, без secrets.     |
| `configuration.rejected`      | error     | `reason`, `error_code`                                                        | Terminal invalid config.            |
| `endpoint.bound`              | info      | `component`                                                                   | Required listener/socket создан.    |
| `endpoint.bind_failed`        | error     | `component`, `reason`, `error_code`                                           | Bind failure.                       |
| `workload.starting`           | info      | —                                                                             | Перед start attempt.                |
| `workload.started`            | info      | `workload_pid`, `workload_pgid`                                               | После успешного start.              |
| `workload.start_failed`       | error     | `reason`, `error_code`                                                        | Start failure.                      |
| `workload.signal_forwarded`   | info      | `signal`, PID/PGID                                                            | Каждый forwarded signal.            |
| `workload.exited`             | info      | `exit_code`, `forced`                                                         | Только один раз после resolution.   |
| `child.reaped`                | debug     | `kind`                                                                        | Reaped managed child.               |
| `snapshot.accepted`           | debug     | `transport`, `snapshot_generation`, `snapshot_bytes`, `series`, `duration_ms` | После atomic installation.          |
| `snapshot.rejected`           | warn      | `transport`, `reason`, `duration_ms`                                          | Каждый candidate rejection.         |
| `ingestion.overloaded`        | warn      | `transport`, `outcome`                                                        | Queue/semaphore/connection refusal. |
| `file.reconciled`             | debug     | `trigger`, `outcome`, `duration_ms`                                           | Reconciliation.                     |
| `file.watch_recovered`        | warn      | `watch_event`                                                                 | Overflow/invalidation/reinstall.    |
| `socket.transaction_expired`  | warn      | `reason`                                                                      | Multipart expiry.                   |
| `exposition.failed`           | warn      | `outcome`, `http_status`                                                      | Encoding/limit/write failure.       |
| `final_wait.started`          | info      | `mode`; `deadline` для duration/scrapes                                       | После freeze final snapshot.        |
| `final_scrape.counted`        | debug     | `request_id`, `snapshot_generation`                                           | Eligible complete response counted. |
| `final_scrape.not_counted`    | debug     | `request_id`, `outcome`                                                       | Ineligible/cancelled/failed.        |
| `final_wait.completed`        | info      | `reason`, `duration_ms`                                                       | Только один terminal condition.     |
| `shutdown.started`            | info      | `signal`, `deadline`, `remaining_ms`                                          | External termination.               |
| `shutdown.forced`             | warn      | `signal`, PID/PGID                                                            | Grace expired.                      |
| `shutdown.completed`          | info      | `duration_ms`, `exit_code`                                                    | Shutdown phases finished.           |
| `runtime.failed`              | error     | `reason`, `error_code`                                                        | Unrecoverable failure.              |
| `runtime.terminated`          | info      | `exit_code`, `duration_ms`                                                    | Последний MetricShell record.       |
| `logging.suppression_summary` | warn      | `suppressed_event`, `suppressed_count`, `window_ms`; optional `reason`        | Summary rate-limited records.       |

High-frequency success events имеют debug level; rejection и lifecycle boundaries видны на normal levels.

При `log.selector_values=true` событие `configuration.validated` может содержать normalized array `selectors`. При
значении false field отсутствует. Другие events не содержат selector values.

## Закрытые field/error registries

Shared fields используют точные значения self-metrics: `kind=direct|adopted`; file `trigger` и `outcome`;
`watch_event` использует значения self-metric `event`; final-wait `mode` и completion `reason`; snapshot rejection
`reason`; socket-frame `reason`; exposition `outcome`. `ingestion.overloaded` использует `outcome=busy`. Нельзя заменять
event-specific field общим `reason`.

Следующая таблица является единственным нормативным mapping для process failures MetricShell:

| Failure symbol          | Exit | Runtime failure reason | Structured event         | `error_code`            |
|-------------------------|-----:|------------------------|--------------------------|-------------------------|
| `configuration_invalid` |   64 | `configuration`        | `configuration.rejected` | `CONFIG_INVALID`        |
| `internal_failure`      |   70 | `internal`             | `runtime.failed`         | `INTERNAL_FAILURE`      |
| `resource_unavailable`  |   71 | `resource`             | `runtime.failed`         | `RESOURCE_UNAVAILABLE`  |
| `endpoint_bind_failed`  |   72 | `bind`                 | `endpoint.bind_failed`   | `BIND_FAILED`           |
| `workload_start_failed` |   73 | `workload_start`       | `workload.start_failed`  | `WORKLOAD_START_FAILED` |

Non-process operational failures дополнительно используют closed codes `SNAPSHOT_MALFORMED`, `SNAPSHOT_LIMIT`,
`INGESTION_BUSY`, `SOCKET_PROTOCOL` и `FINAL_WAIT_TIMEOUT`. Другие error codes schema version 1 невалидны. Raw external
error допустим только в sanitized `error_message` и не становится enum value.

## Privacy

По умолчанию нельзя логировать:

- значения environment variables;
- полный command argv;
- metric payloads;
- значения metrics или labels;
- credentials, tokens, headers или cookies;
- raw file contents;
- произвольные remote addresses;
- unbounded parser errors.

Известные secret configuration keys заменяются на `[REDACTED]`. Newline, tab и control characters во внешнем error text
экранируются JSON encoding. Error messages обрезаются после sanitization.

## Size/rate limits

- Maximum encoded event: `16KiB`.
- `message`/`error_message`: максимум `4KiB`.
- При truncation добавляется `truncated: true`.
- Одинаковые warn/error по `event + reason`: 10 records/s, burst 20.
- Первый suppressed event остаётся наблюдаемым; `logging.suppression_summary` выпускается не реже раза в 60 секунд во
  время suppression и один раз при закрытии окна.
- Lifecycle terminal events не rate-limit.

## State transitions

Каждый externally observable state transition ровно один раз создаёт `runtime.state_changed` после вступления нового state в силу и до завершения state-specific work.
Record содержит новый `state` и `previous_state`, кроме initial transition.

## Примеры

```json
{
  "timestamp": "2026-08-04T09:30:00.123456789Z",
  "sequence": 12,
  "schema_version": "1",
  "level": "info",
  "event": "workload.started",
  "component": "workload",
  "runtime_id": "7bd6c74b",
  "state": "running",
  "message": "workload started",
  "workload_pid": 42,
  "workload_pgid": 42
}
```

```json
{
  "timestamp": "2026-08-04T09:31:00.000000000Z",
  "sequence": 58,
  "schema_version": "1",
  "level": "info",
  "event": "final_wait.completed",
  "component": "final_wait",
  "runtime_id": "7bd6c74b",
  "state": "final_wait",
  "message": "final wait completed",
  "reason": "required_scrapes",
  "duration_ms": 842.7,
  "exit_code": 17
}
```

```json
{
  "timestamp": "2026-08-04T09:30:03.100000000Z",
  "sequence": 31,
  "schema_version": "1",
  "level": "warn",
  "event": "snapshot.rejected",
  "component": "ingestion",
  "runtime_id": "7bd6c74b",
  "state": "running",
  "message": "candidate snapshot rejected",
  "transport": "unix",
  "reason": "duplicate_series",
  "snapshot_bytes": 8120,
  "duration_ms": 1.42
}
```

## Compatibility

Удаление/rename event, field или изменение field type требует major schema version. Добавление optional fields backward
compatible. Consumers обязаны принимать unknown events/fields.

## Conformance

Тесты проверяют JSON Lines, mandatory fields, monotonic sequence, UTC timestamps, event-specific fields, redaction,
truncation, rate limits, exactly-once terminal events, отсутствие raw payload и совпадение reason enums с self-metrics.
Matrix покрывает info/debug, selector-values false/true, все пять rows process-failure mapping и каждый supplementary
operational error code.

## Ссылки

- [Функциональные требования](../03-requirements/functional-requirements.md)
- [Нефункциональные требования](../03-requirements/non-functional-requirements.md)
- [Configuration](configuration.md)
- [Runtime defaults и ограничения ресурсов](runtime-defaults-and-resource-limits.md)
- [Self-metrics](self-metrics.md)
- [Runtime State Machine](runtime-state-machine.md)
