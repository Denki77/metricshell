# Отчёт INV-005 — Сравнение transports приёма метрик

[English version](report.md)

**Статус:** завершено  
**Дата прогона:** 2026-07-23  
**Docker Servers:** 29.4.3, 27.4.0  
**Платформы:** `linux/aarch64`, `linux/x86_64`  
**Эталонные прогоны:** `results/20260723T152957Z`, `results/20260723T153335Z`  
**Сводки:** `results/20260723T152957Z/summary.tsv`, `results/20260723T153335Z/summary.tsv`

## Цель

Сравнить варианты INV-005, не смешивая semantics публикации, наблюдения consumer и acknowledgement, и сузить набор
transports первого release перед специализированными исследованиями INV-006–009.

## Прототип

`prototype/cmd/bench` реализует семь transports, три delivery profiles, raw latency, wall-clock timing, boundary probes
и one-shot servers для PHP integration. `run-bench.sh` собирает image, выполняет profiles, проверяет PHP
file/stream/HTTP paths и сохраняет только актуальный evidence. Artifacts записываются в temporary Docker volume и
извлекаются `docker cp`.

## Команды запуска

```bash
./research/INV-005/run-bench.sh
```

## Среды и fingerprint

| Среда  | Docker | Architecture | Kernel           | Scope clean | Untracked | Result set         |
|--------|-------:|--------------|------------------|-------------|-----------|--------------------|
| macOS  | 29.4.3 | aarch64      | LinuxKit 6.12.76 | false       | 0         | `20260723T152957Z` |
| Ubuntu | 27.4.0 | x86_64       | LinuxKit 6.10.14 | true        | 0         | `20260723T153335Z` |

Оба прогона использовали 2 CPU / 512 MiB и fingerprint:

```text
71eb92f8d9eb1fd400f040706197d2d8edd7f84c9580bf61cbdf66621d0002b1
```

## Контракты benchmark

| Profile           | Точка завершения                 | Candidates                                  |
|-------------------|----------------------------------|---------------------------------------------|
| publish-only      | producer API вернул управление   | file, stream, datagram, shared memory, mmap |
| consumer-observed | consumer наблюдал exact sequence | file, stream, datagram, shared memory, mmap |
| acknowledged      | получен application response     | stream, HTTP, gRPC                          |

Datagram не считается acknowledged. File readback и shared-memory/mmap observation выполняет consumer loop. Результаты
сравниваются только внутри одного profile. `observation-contracts.tsv` делает правила исполняемыми.

## Метод throughput

`aggregate_ops_s = total operations / wall-clock elapsed`. Individual latency samples используются только для
p50/p95/p99.

## Результаты

Во всех средах сформированы 52 cells, прошли 15/15 assertions и 20/20 contracts. Consumer observation: 17,482/17,500.
Superseded intermediate states — 18 total в file/shared-memory/mmap four-producer cells.

### Publish-only, 64 B, один producer

| Transport     |     ops/s | p50 µs | p95 µs |  p99 µs |
|---------------|----------:|-------:|-------:|--------:|
| File          |    22,177 | 39.458 | 77.875 | 128.041 |
| Unix stream   |   153,927 |  4.125 |  9.625 |  14.000 |
| Unix datagram |   136,900 |  4.209 |  9.458 |  14.208 |
| Shared memory | 9,002,197 |  0.042 |  0.042 |   0.042 |
| mmap          | 5,943,536 |  0.042 |  0.084 |   0.084 |

### Consumer-observed, 64 B, один producer

| Transport     |     ops/s | p50 µs | p95 µs |  p99 µs |
|---------------|----------:|-------:|-------:|--------:|
| File          |    19,214 | 48.958 | 83.375 | 102.083 |
| Unix stream   |    58,390 |  9.625 | 31.917 |  52.917 |
| Unix datagram |    98,071 |  6.125 | 26.334 |  35.208 |
| Shared memory | 1,544,602 |  0.500 |  0.792 |   1.209 |
| mmap          | 1,030,486 |  0.500 |  1.417 |   7.458 |

### Acknowledged, 64 B, один producer

| Transport   |  ops/s | p50 µs | p95 µs | p99 µs |
|-------------|-------:|-------:|-------:|-------:|
| Unix stream | 62,279 | 12.042 | 34.917 | 38.041 |
| HTTP        | 36,562 | 15.584 | 50.167 | 82.375 |
| gRPC        | 42,881 | 15.458 | 31.084 | 88.792 |

