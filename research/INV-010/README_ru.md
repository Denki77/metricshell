# INV-010 — Prometheus Exposition

**Статус:** завершено
**Эталонный прогон macOS:** `results/20260803T182806Z`
**Эталонный прогон Ubuntu/LinuxKit:** `results/20260803T184050Z`
**Отчёт:** [report_ru.md](report_ru.md)
**Решение:** [ADR-010](../../docs-ru/06-architecture/adr/ADR-010.md)

## Вопрос

Какие форматы exposition и гарантии согласованности должен предоставлять MetricShell?

## Контекст и гипотезы

ADR-004 требует принимать по одному полному, бесконфликтному snapshot приложения. MetricShell валидирует кандидата
целиком и атомарно заменяет предыдущий принятый snapshot; он не суммирует snapshot’ы, не сливает серии, не применяет
instrumentation operations и не агрегирует реестры producers. Поэтому scrape должен один раз выбрать immutable
snapshot приложения и отдать его целиком вместе с отдельно принадлежащими MetricShell self-metrics.

Исходная гипотеза: канонический текст и проверку грамматики должен задавать готовый Prometheus parser/encoder, а не
самописный formatter. Prometheus text 0.0.4 должен быть совместимым baseline, OpenMetrics 1.0 — включаться content
negotiation. Лимит ответа должен срабатывать до фиксации частичного успешного HTTP response.

## Необходимые доказательства

- negotiation Prometheus text/OpenMetrics, metadata, classic histogram и optional timestamp;
- внешняя проверка `promtool check metrics`;
- атомарная замена полных snapshot’ов во время concurrent scrape;
- сохранение последнего валидного snapshot после malformed input;
- наблюдения для 0, 1k, 10k и 100k application series;
- concurrent, slow и disconnected scrapers;
- gzip и ограничение размера response;
- повтор на Ubuntu/LinuxKit с тем же fingerprint.

## Подтверждённый результат

Прогоны macOS/LinuxKit aarch64 и Ubuntu/LinuxKit x86_64 с одинаковым fingerprint прошли 18/18 переносимых assertions
каждый. Официальный Prometheus `promtool` принял exposition в обеих средах. Во время 120 scrape и 120 чередующихся
полных замен A/B каждый response содержал ровно 250 series одного поколения; пустых, частичных и смешанных snapshot не
было. Malformed candidate вернул `400` и сохранил предыдущий полный snapshot. Response для 100k series имел размер
6 378 736 bytes в обеих средах.

В обеих средах также пройдены 32 concurrent scrapers, gzip, slow/disconnected clients и предварительный `503` при
лимите 1 024 bytes. Одинаковый fingerprint:
`edfea8c5efb2528bb1a131b8e1125c4a00aa354a011a5015272bb83086969456`.

## Принятые значения

- application state: один immutable полный принятый snapshot, выбираемый один раз на scrape;
- update: полная валидация кандидата и атомарная замена без суммирования или merge;
- baseline: Prometheus text `0.0.4`;
- negotiated format: OpenMetrics text `1.0.0` с `# EOF`;
- metadata: валидные HELP, TYPE, classic histogram и optional timestamp сохраняются;
- malformed candidate: атомарный отказ с сохранением последнего валидного snapshot;
- response limit: проверяется по полному uncompressed response до успешного статуса;
- gzip не меняет identity application snapshot;
- self-metrics добавляются из отдельного MetricShell-owned state;
- исследованный диапазон: 0–100 000 synthetic gauge series и 1–32 concurrent scrapers.

Значения приняты в [ADR-010](../../docs-ru/06-architecture/adr/ADR-010.md).

## Запуск прототипа

На macOS и Ubuntu из корня репозитория используется одна команда:

```bash
./research/INV-010/run-bench.sh
```

Просмотр доказательств:

