# Отчёт INV-007 — Приём метрик через socket

[English version](report.md)

**Статус:** в процессе

**Дата прогона:** 2026-07-29

**Reference run:** `results/20260729T072602Z`

**Fingerprint:** `cb1dd3d415e35ee1f58fa3de0de6e42a2583eecb24771c915402a64776eabfa7`

## Цель

Повторно проверить socket ingestion после устранения gaps в bounded parsing, shutdown, reconnect, acknowledgement,
authoritative snapshots, memory naming и application connection limits.

## Среда

| Среда                         |                               Docker | Kernel           | Architecture | Result set         |
|-------------------------------|-------------------------------------:|------------------|--------------|--------------------|
| macOS Docker Desktop/LinuxKit |                               29.6.2 | LinuxKit 6.12.76 | aarch64      | `20260729T072602Z` |
| Ubuntu/LinuxKit               | ожидается matching-fingerprint rerun | —                | x86_64       | —                  |

Текущая evidence environment использует LinuxKit. Native Linux без LinuxKit не проверен.

## Результаты

Все текущие macOS assertions прошли:

- correctness: 45/45;
- performance delivery: 81/81;
- pressure/resource: 6/6;
- bounded memory: 1/1;
- snapshot transaction: 2/2.

### Bounded parser и memory

`ReadBytes` заменён на bounded parser через `ReadSlice`. `bufio.ErrBufferFull` классифицирует message как oversized,
после чего line дренируется bounded chunks.

Для 16 messages по 131 075 B максимальный parser chunk составил 65 537 B. RSS изменился с 9 028 до 6 144 KiB
(-2 884 KiB), то есть не вырос и остался ниже test allowance 16 MiB.

Exactly-limit line без newline удерживался открытым до shutdown. Shutdown закрыл connection и завершился внутри
one-second assertion bound.

`performance.tsv` различает `go_runtime_sys_kib` (`MemStats.Sys`) и actual `rss_kib` из `/proc/self/statm`.

### Shutdown и reconnect

Server учитывает accepted stream connections и закрывает их при shutdown. Подтверждены bounded shutdown с active
client, error старой connection, отказ новых connections после shutdown и bounded reconnect в новую metric-state epoch.

### ACK/NACK contract

Confirmed mode использует correlation/message ID:

- `ACK <id>` отправляется после validation и atomic-application callback;
- `NACK <id> <reason>` сообщает rejection;
- disconnect до ACK имеет ambiguous result;
- retry с duplicate suppression использует producer identity и message/snapshot sequence.

Prototype подтвердил `ACK 42` и `NACK 43 invalid` для обоих stream framings.

### Authoritative snapshots

Большой authoritative snapshot передаётся transaction:

1. `snapshot_begin`;
2. bounded `snapshot_part`;
3. `snapshot_commit`.

Только commit атомарно заменяет producer snapshot. Committed 12 000-byte snapshot из трёх parts прошёл; следующий
incomplete snapshot сохранил предыдущую committed version. Это согласует socket ingestion с ADR-004.

### Resource limits

Application limit восемь concurrent stream connections отклонил excess clients и принял новую connection после
освобождения. Отдельный `RLIMIT_NOFILE=128` case доказывает bounded OS errors, но не application limit.

### Datagram

Текущий slow-reader case доставил 1 875/2 000 и сообщил 125 failures. Experiment не изолирует kernel, socket buffers,
runtime scheduling или host load. Корректный вывод: datagram не продемонстрировал portable reliable-delivery contract.

## Состояние решения

Направление остаётся Unix stream + versioned line protocol с acknowledged mode и multipart authoritative snapshots.
ADR-007 имеет статус Proposed до matching-fingerprint Ubuntu run.

Initial configurable frame default — 8 KiB. Это conservative operational value, не benchmark-derived limit.
`65 536 B` — maximum payload exercised в reference run, не built-in hard ceiling.

## Signal-to-exit

Не применимо. INV-007 не supervises workload и не отправляет signals. Релевантная latency — producer timestamp →
protocol acceptance.

## Оставшаяся работа

- выполнить полный Ubuntu runner с fingerprint
  `cb1dd3d415e35ee1f58fa3de0de6e42a2583eecb24771c915402a64776eabfa7`;
- сравнить assertions и revised TSV schemas;
- переводить status в completed/accepted только после matching evidence;
- измерить realistic snapshot format/cardinality;
- определить final ACK/NACK и multipart snapshot wire grammar;
- отдельно проверить native non-LinuxKit Linux.
