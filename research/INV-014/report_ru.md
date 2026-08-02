# Отчёт INV-014 — Безопасность и limits

**Статус:** в процессе
**Дата:** 2026-08-02
**Прогон:** `results/20260802T173138Z`
**Fingerprint:** `5a5066721a556bba9ce836e691757ceeefbab9eac05e3d6f5cab7c6ae027c1b3`

## Цель и ADR boundary

Выбрать bounded default posture. Candidate целиком валидируется до atomic replacement; rejection никогда не merge с
active state.

## Результаты

Все 27 assertions прошли. Omitted series удаляется. Malformed/duplicate/secret дают 400, label/cardinality — 422,
payload — 413, concurrency — 429. Last-valid snapshot сохраняется. 4 concurrent requests accepted, 8 rejected. Slow и
100 malformed не нарушили health.

Container реально проверен: non-root, read-only, NNP, cap-drop ALL, loopback, FD 64, memory 64 MiB, path 0700. Invalid
bind дал 70. Allocation 128 MiB под cgroup 32 MiB дал OOMKilled и exit 137.

## Проверка гипотез и policy

Whole-candidate limits сохраняют integrity; bounded semaphore лучше unlimited queue; hardening не мешает работе;
cgroup детерминированно ограничивает exhaustion. Полностью отклонять candidate, сохранять last valid, использовать
400/413/422/429, не отражать rejected labels в self-metrics. Defaults: local-only, non-root, read-only, NNP, no caps,
private paths и explicit FD/PID/memory/time limits.

Числа prototype — admissible research defaults, не финальная capacity.

## Ограничения и Additional

Нет full Prometheus parser, TLS/auth, Unix ACL и MAC profiles. Локальная матрица выполнена полностью; Ubuntu, production
fuzz corpus, soak и deployment security остаются.

## Вывод

Предварительно принят bounded non-root local posture и immutable complete replacement. Ни один failure не повредил
active snapshot. Завершение после Ubuntu и production parser validation.

## Выход

- Evidence: `results/20260802T173138Z/`
- Ubuntu/ADR: ожидаются
