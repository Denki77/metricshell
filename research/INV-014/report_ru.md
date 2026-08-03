# Отчёт INV-014 — Безопасность и resource limits

**Статус:** завершено
**Даты прогонов:** 2026-08-02–2026-08-03
**Эталонные прогоны:** `results/20260802T173138Z`, `results/20260803T071713Z`
**Fingerprint:** `5a5066721a556bba9ce836e691757ceeefbab9eac05e3d6f5cab7c6ae027c1b3`
**Решение:** [ADR-014](../../docs-ru/06-architecture/adr/ADR-014.md)

## Цель и граница ADR-004

Выбрать bounded security posture для локальных producers и scrapers. Каждая публикация содержит один полный candidate
snapshot. Syntax, policy и resource checks завершаются до atomic replacement; отклонение не объединяет candidate с
active state и не сохраняет omitted series старого snapshot.

## Threat model

Недоверенный локальный процесс может отправлять malformed, oversized, high-cardinality, label-heavy, slow или concurrent
complete candidates. Scraper может открывать slow или excess connections. Attacker не имеет Docker control, host root,
kernel control и разрешения внешней публикации endpoint. Владельцы instrumentation отвечают за secrets внутри иначе
допустимых metric values.

## Прототип

Исследовательский Go server предоставляет local ingestion и metrics, хранит immutable complete snapshot через atomic
pointer и применяет payload, series, label, concurrency и timeout bounds. Docker задаёт user, filesystem, capabilities,
descriptor и memory isolation. Runner проверяет normal replacement, validation failures, overload, slow clients,
повторный malformed input, bind failure и намеренный bounded cgroup OOM.

## Среды прогонов

| Среда                             | Дата       | Docker | Архитектура | Результат                  | Статус                |
|-----------------------------------|------------|-------:|-------------|----------------------------|-----------------------|
| Docker Desktop на macOS/LinuxKit  | 2026-08-02 | 29.6.2 | aarch64     | `results/20260802T173138Z` | 27/27 assertions pass |
| Docker Desktop на Ubuntu/LinuxKit | 2026-08-03 | 27.4.0 | x86_64      | `results/20260803T071713Z` | 27/27 assertions pass |

Оба прогона имеют одинаковый fingerprint; все 27 assertions прошли в каждой среде. Обе container-среды используют
LinuxKit. Native Linux без LinuxKit и deployment-specific mandatory access control не проверены.

## Результаты

### Complete replacement и validation

Двухсерийный snapshot заменён одно-серийным. Omitted series `alpha` исчезла, осталась только `beta 7`; значения разных
snapshot не суммировались. Malformed candidate вернул HTTP 400 и не изменил last valid state. Duplicate series и
secret-like label names вернули 400, нарушения label/cardinality — 422, payload более 64 KiB — 413.

### Concurrency и hostile clients

В обеих средах 12 одновременно удерживаемых публикаций дали одинаковый aggregate result: 4 accepted и 8 responses HTTP
с кодом 429. Порядок конкретных requests различался ожидаемо, но configured ceiling сохранился. Partial slow request и
100 malformed requests не нарушили health и не изменили active snapshot.

| Наблюдение                       | macOS/LinuxKit | Ubuntu/LinuxKit |
|----------------------------------|---------------:|----------------:|
| Accepted concurrent publications |              4 |               4 |
| Rejected concurrent publications |              8 |               8 |
| Mode/UID/GID private path        |    700:100:101 |     700:100:101 |

### Container и resource controls

| Control           | Проверенное значение     | Результат           |
|-------------------|--------------------------|---------------------|
| UID               | non-zero                 | pass в обеих средах |
| root filesystem   | read-only                | pass в обеих средах |
| capabilities      | drop ALL                 | pass в обеих средах |
| no-new-privileges | enabled                  | pass в обеих средах |
| host binding      | 127.0.0.1                | pass в обеих средах |
| open files        | 64                       | pass в обеих средах |
| normal memory     | 64 MiB                   | pass в обеих средах |
| private path      | mode 0700, app-owned     | pass в обеих средах |
| invalid bind      | internal exit 70         | pass в обеих средах |
| forced allocation | 128 MiB при limit 32 MiB | OOMKilled, exit 137 |

