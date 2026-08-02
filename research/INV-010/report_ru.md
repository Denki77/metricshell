# Отчёт INV-010 — Prometheus Exposition

**Статус:** в процессе
**Дата прогона:** 2026-08-02
**Docker server:** 29.6.2 (macOS/LinuxKit)
**Docker platform:** linux/aarch64
**Эталонный прогон:** `results/20260803T182806Z`
**Fingerprint:** `edfea8c5efb2528bb1a131b8e1125c4a00aa354a011a5015272bb83086969456`

## Цель

Определить минимальный корректный Prometheus-compatible exposition contract, доказать consistency scrape при замене
полных snapshot’ов и выбрать предварительные policies формата, concurrency и resource limits без нарушения ADR-004.

## Граница ADR-004

Benchmark никогда не суммирует snapshot’ы и не объединяет independently owned registries. Каждый A/B/cardinality body
— один полный workload-owned candidate. Прототип полностью парсит кандидата, канонически кодирует его и только после
успеха заменяет immutable pointer. Scrape загружает pointer один раз. Self-metrics MetricShell добавляются из отдельного
state. Malformed candidate не может изменить active application state.

## Прототип и команды

- `prototype/cmd/inv010` — parser/encoder, atomic snapshot holder и HTTP server;
- `prototype/Dockerfile` — воспроизводимый Linux build/runtime;
- `run-bench.sh` — полная матрица correctness и observations;
- `results/<timestamp>` — assertions, observations, bodies, headers и logs.

```bash
./research/INV-010/run-bench.sh
latest="$(cat research/INV-010/latest-results.txt)"
cat "$latest/assertions.tsv"
cat "$latest/cardinality.tsv"
cat "$latest/concurrent-scrapes.tsv"
cat "$latest/environment.tsv"
```

Закреплённый digest Prometheus image запускает `promtool check metrics`. На Ubuntu используются тот же runner и тот же
fingerprint; отдельной копии кода для Ubuntu нет.

## Среды прогонов

| Среда                          |       Дата |    Docker | Архитектура      | Результат                  | Статус                |
|--------------------------------|-----------:|----------:|------------------|----------------------------|-----------------------|
| Docker Desktop macOS/LinuxKit  | 2026-08-02 |    29.6.2 | aarch64          | `results/20260803T182806Z` | 18/18 assertions pass |
| Docker Desktop Ubuntu/LinuxKit |  ожидается | ожидается | x86_64 ожидается | ожидается                  | не запускалось        |

Fingerprint macOS: `edfea8c5efb2528bb1a131b8e1125c4a00aa354a011a5015272bb83086969456`. Ubuntu evidence
сопоставимо только при точном совпадении.

## Результаты

### Форматы и validation

Prometheus requests получили `text/plain; version=0.0.4`, OpenMetrics —
`application/openmetrics-text; version=1.0.0` и `# EOF`. HELP/TYPE и classic histogram с `+Inf == count == 5`
присутствовали. Timestamp `1700000000000` был принят и сохранён. Официальный закреплённый `promtool` завершился с 0.

### Atomic replacement и partial failure

Runner чередовал 120 полных A/B installations одновременно со 120 scrape при concurrency 16. Во всех responses header
и все 250 labels указывали одно поколение; смешанных A/B состояний не было. Malformed candidate вернул HTTP 400, а
ранее принятый timestamped snapshot сохранился. Это подтверждает whole-candidate validation и replacement, не merge.

### Cardinality observations

| Gauge series приложения | Response bytes | Install wall time | Scrape wall time |
|------------------------:|---------------:|------------------:|-----------------:|
|                       0 |            874 |         25.611 ms |        23.163 ms |
|                   1 000 |         58 734 |         31.138 ms |        22.968 ms |
|                  10 000 |        608 735 |         66.632 ms |        30.785 ms |
|                 100 000 |      6 378 736 |        305.967 ms |       115.676 ms |

Это single-run host wall time с curl и Docker Desktop scheduling, а не SLO или production limit.

### Clients и limits

Все 32 parallel scrape завершились; aggregate wall time записан в `observations.tsv`. Gzip дал корректный
decompressed body. Slow client 1 KiB/s с timeout 1 s и намеренно disconnected raw TCP client не нарушили health server.
При лимите 1 024 bytes и 10k series server вернул HTTP 503 с малым error body до exposition headers.

## Проверка гипотез

### Использовать готовый Prometheus parser/encoder

