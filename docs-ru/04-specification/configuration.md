# Спецификация configuration

[English version](../../docs/04-specification/configuration.md)

> Статус: принятая нормативная спецификация
> Требования: FR-006, FR-013, FR-024, FR-046, FR-052, FR-080–FR-082
> Критерии приёмки: AC-RUN-001, AC-ING-008–AC-ING-009, AC-CONF-001–AC-CONF-005
> Решения: ADR-001–ADR-003, ADR-005–ADR-008, ADR-010–ADR-014

## Синтаксис команды

~~~text
metricshell [MetricShell options] -- executable [argument ...]
metricshell --version
metricshell --help
~~~

Первый самостоятельный -- обязателен для запуска workload. Каждый token после него является workload argv и передаётся
без shell interpretation. Пустой workload argv даёт configuration_invalid. MetricShell options после -- являются
аргументами workload. Shell behavior требует явный workload, например -- /bin/sh -c command.

У Core нет option mode. В частности, --mode, --mode=snapshot и --mode=managed-registry невалидны.

## Источники и precedence

Version 1 поддерживает CLI options, environment variables и compiled defaults. Configuration file не поддерживается.
Precedence:

~~~text
CLI > environment > compiled default
~~~

Repeatable CLI option сохраняет occurrence order. Repeatable environment value использует списковую грамматику из
[грамматики значений конфигурации](configuration-value-grammar.md); в version 1 escaping отсутствует. Пустое environment
value считается явно пустым, а не отсутствующим. Unknown options и невалидные значения определённых здесь METRICSHELL_
variables фатальны до запуска workload.

## Core endpoints и transport

| Property              | CLI                     | Environment                       | Default                        |
|-----------------------|-------------------------|-----------------------------------|--------------------------------|
| ingestion.transport   | --ingestion-transport   | METRICSHELL_INGESTION_TRANSPORT   | unix                           |
| exposition.listen     | --exposition-listen     | METRICSHELL_EXPOSITION_LISTEN     | 0.0.0.0:9090                   |
| http_ingestion.listen | --http-ingestion-listen | METRICSHELL_HTTP_INGESTION_LISTEN | 127.0.0.1:9091                 |
| socket.path           | --unix-socket-path      | METRICSHELL_UNIX_SOCKET_PATH      | /run/metricshell/ingest.sock   |
| file.path             | --snapshot-file-path    | METRICSHELL_SNAPSHOT_FILE_PATH    | /run/metricshell/snapshot.json |
| shutdown.deadline     | --shutdown-deadline     | METRICSHELL_SHUTDOWN_DEADLINE     | пусто                          |

ingestion.transport имеет только одно значение: file, unix или http. Активируется только выбранный ingestion
listener/watcher. Явные transport-specific options неактивного transport отклоняются для обнаружения configuration
mistakes.

Exposition server владеет фиксированными version 1 paths:

~~~text
GET /metrics
GET /healthz
GET /readyz
GET /debug/config
~~~

Local HTTP ingestion server владеет POST /v1/metrics. Paths не настраиваются в version 1. /debug/config возвращает
effective non-secret configuration и никогда не возвращает workload argv или environment values. Probe semantics
определены runtime state specification.

Listen addresses используют Go host:port syntax; пустой host невалиден. HTTP ingestion обязан resolve-иться в loopback.
Unix и file paths абсолютны, имеют private parent directory и не являются symlinks. MetricShell создаёт Unix socket с
mode 0660 и runtime directory с mode 0700. Владелец и группа socket соответствуют настроенной runtime identity; доступ
producer с другим UID предоставляется через эту группу согласно ADR-007.

## Options final wait и shutdown