### Ubuntu/LinuxKit x86_64

| Transport     | Profile           |     ops/s |  p50 µs |    p95 µs |    p99 µs |
|---------------|-------------------|----------:|--------:|----------:|----------:|
| File          | publish-only      |     3,328 | 111.453 | 1,058.916 | 4,162.222 |
| File          | consumer-observed |     8,916 | 110.210 |   151.584 |   212.376 |
| Unix stream   | publish-only      |    18,585 |  21.032 |    29.055 |    35.842 |
| Unix stream   | consumer-observed |     8,217 |  69.317 |   347.134 |   422.273 |
| Unix stream   | acknowledged      |    20,084 |  41.323 |    74.891 |    88.654 |
| Unix datagram | publish-only      |    63,352 |  14.194 |    18.068 |    27.765 |
| Unix datagram | consumer-observed |    45,238 |  16.985 |    40.548 |    62.738 |
| HTTP          | acknowledged      |    14,434 |  51.697 |   153.210 |   231.362 |
| gRPC          | acknowledged      |     9,574 |  68.163 |   324.494 |   555.286 |
| Shared memory | publish-only      | 1,430,059 |   0.254 |     0.422 |     0.435 |
| Shared memory | consumer-observed |   333,398 |   2.335 |     3.294 |    17.304 |
| mmap          | publish-only      | 6,909,513 |   0.045 |     0.046 |     0.047 |
| mmap          | consumer-observed |   802,955 |   0.891 |     1.400 |     1.682 |

Signal-to-exit не является метрикой INV-005: prototype не supervises workload и не отправляет termination signals.

## PHP integration evidence

| Path               | Evidence                           | Результат |
|--------------------|------------------------------------|-----------|
| file atomic rename | payload прочитан из mounted path   | pass      |
| Unix stream        | Go consumer сохранил bytes         | pass      |
| loopback HTTP      | handler сохранил body и вернул 204 | pass      |

## Failure-mode evidence

| Probe                      | Наблюдение                                   |
|----------------------------|----------------------------------------------|
| missing Unix socket        | connection rejected                          |
| closed HTTP port           | connection rejected                          |
| `/tmp` → `/dev/shm` rename | cross-filesystem rejection                   |
| mmap growth                | existing mapping остался 4096 B; нужен remap |
| AF_UNIX datagram           | max accepted 212,960 B в среде               |

## Матрица оценки

| Критерий                   | File                | Stream    | Datagram       | HTTP      | gRPC        | Shared memory   | mmap            |
|----------------------------|---------------------|-----------|----------------|-----------|-------------|-----------------|-----------------|
| PHP path                   | executed            | executed  | example        | executed  | high effort | high effort     | high effort     |
| Ack                        | no                  | yes       | no             | yes       | yes         | no              | no              |
| Multi-producer observation | supersedes          | 100%      | 100% run       | 100% ack  | 100% ack    | supersedes      | supersedes      |
| Recovery                   | persistent snapshot | reconnect | loss/retry     | retry     | retry       | protocol needed | remap/reconcile |
| Role                       | stable v1           | stable v1 | optional/lossy | stable v1 | reject v1   | reject v1       | reject v1       |

## Допустимые значения и политики

- payload range: 64 B–16 KiB;
- producers: 1 и 4;
- v1 candidates: file snapshot, Unix stream, loopback HTTP;
- datagram optional, unacknowledged, ниже environment-specific limit;
- snapshot transports имеют latest-state semantics;
- gRPC/shared memory/mmap не входят в v1 без отдельного требования;
- minimum 500 samples/cell, 5000 recommended.

## Ограничения прототипа

Consumer остаётся goroutine в том же процессе; consumer-observed не является producer-visible ack. HTTP — loopback TCP.
File polling не является production design. gRPC raw codec исключает protobuf serialization. Обе среды используют
LinuxKit.

## Дополнительные benchmarks

Все profiles запускаются по умолчанию. Follow-up: separate-process consumer, crash/restart, slow consumer, saturation,
sustained datagram loss, cgroup measurements и native Linux.

## Вывод

Подтверждено: v1 включает file snapshot, Unix stream и loopback HTTP; Unix datagram — optional unacknowledged adapter;
gRPC/shared memory/mmap исключаются. Snapshot transports являются replaceable latest state, а не event log.
