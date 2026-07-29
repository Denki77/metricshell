# INV-007 — Приём метрик через socket

[English version](README.md)

Статус: в процессе

Текущий эталонный прогон: `results/20260729T072602Z`

Отчёт: [report_ru.md](report_ru.md)

Предлагаемое решение: [ADR-007](../../docs-ru/06-architecture/adr/ADR-007.md)

## Уточнённое направление protocol

- Primary transport: Unix stream.
- Native protocol: versioned text с newline framing.
- Confirmed acceptance mode: message ID и `ACK <id>` после syntactic/semantic validation и atomic application либо
  `NACK <id> <reason>`.
- Disconnect до ACK имеет ambiguous result; retry может дать duplicate, поэтому требуются producer identity,
  snapshot ID и sequence/message ID.
- Authoritative snapshot передаётся через `snapshot_begin`, bounded `snapshot_part` и `snapshot_commit`. Новое состояние
  применяется атомарно только при valid commit. Operations остаются optional acceleration path согласно ADR-004.
- Initial configurable default одного frame: 8 KiB. Это консервативное начальное значение, не architecture limit,
  выведенный из benchmark.
- Maximum payload, проверенный текущим reference run: 65 536 B.

## Текущая среда

| Среда                   |                 Docker | Kernel           | Architecture | Result set                 | Fingerprint                                                        |
|-------------------------|-----------------------:|------------------|--------------|----------------------------|--------------------------------------------------------------------|
| Docker Desktop на macOS |                 29.6.2 | LinuxKit 6.12.76 | aarch64      | `results/20260729T072602Z` | `cb1dd3d415e35ee1f58fa3de0de6e42a2583eecb24771c915402a64776eabfa7` |
| Ubuntu                  | ожидается новый прогон | —                | x86_64       | —                          | должен совпасть                                                    |

Текущая container environment использует LinuxKit. Native Linux без LinuxKit не проверен.

## Результаты

| Группа assertions    | Результат |
|----------------------|----------:|
| Correctness          |     45/45 |
| Performance delivery |     81/81 |
| Pressure/resource    |       6/6 |
| Bounded memory       |       1/1 |
| Snapshot transaction |       2/2 |

### Bounded line parser

Line reader использует bounded `ReadSlice` chunks и drain oversized line без накопления полного message.

| Message bytes | Messages | Max parser buffer | RSS before | RSS after |  RSS delta | Allowed delta |
|--------------:|---------:|------------------:|-----------:|----------:|-----------:|--------------:|
|       131 075 |       16 |          65 537 B |  9 028 KiB | 6 144 KiB | -2 884 KiB |    16 384 KiB |

Отдельный case удерживает ровно limit bytes без newline и не закрывает connection. Shutdown сервера закрывает active
connection и завершается менее чем за одну секунду.

### Shutdown и restart

- Active idle/partial stream connections закрываются server shutdown.
- Old connection получает error.
- Bounded reconnect доставляет message только в новую epoch.
- После shutdown новые connections отклоняются.

### ACK/NACK

Для обоих stream framings подтверждены:

- valid `id=42` → `ACK 42`;
- invalid `id=43` → `NACK 43 invalid`.

ACK разрешён только после callback, представляющего validation и atomic application. Disconnect до ACK имеет unknown
result и требует protocol retry/deduplication policy.

### Authoritative snapshots

Synthetic snapshot размером 12 000 B передан тремя bounded parts и committed как `snap-1`. Следующий incomplete snapshot
не изменил committed version. Final grammar и cardinality limits остаются отдельной protocol work.

### Pressure и resource limits

| Case                                  | Input |            Delivered | Failed/rejected |
|---------------------------------------|------:|---------------------:|----------------:|
| stream-line slow reader               | 2 000 |                2 000 |               0 |
| stream-framed slow reader             | 2 000 |                2 000 |               0 |
| datagram slow reader                  | 2 000 |                1 875 |             125 |
| application connection limit/recovery |    32 | 1 после освобождения |              22 |
| system FD exhaustion                  |   256 |                   71 |             185 |
| datagram FD model                     |   256 |                  256 |               0 |

Experiment не изолирует причину различий datagram между runs или hosts. Подтверждено только отсутствие portable
reliable-delivery contract в bounded-pressure scenario.

### Memory columns

`performance.tsv` теперь различает:

- `go_runtime_sys_kib` — Go `MemStats.Sys`;
- `rss_kib` — actual process resident pages из Linux `/proc/self/statm`.

Это combined producer/server observations, не isolated peak RSS сервера.

## Запуск

Команда одинакова на macOS и Ubuntu:

```bash
./research/INV-007/run-bench.sh
```

```bash
cat "$(cat research/INV-007/latest-results.txt)/summary.tsv"
cat "$(cat research/INV-007/latest-results.txt)/correctness.tsv"
cat "$(cat research/INV-007/latest-results.txt)/memory.tsv"
cat "$(cat research/INV-007/latest-results.txt)/performance.tsv"
cat "$(cat research/INV-007/latest-results.txt)/pressure.tsv"
cat "$(cat research/INV-007/latest-results.txt)/snapshot.tsv"
cat "$(cat research/INV-007/latest-results.txt)/environment.tsv"
```

Ubuntu evidence сравнимо только при fingerprint
`cb1dd3d415e35ee1f58fa3de0de6e42a2583eecb24771c915402a64776eabfa7`.

## Ограничения и оставшаяся работа

- До завершения нужен matching-fingerprint Ubuntu run.
- Обе планируемые reference environments используют LinuxKit; native Linux остаётся отдельным gap.
- ACK/NACK и snapshot grammar являются research forms, не final wire specification.
- Configurable default 8 KiB требует realistic parser/cardinality benchmark.
- Memory test доказывает bounded parser buffer и RSS delta своего workload, не universal memory bound.
- Race-enabled build проходит; production implementation требует отдельных unit/integration race tests.
