# Отчёт INV-008 — Локальный push ingestion

[English version](report.md)

Статус: завершено  
Даты прогонов: 2026-07-27, 2026-07-28  
Docker servers: 29.4.3, 27.4.0  
Платформы: LinuxKit linux/aarch64, LinuxKit linux/x86_64  
Эталонные прогоны: `results/20260727T185055Z`, `results/20260728T114459Z`

## Цель

Проверить, создаёт ли container-local HTTP или gRPC ingestion API достаточную integration или performance ценность по
сравнению с Unix socket, чтобы оправдать дополнительный production protocol, dependency stack и TCP endpoint.

## Прототип и семантика

Все adapters используют один atomic accepted-record store. Каждый transport выполняет одну операцию: один request
содержит `N` records, store принимает массив records и увеличивает accepted count на реальное `N`.

- Unix: record count, затем повторяющиеся record length и record;
- HTTP: `POST /v1/metrics`, JSON-массив bytes;
- gRPC: unary protobuf `ingest.Ingest/Push`.

Это обеспечивает одинаковую wire-level и store-level batch semantics.

## Окружения

| Окружение             | Дата       | Docker | Kernel           | Architecture | Result set                 | Fingerprint                                                        |
|-----------------------|------------|--------|------------------|--------------|----------------------------|--------------------------------------------------------------------|
| Docker Desktop macOS  | 2026-07-27 | 29.4.3 | LinuxKit 6.12.76 | aarch64      | `results/20260727T185055Z` | `34bee766d38ee43421cd100d3b23a387b7736c660d13bd6e28b591505bd101d4` |
| Docker Desktop Ubuntu | 2026-07-28 | 27.4.0 | LinuxKit 6.10.14 | x86_64       | `results/20260728T114459Z` | `34bee766d38ee43421cd100d3b23a387b7736c660d13bd6e28b591505bd101d4` |

Оба прогона зафиксировали clean benchmark scope и zero untracked benchmark files. Repository HEAD и image ID
различаются ожидаемо; переносимой идентичностью является fingerprint.

## Correctness

Все 14 assertions прошли в обоих окружениях:

- malformed HTTP → `400`;
- empty HTTP records → `422`;
- encoded body over limit → `413`;
- Unix/HTTP/gRPC принимают decoded `1 MiB`;
- Unix/HTTP/gRPC отклоняют decoded `1 MiB + 1 byte`;
- 36 matrix rows не содержат request errors;
- HTTP и gRPC bound к `127.0.0.1`;
- shutdown adapter сохраняет accepted state;
- HTTP restart принимает новые requests.

Assertion `empty HTTP records` использовал синтаксически валидный request, decoded в ноль records. Он не отправлял
пустой HTTP body и не моделировал production contract zero-series snapshot. ADR-004 отменяет rejection этого decoded
representation; assertion остаётся точным историческим prototype evidence, а не production requirement.

Application limit равен одному decoded MiB во всех transports. HTTP encoded-body guard превышает размер корректного
base64/JSON-представления decoded `1 MiB`; authoritative decoded limit применяется общим store.

## Performance

Оба окружения завершили 36/36 rows без errors и приняли 385,313 records.

