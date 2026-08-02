# Отчёт INV-012 — Пригодность Kubernetes Job и CronJob

**Статус:** в процессе
**Дата прогона:** 2026-08-03
**Среда:** Minikube v1.38.1, Kubernetes v1.34.0
**Мониторинг:** kube-prometheus-stack 88.1.2, два экземпляра Prometheus
**Эталонный прогон:** `results/20260803T143134Z`
**Сводка:** [summary.tsv](results/20260803T143134Z/summary.tsv)

## Цель

Проверить, может ли Pod объекта Kubernetes Job удерживать MetricShell после завершения workload достаточно долго, чтобы
один или несколько реальных scrapers получили стабильный полный Prometheus snapshot, а deadline, TTL, termination и
CronJob controls Kubernetes ограничивали время жизни.

ADR-004 является жёстким ограничением: прототип никогда не суммирует snapshots. Он публикует один полный snapshot и
записывает только наблюдения HTTP scrape.

## Прототип

- `prototype/cmd/inv012/main.go` — HTTP-сервер после workload с `/metrics`, `/readyz`, ограниченным ожиданием и
  завершением по наблюдаемым scrape.
- `prototype/Dockerfile` — непривилегированный multi-stage image.
- `prototype/k8s/monitoring-values.yaml` — закреплённая конфигурация двух экземпляров Prometheus.
- `run-bench.sh` — bootstrap проверенного по checksum Minikube, соответствующий kubectl, lifecycle изолированного
  кластера, Helm install, Kubernetes-сценарии и сбор доказательств; ранняя ошибка записывает фазу/команду вместо внешне
  пустого результата.
- `results/<timestamp>` — assertions, observations, manifests, events, logs и идентификатор окружения.

На каждый запрос прототип отдаёт полный Prometheus exposition. Scrape увеличивает только внутренний исследовательский
счётчик, определяющий конец окна после workload; он не изменяет, не объединяет и не суммирует опубликованный snapshot.

## Команды запуска

```bash
./research/INV-012/run-bench.sh
```

```bash
cat research/INV-012/latest-results.txt
cat "$(cat research/INV-012/latest-results.txt)/summary.tsv"
cat "$(cat research/INV-012/latest-results.txt)/assertions.tsv"
cat "$(cat research/INV-012/latest-results.txt)/observations.tsv"
cat "$(cat research/INV-012/latest-results.txt)/environment.tsv"
```

Сохранение изолированного кластера допускается только для диагностики:

```bash
INV012_KEEP_MINIKUBE=1 ./research/INV-012/run-bench.sh
```

Вручную запускать Minikube не нужно. Runner скачивает проверенный Minikube `v1.38.1`, сам создаёт и удаляет
профиль/context `metricshell-inv012` и использует соответствующий Kubernetes kubectl `v1.34.0` вместо системного.
Запуск Minikube ограничен 15 минутами, сбор failure-log — двумя минутами, cleanup — 60 секундами. Быстрое завершение
является ошибкой: нужно открыть `run-summary.tsv`, `failure.tsv` и log указанной фазы.

## Окружения запуска

| Окружение                         | Дата       | Docker server | Архитектура      | Набор                      | Статус                  |
|-----------------------------------|------------|--------------:|------------------|----------------------------|-------------------------|
| Docker Desktop на macOS, Minikube | 2026-08-03 |        29.6.2 | aarch64          | `results/20260803T143134Z` | 21/21 assertions прошли |
| Docker на Ubuntu, Minikube        | ожидается  |     ожидается | ожидается x86_64 | ожидается                  | не запускалось          |

Отпечаток macOS: `caa34fb8ba97176b3c199b56ffaeb21dcabbb654fa5efd2b4da3203d79274e6c`. Ubuntu-валидация
обязана использовать тот же отпечаток. `repository_head_sha` сохраняется только как контекст и может измениться после
редактирования документации.

## Результаты

| Сценарий                           | Проверенный результат                    | Итог |
|------------------------------------|------------------------------------------|------|
| Toolchain                          | Minikube `1.38.1`, kubectl `1.34.0`      | pass |
| Два экземпляра Prometheus          | работают 2 replicas                      | pass |
| Прямой Pod discovery               | наблюдался финальный scrape              | pass |
| ServiceMonitor                     | не менее двух агрегированных HTTP scrape | pass |
| TSDB Prometheus replica 0          | серия сохранена, значение `42`           | pass |
| TSDB Prometheus replica 1          | серия сохранена, значение `42`           | pass |
| Готовый Pod                        | ограниченное окно завершилось по timeout | pass |
| Неготовый Pod через ServiceMonitor | ограниченное окно завершилось по timeout | pass |
| Неготовый Pod через PodMonitor     | не менее двух scrape                     | pass |
| Active deadline                    | Job завершился с `DeadlineExceeded`      | pass |
| TTL после завершения               | объект Job удалён                        | pass |
| Явный termination                  | Pod удалён                               | pass |
| Расписание CronJob                 | создан первый плановый Job               | pass |
| Политика CronJob                   | сохранён `Forbid`                        | pass |
| Пересечение                        | один активный Pod и нет второго Job      | pass |

Измеренные наблюдения:

