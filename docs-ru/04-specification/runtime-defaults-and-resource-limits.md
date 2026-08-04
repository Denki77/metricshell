# Спецификация defaults и resource limits

[English version](../../docs/04-specification/runtime-defaults-and-resource-limits.md)

> Статус: принятая нормативная спецификация
> Требования: FR-024, FR-046, FR-052, FR-080, FR-081, FR-082
> Критерии приёмки: AC-ING-008, AC-MET-007, AC-MET-008, AC-FIN-004, AC-FIN-006–AC-FIN-008, AC-CONF-002–AC-CONF-004
> Решения: ADR-003, ADR-006, ADR-007, ADR-008, ADR-010, ADR-011, ADR-014, ADR-015

## Назначение

Документ выбирает детерминированные defaults первой стабильной версии и безопасные диапазоны ожиданий, ingestion,
exposition и runtime resources. Значения являются product defaults, но не SLO и не универсальными capacity guarantees.

## Синтаксис значений

Все durations, byte sizes, counts и environment lists используют принятую
[грамматику значений конфигурации](configuration-value-grammar.md). Таблицы ниже задают семантические диапазоны, но не
заменяют и не сужают лексическую грамматику. Невалидные, отрицательные, overflow, out-of-range и противоречивые значения
отклоняются до запуска workload, когда это возможно.

## Defaults естественного завершения

| Property                      | Environment                               |   Default |                          Допустимо | Значение                                      |
|-------------------------------|-------------------------------------------|----------:|-----------------------------------:|-----------------------------------------------|
| `final_wait.mode`             | `METRICSHELL_FINAL_WAIT_MODE`             | `scrapes` | `immediate`, `duration`, `scrapes` | Post-workload wait после natural completion.  |
| `final_wait.duration`         | `METRICSHELL_FINAL_WAIT_DURATION`         |     `30s` |                           `0`–`1h` | Только для `duration`.                        |
| `final_wait.timeout`          | `METRICSHELL_FINAL_WAIT_TIMEOUT`          |     `60s` |                          `1s`–`1h` | Обязательная граница `scrapes`.               |
| `final_wait.required_scrapes` | `METRICSHELL_FINAL_WAIT_REQUIRED_SCRAPES` |       `1` |                           `1`–`16` | Число eligible complete final responses.      |
| `final_wait.completion_grace` | `METRICSHELL_FINAL_WAIT_COMPLETION_GRACE` |   `500ms` |                           `0`–`5s` | Drain уже принятых responses после threshold. |

Default `scrapes` следует ADR-011. Старый draft-default `delay` отменяется. `30s` остаётся default только при явном
`duration` mode. После начала external termination новый post-exit wait не запускается.

## Defaults external termination

| Property                    | Environment                             | Default |    Допустимо | Валидация                                                      |
|-----------------------------|-----------------------------------------|--------:|-------------:|----------------------------------------------------------------|
| `shutdown.total_grace`      | `METRICSHELL_SHUTDOWN_TOTAL_GRACE`      |   `30s` |    `1s`–`1h` | Должен быть равен или больше external runtime grace            |
| `shutdown.workload_timeout` | `METRICSHELL_WORKLOAD_SHUTDOWN_TIMEOUT` |   `28s` |     `0`–`1h` | Ограничен оставшимся абсолютным deadline                       |
| `shutdown.reserve`          | `METRICSHELL_SHUTDOWN_RESERVE`          |    `2s` | `250ms`–`1m` | Зарезервировано для завершения работы MetricShell и HTTP drain |

`workload_timeout + reserve` не может превышать `total_grace`. Absolute deadline является authoritative и может сократить effective workload timeout до нуля.

## File ingestion

| Property                  | Environment                           | Default |    Допустимо |
|---------------------------|---------------------------------------|--------:|-------------:|
| `file.reconcile_interval` | `METRICSHELL_FILE_RECONCILE_INTERVAL` |    `1s` | `100ms`–`1s` |

Periodic reconciliation нельзя отключить.

## Snapshot и validation

