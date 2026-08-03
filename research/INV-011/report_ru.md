# Отчёт INV-011 — Финальное состояние и подсчёт scrape

**Статус:** завершено
**Даты прогонов:** 2026-08-03
**Эталонные прогоны:** `results/20260803T065931Z`, `results/20260803T070500Z`
**Fingerprint:** `4b8b5d48b85b5c3c2c74e94c2b0ed59494708110ce53b6f8645d16e4d5d0c7d9`

## Цель

Определить freeze boundary application state и минимальное правдивое правило final scrape для immediate, duration,
one/N scrape, concurrent, ineligible, aborted и timeout scenarios.

## Граница ADR-004

Final application state — последний валидный полный snapshot либо валидное начальное zero-series состояние, если
публикаций не было. Benchmark представляет state одним immutable body со стабильным SHA-256. Snapshot’ы не суммируются,
self-metric changes не включаются в application values. После finalization публикация отклоняется; self-metrics остаются
отдельными и могут изменяться.

## Прототип и команды

- `prototype/cmd/inv011` — final-state HTTP server и state machine wait modes;
- `prototype/Dockerfile` — воспроизводимый Linux image;
- `run-bench.sh` — correctness и timing-observation matrix;
- `results/<timestamp>` — assertions, observations, response bodies и per-case logs.

```bash
./research/INV-011/run-bench.sh
latest="$(cat research/INV-011/latest-results.txt)"
cat "$latest/assertions.tsv"
cat "$latest/observations.tsv"
cat "$latest/aborted_scrape.log"
cat "$latest/timeout.log"
```

macOS и Ubuntu используют одну команду и benchmark fingerprint.

## Среды прогонов

| Среда                          |       Дата | Docker | Архитектура | Результат                  | Статус                |
|--------------------------------|-----------:|-------:|-------------|----------------------------|-----------------------|
| Docker Desktop macOS/LinuxKit  | 2026-08-03 | 29.6.2 | aarch64     | `results/20260803T065931Z` | 26/26 assertions pass |
| Docker Desktop Ubuntu/LinuxKit | 2026-08-03 | 27.4.0 | x86_64      | `results/20260803T070500Z` | 26/26 assertions pass |

Оба прогона имеют fingerprint `4b8b5d48b85b5c3c2c74e94c2b0ed59494708110ce53b6f8645d16e4d5d0c7d9`; все 26
assertions прошли в обеих средах.

| Наблюдение                              | macOS/LinuxKit aarch64 | Ubuntu/LinuxKit x86_64 |
|-----------------------------------------|-----------------------:|-----------------------:|
| Диапазон readiness повторных startup    |     239,767–302,485 ms | 3 008,190–4 014,317 ms |
| Lifecycle duration при настройке 500 ms |             813,244 ms |           4 155,560 ms |
| Timeout без scrape при настройке 500 ms |             858,604 ms |           4 108,646 ms |
| Успешные повторные startup cycles       |                  10/10 |                  10/10 |

## Результаты

### Freeze boundary

Два scrape при bounded fixed wait показали `application_jobs_total 42`; MetricShell attempt counter вырос. Header и
final
log сохранили SHA-256 `66aab7e584c5d4eb1187ab30d0a46c68b7448dbbd58bc99e6c60982883090a15`. Publication после
finalization вернула HTTP 409. Это подтверждает freeze application state до wait при живых self-metrics.

### Exit modes

Immediate завершился 0 с reason `immediate`. Duration 500 ms завершился 0 с `duration_elapsed`; полный host lifecycle
занял 813.244 ms. Scrape timeout 500 ms завершился 0 с `timeout`, нулём completed scrapes и host lifecycle 858.604 ms.
Host times включают startup/polling и не измеряют точность configured budget.

### Counting и eligibility

Пять health и пять readiness requests оставили `completed=0 attempted=0`. Обычный manual curl выполнил N=1. Два
requests оставили N=3 running на count 2, третий завершил ожидание. Значит default counting не различает manual client и
Prometheus и не deduplicate один client.

Для concurrent case запущено 20 requests. Все 20 clients получили полные HTTP responses. Десять eligible handlers
достигли threshold, counter насытился на N=10, а 500 ms completion grace дренировал уже принятые handlers до shutdown.
`concurrent-clients.log` пуст, поэтому curl transport errors не скрыты.

Ephemeral port опрашивается через container inspect до появления числового binding и готовности `/healthz`. Наблюдаемая
Десять отдельных повторов дали HTTP 200 и чистые curl/container exits; readiness составила примерно 240–302 ms. В
остальных сценариях readiness не превысила примерно 400 ms. Выход до readiness теперь является strict startup failure.
Короткие 500 ms duration и timeout cases запускаются без публикации HTTP-порта.

С configured `X-Final-Scrape-Token` обычный complete response был отдан, но остался ineligible:
`completed=0 attempted=1`. Response с token засчитался. Это eligibility gate, но не authentication/storage ack.

### Aborted response

Server создал примерно 8 MiB response из flushed chunks по 16 KiB с delay 1 ms. Raw TCP client отключился после
request. State показал 0 completed и как минимум 1 attempt. Затем normal client получил body целиком, засчитался и
освободил wait. Count после write loop отличил проверенный disconnect от complete handler write.

## Проверка гипотез

### Default count равен одному

Подтверждено. N=1 — минимальный полезный contract. Нужен bounded timeout, потому что scraper может не
прийти. N>1 работает, но увеличивает ожидание и не повышает certainty persistence.