| Property                    | CLI                           | Environment                             |
|-----------------------------|-------------------------------|-----------------------------------------|
| final_wait.mode             | --final-wait-mode             | METRICSHELL_FINAL_WAIT_MODE             |
| final_wait.duration         | --final-wait-duration         | METRICSHELL_FINAL_WAIT_DURATION         |
| final_wait.timeout          | --final-wait-timeout          | METRICSHELL_FINAL_WAIT_TIMEOUT          |
| final_wait.required_scrapes | --final-wait-required-scrapes | METRICSHELL_FINAL_WAIT_REQUIRED_SCRAPES |
| final_wait.completion_grace | --final-wait-completion-grace | METRICSHELL_FINAL_WAIT_COMPLETION_GRACE |
| shutdown.total_grace        | --shutdown-total-grace        | METRICSHELL_SHUTDOWN_TOTAL_GRACE        |
| shutdown.workload_timeout   | --workload-shutdown-timeout   | METRICSHELL_WORKLOAD_SHUTDOWN_TIMEOUT   |
| shutdown.reserve            | --shutdown-reserve            | METRICSHELL_SHUTDOWN_RESERVE            |

Durations используют нормативную [грамматику значений конфигурации](configuration-value-grammar.md); `0` разрешён только
там, где это допускает defaults specification. shutdown.deadline пуст или является RFC3339 timestamp с явным offset,
например
2026-08-05T10:30:00Z. Это external absolute deadline, он находится в будущем при startup и ограничивает все derived
durations. MetricShell никогда его не продлевает.

## Options file, socket и HTTP

| Property                           | CLI                             | Environment                               |
|------------------------------------|---------------------------------|-------------------------------------------|
| file.reconcile_interval            | --file-reconcile-interval       | METRICSHELL_FILE_RECONCILE_INTERVAL       |
| socket.frame_bytes                 | --socket-max-frame-bytes        | METRICSHELL_SOCKET_MAX_FRAME_BYTES        |
| socket.parts                       | --socket-max-parts              | METRICSHELL_SOCKET_MAX_PARTS              |
| socket.connections                 | --socket-max-connections        | METRICSHELL_SOCKET_MAX_CONNECTIONS        |
| socket.transactions                | --socket-max-transactions       | METRICSHELL_SOCKET_MAX_TRANSACTIONS       |
| socket.transaction_timeout         | --socket-transaction-timeout    | METRICSHELL_SOCKET_TRANSACTION_TIMEOUT    |
| socket.read_timeout                | --socket-read-timeout           | METRICSHELL_SOCKET_READ_TIMEOUT           |
| socket.write_timeout               | --socket-write-timeout          | METRICSHELL_SOCKET_WRITE_TIMEOUT          |
| http_ingestion.wire_bytes          | --http-ingestion-max-wire-bytes | METRICSHELL_HTTP_INGESTION_MAX_WIRE_BYTES |
| http_ingestion.read_header_timeout | --http-read-header-timeout      | METRICSHELL_HTTP_READ_HEADER_TIMEOUT      |
| http_ingestion.read_timeout        | --http-read-timeout             | METRICSHELL_HTTP_READ_TIMEOUT             |
| http_ingestion.write_timeout       | --http-write-timeout            | METRICSHELL_HTTP_WRITE_TIMEOUT            |
| http_ingestion.idle_timeout        | --http-idle-timeout             | METRICSHELL_HTTP_IDLE_TIMEOUT             |
| http_ingestion.max_header_bytes    | --http-max-header-bytes         | METRICSHELL_HTTP_MAX_HEADER_BYTES         |

Multipart socket publication поддерживается всегда и не имеет enable flag. socket.parts, socket.transactions, frame
size, snapshot size и transaction timeout — полный configuration surface. One-part publication использует ту же grammar
BEGIN/PART/COMMIT.

## Snapshot и exposition limits