| Property                       | Environment                             | Default |        Допустимо |
|--------------------------------|-----------------------------------------|--------:|-----------------:|
| `limits.snapshot_bytes`        | `METRICSHELL_MAX_SNAPSHOT_BYTES`        |  `1MiB` |  `64KiB`–`64MiB` |
| `limits.decoded_input_bytes`   | `METRICSHELL_MAX_DECODED_INPUT_BYTES`   |  `2MiB` | `64KiB`–`128MiB` |
| `limits.series`                | `METRICSHELL_MAX_SERIES`                | `10000` |     `1`–`100000` |
| `limits.labels_per_series`     | `METRICSHELL_MAX_LABELS_PER_SERIES`     |     `8` |         `0`–`64` |
| `limits.metric_name_bytes`     | `METRICSHELL_MAX_METRIC_NAME_BYTES`     |   `256` |       `1`–`1024` |
| `limits.label_name_bytes`      | `METRICSHELL_MAX_LABEL_NAME_BYTES`      |   `128` |       `1`–`1024` |
| `limits.label_value_bytes`     | `METRICSHELL_MAX_LABEL_VALUE_BYTES`     |  `1024` |      `1`–`16KiB` |
| `limits.help_bytes`            | `METRICSHELL_MAX_HELP_BYTES`            |  `4KiB` |      `0`–`64KiB` |
| `limits.concurrent_ingestions` | `METRICSHELL_MAX_CONCURRENT_INGESTIONS` |     `4` |         `1`–`64` |
| `limits.pending_ingestions`    | `METRICSHELL_MAX_PENDING_INGESTIONS`    |     `0` |         `0`–`64` |

`pending_ingestions=0` означает fail-fast overload rejection. Нарушение любого лимита атомарно отклоняет candidate.

## Unix socket

| Property                     | Environment                              | Default |      Допустимо |
|------------------------------|------------------------------------------|--------:|---------------:|
| `socket.frame_bytes`         | `METRICSHELL_SOCKET_MAX_FRAME_BYTES`     |  `8KiB` | `1KiB`–`64KiB` |
| `socket.parts`               | `METRICSHELL_SOCKET_MAX_PARTS`           |   `256` |     `1`–`1024` |
| `socket.connections`         | `METRICSHELL_SOCKET_MAX_CONNECTIONS`     |     `8` |       `1`–`64` |
| `socket.transactions`        | `METRICSHELL_SOCKET_MAX_TRANSACTIONS`    |     `4` |       `1`–`32` |
| `socket.transaction_timeout` | `METRICSHELL_SOCKET_TRANSACTION_TIMEOUT` |    `5s` |   `100ms`–`1m` |
| `socket.read_timeout`        | `METRICSHELL_SOCKET_READ_TIMEOUT`        |    `5s` |   `100ms`–`1m` |
| `socket.write_timeout`       | `METRICSHELL_SOCKET_WRITE_TIMEOUT`       |    `5s` |   `100ms`–`1m` |

Для part с index `i` определяется консервативная decoded payload capacity:

```text
payload_chars(i) = socket.frame_bytes
  - byte_len("MSP/1 SNAPSHOT_PART ")
  - 64                         # максимальные bytes publication-id
  - 1 - decimal_digits(i) - 1 # separators и index
  - 1                          # LF
decoded_part_capacity(i) = floor(payload_chars(i) * 3 / 4)
effective_socket_decoded_capacity = sum(decoded_part_capacity(i), i=0..socket.parts-1)
```

Отрицательное `payload_chars` означает нулевую capacity. Startup требует
`effective_socket_decoded_capacity >= limits.snapshot_bytes`. Default configuration `8KiB × 256` выполняет invariant
после учёта worst-case MSP/1 overhead и unpadded base64url expansion. Assembled input ограничивается
`limits.decoded_input_bytes`, а canonical form — `limits.snapshot_bytes`.

## Local HTTP ingestion

| Property                             | Environment                                 | Default |        Допустимо |
|--------------------------------------|---------------------------------------------|--------:|-----------------:|
| `http_ingestion.wire_bytes`          | `METRICSHELL_HTTP_INGESTION_MAX_WIRE_BYTES` |  `2MiB` | `64KiB`–`128MiB` |
| `http_ingestion.read_header_timeout` | `METRICSHELL_HTTP_READ_HEADER_TIMEOUT`      |    `2s` |    `100ms`–`30s` |
| `http_ingestion.read_timeout`        | `METRICSHELL_HTTP_READ_TIMEOUT`             |   `10s` |     `100ms`–`1m` |
| `http_ingestion.write_timeout`       | `METRICSHELL_HTTP_WRITE_TIMEOUT`            |   `30s` |     `100ms`–`2m` |
| `http_ingestion.idle_timeout`        | `METRICSHELL_HTTP_IDLE_TIMEOUT`             |   `30s` |        `1s`–`5m` |
| `http_ingestion.max_header_bytes`    | `METRICSHELL_HTTP_MAX_HEADER_BYTES`         |  `8KiB` |   `1KiB`–`64KiB` |