### Count только после полной успешной write

Подтверждено в server-observable semantics. Disconnected large response не засчитался, subsequent full write — да. Это
слабее remote receipt, parsing Prometheus или TSDB commit.

### Health/readiness не считаются

Подтверждено: оба endpoints не изменили attempted/completed counters.

### Concurrent scrapes независимы

Подтверждено с saturating threshold: каждый eligible complete handler получает один count до N. Scraper identity/IP
uniqueness не требуется.

### Response не доказывает TSDB persistence

Подтверждено protocol reasoning: HTTP write не имеет causal acknowledgement от Prometheus storage. Token доказывает
только configured eligibility.

## Оценка по критериям

| Критерий                    | Подтверждённый результат             |
|-----------------------------|--------------------------------------|
| immutable application state | stable value/SHA-256                 |
| live self-metrics           | attempt counter изменялся            |
| freeze ordering             | late publication HTTP 409            |
| immediate/duration          | пройдены                             |
| N=1 и N>1                   | пройдены при 1, 3, 10                |
| health/readiness            | исключены                            |
| manual/same-client          | считаются независимо                 |
| optional eligibility        | token gate показан                   |
| abort                       | disconnected 8 MiB write не засчитан |
| timeout                     | bounded exit, 0 fake completions     |
| Ubuntu reproducibility      | 26/26, fingerprint совпал            |

## Принятые policies

- До final wait закрыть ingestion и заморозить last valid complete application snapshot.
- Application identity immutable, self-metrics mutable и separate.
- Default: один eligible completed scrape плюс обязательный finite timeout.
- Поддержать immediate, fixed duration и positive N.
- Increment только после всех writes без error и без cancelled context.
- Не считать health/readiness/debug, failed или explicitly ineligible responses.
- Concurrent responses считать atomic saturating counter до N.
- После достижения N использовать bounded completion grace до server shutdown для уже принятых handlers.
- Не deduplicate IP/scraper identity по умолчанию.
- Token — optional policy, не storage proof.
- При timeout следовать lifecycle/exit policy и не синтезировать completion.

## Ограничения

- Обе container-среды используют LinuxKit: macOS/LinuxKit aarch64 и Ubuntu/LinuxKit x86_64. Native Linux без LinuxKit
  не проверен.
- Synthetic final state, без full process supervision/signal race.
- Go write completion не доказывает remote/durable receipt.
- Token без threat model и credential lifecycle.
- Только direct HTTP/1.1, без proxy/TLS/HTTP2.
- Host lifecycle times не являются shutdown limits.
- Несколько периодических пустых HTTP-ответов наблюдались вручную вне сохранённого benchmark stream. Они не
  воспроизвелись в 10/10 успешных lifecycle repetitions, а коррелированные evidence клиента, сервера и port state не
  сохранены. Это неклассифицированное эксплуатационное наблюдение, а не failed assertion и не доказательство дефекта
  сервера.
- Проверенный 500 ms completion grace дренировал 20 local clients; production value требует отдельной проверки с
  выбранными proxy/network.

## Дополнительные benchmarks

| Пункт                                | Статус            | Доказательство/причина               |
|--------------------------------------|-------------------|--------------------------------------|
| freeze и mutable self                | покрыто           | bodies/stable hash                   |
| ingestion close                      | покрыто           | HTTP 409                             |
| immediate/duration                   | покрыто           | logs                                 |
| one/N                                | покрыто           | N=1, 3, 10                           |
| health/readiness                     | покрыто           | zero counters                        |
| manual/same client                   | покрыто           | default eligible                     |
| concurrent и drain                   | покрыто           | 20 полных responses, saturation 10   |
| port/readiness startup               | покрыто           | inspect polling и strict readiness   |
| repeated port lifecycle              | покрыто           | 10/10 HTTP 200, curl 0, container 0  |
| пустой ответ по наблюдению оператора | не воспроизведено | нет коррелированных request evidence |
| token                                | покрыто           | ineligible/eligible                  |
| aborted connection                   | покрыто           | 8 MiB chunks                         |
| timeout                              | покрыто           | 500 ms, zero count                   |
| Ubuntu fingerprint                   | покрыто           | 26/26, fingerprint совпал            |
| real Prometheus/TSDB                 | follow-up         | write не равен persistence           |
| proxy/TLS/HTTP2                      | рекомендуется     | deployment-dependent                 |
| workload/signal race                 | рекомендуется     | integration с INV-003                |

## Вывод

Совпадающие evidence macOS и Ubuntu поддерживают default: один eligible completed scrape плюс finite timeout.
Application
metrics замораживаются до wait, self-metrics остаются live. Health/readiness не считаются, repeated/concurrent eligible
responses считаются независимо до N, failed/aborted server write не считается.

Гарантия сформулирована узко: полная успешная server write, не TSDB persistence. INV-011 завершено; policy зафиксирована
в [ADR-011](../../docs-ru/06-architecture/adr/ADR-011.md).

## Выход исследования

- Prototype: `prototype/`
- Runner: `run-bench.sh`
- macOS evidence: `results/20260803T065931Z/`
- Ubuntu evidence: `results/20260803T070500Z/`
- Направление: freeze then wait, default N=1, complete eligible writes, finite timeout
- ADR: [ADR-011](../../docs-ru/06-architecture/adr/ADR-011.md)
