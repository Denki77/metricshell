# Отчёт INV-009 — Адаптер shared memory и mmap

**Статус:** завершено  
**Дата прогона:** 2026-08-02  
**Docker servers:** 29.6.2 (macOS/LinuxKit), 27.4.0 (Ubuntu/LinuxKit)  
**Docker platforms:** linux/aarch64, linux/x86_64  
**Референсные прогоны:** `results/20260802T162350Z`, `results/20260802T163406Z`  
**Fingerprint:** `41c97e834fba84c771980e2563a9817509add8fd419cde367b34cde5f2e77d07`

## Цель

Определить, даёт ли shared memory достаточно широкое преимущество transport performance, чтобы оправдать native binary
ABI с рисками synchronization и memory safety вместо framed Unix socket, при точном сохранении complete-snapshot
semantics из ADR-004.

## Коррекция scope по ADR-004

MetricShell принимает один полный, не содержащий конфликтов application snapshot за публикацию. Он структурно
валидирует candidate целиком и атомарно заменяет последний валидный snapshot, публикуемый через Prometheus endpoint.
Он не суммирует snapshots, не объединяет series, не применяет `increment`/`set`/`observe`, не хранит per-producer
contributions, не выполняет reconciliation последовательностей и не агрегирует независимо принадлежащие registries.

Поэтому benchmark рассматривает каждый payload как полный Prometheus-compatible candidate snapshot. Concurrent
publishers моделируют конкурентные отправки полных workload-owned candidates. Успешные candidates имеют один линейный
порядок замены. Publisher identity существует только для различения candidates в research workload; оно не является
application protocol state, и значения publishers не объединяются.

Ring sequence и commit markers — transport implementation mechanics для обнаружения полностью опубликованных bytes и
overwrites. Они не являются metric-state semantics и не разрешают partial snapshots или replay operations.

## Прототип

- `prototype/cmd/inv009-bench` — complete-snapshot correctness, failure и performance matrix.
- `prototype/Dockerfile` — воспроизводимый Linux image.
- `run-bench.sh` — одинаковый macOS/Ubuntu runner, evidence extraction, fingerprint и cgroup memory test.
- `results/<timestamp>` — assertions, performance observations, environment metadata и raw logs.

Каждый candidate body — синтаксически Prometheus-compatible полный snapshot, дополненный до выбранного размера. mmap
candidates используют versioned mapping header, fixed slots, atomic allocation и per-slot commit sequence. Socket
baseline использует length-delimited frames и exact reads. Все transports передают одинаковые complete-candidate bytes.

## Команды запуска

```bash
./research/INV-009/run-bench.sh
latest="$(cat research/INV-009/latest-results.txt)"
cat "$latest/publisher-performance.tsv"
cat "$latest/acceptance-performance.tsv"
cat "$latest/resources.tsv"
cat "$latest/assertions.tsv"
cat "$latest/external-assertions.tsv"
cat "$latest/environment.tsv"
```

На macOS и Ubuntu используются одна команда и один benchmark fingerprint.

## Окружения прогона

| Окружение                         | Дата       | Docker | Архитектура | Результат                  | Статус                  |
|-----------------------------------|------------|-------:|-------------|----------------------------|-------------------------|
| Docker Desktop на macOS/LinuxKit  | 2026-08-02 | 29.6.2 | aarch64     | `results/20260802T162350Z` | 80/80 assertions pass   |
| Docker Desktop на Ubuntu/LinuxKit | 2026-08-02 | 27.4.0 | x86_64      | `results/20260802T163406Z` | 80/80 проверок пройдено |

Оба прогона использовали fingerprint
`41c97e834fba84c771980e2563a9817509add8fd419cde367b34cde5f2e77d07`. Repository SHA и image ID сохранены только
как контекст.

## Результаты

### Стоимость публикации producer

`publisher-performance.tsv` измеряет только reserve/copy/commit для mmap и write/enqueue для socket. Эта метрика не
измеряет acceptance и не используется как throughput адаптера. Например, mmap publisher p50 `125–166 ns` для
1 publisher/128 B — это стоимость локального commit, тогда как end-to-end p50 составляет `542–1 083 ns`.
`publisher_publications_per_second` — величина, обратная среднему времени локального commit/write, а не wall-clock
throughput принятых snapshots.

### End-to-end acceptance

Для каждого transport работает live consumer: он читает полные bytes, вызывает одинаковую validation function,
атомарно заменяет ссылку на immutable active state, подтверждает конкретный publication sequence и увеличивает accepted
count. Elapsed заканчивается только после всех acknowledgements.

| Publishers | Snapshot | mmap file accepted/s | mmap tmpfs accepted/s | socket accepted/s | Лучший      |
|-----------:|---------:|---------------------:|----------------------:|------------------:|-------------|
|          1 |    128 B |              293 535 |               388 400 |            68 321 | mmap tmpfs  |
|          8 |    128 B |              302 658 |               474 496 |           164 690 | mmap tmpfs  |
|          1 |    4 KiB |              121 243 |               188 694 |            39 838 | mmap tmpfs  |
|          8 |    4 KiB |              173 940 |               174 395 |           124 292 | mmap tmpfs  |
|          1 |   64 KiB |               15 528 |                19 222 |            14 536 | mmap tmpfs  |
|          8 |   64 KiB |               21 240 |                24 282 |            28 513 | Unix socket |