| Environment | Transport | Batch | Producers | Records/s |      p50 |       p95 |       p99 |
|-------------|-----------|------:|----------:|----------:|---------:|----------:|----------:|
| macOS       | Unix      |     1 |         1 |   240,827 | 0.002 ms |  0.005 ms |  0.028 ms |
| macOS       | HTTP      |     1 |         1 |    26,786 | 0.025 ms |  0.057 ms |  0.106 ms |
| macOS       | gRPC      |     1 |         1 |    24,679 | 0.022 ms |  0.077 ms |  0.157 ms |
| Ubuntu      | Unix      |     1 |         1 |    53,048 | 0.011 ms |  0.055 ms |  0.067 ms |
| Ubuntu      | HTTP      |     1 |         1 |     8,334 | 0.088 ms |  0.225 ms |  0.438 ms |
| Ubuntu      | gRPC      |     1 |         1 |     7,391 | 0.092 ms |  0.243 ms |  0.484 ms |
| macOS       | Unix      |    16 |         8 | 1,630,940 | 0.029 ms |  0.245 ms |  0.959 ms |
| macOS       | HTTP      |    16 |         8 |    89,072 | 1.357 ms |  3.246 ms |  4.153 ms |
| macOS       | gRPC      |    16 |         8 |   634,355 | 0.080 ms |  1.046 ms |  1.998 ms |
| Ubuntu      | Unix      |    16 |         8 |   928,096 | 0.074 ms |  0.400 ms |  0.869 ms |
| Ubuntu      | HTTP      |    16 |         8 |    21,172 | 5.727 ms | 11.301 ms | 14.151 ms |
| Ubuntu      | gRPC      |    16 |         8 |   187,377 | 0.295 ms |  2.297 ms |  5.639 ms |

Unix лидирует во всех репрезентативных формах нагрузки. gRPC существенно быстрее HTTP при concurrent batch, но не
достигает throughput Unix. Timing distributions зависят от environment; ranking кандидатов совпадает.

## Resources

| Environment | Phase       |    Elapsed |          CPU, one-core equivalent | Peak RSS/HWM |
|-------------|-------------|-----------:|----------------------------------:|-------------:|
| macOS       | idle        |   2,000 ms |                            0.500% |   54,260 KiB |
| macOS       | gRPC active | 114.716 ms |                          331.251% |   54,260 KiB |
| Ubuntu      | idle        |   2,000 ms | ниже разрешения одного 10 ms tick |   49,264 KiB |
| Ubuntu      | gRPC active | 260.805 ms |                          578.974% |   49,264 KiB |

Active workload использует восемь producers и несколько CPU cores. RSS включает все adapters, clients и Go runtime.

## Оценка гипотезы

Гипотеза подтверждена:

- HTTP действительно упрощает client integration и debugging стандартными средствами;
- HTTP полностью дублирует ingestion capability Unix socket;
- TCP/HTTP добавляет bind configuration, parser surface и риск случайного exposure;
- gRPC даёт преимущество над JSON HTTP при batching, но не даёт преимущества над Unix socket;
- protobuf schema, generated clients и HTTP/2 runtime повышают implementation и adoption cost.

## Принятые значения и policy

- default transport: Unix socket;
- HTTP JSON: stable request/response transport, выбранный ADR-005;
- gRPC: исключён из default surface;
- HTTP/gRPC bind: `127.0.0.1`;
- decoded request limit: `1 MiB`;
- tested batch: `1–16`;
- tested concurrency: `1–8`;
- prototype empty decoded record set: HTTP `422`;
- production contract: пустой transport payload является malformed, а синтаксически валидный snapshot с нулём metric
  families или series является валидным zero-series snapshot и должен приниматься;
- malformed/oversized request отклоняется атомарно, accepted state сохраняется;
- API и schema должны иметь явное versioning.

## Ограничения

Оба evidence environments используют LinuxKit. Результаты подтверждают cross-architecture behavior внутри
macOS/LinuxKit aarch64 и Ubuntu/LinuxKit x86_64, но не native non-LinuxKit Linux, containerd/CRI-O или Kubernetes.

Prototype не моделирует production metric parser, conflicts, cardinality enforcement, persistence или queue
backpressure. TLS/mTLS не проверялись, поскольку external exposure исключён из выбранного local-only default.

## Итог

Исследование завершено одинаковыми по fingerprint и полностью проходящими прогонами. MetricShell сохраняет Unix socket
как default ingestion transport, сохраняет loopback HTTP как stable request/response transport из ADR-005 и исключает
gRPC из default surface.

Решение зафиксировано в [ADR-008](../../docs-ru/06-architecture/adr/ADR-008.md).
