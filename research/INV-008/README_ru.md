# INV-008 — Локальный push ingestion

[English version](README.md)

Статус: завершено  
Эталонный прогон macOS: `results/20260727T185055Z`  
Эталонный прогон Ubuntu: `results/20260728T114459Z`  
Отчёт: [report_ru.md](report_ru.md)  
Решение: [ADR-008](../../docs-ru/06-architecture/adr/ADR-008.md)

## Вопрос

Даёт ли локальный HTTP- или gRPC ingestion API достаточно преимуществ по сравнению с Unix socket adapter, чтобы
оправдать его реализацию и сопровождение?

## Контекст и кандидаты

Исследование относится только к producer-to-MetricShell ingestion внутри одного container/network namespace. Отправка
метрик из MetricShell в Prometheus, Pushgateway или central collector не рассматривается.

Сравнивались:

- отсутствие дополнительного push API: framed Unix domain socket;
- локальный HTTP/1.1 JSON API;
- локальный unary gRPC/protobuf API.

## Исходная гипотеза

Локальный HTTP может упростить интеграцию для языков со зрелыми HTTP clients, но дублирует возможности socket adapter и
увеличивает attack surface.

## Эксперимент

Runner выполняет 36 performance combinations: три транспорта, payload `64 B`, `1 KiB`, `16 KiB`, batch `1` и `16`,
один и восемь producers, по 100–300 requests на producer. Во всех транспортах один request содержит одинаковые `N`
отдельных records, а общий store увеличивает счётчик на фактически принятое количество records.

Дополнительно проверяются malformed, empty и encoded-oversized HTTP requests, одинаковая decoded-граница
`1 MiB`/`1 MiB + 1 byte` во всех транспортах, loopback bind, shutdown с сохранением state, restart и resources.

## Результаты

Оба прогона использовали fingerprint:

```text
34bee766d38ee43421cd100d3b23a387b7736c660d13bd6e28b591505bd101d4
```

В macOS/LinuxKit aarch64 и Ubuntu/LinuxKit x86_64:

- 14/14 correctness assertions прошли;
- 36/36 performance rows завершились без request errors;
- принято 385,313 records;
- benchmark scope был clean, untracked count равен нулю;
- decoded `1 MiB` принят во всех транспортах;
- decoded `1 MiB + 1 byte` отклонён во всех транспортах;
- Unix socket сохранил преимущество по throughput и latency;
- gRPC был быстрее HTTP при concurrent batching, но медленнее Unix socket.

Репрезентативные результаты для `1 KiB`:

| Environment | Форма нагрузки                   | Unix socket | HTTP JSON | gRPC protobuf |
|-------------|----------------------------------|------------:|----------:|--------------:|
| macOS       | 1 record, 1 producer, records/s  |     240,827 |    26,786 |        24,679 |
| Ubuntu      | 1 record, 1 producer, records/s  |      53,048 |     8,334 |         7,391 |
| macOS       | batch 16, 8 producers, records/s |   1,630,940 |    89,072 |       634,355 |
| Ubuntu      | batch 16, 8 producers, records/s |     928,096 |    21,172 |       187,377 |
| macOS       | batch 16, 8 producers, p95       |    0.245 ms |  3.246 ms |      1.046 ms |
| Ubuntu      | batch 16, 8 producers, p95       |    0.400 ms | 11.301 ms |      2.297 ms |

Timing distributions различаются и являются observations, а не portable pass criteria. Correctness, границы payload и
ранжирование кандидатов совпали.

## Итоговый вывод

Гипотеза подтверждена одинаковыми по fingerprint прогонами:

- дополнительный local push API не является обязательным;
- Unix socket остаётся default ingestion transport;
- loopback-only HTTP JSON допустим как опциональный compatibility adapter при наличии подтверждённой client requirement;
- gRPC не включается в default surface: он эффективнее JSON HTTP для batched concurrency, но дублирует socket semantics
  и требует protobuf schema, generated clients и HTTP/2 runtime.

## Допустимые значения

- bind HTTP/gRPC по умолчанию: `127.0.0.1`;
- maximum decoded request: `1 MiB`;
- протестированный batch: `1–16` records;
- протестированная concurrency: `1–8` producers;
- malformed JSON: HTTP `400`;
- empty records: HTTP `422`;
- decoded/encoded oversize: HTTP `413`;
- invalid request отклоняется атомарно без изменения принятого state;
- HTTP path versioning: `/v1/metrics`;
- breaking schema changes требуют новой версии path/protobuf package.

## Запуск стенда

Одинаковая команда для macOS и Ubuntu:

```bash
./research/INV-008/run-bench.sh
```

Проверка evidence:

```bash
latest="$(cat research/INV-008/latest-results.txt)"
cat "$latest/summary.tsv"
cat "$latest/correctness.tsv"
cat "$latest/performance.tsv"
cat "$latest/resources.tsv"
cat "$latest/environment.tsv"
```

## Ограничения

- Оба протестированных container environments используют LinuxKit. Результаты подтверждают cross-architecture behavior
  в macOS/LinuxKit aarch64 и Ubuntu/LinuxKit x86_64, но не native Linux без LinuxKit.
- Прототип не моделирует production parser, cardinality limits, metric conflicts, queue или persistence.
- HTTP использует JSON/base64 для bytes.
- Исследовался unary gRPC; streaming gRPC является отдельным protocol/lifecycle вариантом.
- CPU/RSS относятся к общему benchmark process с servers и clients, а не к изолированному production adapter.
- Loopback bind не является authentication относительно процессов в том же network namespace.

## Выход решения

- Prototype: `prototype/`
- Runner: `run-bench.sh`
- Evidence: `results/20260727T185055Z/`, `results/20260728T114459Z/`
- Подробный отчёт: [report_ru.md](report_ru.md)
- Принятое решение: [ADR-008](../../docs-ru/06-architecture/adr/ADR-008.md)