| Наблюдение                                 |      Значение |
|--------------------------------------------|--------------:|
| Завершение прямого discovery               |  8 017,520 ms |
| Завершение ServiceMonitor                  | 63 724,188 ms |
| Агрегированные HTTP scrape ServiceMonitor  |            51 |
| Scrape неготового Pod через ServiceMonitor |             1 |
| Явное удаление Pod                         |  2 022,144 ms |

## Оценка гипотез

### Job Pod способен отдавать финальные метрики после workload

Подтверждено в Minikube. Direct discovery, ServiceMonitor и PodMonitor вызвали реальные HTTP scrape до завершения
ограниченного окна прототипа.

### Два экземпляра Prometheus способны получить endpoint

Подтверждено. После завершения Job runner отдельно подключился к каждому Prometheus Pod и независимо выполнил
`/api/v1/query`. Обе TSDB вернули серию `inv012_final_snapshot` одного Job Pod со значением `42`. Запрос использует
зафиксированное историческое evaluation time за десять секунд до завершения, потому что после исчезновения target
Prometheus корректно помечает серию stale. Сырые ответы и evidence каждой replica сохранены.

### Только readiness определяет видимость scrape

Не установлено как универсальное правило. В эталонном прогоне неготовый ServiceMonitor получил 1 scrape, но одно
наблюдение Minikube не определяет поведение всех endpoint controllers и monitoring stacks. Переносимый assertion
проверяет timeout ограниченного окна, а число scrape остаётся observation. PodMonitor напрямую опросил неготовый Pod и
является более ясным вариантом при намеренном `readiness=false` после workload.

### Kubernetes способен ограничить время жизни и пересечения

Подтверждено. `activeDeadlineSeconds` дал `DeadlineExceeded`; TTL удалил завершённый Job; явное удаление Pod заняло
около
2,0 секунды; `concurrencyPolicy: Forbid` сохранил один Job/Pod на следующем тике расписания.

## Допустимые значения и политики

- Discovery: предпочесть PodMonitor/direct Pod discovery для намеренно неготового Pod после workload.
- Snapshot semantics: отдавать один атомарно выбранный полный snapshot; никогда не складывать результаты scrape.
- Replicas: необходимо доказательство сохранённого sample из каждой configured replica; общего handler count мало.
- Окно после workload: должно превышать задержку discovery и scrape. Проверенные 60 секунд — исследовательское
  значение, а не production default.
- Deadline: `activeDeadlineSeconds` должен быть больше целевого post-exit window и оставаться внешней границей.
- Cleanup: для завершённых Job следует задать `ttlSecondsAfterFinished`.
- CronJob: при недопустимости пересечения metrics windows следует задать `concurrencyPolicy: Forbid`.
- Readiness: нельзя кодировать предположение о нуле scrape неготовой ServiceMonitor-цели без доказательства для
  кластера.

## Ограничения прототипа

- Проверен только Minikube на macOS Docker Desktop; подтверждение Ubuntu ожидается.
- Версии Helm chart и Kubernetes закреплены; другие версии могут иначе согласовывать endpoints.
- Хранение sample проверяется через отдельное API-соединение с каждой replica. Историческое evaluation time необходимо,
  поскольку current-time query корректно исключает серию после stale marker исчезнувшего target.
- Времена включают scheduling и reconciliation и не являются гарантиями производительности.
- Не проверялись NetworkPolicy, disruption, node drain, отказ API server и managed clusters.
- Snapshot намеренно мал; масштабирование payload исследует INV-015.

## Дополнительные benchmarks

| Benchmark                                          | Статус    | Доказательство                                                 |
|----------------------------------------------------|-----------|----------------------------------------------------------------|
| Прямой Pod discovery через `kubernetes_sd_configs` | выполнен  | `direct-job.log`                                               |
| ServiceMonitor с двумя replicas                    | выполнен  | `service-ready.log`, `helm-manifest.yaml`                      |
| Сохранённый sample в TSDB каждой replica           | выполнен  | `prometheus-replica-evidence.tsv`, query JSON каждой replica   |
| Ready/unready ServiceMonitor                       | выполнен  | `service-ready.log`, `service-unready.log`, `observations.tsv` |
| PodMonitor неготового Pod                          | выполнен  | `pod-unready.log`, `monitors.yaml`                             |
| Active deadline и TTL                              | выполнен  | `assertions.tsv`, `events.txt`, `ttl.log`                      |
| Latency явного termination                         | выполнен  | `observations.tsv`                                             |
| Настоящее пересечение CronJob                      | выполнен  | `assertions.tsv`, `events.txt`                                 |
| Ubuntu с тем же отпечатком                         | ожидается | должен создать единственный Ubuntu-набор                       |
| Сравнение managed cluster/CNI                      | ожидается | отдельное явно названное окружение                             |
| NetworkPolicy, disruption, node drain              | ожидается | расширение для конкретного окружения                           |

## Вывод

INV-012 поддержан доказательствами macOS/Minikube. Job Pod может оставаться доступным для scrape финального полного
snapshot, в том числе несколькими scrapers, а Kubernetes способен ограничить lifecycle. Для неготового Pod безопаснее
направление PodMonitor/direct Pod discovery. Результат неготового ServiceMonitor записан как observation и не
устанавливает универсальную гарантию нулевого числа scrape.

Статус остаётся **в процессе** до успешного прогона точного отпечатка под Docker/Minikube на Ubuntu и фиксации политики
в ADR.
