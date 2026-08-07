# Спецификация application snapshot protocol

[English version](../../docs/04-specification/application-snapshot-protocol.md)

> Статус: принятая нормативная спецификация
> Требования: FR-010–FR-026
> Критерии приёмки: AC-ING-001–AC-ING-012, AC-MET-001–AC-MET-011
> Решения: ADR-004–ADR-008, ADR-010, ADR-014

## Контракт и media type

Все ingestion adapters Core передают один полный application snapshot по schema version 1. Media type: application/vnd.metricshell.snapshot+json;version=1.
Обязателен UTF-8. Compression относится к transport и не меняет snapshot identity.

Принятый документ заменяет весь application snapshot. Отсутствующие families и series удаляются. Пустое transport body
невалидно. Документ без series является валидным empty application state и канонизируется в zero-series document ниже.

## Документ version 1

~~~json
{
  "schema_version": 1,
  "families": [
    {
      "name": "requests",
      "help": "Completed requests.",
      "type": "counter",
      "series": [
        {
          "labels": {
            "method": "GET"
          },
          "value": "42"
        }
      ]
    },
    {
      "name": "request_duration_seconds",
      "help": "Request duration.",
      "type": "histogram",
      "series": [
        {
          "labels": {
            "method": "GET"
          },
          "histogram": {
            "count": "7",
            "sum": "1.25",
            "buckets": [
              {
                "le": "0.1",
                "count": "2"
              },
              {
                "le": "0.5",
                "count": "6"
              },
              {
                "le": "+Inf",
                "count": "7"
              }
            ]
          }
        }
      ]
    }
  ]
}
~~~

Top-level object содержит только schema_version и families. Unknown fields в version 1 отклоняются.

Family содержит только:

- name: валидное Prometheus metric-family name;
- help: UTF-8 string, может быть пустым;
- type: counter, gauge или histogram;
- series: array, который может быть пустым.

Series counter или gauge содержит только labels и value. Histogram series содержит только labels и histogram. Labels —
JSON object строковых name/value pairs. Metric names, label names, reserved label __name__, duplicate label names и
namespace metricshell_ валидируются до activation.

## Family names и encoded sample names

`family.name` всегда является base metric-family name. Filtering, series identity, metadata conflict detection и
lifetime name-to-type binding используют это base name. Encoders получают metadata/sample names так:

| Type        | Prometheus text 0.0.4 metadata / samples                  | OpenMetrics 1.0 metadata / samples                        |
|-------------|-----------------------------------------------------------|-----------------------------------------------------------|
| `counter`   | HELP/TYPE `base_total`; sample `base_total`               | HELP/TYPE `base`; sample `base_total`                     |
| `gauge`     | HELP/TYPE `base`; sample `base`                           | HELP/TYPE `base`; sample `base`                           |
| `histogram` | HELP/TYPE `base`; `base_bucket`, `base_sum`, `base_count` | HELP/TYPE `base`; `base_bucket`, `base_sum`, `base_count` |

Base name counter, оканчивающееся на `_total`, невалидно.  
Base name histogram, оканчивающееся на `_bucket`, `_sum` или`_count`, невалидно.  
До activation validator строит все encoded sample names для обоих форматов.  
Пересечение имён разных families даёт `metadata_conflict`; это обнаруживает counter `foo` вместе с gauge `foo_total` и histogram `foo` вместе с gauge `foo_bucket`.  
Base и derived names обязаны удовлетворять `limits.metric_name_bytes`; oversized derived component даёт `name_limit`.  
Derived name с reserved prefix `metricshell_` даёт `reserved_name`.  
Histogram series не может содержать пользовательский label `le`: bucket labels `le` создаёт только encoder, collision даёт `histogram_invalid`.

## Представление чисел

Numeric values являются JSON strings, чтобы исключить зависимое от parser преобразование JSON numbers.

- grammar finite decimal: `-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?`;
- grammar special float содержит только `NaN`, `+Inf` или `-Inf`;
- каждый finite decimal разбирается точно как Go `strconv.ParseFloat(token, 64)` с IEEE-754 binary64
  round-to-nearest, ties-to-even; range error, overflow в infinity или underflow ненулевого token в zero дают
  `numeric_invalid`;
- canonical finite rendering точно соответствует Go `strconv.FormatFloat(value, 'g', -1, 64)`; special values сохраняют
  tokens `NaN`, `+Inf` и `-Inf`;