| Property                      | CLI                         | Environment                           |
|-------------------------------|-----------------------------|---------------------------------------|
| limits.snapshot_bytes         | --max-snapshot-bytes        | METRICSHELL_MAX_SNAPSHOT_BYTES        |
| limits.decoded_input_bytes    | --max-decoded-input-bytes   | METRICSHELL_MAX_DECODED_INPUT_BYTES   |
| limits.series                 | --max-series                | METRICSHELL_MAX_SERIES                |
| limits.labels_per_series      | --max-labels-per-series     | METRICSHELL_MAX_LABELS_PER_SERIES     |
| limits.metric_name_bytes      | --max-metric-name-bytes     | METRICSHELL_MAX_METRIC_NAME_BYTES     |
| limits.label_name_bytes       | --max-label-name-bytes      | METRICSHELL_MAX_LABEL_NAME_BYTES      |
| limits.label_value_bytes      | --max-label-value-bytes     | METRICSHELL_MAX_LABEL_VALUE_BYTES     |
| limits.help_bytes             | --max-help-bytes            | METRICSHELL_MAX_HELP_BYTES            |
| limits.concurrent_ingestions  | --max-concurrent-ingestions | METRICSHELL_MAX_CONCURRENT_INGESTIONS |
| limits.pending_ingestions     | --max-pending-ingestions    | METRICSHELL_MAX_PENDING_INGESTIONS    |
| exposition.response_bytes     | --max-response-bytes        | METRICSHELL_MAX_RESPONSE_BYTES        |
| exposition.concurrent_scrapes | --max-concurrent-scrapes    | METRICSHELL_MAX_CONCURRENT_SCRAPES    |
| exposition.write_timeout      | --exposition-write-timeout  | METRICSHELL_EXPOSITION_WRITE_TIMEOUT  |

Byte sizes и counts используют нормативную [грамматику значений конфигурации](configuration-value-grammar.md). Точные
defaults, ranges и cross-field validation заданы в defaults/resource-limits specification.

## Options logging

| Property            | CLI                   | Environment                     |
|---------------------|-----------------------|---------------------------------|
| log.level           | --log-level           | METRICSHELL_LOG_LEVEL           |
| log.selector_values | --log-selector-values | METRICSHELL_LOG_SELECTOR_VALUES |

`log.level` имеет только значения `info` или `debug`. `log.selector_values` использует нормативную boolean grammar.
MetricShell-owned JSON Lines всегда направляются в stderr в version 1; destination не настраивается. Filtering добавляет
repeatable options --metrics-include и --metrics-exclude и их environment variables точно по filtering specification.
Других Core CLI options в version 1 нет.

## File-descriptor validation

До запуска workload MetricShell вычисляет:

~~~text
required_nofile =
  16
  + socket.connections when transport=unix
  + exposition.concurrent_scrapes
  + limits.concurrent_ingestions
~~~

Reserve 16 покрывает stdio, signal/process handles, listeners, file watch, reconciliation file и bounded internal
overhead. Если soft RLIMIT_NOFILE меньше required_nofile, configuration отклоняется. MetricShell не увеличивает limit
автоматически. Та же формула возвращается в /debug/config.

## Startup validation

Вся static validation, directory checks, listener binds и file-watch setup выполняются до запуска workload. Failure
закрывает все частично полученные resources. Contradictory values, unavailable required endpoint, invalid absolute
deadline, недостаточный RLIMIT_NOFILE или явно заданный option неактивного transport являются fatal.

## Собственные exit codes MetricShell

Постоянный registry version 1:

| Code | Symbol                | Значение                                                                          |
|-----:|-----------------------|-----------------------------------------------------------------------------------|
|   64 | configuration_invalid | Ошибка CLI, environment, cross-field, path или resource-limit validation.         |
|   70 | internal_failure      | Invariant violation или неклассифицированный software failure MetricShell.        |
|   71 | resource_unavailable  | Недоступен обязательный OS resource или runtime facility.                         |
|   72 | endpoint_bind_failed  | Не удалось создать обязательный exposition, ingestion, socket или watch endpoint. |
|   73 | workload_start_failed | Не удалось запустить workload executable.                                         |

Нормативный mapping этих symbols в runtime failure reasons, structured events и `error_code` задаёт единственная таблица
в [спецификации structured logs](structured-logging.md#закрытые-fielderror-registries).

После успешного запуска результат workload сохраняется: 0, произвольные non-zero codes и mapping 128+signal. Workload
может вернуть любой byte-sized code, поэтому logs и workload-started flag являются источником origin; MetricShell не
переназначает уже полученный workload result только из-за численного совпадения с registry.

## Ссылки

- [Runtime state machine](runtime-state-machine.md)
- [Application snapshot protocol](application-snapshot-protocol.md)
- [Defaults и resource limits](runtime-defaults-and-resource-limits.md)
- [Грамматика значений конфигурации](configuration-value-grammar.md)
- [Фильтрация метрик](metrics-filtering.md)
- [Structured logging](structured-logging.md)
