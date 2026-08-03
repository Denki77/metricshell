# INV-014 — Безопасность и resource limits

**Статус:** завершено
**Эталонный прогон macOS:** `results/20260802T173138Z`
**Эталонный прогон Ubuntu/LinuxKit:** `results/20260803T071713Z`
**Отчёт:** [report_ru.md](report_ru.md)
**Решение:** [ADR-014](../../docs-ru/06-architecture/adr/ADR-014.md)

## Вопрос и контекст

Какие security defaults и limits защищают MetricShell от untrusted local producer/scraper? По ADR-004 каждый request —
полный snapshot; checks применяются ко всему candidate, rejection не может частично добавить series.

## Необходимые доказательства

Complete replacement, malformed/duplicate retention, payload/series/label bounds, secret policy, concurrency/slow
clients, non-root/loopback/read-only/NNP/cap-drop, FD/memory/private path, bind failure, OOM и Ubuntu repeat.

## Подтверждённый результат

В обеих средах пройдено 27/27 assertions с одинаковым fingerprint. Двухсерийный snapshot заменён одно-серийным,
omitted series исчезла без sum. Все invalid/oversized
candidates отклонены с last-valid retention. Из 12 held requests принято 4, восемь получили 429. Server пережил slow
body и 100 malformed requests. Container: non-root, read-only, NNP, cap-drop ALL, 64 MiB, 64 FD, path 0700. 128 MiB
allocation под 32 MiB дала OOMKilled/137.

## Принятые значения

- payload 64 KiB; 1 000 series; 8 labels; 64 bytes на label field;
- concurrency 4, excess 429;
- bounded HTTP timeouts;
- 64 FD и 64 MiB research limits;
- non-root, read-only, NNP, cap-drop ALL, host loopback, path 0700;
- whole-candidate rejection и last-valid retention;
- secret-like label names отклоняются в research policy.

## Запуск

```bash
./research/INV-014/run-bench.sh
latest="$(cat research/INV-014/latest-results.txt)"
cat "$latest/assertions.tsv"
cat "$latest/concurrency.tsv"
cat "$latest/observations.tsv"
cat "$latest/environment.tsv"
```

OOM case намеренно ограничен named 32 MiB container. На Ubuntu команда та же.

## Threat model и ограничения

Attacker имеет доступ только к local ingestion/scrape и может слать malformed/large/concurrent/slow complete candidates.
Docker control, host root и kernel compromise не входят. Parser упрощён; secret detection name-based. Unix ACL,
SELinux/AppArmor/seccomp и production parser требуют отдельных deployment tests. Обе container-среды используют
LinuxKit: macOS/LinuxKit aarch64 и Ubuntu/LinuxKit x86_64; native Linux без LinuxKit не проверен.

## Additional Benchmarks

| Пункт                         | Статус         |
|-------------------------------|----------------|
| ADR replacement/no sum        | покрыто        |
| validation/last-valid         | покрыто        |
| payload/series/labels         | покрыто        |
| concurrency/slow/fuzz         | покрыто        |
| container hardening           | покрыто        |
| FD/memory/path/bind/OOM       | покрыто        |
| Ubuntu                        | покрыто: 27/27 |
| ACL/MAC/full parser fuzz/soak | follow-up      |

## Выход

- Prototype: `prototype/`
- macOS evidence: `results/20260802T173138Z/`
- Ubuntu evidence: `results/20260803T071713Z/`
- ADR: [ADR-014](../../docs-ru/06-architecture/adr/ADR-014.md)
- Report: [report_ru.md](report_ru.md)