Предварительно подтверждено. Common library приняла валидный input, отклонила malformed syntax и создала canonical
families, принятые `promtool`. HTTP negotiation, snapshot selection, preflight и lifecycle остаются кодом MetricShell,
но grammar не следует писать вручную.

### Prometheus text baseline, OpenMetrics negotiated

Подтверждено для проверенных metric types. Оба формата несут один выбранный application snapshot. OpenMetrics добавляет
EOF. Exemplars/native histograms не покрыты.

### Consistent concurrent scrape без registry lock

Подтверждено для immutable pointer replacement: одна загрузка pointer на request дала ноль mixed bodies.

### Partial failure не должен давать partial success

Подтверждено: malformed candidate не заменил state, oversized response дал preflight 503. Production обязан сохранить
порядок при добавлении всех structural rules и streaming/compression.

## Оценка по критериям

| Критерий                      | Предварительный результат               |
|-------------------------------|-----------------------------------------|
| Prometheus compatibility      | text 0.0.4 принят `promtool`            |
| OpenMetrics                   | negotiation 1.0 и EOF покрыты           |
| complete-snapshot consistency | 120/120 bodies одного generation        |
| partial candidate failure     | atomic rejection и last-valid retention |
| large registry                | 0–100k series                           |
| concurrent clients            | 32 clients                              |
| slow/disconnected             | survival покрыт                         |
| response limit                | preflight 503                           |
| Ubuntu reproducibility        | ожидается                               |

## Предварительные policies

- Prometheus text 0.0.4 — default compatibility format.
- OpenMetrics text 1.0 — по Accept negotiation и с EOF.
- Scrape один раз выбирает полный immutable application snapshot и добавляет отдельные self-metrics.
- Candidate активируется только после полной parsing/validation; отказ сохраняет last valid.
- Snapshot’ы заменяются, никогда не суммируются, не merge и не reconcile.
- Лимит применяется к полному encoded uncompressed body до success status.
- Compression не меняет snapshot identity.
- 100k series и 32 clients — исследовательский envelope, не defaults.

## Ограничения

- Только macOS/LinuxKit; Ubuntu ожидается.
- Не все structural rules ADR-004 и не explicit zero-family encoding.
- Нет exemplars, native histograms, protobuf, TLS, HTTP/2 и proxy.
- Timings single-run и включают host tools/Docker Desktop.
- Disconnect доказывает только server-side write behavior, не TSDB persistence.
- Нет распределений 30 runs и CPU/RSS/allocation profile.

## Дополнительные benchmarks

| Пункт                          | Статус        | Доказательство/причина                |
|--------------------------------|---------------|---------------------------------------|
| formats, negotiation, metadata | покрыто       | bodies/headers                        |
| histogram/timestamp            | покрыто       | `prometheus.txt`, `timestamp.metrics` |
| promtool                       | покрыто       | `promtool.log` и assertion            |
| malformed retention            | покрыто       | HTTP 400 и retained timestamp         |
| concurrent replace/scrape      | покрыто       | `concurrent-scrapes.tsv`              |
| cardinality matrix             | покрыто       | `cardinality.tsv`                     |
| 32 clients                     | покрыто       | assertion/observation                 |
| gzip, slow, disconnect         | покрыто       | headers/logs/health                   |
| size preflight                 | покрыто       | 1 024 bytes, HTTP 503                 |
| Ubuntu fingerprint             | ожидается     | необходимо для завершения             |
| CPU/RSS/latency 30+            | рекомендуется | pinned CPUs на Ubuntu                 |
| exemplars/native histograms    | отложено      | только при requirement                |
| proxy/TLS/HTTP2                | отложено      | deployment research                   |

## Вывод

macOS evidence предварительно подтверждает library-based exposition. Prometheus text 0.0.4 и negotiated OpenMetrics
1.0 достаточны для проверенных типов; immutable complete-snapshot replacement даёт consistent concurrent scrapes.
Pre-encoding позволяет честно вернуть size failure до partial success.

Исследование остаётся в процессе. ADR нельзя принимать до прогона неизменённого fingerprint на Ubuntu и сравнения
environment-dependent observations.

## Выход исследования

- Prototype: `prototype/`
- Runner: `run-bench.sh`
- macOS evidence: `results/20260803T182806Z/`
- Ubuntu evidence: ожидается
- Предварительное направление: text default, OpenMetrics negotiated, immutable snapshot, preflight limit
- ADR: ожидается