HTTP wire bytes ограничиваются до decompression. Затем gzip/identity content ограничивается
`limits.decoded_input_bytes` до parsing, а canonical form — `limits.snapshot_bytes`. Тот же decoded limit применяется к
raw file content и assembled socket content, предотвращая amplification через whitespace или decompression.

## Exposition

| Property                        | Environment                            | Default |       Допустимо |
|---------------------------------|----------------------------------------|--------:|----------------:|
| `exposition.response_bytes`     | `METRICSHELL_MAX_RESPONSE_BYTES`       |  `8MiB` | `64KiB`–`64MiB` |
| `exposition.concurrent_scrapes` | `METRICSHELL_MAX_CONCURRENT_SCRAPES`   |    `32` |       `1`–`128` |
| `exposition.write_timeout`      | `METRICSHELL_EXPOSITION_WRITE_TIMEOUT` |   `30s` |       `1s`–`2m` |

Response limit проверяется по полному uncompressed body до success headers.

## Defaults logging

| Canonical property    | Environment variable              | Default | Допустимые значения |
|-----------------------|-----------------------------------|---------|---------------------|
| `log.level`           | `METRICSHELL_LOG_LEVEL`           | `info`  | `info`, `debug`     |
| `log.selector_values` | `METRICSHELL_LOG_SELECTOR_VALUES` | `false` | `true`, `false`     |

Info, warn и error events эмитятся всегда. Значение `debug` дополнительно включает registry events уровня debug.
Selector values отсутствуют, если не задано `log.selector_values=true`; destination остаётся фиксированным stderr.

## Reference container profile

| Resource               | Значение |
|------------------------|---------:|
| memory                 |  `64MiB` |
| PID limit              |     `64` |
| `nofile` soft/hard     |  `64/64` |
| runtime directory mode |   `0700` |
| Unix socket mode       |   `0660` |

Более низкие значения unsupported без полного conformance suite. Hard memory boundary обеспечивает cgroup.

## Cross-field validation

Startup завершается ошибкой, если выполняется хотя бы одно условие:

- явное значение находится вне допустимого диапазона;
- сумма workload timeout и reserve превышает total grace;
- mode final wait равен `scrapes`, но timeout или required count не положителен;
- response limit меньше минимального валидного ответа только с self-metrics;
- decoded input limit меньше canonical snapshot limit;
- размер socket frame превышает decoded input limit;
- effective socket decoded capacity меньше canonical snapshot limit;
- configured concurrency не помещается в process file-descriptor limit с учётом зарезервированных descriptors;
- adapter не применяет общий decoded/uncompressed input limit до parsing;
- запрошен zero или unlimited timeout там, где таблица требует конечное значение.

## Поведение на лимитах

- malformed/policy: deterministic rejection по mapping snapshot protocol;
- payload, series, label, name и policy limits используют HTTP/socket/self-metric mapping из snapshot protocol; этот
  документ не назначает альтернативные statuses;
- concurrency: HTTP `429` или эквивалентный busy publication outcome;
- response size: HTTP `503` до partial success;
- timeout: cancellation с сохранением last-valid state;
- cgroup OOM не может быть представлен как успешный workload outcome.

## Compatibility

Изменение stable default является compatibility change. Raising hard ceiling требует benchmark/security evidence.
Lowering default требует migration note.

## Требования соответствия

Тесты обязаны покрывать каждый default, обе границы каждого диапазона, одно значение за каждой границей,
противоречивые budget values, сохранение active state после каждого класса rejection, fail-fast overload, timeout
final wait, приоритет external termination, exact socket capacity на snapshot limit и byte below, оба logging levels,
selector-value settings и отказ response preflight.

## Ссылки

- [Ограничения](../03-requirements/constraints.md)
- [Грамматика значений конфигурации](configuration-value-grammar.md)
- [ADR-003](../06-architecture/adr/ADR-003.md)
- [ADR-006](../06-architecture/adr/ADR-006.md)
- [ADR-007](../06-architecture/adr/ADR-007.md)
- [ADR-008](../06-architecture/adr/ADR-008.md)
- [ADR-011](../06-architecture/adr/ADR-011.md)
- [ADR-014](../06-architecture/adr/ADR-014.md)
- [ADR-015](../06-architecture/adr/ADR-015.md)