End-to-end результаты Ubuntu/LinuxKit x86_64:

| Publishers | Snapshot | mmap file accepted/s | mmap tmpfs accepted/s | socket accepted/s | Лучший      |
|-----------:|---------:|---------------------:|----------------------:|------------------:|-------------|
|          1 |    128 B |              118 574 |               111 120 |            20 864 | mmap file   |
|          8 |    128 B |              352 135 |               340 843 |           126 318 | mmap file   |
|          1 |    4 KiB |               27 917 |                30 792 |             6 506 | mmap tmpfs  |
|          8 |    4 KiB |               24 576 |                28 138 |            23 513 | mmap tmpfs  |
|          1 |   64 KiB |                3 903 |                 3 642 |             2 503 | mmap file   |
|          8 |   64 KiB |                2 289 |                 2 868 |             3 985 | Unix socket |

В каждом окружении все 16 740 candidates были валидированы, установлены и подтверждены без ошибок. Ни один transport
не выигрывает стабильно. Параллельные readers выполнили 158 526 чтений active state на aarch64 и 84 677 на x86_64 и
видели только точные известные старый или новый snapshots. В обоих окружениях пройдены все 78 внутри-контейнерных и
обе внешние проверки.

### Correctness и failure behavior

- Active state — immutable snapshot за atomic pointer. Concurrent readers видели только полный старый или новый snapshot
  и ноль torn/mixed states для каждого transport и shape.
- Ring на 64 slots сообщил о 936 overwritten candidates после 1000 публикаций. Overwrite accounting — transport
  evidence, а не признание потерянных candidates принятым состоянием.
- Реальный child writer завершился 99 после изменения payload bytes без публикации slot commit marker. После reopen этот
  candidate считался uncommitted.
- Mapping version 99 была отклонена.
- Reopen восстановил transport committed sequence 100. Это проверка mapping recovery; ADR-004 не требует, чтобы новое
  выполнение MetricShell reconciled независимо переживших его producers.
- Mapping permissions были `0600`.
- Выделение 128 MiB при cgroup limit 32 MiB завершилось с кодом 137; Docker сообщил `OOMKilled=true`.

### Ресурсы

`resources.tsv` сообщает CPU на принятый snapshot. Для 8 publishers/4 KiB на aarch64 значения составили `17 267 ns`
для file mmap, `16 138 ns` для tmpfs mmap и `26 787 ns` для socket; на x86_64 — `133 576 ns`, `122 461 ns` и
`232 871 ns`. Peak RSS составил соответственно `21 576 KiB` и `19 828 KiB`. Это environment-specific observations,
а не acceptance limits.

## Оценка гипотезы

### Shared memory даёт максимальную raw transport performance

Не подтверждено как общее утверждение об adapter. После включения одинаковой consumer work и acknowledgement победитель
зависит от размера snapshot и concurrency. mmap выигрывает проверенные формы 128 B и 4 KiB, а Unix socket лидирует при
8 publishers и snapshot 64 KiB.

### Выигрыш не оправдывает общую client complexity

Подтверждено. Correctness требует binary mapping schema, cross-process atomics, правил alignment и memory
order, per-slot commit visibility, поведения overflow, capacity enforcement и crash handling. Ничто из этого не заменяет
ADR-004 validation или atomic replacement active snapshot; это дополнительная transport complexity.

### Стоимость внедрения для PHP существенно выше socket

Подтверждено анализом interface, а не изменением metric contract. PHP имеет переносимые stream/socket APIs, но не имеет
переносимого встроенного mmap плюс atomic-ring API. Требуется FFI или extension, который должен воспроизвести точный
binary ABI и memory-order contract. Benchmark одного extension не устраняет deployment и memory-safety cost.

### Portability слабее socket adapter

Подтверждено в пределах проверенного container scope. File mmap широко доступен, но atomics, endianness, alignment,
mapping lifetime, tmpfs sizing, descriptor inheritance и SIGBUS behavior образуют платформозависимый ABI.

## Оценка по критериям

| Критерий                         | Shared memory/mmap                               | Framed Unix socket                         |
|----------------------------------|--------------------------------------------------|--------------------------------------------|
| complete snapshot contract       | возможен, явно проверен                          | естественный framed message                |
| throughput принятых snapshots    | выигрывает часть форм; стабильного лидерства нет | выигрывает часть форм; более простой ABI   |
| atomic application replacement   | всё равно требуется после candidate commit       | всё равно требуется после frame validation |
| cross-snapshot aggregation       | запрещена                                        | запрещена                                  |
| обнаружение crash/torn candidate | требуется явный commit protocol                  | truncated frame отклоняется                |
| overflow/backpressure            | требуются shared capacity и overwrite policy     | socket buffering/backpressure              |
| schema/versioning                | binary ABI плюс application format               | framing плюс application format            |
| PHP client                       | FFI/extension                                    | встроенные stream APIs                     |
| portability/debugging            | ниже                                             | выше                                       |

