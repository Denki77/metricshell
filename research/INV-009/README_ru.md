# INV-009 — Адаптер shared memory и mmap

Статус: завершено

Референсный прогон macOS: `results/20260802T162350Z`

Референсный прогон Ubuntu/LinuxKit: `results/20260802T163406Z`

Отчёт: [report_ru.md](report_ru.md)

## Вопрос

Может ли shared memory или mmap дать полезный высокопроизводительный адаптер, не делая клиенты небезопасными или
платформозависимыми?

## Контекст и гипотеза

ADR-004 фиксирует контракт приложения: каждый transport передаёт один полный, не содержащий конфликтов
Prometheus-compatible application snapshot. MetricShell валидирует candidate целиком и атомарно заменяет последний
валидный snapshot. Он не суммирует snapshots, не объединяет series, не принимает instrumentation operations, не хранит
per-producer contributions и не агрегирует независимые registries.

Поэтому INV-009 сравнивает только transport mechanics для одного и того же полного snapshot. «Concurrent publishers»
в benchmark — это конкурентные отправки полных workload-owned candidate snapshots. MetricShell назначает принятым
candidates единый линейный порядок замены; они не являются независимыми владельцами метрик, и их значения никогда не
объединяются.

Гипотеза состоит в том, что shared memory может ускорить публикацию малых snapshots, но выигрыш уменьшится с ростом
snapshot и не оправдает стоимость binary ABI, synchronization, recovery, portability и PHP client.

## Требуемые доказательства

- одинаковая complete-snapshot semantics для file mmap, `/dev/shm` mmap и framed Unix socket;
- 1 и 8 concurrent publishers полных snapshots размером 128 B, 4 KiB и 64 KiB;
- раздельные метрики стоимости commit на стороне publisher и end-to-end acceptance;
- одинаковый live consumer path: чтение, validation, atomic install, acknowledgement и подсчёт;
- throughput принятых snapshots, end-to-end p50/p95/p99, CPU на acceptance и RSS;
- atomic replacement без cross-snapshot merge;
- overflow, crash процесса во время незакоммиченного candidate, schema mismatch, reopen и memory limits;
- приватные права, client complexity, portability и Ubuntu repeat с совпадающим fingerprint.

## Текущий результат

Оба прогона, macOS/LinuxKit aarch64 и Ubuntu/LinuxKit x86_64, прошли 78/78 внутри-контейнерных assertions и обе внешние
OOM-проверки. В каждом окружении все 16 740 полных candidates были прочитаны, валидированы, атомарно установлены и
подтверждены живыми consumers. Параллельные readers
active state выполнили соответственно 158 526 и 84 677 чтений и не увидели ни одного malformed/torn или неизвестного
состояния.

Стоимость публикации producer и end-to-end acceptance приведены раздельно. Первая измеряет только локальный commit или
socket write/enqueue и не называется throughput адаптера. У сопоставимых end-to-end результатов нет универсального
победителя: tmpfs mmap лидировал для 1 publisher/128 B (`388 400 accepted/s`) и 8 publishers/128 B (`474 496/s`),
file и tmpfs mmap почти сравнялись при 8 publishers/4 KiB (`173 940/s` и `174 395/s`), а Unix socket лидировал при
8 publishers/64 KiB (`28 513/s`).
Гипотеза подтверждена лишь частично: выигрыш зависит от формы нагрузки и не
оправдывает default native binary ABI.

Оба прогона зафиксировали fingerprint `41c97e834fba84c771980e2563a9817509add8fd419cde367b34cde5f2e77d07`.
Исследование завершено, решение зафиксировано в [ADR-009](../../docs-ru/06-architecture/adr/ADR-009.md).

## Допустимые значения

- application operation: валидация полного candidate с последующей atomic replacement active snapshot;
- никакого суммирования, merge, replay operations, producer identity или cross-producer aggregation;
- проверенные размеры snapshots: `128 B`, `4 KiB`, `64 KiB`;
- проверенная concurrency: `1–8` candidate publishers с единым линейным acceptance order;
- права mapping file: `0600`;
- transport metadata shared memory может использовать versioned header и per-slot commit marker, но это transport
  mechanics, которые не должны становиться application metric semantics;
- overwrite допустим только при явной наблюдаемости; overwritten candidate не является принятым application state;
- неизвестная mapping version отклоняется; после crash доступны только полностью committed candidate bytes;
- mapped bytes ограничиваются ниже container memory и `/dev/shm` backing limits;
- shared memory не выбран как default ingestion adapter.

## Запуск прототипа

Из корня репозитория одинаково на macOS и Ubuntu:

```bash
./research/INV-009/run-bench.sh
```

Просмотр последнего результата:

```bash
latest="$(cat research/INV-009/latest-results.txt)"
cat "$latest/assertions.tsv"
cat "$latest/external-assertions.tsv"
cat "$latest/publisher-performance.tsv"
cat "$latest/acceptance-performance.tsv"
cat "$latest/resources.tsv"
cat "$latest/environment.tsv"
```

