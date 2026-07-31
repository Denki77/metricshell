# Отчёт INV-007 — Приём метрик через socket

[English version](report.md)

**Статус:** завершено  
**Дата прогонов:** 2026-07-29  
**Эталонные прогоны:** `results/20260729T072602Z`, `results/20260729T164723Z`  
**Fingerprint:** `cb1dd3d415e35ee1f58fa3de0de6e42a2583eecb24771c915402a64776eabfa7`

## Цель

Выбрать bounded acknowledged local socket protocol для complete application snapshot contract пересмотренного ADR-004.

## Коррекция scope

MetricShell получает один complete application snapshot за publication. Он не агрегирует независимые producer
registries, не принимает instrumentation operations и не владеет producer sequencing/reconciliation.

Multipart case является transport framing одного complete candidate. Ни одна part не становится metric state. ACK
подтверждает complete validation и atomic installation.

## Среды

| Среда                    | Docker | Kernel           | Architecture | Result set         |
|--------------------------|-------:|------------------|--------------|--------------------|
| Docker Desktop на macOS  | 29.6.2 | LinuxKit 6.12.76 | aarch64      | `20260729T072602Z` |
| Docker Desktop на Ubuntu | 27.4.0 | LinuxKit 6.10.14 | x86_64       | `20260729T164723Z` |

Fingerprints идентичны. Обе container environments используют LinuxKit; native Linux без LinuxKit не покрыт.

## Cross-environment confirmation

| Evidence                       |       macOS |      Ubuntu |
|--------------------------------|------------:|------------:|
| Correctness                    |       45/45 |       45/45 |
| Performance delivery           |       81/81 |       81/81 |
| Pressure/resource              |         6/6 |         6/6 |
| Memory                         |         1/1 |         1/1 |
| Snapshot                       |         2/2 |         2/2 |
| Max parser buffer              |    65 537 B |    65 537 B |
| RSS delta                      |  -2 884 KiB |  -2 200 KiB |
| Stream-line slow reader        | 2 000/2 000 | 2 000/2 000 |
| Datagram slow reader           | 1 875/2 000 | 2 000/2 000 |
| App-limit rejections           |          23 |           5 |
| System FD established/rejected |      71/185 |      61/195 |

Все portable assertions прошли в обеих средах. Exact timings и resource counts являются observations.

## Результаты

### Bounded framing

Line reader использует bounded `ReadSlice`. Oversized input дренируется без buffering полного line. В обеих средах
messages 131 075 B дали максимум 65 537 B parser buffer и отсутствие RSS growth в measured case.

### Shutdown и reconnect

Server закрывает tracked active connections. Bounded shutdown, error old connection, refusal после shutdown и reconnect
в новую epoch прошли в обеих средах.

### Application acknowledgement

`ACK <id>` отличает application acceptance от successful socket write. Valid input получил ACK, invalid input — NACK.
Согласно ADR-004 success означает complete candidate validation и atomic installation.

Disconnect до ACK имеет ambiguous result. Повтор complete snapshot может создать ещё одну linear acceptance, но не
требует per-producer operation sequences внутри MetricShell.

### Complete candidate framing

Complete candidate 12 000 B собран из трёх bounded frames и committed. Следующая incomplete transaction сохранила
previous snapshot. Production grammar должна ограничивать total bytes, parts, lifetime и concurrent candidates.

### Datagram и connection limits

Datagram результаты различались; причина не изолирована. Подтверждено только отсутствие portable reliable contract.

Application limit отклонил excess connections и восстановился после освобождения. `RLIMIT_NOFILE` case отдельно
подтверждает bounded system errors.

## Принятое направление

- Unix stream primary transport.
- Versioned newline-delimited framing.
- Confirmed publication mode с correlation ID и ACK/NACK.
- ACK после complete validation и atomic installation.
- Multipart framing, если candidate больше одного frame.
- Initial configurable frame default 8 KiB; final value определяется realistic snapshot benchmarks.
- 65 536 B — только maximum individual payload exercised.
- Datagram отклонён как reliable primary.
- Application connection limit ниже `RLIMIT_NOFILE`.
- Bounded deadlines.

## Signal-to-exit

Не применимо. Релевантная метрика — producer timestamp → protocol acceptance.

## Ограничения

Обе среды используют LinuxKit. Native Linux, Kubernetes и multi-user permissions не проверены. Final snapshot grammar
и production structural parser не реализованы. Memory assertion ограничено defined workload.

## Вывод

Matching-fingerprint evidence LinuxKit aarch64/x86_64 подтверждает Unix stream с bounded versioned line framing и
application ACK/NACK для complete application snapshots. Все assertions прошли в обеих средах.

Решение зафиксировано в [ADR-007](../../docs-ru/06-architecture/adr/ADR-007.md).