```bash
latest="$(cat research/INV-010/latest-results.txt)"
cat "$latest/assertions.tsv"
cat "$latest/summary.tsv"
cat "$latest/cardinality.tsv"
cat "$latest/concurrent-scrapes.tsv"
cat "$latest/observations.tsv"
cat "$latest/environment.tsv"
cat "$latest/promtool.log"
```

Runner собирает один Linux image, использует ephemeral loopback ports, выполняет всю матрицу и пишет raw evidence в
`results/<UTC timestamp>/`. На Ubuntu команда не меняется. Сравнивать нужно `benchmark_code_fingerprint_sha256`;
repository HEAD и container/image IDs — только контекст. В fingerprint входят только `prototype/` и `run-bench.sh`,
поэтому документация и результаты его не меняют.

Ручной запуск:

```bash
docker build -t metricshell-inv010:prototype research/INV-010/prototype
docker run --rm -p 127.0.0.1:19100:19100 metricshell-inv010:prototype
curl -H 'Accept: application/openmetrics-text; version=1.0.0' http://127.0.0.1:19100/metrics
```

## Ограничения прототипа

- Это исследовательский server, не production ingestion/exposition MetricShell.
- Используется Prometheus common parser/encoder, но не реализованы все правила ADR-004, включая lifetime
  name-to-type binding и явное product encoding для zero-family snapshot.
- OpenMetrics использует канонические metric families и EOF; exemplars, native histograms и protobuf не исследованы.
- Cardinality timings — single-run observations Docker Desktop, а не SLO или defaults.
- Успешная HTTP write означает только завершение server-side write, не TSDB persistence.
- Обе container-среды используют LinuxKit: macOS/LinuxKit aarch64 и Ubuntu/LinuxKit x86_64. Native Linux без LinuxKit,
  иные HTTP stacks и Kubernetes не покрыты.

## Дополнительные benchmarks

| Benchmark                                   | Статус                                                     |
|---------------------------------------------|------------------------------------------------------------|
| Prometheus text 0.0.4 и OpenMetrics 1.0     | покрыто                                                    |
| HELP, TYPE, classic histogram, timestamp    | покрыто                                                    |
| официальный `promtool check metrics`        | покрыто закреплённым digest image                          |
| malformed candidate и last-valid retention  | покрыто                                                    |
| concurrent replacement/scrape               | покрыто: 120/120, смешанных body нет                       |
| cardinality 0, 1k, 10k, 100k                | покрыто                                                    |
| 32 concurrent scrapers                      | покрыто                                                    |
| gzip                                        | покрыто                                                    |
| slow/disconnected clients                   | покрыто                                                    |
| response-size preflight                     | покрыто при 1 024 bytes                                    |
| self-metrics рядом с snapshot приложения    | покрыто                                                    |
| Ubuntu с тем же fingerprint                 | покрыто: 18/18 assertions                                  |
| распределения latency, CPU/RSS, allocations | рекомендуется на Ubuntu, 30+ повторов                      |
| exemplars/native histograms                 | не покрыто; вне принятой области text/OpenMetrics snapshot |
| HTTP/2, TLS, reverse proxy                  | после выбора deployment model                              |

## Как лучше снять дополнительные benchmarks

Для дополнительного performance-анализа следует повторить каждую пару
cardinality/concurrency не менее 30 раз, по возможности закрепить CPUs, записать cgroup CPU/RSS и allocations, показать
median и p95/p99. Любое расширение обязано продолжать передавать полные snapshot’ы и проверять, что scrape видит один
старый или новый snapshot, но никогда сумму или partial merge.

## Выход исследования

- Prototype: `prototype/`
- Runner: `run-bench.sh`
- Raw evidence macOS: `results/20260803T182806Z/`
- Raw evidence Ubuntu: `results/20260803T184050Z/`
- Подробный анализ: [report_ru.md](report_ru.md)
- ADR: [ADR-010](../../docs-ru/06-architecture/adr/ADR-010.md)