- counter values являются finite со сброшенным sign bit (`-0` невалиден); histogram counts неотрицательны;
- counts — base-10 unsigned 64-bit integers без sign и leading zeroes, кроме 0;
- gauge values — finite decimals или special float values; реализация сохраняет все четыре класса, не интерпретируя их
  business meaning;
- histogram sums — неотрицательные finite decimals или `+Inf`; negative zero, отрицательные values, `NaN` и `-Inf` дают
  `histogram_invalid`;
- histogram `le` — неотрицательный finite decimal или `+Inf`; negative zero и отрицательные boundaries невалидны;
- histogram buckets строго возрастают, cumulative counts не убывают, последний bucket равен +Inf, а его count равен
  histogram count. Boundaries сравниваются после binary64 conversion; duplicate или non-increasing converted values
  дают `histogram_invalid`, даже если их input strings различаются.

Timestamps, exemplars, summaries, native histograms и untyped families не входят в version 1.

## Identity, conflicts и canonical form

Series identity — family name плюс отсортированные label-name/value pairs. Candidate атомарно отклоняется при duplicate
series, duplicate family names, conflicting metadata или несовпадении family type с type binding текущего запуска.

После validation canonical form строится так:

1. families сортируются по name;
2. каждый labels object сортируется по label name;
3. families с пустым `series` array удаляются;
4. series внутри family сортируются лексикографически по label pairs;
5. finite numeric strings заменяются canonical binary64 rendering, порядок validated buckets сохраняется;
6. UTF-8 JSON кодируется без незначащих пробелов и с final newline.

`limits.decoded_input_bytes` применяется к uncompressed/decoded bytes до parsing, включая whitespace. Snapshot byte
limits отдельно применяются к canonical uncompressed form. Transport wire limits применяются до decompression или
base64 decoding согласно adapter.

## Валидный zero-series snapshot

Точный semantic zero-series document:

~~~json
{
  "schema_version": 1,
  "families": []
}
~~~

Family с пустым `series` array полностью проходит name/metadata/label validation, но не создаёт lifetime type binding,
не
эмитит HELP/TYPE и удаляется из canonical form. Документ только с empty families канонизируется точно в документ выше.
Acceptance любого представления атомарно очищает все application series и увеличивает generation. Final newline
разрешён.
Zero-byte file, пустой HTTP body или multipart transaction с нулём assembled bytes возвращает empty_payload и не меняет
active state.

## File adapter

Configured snapshot file содержит только один version 1 document и optional final newline. Publication использует
temporary file в том же directory, flush/close и atomic rename в configured path. MetricShell читает только target после
rename/inotify или reconciliation. Partial JSON, non-regular files, symlinks, deletion и content, превышающий
`limits.decoded_input_bytes`, отклоняются или считаются absent согласно file adapter policy; ни один случай не очищает
active state. Лимит применяется во время чтения, до JSON parsing.

## HTTP adapter

Endpoint: POST /v1/metrics. Content-Type обязан быть snapshot media type или application/json. Content-Encoding:
identity или gzip. Другие methods, media types и encodings отклоняются. Decompression останавливается с
`payload_limit`, как только output превысил бы `limits.decoded_input_bytes`; wire и canonical bytes имеют независимые
лимиты.

Success response после atomic installation:

~~~json
{
  "schema_version": 1,
  "status": "ack",
  "generation": 7
}
~~~

Failure response:

~~~json
{
  "schema_version": 1,
  "status": "nack",
  "code": "duplicate_series"
}
~~~

ACK не отправляется до validation и installation. HTTP status следует единому rejection mapping ниже. Busy и timeout —
publication outcomes: HTTP использует 429 и 408, socket — NACK `busy` и `timeout`; в registry candidate-rejection
reasons
они не входят.

## Unix socket protocol

Unix stream protocol использует UTF-8 line-framed ASCII control data и unpadded RFC 4648 base64url payload parts.
Padding
`=` и символы не из URL-safe alphabet недопустимы. Каждая строка заканчивается LF и помещается в socket.frame_bytes.
Разделитель — один ASCII space. publication-id соответствует
[A-Za-z0-9_-]{1,64}; indexes и sizes — canonical unsigned decimal integers.

Client frames:

~~~text
MSP/1 SNAPSHOT_BEGIN <publication-id> <part-count> <decoded-bytes>
MSP/1 SNAPSHOT_PART <publication-id> <zero-based-index> <base64url-data>
MSP/1 SNAPSHOT_COMMIT <publication-id>
~~~

Server frames:

