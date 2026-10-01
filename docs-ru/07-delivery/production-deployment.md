# Production deployment

[English version](../../docs/07-delivery/production-deployment.md)

Это единая operational-точка входа для MetricShell 0.2.0. MetricShell работает как PID 1, управляет одним process tree,
принимает instrumentation локально и отдаёт `GET /metrics` Prometheus. Workload не знает monitoring destination:
discovery и scrape configuration принадлежат инфраструктуре.

## Установка

В GitHub Release публикуются `metricshell-linux-amd64`, `metricshell-linux-arm64`, `SHA256SUMS`, `SBOM.json`,
`PROVENANCE.json`, signatures и public verification material. Скачайте binary и checksum из одного release, выполните
`sha256sum -c SHA256SUMS`, проверьте signature по `VERIFY.md`, установите binary как `metricshell` и убедитесь, что
`metricshell --version` сообщает `0.2.0`.

Официальные multi-arch images: `ghcr.io/denki77/metricshell:0.2.0` и
`ghcr.io/denki77/metricshell-artifact:0.2.0`. Tags нужны для discovery, production pin использует только
`image@sha256:<release-digest>` из release notes. `latest` не используется.

Для Kubernetes возьмите Job, CronJob и Deployment из `implementation/examples/kubernetes/production`. В repository это
явно помеченные templates; release pipeline подставляет и валидирует опубликованный digest.

## Runtime

MetricShell должен быть PID 1; workload указывается после `--`. Он пересылает signals process group, reap'ит descendants
и сохраняет workload exit status. Exposition использует port 9090. Snapshot/Managed ingestion остаётся private:
предоставьте writable directory для Unix socket (обычно `/run/metricshell`) или atomic snapshot, оставив rootfs
read-only. Kubernetes API и service-account token не нужны.

Prometheus scrapes `GET /metrics`; `/healthz` и `/readyz` — probes и не считаются final scrape. Используйте discovery,
annotations или PodMonitor, а scrape timeout держите ниже interval. Для finite workload задайте mode `scrapes`, конечный
timeout и pod lifetime, достаточный хотя бы для одного eligible scrape.

## Security

Используйте фиксированный non-root UID/GID, read-only rootfs, drop `ALL` capabilities,
`allowPrivilegeEscalation: false`, `no-new-privileges` и seccomp `RuntimeDefault`. Отключите service-account token.
Открывайте только 9090. Managed socket храните в mode-0700 directory, доступном только workload. HTTP ingestion нельзя
публиковать в сеть.

## Resources и lifecycle

Начните с requests/limits в manifests и настройте их по измерениям. Ограничьте memory, PIDs и `nofile`, snapshot/decoded
bytes, scrape concurrency, Managed connections, queue, descriptors, series, label bytes и histogram buckets.

`terminationGracePeriodSeconds` должен быть больше внутреннего shutdown grace. Примеры используют 32 и 30 секунд с
двухсекундным reserve. Workload shutdown, admission close/drain, freeze, final Core installation и final wait bounded.
При недоступном Prometheus timeout завершает ожидание, сохраняя исходный workload result.

## Локальная instrumentation

Clients по умолчанию используют `/run/metricshell/managed.sock`, поэтому shell workload не передаёт destination:

```sh
metricshell managed declare jobs counter "Processed jobs" worker
metricshell managed counter-initialize jobs 0 worker=batch
metricshell managed counter-add jobs 1 worker=batch
metricshell managed declare queue_depth gauge "Queued jobs"
metricshell managed gauge-set queue_depth 7
metricshell managed declare job_seconds histogram "Job duration" - 0.1 1 +Inf
metricshell managed histogram-observe job_seconds 0.42
```

PHP 5.4 example находится в `implementation/examples/clients/php54`. Оба client знают только local Unix socket.

## Troubleshooting

- Workload не стартует: проверьте `workload.start_failed`, path, permissions и exit 73.
- Malformed metrics/Core candidate rejected: смотрите bounded reason; last valid snapshot сохраняется.
- Resource rejection/overload: смотрите self-metrics и меняйте конкретный bound только после измерений.
- Final scrape timeout: проверьте discovery, readiness, interval и timeout; ожидание намеренно конечное.
- Bind failure: исключите collision адресов и проверьте writable/private socket directory.
- Managed `UNKNOWN`: операция могла commit'иться при потере ACK; выполняйте осознанную reconciliation.
- Prometheus не видит target: проверьте selectors/annotations/PodMonitor и network policy до 9090.
- Restart: новый Managed process начинает пустую epoch; MetricShell не хранит application state между запусками.

Correctness guarantees задаются specifications и tests. Resource limits — hard bounds; benchmark observations не SLO.
