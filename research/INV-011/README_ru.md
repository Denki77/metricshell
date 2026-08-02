# INV-011 — Финальное состояние application metrics и подсчёт scrape

**Статус:** в процессе
**Эталонный прогон macOS:** `results/20260803T065931Z`
**Прогон Ubuntu/LinuxKit:** ожидается
**Отчёт:** [report_ru.md](report_ru.md)

## Вопрос

Что остаётся immutable после завершения workload и когда final scrape считается завершённым?

## Контекст и гипотезы

ADR-004 определяет последний валидный полный application snapshot как финальное состояние. MetricShell не суммирует и
не сливает snapshot’ы. Ingestion должен закрыться до final wait; application metrics после этого immutable, а
MetricShell self-metrics могут меняться при requests и ходе времени.

Исходные гипотезы: default count равен одному, `N > 1` optional, health/readiness никогда не считаются, а eligible
response считается только после успешной записи всех bytes. Отдача bytes не доказывает persistence в Prometheus TSDB.
Без отдельного eligibility token ручной `curl` неотличим от scraper.

## Необходимые доказательства

- immutable application values и mutable self-metrics после finalization;
- отказ ingestion после freeze boundary;
- immediate, fixed duration, one scrape и N scrapes;
- исключение health/readiness и timeout;
- manual, repeated same-client и concurrent scrapes;
- optional eligibility token;
- disconnected large response не считается, последующий complete response считается;
- Ubuntu/LinuxKit повтор с тем же fingerprint.

## Текущий результат

Прогон macOS Docker Desktop/LinuxKit aarch64 прошёл 26/26 assertions. Десять повторных ephemeral-port startup cycles
дали HTTP 200, curl exit 0 и container exit 0. Application value `42` и его SHA-256 оставались
неизменны, self-metric попыток росла; публикация после finalization вернула HTTP 409. Immediate и duration modes
завершились корректно. Health/readiness оставили count нулевым. Один manual curl выполнил `N=1`, три запроса того же
client выполнили `N=3`, а все 20 concurrent clients получили полные responses при насыщении configured count на 10 и
500 ms completion-drain window.

TCP client отключился во время 8 MiB chunked response: attempt был виден, completed count остался 0. Последующий полный
response засчитался и завершил wait. Timeout 500 ms завершился с нулём completed scrapes. До Ubuntu-повтора с тем же
fingerprint статус остаётся «в процессе».

## Предварительно допустимые значения

- порядок: закрыть application publications, заморозить последний валидный полный snapshot, затем ждать;
- application state immutable; никаких sum, merge или late replacement;
- self-metrics mutable и исключены из identity final application state;
- default mode: один eligible completed scrape и обязательный bounded timeout;
- optional modes: immediate, fixed duration и positive `N`;
- момент count: после успешной записи всех response bytes HTTP handler’ом;
- не считаются health, readiness, debug/state, failed writes и ineligible responses;
- concurrent responses считаются независимо, counter насыщается на `N`;
- после достижения `N` bounded completion grace дренирует уже принятые HTTP handlers;
- uniqueness по IP/scraper identity по умолчанию нет;
- optional token может задавать eligibility, но не доказывает TSDB persistence;
- timeout не создаёт фиктивный completed scrape.

Значения предварительны до Ubuntu-подтверждения и ADR.

## Запуск прототипа

Одинаковая команда на macOS и Ubuntu:

```bash
./research/INV-011/run-bench.sh
```

Просмотр результатов:

```bash
latest="$(cat research/INV-011/latest-results.txt)"
cat "$latest/assertions.tsv"
cat "$latest/summary.tsv"
cat "$latest/observations.tsv"
cat "$latest/environment.tsv"
cat "$latest/aborted_scrape.log"
cat "$latest/concurrent_scrapes.log"
cat "$latest/timeout.log"
```

Runner использует один Linux image и ephemeral loopback ports. Он опрашивает Docker inspect до появления числового
host binding и успешного `/healthz`; выход контейнера до readiness является ошибкой. Короткие duration/timeout cases не
публикуют порт. Сравнивать следует
`benchmark_code_fingerprint_sha256`; repository HEAD — контекст. В fingerprint входят только `prototype/` и
`run-bench.sh`, поэтому docs/results его не меняют.

Ручной пример:

```bash
docker build -t metricshell-inv011:prototype research/INV-011/prototype
docker run --rm -p 127.0.0.1:19111:19111 metricshell-inv011:prototype \
  --mode=scrapes --required=1 --wait=5s
curl http://127.0.0.1:19111/metrics
```

## Ограничения прототипа

- Это research lifecycle server, не полный supervisor MetricShell.
- “Successfully written” значит: Go handler закончил все writes без error и request context не cancelled. Успех socket
  не доказывает remote parsing, Prometheus acceptance или TSDB commit.
- Eligibility token — механизм исследования, не выбранный authentication protocol.
- Synthetic snapshot уже финален при старте HTTP server; реальный workload process не интегрирован.
- Counter saturation делает release condition детерминированным. Ограниченный completion grace позволяет уже принятым
  concurrent handlers полностью завершить HTTP responses до shutdown.
- Timings включают Docker startup и polling и не являются shutdown budget recommendations.
- macOS evidence — LinuxKit aarch64; native Linux и Kubernetes discovery относятся к последующим исследованиям.

## Дополнительные benchmarks

| Benchmark                                    | Статус                                                |
|----------------------------------------------|-------------------------------------------------------|
| immutable application и mutable self-metrics | покрыто                                               |
| ingestion rejected after finalization        | покрыто: HTTP 409                                     |
| immediate                                    | покрыто                                               |
| fixed duration                               | покрыто при 500 ms test setting                       |
| one completed scrape                         | покрыто                                               |
| configurable N                               | покрыто при N=3 и N=10                                |
| health/readiness exclusion                   | покрыто                                               |
| manual curl                                  | покрыто: считается без gate                           |
| same-client uniqueness                       | покрыто: requests независимы                          |
| concurrent counting и response drain         | покрыто: 20 полных responses, saturation 10           |
| ephemeral port/readiness race                | покрыто: inspect polling, HTTP readiness, strict exit |
| repeated ephemeral-port lifecycle            | покрыто: 10/10 HTTP 200, curl 0, container 0          |
| optional token                               | покрыто                                               |
| aborted response                             | покрыто, 8 MiB chunks                                 |
| timeout                                      | покрыто, 500 ms и 0 synthetic completions             |
| Ubuntu matching fingerprint                  | ожидается                                             |
| real Prometheus/TSDB query                   | не создаёт causal proof без acknowledgement protocol  |
| HTTP/2/proxy/TLS                             | после выбора deployment topology                      |
| real workload/signal race                    | после интеграции INV-003 и lifecycle code             |

## Как лучше снять дополнительные benchmarks

Сначала запустить неизменённый fingerprint на Ubuntu. Затем повторить abort на нескольких размерах body и socket buffer,
проверить HTTP/2 и reverse proxy, подключить реальный Prometheus, чтобы явно показать различие response completion и
query visibility. Multiple responses нельзя трактовать как aggregation: каждый response обязан содержать тот же frozen
полный application snapshot плюс отдельно изменяющиеся self-metrics.

## Выход исследования

- Prototype: `prototype/`
- Runner: `run-bench.sh`
- macOS evidence: `results/20260803T065931Z/`
- Ubuntu evidence: ожидается от того же runner/fingerprint
- Подробный анализ: [report_ru.md](report_ru.md)
- ADR: после Ubuntu-подтверждения
