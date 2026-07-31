# INV-007 — Приём метрик через socket

[English version](README.md)

**Статус:** завершено  
**Эталонные прогоны:** `results/20260729T072602Z`, `results/20260729T164723Z`  
**Отчёт:** [report_ru.md](report_ru.md)  
Решение: [ADR-007](../../docs-ru/06-architecture/adr/ADR-007.md)

## Вопрос

Какие local socket type, framing и acknowledgement model должны передавать complete application snapshots из ADR-004?

## Согласование scope

ADR-004 был сужен во время INV-007. MetricShell принимает один полный conflict-free application snapshot за
publication. Он не принимает instrumentation operations, не агрегирует per-producer registries и не владеет producer
identity, sequencing или reconciliation.

Multipart framing является только transport-level сборкой одного complete candidate. Parts не устанавливаются и не
публикуются независимо. Valid commit запускает complete validation и atomic replacement.

## Выбранное направление

- Primary transport: Unix stream.
- Native framing: versioned newline-delimited text.
- Confirmed mode: publication ID и `ACK <id>` либо `NACK <id> <reason>`.
- ACK отправляется только после structural validation и atomic installation полного candidate.
- Большой candidate использует `snapshot_begin`, bounded `snapshot_part` и `snapshot_commit`.
- Initial configurable frame default: 8 KiB; это conservative starting value, не benchmark-derived architecture limit.
- 65 536 B — maximum individual payload exercised, не built-in hard ceiling.
- Datagram отклонён как reliable primary transport.

## Среды и fingerprint

| Среда                    | Docker | Kernel           | Architecture | Result set                 | Fingerprint                                                        |
|--------------------------|-------:|------------------|--------------|----------------------------|--------------------------------------------------------------------|
| Docker Desktop на macOS  | 29.6.2 | LinuxKit 6.12.76 | aarch64      | `results/20260729T072602Z` | `cb1dd3d415e35ee1f58fa3de0de6e42a2583eecb24771c915402a64776eabfa7` |
| Docker Desktop на Ubuntu | 27.4.0 | LinuxKit 6.10.14 | x86_64       | `results/20260729T164723Z` | `cb1dd3d415e35ee1f58fa3de0de6e42a2583eecb24771c915402a64776eabfa7` |

Fingerprints идентичны. Repository HEAD, image ID и architecture различаются ожидаемо.

Обе container environments используют LinuxKit. Evidence подтверждает cross-architecture behavior внутри LinuxKit
aarch64/x86_64, но не native Linux без LinuxKit.

## Assertions

Все portable assertions прошли в обеих средах:

| Группа               | macOS | Ubuntu |
|----------------------|------:|-------:|
| Correctness          | 45/45 |  45/45 |
| Performance delivery | 81/81 |  81/81 |
| Pressure/resource    |   6/6 |    6/6 |
| Bounded memory       |   1/1 |    1/1 |
| Snapshot transaction |   2/2 |    2/2 |

## Основные доказательства

### Bounded parser

| Среда  | Message bytes | Messages | Max parser buffer | RSS before | RSS after |  RSS delta |
|--------|--------------:|---------:|------------------:|-----------:|----------:|-----------:|
| macOS  |       131 075 |       16 |          65 537 B |  9 028 KiB | 6 144 KiB | -2 884 KiB |
| Ubuntu |       131 075 |       16 |          65 537 B |  8 632 KiB | 6 432 KiB | -2 200 KiB |

Parser использует bounded `ReadSlice` chunks и drain oversized line без накопления полного message.
`performance.tsv` разделяет `go_runtime_sys_kib` и actual `/proc/self/statm` `rss_kib`.

### Shutdown, reconnect и ACK/NACK

В обеих средах active/unterminated connections закрылись внутри bounded shutdown, старая connection получила error,
reconnect доставил в новую epoch, valid `id=42` получил `ACK 42`, invalid `id=43` — `NACK 43 invalid`.

Для production complete-snapshot contract ACK выдаётся после atomic installation согласно ADR-004.

### Complete candidate assembly

Synthetic candidate 12 000 B собран из трёх bounded parts и committed атомарно. Следующий incomplete candidate не
заменил committed `snap-1`. Snapshot ID является transport correlation одной application publication, а не producer
ownership или aggregation key.

### Pressure и resources

| Среда  | Stream line | Stream framed | Datagram    | App-limit rejected | FD established/rejected |
|--------|-------------|---------------|-------------|-------------------:|------------------------:|
| macOS  | 2 000/2 000 | 2 000/2 000   | 1 875/2 000 |                 23 |                71 / 185 |
| Ubuntu | 2 000/2 000 | 2 000/2 000   | 2 000/2 000 |                  5 |                61 / 195 |

Application connection limit восстановился после освобождения connections. Отдельный `RLIMIT_NOFILE` case доказывает
только bounded OS errors.

Причина различия datagram не изолирована. Доказано только отсутствие portable reliable-delivery contract.

### Latency

| Среда  | Protocol    | Producers | Payload | Messages/s |    p50 |       p95 |       p99 |
|--------|-------------|----------:|--------:|-----------:|-------:|----------:|----------:|
| macOS  | stream-line |         1 |   1 KiB |    347 520 |  11 µs |    173 µs |    354 µs |
| Ubuntu | stream-line |         1 |   1 KiB |    257 842 |   7 µs |     64 µs |    403 µs |
| macOS  | stream-line |        32 |   8 KiB |    118 566 | 351 µs | 13.799 ms | 22.930 ms |
| Ubuntu | stream-line |        32 |   8 KiB |     64 249 | 268 µs | 26.546 ms | 44.630 ms |

INV-007 не имеет signal-to-exit: prototype не supervises workload и не отправляет signals.

## Запуск

```bash
./research/INV-007/run-bench.sh
```

## Ограничения

- Обе reference environments используют LinuxKit; native Linux не проверен.
- ACK/NACK и multipart grammar являются research forms, не final wire specification.
- Frame default 8 KiB требует realistic complete-snapshot cardinality benchmark.
- Memory evidence ограничено defined workload.
- Production code требует независимых unit/integration/fuzz/race/security tests.
