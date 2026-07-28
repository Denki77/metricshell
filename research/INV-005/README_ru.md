# INV-005 — Сравнение transports приёма метрик

[English version](README.md)

**Статус:** завершено  
**Эталонные прогоны:** `results/20260723T152957Z`, `results/20260723T153335Z`  
**Подробный отчёт:** [report.md](report_ru.md)

## Вопрос

Какие ingestion transports следует поддержать в первом stable release, а какие оставить optional adapters?

## Кандидаты

File snapshot, Unix stream, Unix datagram, loopback TCP HTTP, local gRPC через Unix socket, POSIX shared memory и
обычный mmap file.

## Исходные гипотезы

- file snapshot — самый простой baseline публикации state для PHP;
- Unix stream — сильнейший event-oriented candidate первого release;
- datagram снижает publication cost ценой отсутствия acknowledgement;
- HTTP — самый простой request/response integration;
- gRPC, shared memory и mmap должны показать существенную выгоду, оправдывающую protocol/client complexity.

## Необходимые доказательства

Containerized prototypes, одинаковый benchmark code на macOS/Ubuntu, эквивалентные delivery profiles, throughput,
latency, payload/concurrency profiles, исполняемые PHP integrations и failure probes.

## Эксперименты

Runner разделяет три contracts:

- `publish-only`: producer API завершил публикацию;
- `consumer-observed`: независимый consumer наблюдал точный sequence;
- `acknowledged`: transport вернул application response.

Сравнивать можно только строки одного profile. Каждый transport/profile выполняется для 64 B, 1 KiB, 16 KiB и
four-producer 64 B, по 500 operations на producer. Aggregate throughput считается по wall-clock времени всего scenario;
percentiles — по individual operation latency.

## Результаты

Оба matching-fingerprint LinuxKit run сформировали все 52 cells, прошли 15/15 assertions и 20/20 observation contracts.
Publish-only и acknowledged operations завершились полностью. Consumer-observed зафиксировал 17,482 из 17,500
publications. По шесть intermediate publications были superseded в four-producer file, shared-memory и mmap cells;
stream и datagram наблюдали все publications.

| Среда                 | Architecture | Docker | Kernel           | Result set         | Fingerprint     |
|-----------------------|--------------|-------:|------------------|--------------------|-----------------|
| macOS Docker Desktop  | aarch64      | 29.4.3 | LinuxKit 6.12.76 | `20260723T152957Z` | `71eb92f8…02b1` |
| Ubuntu Docker Desktop | x86_64       | 27.4.0 | LinuxKit 6.10.14 | `20260723T153335Z` | `71eb92f8…02b1` |

### 64 B, один producer, macOS

| Transport     | Profile           |     ops/s | p50 µs | p95 µs |  p99 µs |
|---------------|-------------------|----------:|-------:|-------:|--------:|
| File          | publish-only      |    22,177 | 39.458 | 77.875 | 128.041 |
| File          | consumer-observed |    19,214 | 48.958 | 83.375 | 102.083 |
| Unix stream   | publish-only      |   153,927 |  4.125 |  9.625 |  14.000 |
| Unix stream   | consumer-observed |    58,390 |  9.625 | 31.917 |  52.917 |
| Unix stream   | acknowledged      |    62,279 | 12.042 | 34.917 |  38.041 |
| Unix datagram | publish-only      |   136,900 |  4.209 |  9.458 |  14.208 |
| Unix datagram | consumer-observed |    98,071 |  6.125 | 26.334 |  35.208 |
| HTTP          | acknowledged      |    36,562 | 15.584 | 50.167 |  82.375 |
| gRPC          | acknowledged      |    42,881 | 15.458 | 31.084 |  88.792 |
| Shared memory | publish-only      | 9,002,197 |  0.042 |  0.042 |   0.042 |
| Shared memory | consumer-observed | 1,544,602 |  0.500 |  0.792 |   1.209 |
| mmap          | publish-only      | 5,943,536 |  0.042 |  0.084 |   0.084 |
| mmap          | consumer-observed | 1,030,486 |  0.500 |  1.417 |   7.458 |

Полные Ubuntu values и все payload/multi-producer cells находятся в `summary.tsv` соответствующего result set.

## Вывод

В первом stable release поддержать file snapshot, Unix stream и loopback HTTP. Unix datagram оставить optional, явно
unacknowledged adapter. gRPC, shared memory и mmap не включать в первый stable transport set. Snapshot transports
используют latest-state semantics и могут supersede intermediate concurrent publications.

## Запуск прототипа

```bash
./research/INV-005/run-bench.sh
```

```bash
INV005_COUNT=5000 ./research/INV-005/run-bench.sh
```

Runner удаляет старые INV-005 results, собирает image, применяет limits 2 CPU / 512 MiB, выполняет profiles, пишет
`observation-contracts.tsv`, запускает failure probes и PHP checks, после чего обновляет `latest-results.txt`. Artifacts
хранятся в temporary named volume и извлекаются через `docker cp`; host path не bind-mountится.

## Ограничения прототипа

- Обе среды используют LinuxKit; native Linux не проверен.
- Consumer — отдельная goroutine, не отдельный OS process.
- `consumer-observed` использует benchmark tracker, а не transport acknowledgement.
- File polling — research loop, а не design INV-006.
- HTTP использует loopback TCP; HTTP over Unix socket не проверен.
- gRPC использует raw codec без generated protobuf.
- Timing values — comparative observations, не SLO.

## Дополнительные benchmarks

Покрыты 52 cells, wall-clock throughput, p50/p95/p99, payloads 64 B/1 KiB/16 KiB, four producers, failure probes,
AF_UNIX datagram boundary 212,960 B, исполняемые PHP paths, observation contracts и fingerprint. Follow-up:
separate-process consumers, crash/restart, slow consumer, backlog saturation, sustained datagram loss, CPU/RSS и native
Linux.

## Результат решения

- Прототип: `prototype/`
- Runner: `run-bench.sh`
- Evidence: `results/20260723T152957Z/`, `results/20260723T153335Z/`
- Отчёт: [report.md](report_ru.md)
- ADR: [ADR-005](../../docs-ru/06-architecture/adr/ADR-005.md)