~~~text
MSP/1 FRAME_ACCEPTED <publication-id> BEGIN
MSP/1 FRAME_ACCEPTED <publication-id> <zero-based-index>
MSP/1 ACK <publication-id> <generation>
MSP/1 NACK <publication-id> <code>
~~~

SNAPSHOT_BEGIN резервирует одну bounded transaction и возвращает `FRAME_ACCEPTED <publication-id> BEGIN`. part-count:
1..socket.parts; decoded-bytes: 1..limits.decoded_input_bytes. Каждый index встречается только один раз. Сохранённый part
возвращает `FRAME_ACCEPTED <publication-id> <index>`. Parts принимаются только по порядку index; decoded
bytes конкатенируются. COMMIT валиден только при наличии всех объявленных parts и точном decoded size. Assembled bytes
должны быть одним version 1 JSON document. FRAME_ACCEPTED подтверждает только bounded retention; ACK означает atomic
installation.

Один connection writer сериализует complete frames. Concurrent publications используют отдельные transactions и могут
валидироваться concurrently, но installation и generation assignment имеют один linear order. Disconnect до ACK
неоднозначен; retry повторно публикует полный snapshot и безопасен, но может ещё раз увеличить generation.

## Закрытый mapping отклонений candidate

Эта таблица — единственный нормативный mapping отклонения whole candidate. Logs `snapshot.rejected` и
`metricshell_snapshot_rejections_total{reason}` используют только значения из колонки Reason.

| Reason              | HTTP | Socket NACK code    | File/self-metric outcome     |
|---------------------|-----:|---------------------|------------------------------|
| `malformed`         |  400 | `malformed`         | `rejected/malformed`         |
| `numeric_invalid`   |  400 | `numeric_invalid`   | `rejected/numeric_invalid`   |
| `schema_version`    |  400 | `schema_version`    | `rejected/schema_version`    |
| `empty_payload`     |  400 | `empty_payload`     | `rejected/empty_payload`     |
| `payload_limit`     |  413 | `payload_limit`     | `rejected/payload_limit`     |
| `series_limit`      |  422 | `series_limit`      | `rejected/series_limit`      |
| `label_limit`       |  422 | `label_limit`       | `rejected/label_limit`       |
| `name_limit`        |  422 | `name_limit`        | `rejected/name_limit`        |
| `policy`            |  422 | `policy`            | `rejected/policy`            |
| `duplicate_series`  |  400 | `duplicate_series`  | `rejected/duplicate_series`  |
| `type_conflict`     |  400 | `type_conflict`     | `rejected/type_conflict`     |
| `metadata_conflict` |  400 | `metadata_conflict` | `rejected/metadata_conflict` |
| `histogram_invalid` |  400 | `histogram_invalid` | `rejected/histogram_invalid` |
| `reserved_name`     |  400 | `reserved_name`     | `rejected/reserved_name`     |
| `frozen`            |  409 | `frozen`            | `rejected/frozen`            |
| `internal`          |  500 | `internal`          | `rejected/internal`          |

Socket response имеет вид `MSP/1 NACK <publication-id> <code>`. Ошибки framing используют отдельный закрытый transport
registry: `malformed`, `protocol_version`, `frame_limit`, `part_limit`, `duplicate_part`, `missing_part`,
`transaction_invalid` и `transaction_expired`. Admission/deadline outcomes — `busy` и `timeout`; они входят в общие
ingestion outcome metrics, а не candidate/socket-frame rejection labels. Adapters не включают parser text, paths, names,
labels, addresses или IDs в codes.
Эквивалентные invalid candidates дают одинаковый reason через file, socket и HTTP.

## Conformance

Общий corpus прогоняет валидные counter/gauge/histogram и zero-series documents, а также каждый rejection class через
все три adapters. Accepted canonical bytes, generation, exposition и rejection code совпадают. Тесты включают
float64 overflow/underflow и rounding collisions; negative histogram bounds/sums; normalization empty families;
format-specific counter names и component collisions; histogram `le`; truncation; duplicate/out-of-order parts;
decompression limits; disconnects; concurrent validation и atomic replacement.

## Ссылки

- [Спецификация configuration](configuration.md)
- [Defaults и resource limits](runtime-defaults-and-resource-limits.md)
- [ADR-004](../06-architecture/adr/ADR-004.md)
- [ADR-006](../06-architecture/adr/ADR-006.md)
- [ADR-007](../06-architecture/adr/ADR-007.md)
- [ADR-008](../06-architecture/adr/ADR-008.md)