Runner собирает один Linux image, выполняет полную матрицу без host bind mounts, копирует `/out` через `docker cp` и
отдельно выполняет cgroup memory-limit case. На Ubuntu используется та же команда. Сравнивать нужно
`benchmark_code_fingerprint_sha256`; image IDs и repository HEAD — контекст, а не идентичность исследовательского
стенда.

Ручной запуск:

```bash
docker build --pull=false -t metricshell-inv009:prototype research/INV-009/prototype
docker run --rm metricshell-inv009:prototype /out
```

## Ограничения прототипа

- Это research code, а не production parser, registry или формально проверенная multi-process lock-free queue.
- Candidate bodies — синтаксически Prometheus-compatible полные snapshots фиксированных размеров. Benchmark проверяет
  transport copy/publication mechanics, а не полную production structural validation или scrape concurrency.
- Concurrent publishers отправляют полные candidates. Они не моделируют независимые producer registries, и MetricShell
  никогда не агрегирует их значения.
- Ring использует Go atomics. Multi-language production ABI требует заданных alignment и memory ordering, а также
  independent-process conformance tests.
- `publisher_publications_per_second` — величина, обратная среднему времени локального commit/write; это стоимость
  publisher, а не wall throughput и не throughput принятых snapshots.
- mmap consumer использует in-process commit notification как замену production doorbell. Он всё равно читает и
  валидирует mmap bytes и подтверждает acceptance через mapped accepted-sequence marker; выбор eventfd/futex/polling не
  измерен.
- PHP не имеет переносимого встроенного mmap/atomic-ring API. Потребуется FFI или extension; это увеличивает стоимость
  внедрения и риски безопасности памяти, но не разрешает менять контракт полного snapshot.
- Оба окружения доказательств используют LinuxKit. Проверено cross-architecture поведение внутри LinuxKit, но не native
  non-LinuxKit Linux, containerd, CRI-O или Kubernetes.
- CPU/RSS и Docker Desktop timings — архитектурные наблюдения, а не SLO.

## Дополнительные бенчмарки

| Benchmark                                                   | Статус                                                            |
|-------------------------------------------------------------|-------------------------------------------------------------------|
| только полные Prometheus-compatible snapshots               | покрыто                                                           |
| atomic replacement без cross-snapshot merge                 | покрыто                                                           |
| file mmap, `/dev/shm` mmap и framed Unix socket             | покрыто                                                           |
| snapshots 128 B, 4 KiB и 64 KiB                             | покрыто                                                           |
| 1 и 8 concurrent complete-candidate publishers              | покрыто                                                           |
| publisher commit p50/p95/p99 и стоимость публикации         | покрыто отдельно; не называется accepted throughput               |
| live consumer read, validation, install и sequence ack      | покрыто для каждого transport и shape                             |
| accepted throughput и end-to-end p50/p95/p99                | покрыто в `acceptance-performance.tsv`                            |
| CPU на принятый snapshot и peak RSS                         | покрыто в `resources.tsv`                                         |
| точные acknowledged/accepted snapshot counts                | покрыто                                                           |
| concurrent active-state reader без torn/mixed state         | покрыто: ноль bad reads                                           |
| учёт ring overwrite                                         | покрыто: 936 overwritten candidates из 1000 публикаций в 64 slots |
| реальный writer crash до candidate commit                   | покрыто: exit 99, candidate остался uncommitted                   |
| mapping schema mismatch и reopen                            | покрыто                                                           |
| приватные mapping permissions                               | покрыто: `0600`                                                   |
| cgroup memory-limit enforcement                             | покрыто: exit 137 и Docker `OOMKilled=true`                       |
| Ubuntu run с совпадающим fingerprint                        | покрыто; идентичный fingerprint и 80/80 проверок пройдено         |
| PHP FFI/extension client                                    | непереносим; сама необходимость является evidence adoption cost   |
| полная Prometheus structural validation и concurrent scrape | принадлежат INV-010/implementation tests и здесь не дублируются   |
| soak/wraparound production ABI                              | рекомендуется только после выбора transport                       |
| perf/eBPF contention profiling и 30+ повторов               | рекомендуется в Ubuntu-окружении с CPU pinning                    |

## Как улучшить последующие бенчмарки

Для углублённого transport research на native non-LinuxKit Linux
следует выполнить не менее 30 повторов каждой формы, закрепить candidate publishers и consumer за CPU, сообщать median
и dispersion, снимать cgroup v2 CPU/memory и использовать `perf stat` для cycles, instructions, cache misses и context
switches. Любой длительный wraparound soak обязан продолжать публиковать полные snapshots и проверять atomic
replacement;
он не должен вводить semantics суммирования или partial updates.

## Выход решения

- Prototype: `prototype/`
- Runner: `run-bench.sh`
- Raw evidence: `results/20260802T162350Z/`, `results/20260802T163406Z/`
- Подробный анализ: [report_ru.md](report_ru.md)
- ADR: [ADR-009](../../docs-ru/06-architecture/adr/ADR-009.md)