## Допустимые значения и политики

- Точно сохранять ADR-004: один полный candidate, whole-candidate validation, atomic replacement, отсутствие
  суммирования
  и merge, отсутствие per-producer state в MetricShell.
- Не выбирать shared memory default adapter по текущим доказательствам.
- Если transport остаётся expert opt-in: versioned little-endian header, aligned atomics, per-slot commit marker,
  observable overwrite/backpressure policy, права `0600` и hard mapping-size limit обязательны.
- Успешный transport commit недостаточен для application acceptance: полный candidate всё равно проходит structural
  validation и атомарно заменяет последний валидный snapshot.
- Неизвестные mapping versions отклоняются. После crash доступны только committed complete candidate bytes.
- Проверенный диапазон: полные snapshots 128 B–64 KiB и 1–8 concurrent candidate publishers.
- Anonymous inherited mappings ограничены процессами, получившими inherited descriptor, и не являются общим endpoint
  для unrelated post-exec clients.
- Размер `/dev/shm` настраивается явно; касание страниц сверх backing capacity может вызвать SIGBUS.

## Ограничения прототипа

- Оба окружения доказательств используют LinuxKit. Совпавшие прогоны aarch64/x86_64 не проверяют native non-LinuxKit
  Linux, containerd, CRI-O или Kubernetes.
- Прототип проверяет transport publication полных candidates, а не production Prometheus parsing, все structural
  conflicts ADR-004, scrape concurrency active state или final-state freezing. Они остаются обязательными implementation
  tests и вопросами INV-010/INV-011.
- Concurrent publishers — различимые полные candidates, а не независимые владельцы метрик.
- Go ring не является формально проверенной multi-process queue. Independent-process и multi-language conformance не
  проверены.
- In-process commit notification будит mmap consumer вместо production doorbell. Candidate bytes и acknowledgements
  остаются в mapping, но production behavior eventfd/futex/polling не измеряется.
- Default capacity Docker `/dev/shm` ограничивает матрицу. Exploratory oversized mapping дал SIGBUS и был исключён, а не
  опубликован как performance result.
- Timing, CPU и RSS — архитектурные сравнения, а не SLO.

## Дополнительные бенчмарки

| Пункт                                            | Статус           | Доказательство/причина                                     |
|--------------------------------------------------|------------------|------------------------------------------------------------|
| полные snapshots; отсутствие суммирования/merge  | покрыто          | replacement assertions и фиксированные complete candidates |
| backing stores, sizes и concurrency              | покрыто          | 18 сопоставимых rows                                       |
| изолированная стоимость publisher commit         | покрыто          | `publisher-performance.tsv`; это не acceptance             |
| live consumer и exact sequence acknowledgement   | покрыто          | `acceptance-performance.tsv`                               |
| end-to-end p50/p95/p99 и accepted throughput     | покрыто          | `acceptance-performance.tsv`                               |
| CPU на принятый snapshot и RSS                   | покрыто          | `resources.tsv`                                            |
| atomic active-state reader                       | покрыто          | 158 526 reads, ноль bad states                             |
| correctness acknowledged/accepted count          | покрыто          | `assertions.tsv`                                           |
| overwrite, crash/torn candidate, schema и reopen | покрыто          | `assertions.tsv`                                           |
| permissions и cgroup OOM                         | покрыто          | exit 137 и `OOMKilled=true`                                |
| Ubuntu repeat с совпадающим fingerprint          | покрыто          | идентичный fingerprint; 80/80 проверок пройдено            |
| PHP FFI/extension                                | adoption blocker | нет переносимого extension-free atomic mmap API            |
| полная Prometheus structural validation          | не дублируется   | requirement ADR-004; integration INV-010                   |
| concurrent scrape во время replacement           | не дублируется   | requirement ADR-004/INV-010                                |
| native perf/eBPF и 30+ повторов                  | рекомендуется    | native non-LinuxKit Linux с закреплением CPU               |
| production wraparound soak                       | отложен          | требует выбранных ABI и backpressure policy                |

## Вывод

Исходная гипотеза подтверждена частично, исследование завершено. Shared memory уменьшает стоимость
publisher commit, но в сопоставимом end-to-end acceptance нет стабильного победителя. Он добавляет native binary ABI с
рисками synchronization и memory safety, не уменьшая ни одной обязанности ADR-004.

Итоговое решение — не добавлять default shared-memory adapter. Сохранить его только как возможный expert opt-in,
если production profiling обнаружит small-snapshot transport bottleneck, не решаемый batching или socket path.
Решение зафиксировано в ADR-009.

## Выход решения

- Prototype: `prototype/`
- Runner: `run-bench.sh`
- Raw evidence: `results/20260802T162350Z/`, `results/20260802T163406Z/`
- Summary: только complete snapshots; никакой aggregation; end-to-end преимущество mmap нестабильно.
- ADR: [ADR-009](../../docs-ru/06-architecture/adr/ADR-009.md)