## Проверка гипотез

### Whole-candidate bounds сохраняют integrity metric state

Подтверждено. Каждый syntax, policy и resource rejection сохранил предыдущий полный snapshot. Ни один failure не вызвал
partial addition, summation или stale-series retention.

### Excess concurrency должен быстро отклоняться

Подтверждено. Bounded semaphore принял четыре requests и отклонил excess work с 429 вместо unlimited queue. Одинаковый
aggregate behavior получен на обеих архитектурах.

### Container hardening должен быть default

Подтверждено для проверенного local server. Non-root, read-only rootfs, no-new-privileges и dropped capabilities не
мешали normal operation.

### Требуется внешнее resource enforcement

Подтверждено. Controlled allocation превысила cgroup limit 32 MiB и завершилась OOMKilled/137, не изменив валидность
ранее снятого evidence.

## Принятые значения и policies

- Полностью отклонять candidate при syntax, duplicate, secret policy или нарушении configured bound.
- Сохранять last valid complete snapshot при rejection.
- Использовать 400 для malformed/policy input, 413 для payload, 422 для cardinality/labels и 429 для concurrency.
- По умолчанию оставлять ingestion локальным и публиковать container ports только на loopback.
- Запускать non-root с read-only rootfs, `no-new-privileges`, без Linux capabilities и с private runtime paths.
- Настраивать явные payload, series, labels, concurrency, HTTP timeout, FD, PID и memory limits.
- Не копировать attacker-controlled rejected labels в self-metrics.
- Рассматривать 64 KiB, 1 000 series, 8 labels, 64-byte label fields, concurrency 4, 64 FDs и 64 MiB как проверенные
  research starting values, а не универсальные production capacity limits.

## Ограничения

- Обе container-среды используют LinuxKit; native Linux без LinuxKit не проверен.
- Parser намеренно меньше production Prometheus/OpenMetrics validator.
- Secret detection консервативен и name-based; это не DLP guarantee.
- Не реализованы TLS, authentication, Unix socket ACL и custom SELinux/AppArmor/seccomp profiles.
- Проверены одна normal memory/FD configuration и один OOM shape.
- Threat model исключает Docker control, host root и kernel compromise.

## Дополнительные benchmarks

| Benchmark                                    | Статус    | Evidence/граница                      |
|----------------------------------------------|-----------|---------------------------------------|
| ADR-004 complete replacement/no sum          | покрыто   | replacement response и assertions     |
| malformed/duplicate/last-valid retention     | покрыто   | HTTP codes и snapshot checks          |
| payload/series/label limits                  | покрыто   | assertions                            |
| concurrency ceiling                          | покрыто   | 4 accepted, 8 rejected в обеих средах |
| slow body и malformed-input repetition       | покрыто   | health/state assertions               |
| non-root/read-only/NNP/cap-drop              | покрыто   | container assertions                  |
| loopback/FD/memory/path permissions          | покрыто   | environment и assertions              |
| bind failure и cgroup OOM                    | покрыто   | exit 70; OOMKilled/137                |
| Ubuntu с matching fingerprint                | покрыто   | 27/27 assertions, fingerprint совпал  |
| full production parser corpus и fuzz/soak    | follow-up | production implementation             |
| Unix ACL и SELinux/AppArmor/seccomp profiles | follow-up | deployment-specific validation        |

## Вывод

INV-014 подтверждено matching-fingerprint прогонами macOS/LinuxKit и Ubuntu/LinuxKit. MetricShell использует local
non-root operation, immutable complete-snapshot replacement, fail-fast concurrency и явные whole-candidate, timeout и
cgroup bounds. Ни один проверенный failure не повредил active metric state. Решение зафиксировано в
[ADR-014](../../docs-ru/06-architecture/adr/ADR-014.md).

## Выход исследования

- macOS evidence: `results/20260802T173138Z/`
- Ubuntu evidence: `results/20260803T071713Z/`
- ADR: [ADR-014](../../docs-ru/06-architecture/adr/ADR-014.md)
