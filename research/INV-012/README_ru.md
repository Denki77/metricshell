# INV-012 — Пригодность Kubernetes Job и CronJob

**Статус:** завершено
**Эталонные прогоны:** `results/20260803T143134Z`, `results/20260803T153300Z`
**Отчёт:** [report_ru.md](report_ru.md)
**Решение:** [ADR-012](../../docs-ru/06-architecture/adr/ADR-012.md)

## Вопрос

Может ли Prometheus продолжать обнаруживать и опрашивать endpoint MetricShell, пока завершившийся workload удерживается
внутри всё ещё работающего Pod объекта Kubernetes Job?

## Контекст

Окно метрик после завершения workload полезно только тогда, когда Kubernetes discovery сохраняет доступность Pod
достаточно долго, чтобы настроенные экземпляры Prometheus получили финальный полный snapshot. Согласно ADR-004,
MetricShell атомарно заменяет текущий snapshot и никогда не суммирует snapshots. Поэтому исследование считает успешные
scrape одного полного стабильного exposition, но не агрегирует значения метрик между scrape.

В качестве Kubernetes-окружения прототип явно использует Minikube. Это изолированный локальный кластер, а не
утверждение о поведении всех managed Kubernetes.

## Кандидаты

### A. Прямое обнаружение Pod

Prometheus обнаруживает аннотированные IP Pod через `kubernetes_sd_configs` и опрашивает их без Service.

### B. ServiceMonitor

Prometheus Operator обнаруживает Service и его endpoints через настоящий CRD `ServiceMonitor`.

### C. PodMonitor

Prometheus Operator обнаруживает IP Pod напрямую через настоящий CRD `PodMonitor`.

## Исходные гипотезы

- Pod объекта Job может оставаться живым после завершения workload и отдавать замороженный полный snapshot метрик;
- два экземпляра Prometheus могут опросить endpoint до завершения Job;
- readiness влияет на видимость цели для ServiceMonitor, но без фактических данных нельзя считать, что он полностью
  запрещает scrape;
- PodMonitor является более прямой моделью discovery для намеренно неготового Pod после завершения workload;
- `activeDeadlineSeconds`, `ttlSecondsAfterFinished`, удаление и политика CronJob ограничивают время жизни Pod;
- `concurrencyPolicy: Forbid` предотвращает пересечение длинных выполнений CronJob.

## Требуемые доказательства

- настоящий кластер Kubernetes в Minikube;
- два настоящих экземпляра Prometheus, установленных через Prometheus Operator;
- реальные CRD ServiceMonitor и PodMonitor;
- прямое обнаружение Pod реальной конфигурацией Prometheus;
- завершение Job по наблюдаемым scrape, а не по имитации callback;
- доказательства deadline, TTL, явного удаления и работы scheduler;
- отделение переносимых assertions от зависящих от окружения измерений.

## Эксперименты

### E-012.1 — Прямое обнаружение Pod

Отдельный Prometheus `v3.5.0` использует `kubernetes_sd_configs` с `role: pod`. Аннотированный Job завершается после
первого наблюдаемого scrape.

### E-012.2 — ServiceMonitor и два экземпляра Prometheus

`kube-prometheus-stack` `88.1.2` устанавливает Prometheus Operator и два экземпляра Prometheus. Настоящий
ServiceMonitor опрашивает готовый Job в ограниченном 60-секундном окне. После завершения Job runner отдельно выполняет
port-forward к каждому Prometheus Pod и запрашивает `inv012_final_snapshot` во времени внутри последнего активного окна.

### E-012.3 — Варианты readiness

Runner измеряет поведение ServiceMonitor для неготового Pod, не требуя нулевого числа scrape, а затем проверяет, что
PodMonitor способен напрямую опросить неготовый Pod.

### E-012.4 — Ограничители времени жизни

Runner проверяет `activeDeadlineSeconds`, `ttlSecondsAfterFinished` и явное удаление Pod с секундным grace period.

### E-012.5 — Пересечение CronJob

Настоящий CronJob с минутным расписанием работает дольше следующего тика. Runner проверяет, что
`concurrencyPolicy: Forbid` оставляет один активный Pod и один Job.

## Критерии оценки

- реальный discovery и scrape вместо подставных запросов;
- доступность полного snapshot после завершения workload;
- работа нескольких scrapers;
- ограниченное время жизни Job и Pod;
- отсутствие пересечения запусков scheduler;
- воспроизводимость одного отпечатка benchmark на macOS и Ubuntu.

## Открытые вопросы

- Пройдёт ли тот же отпечаток на валидационном Ubuntu-host?
- Отличается ли публикация endpoints и termination в managed Kubernetes?
- Следует ли по умолчанию использовать PodMonitor, если readiness после workload равен false?
- Какое production-окно после завершения достаточно для выбранного scrape interval и числа replicas?

## Результаты

| Окружение                          | Дата       | Набор результатов          | Сводка                                              | Отпечаток benchmark                                                |
|------------------------------------|------------|----------------------------|-----------------------------------------------------|--------------------------------------------------------------------|
| Docker Desktop на macOS, Minikube  | 2026-08-03 | `results/20260803T143134Z` | [summary.tsv](results/20260803T143134Z/summary.tsv) | `caa34fb8ba97176b3c199b56ffaeb21dcabbb654fa5efd2b4da3203d79274e6c` |
| Docker Desktop на Ubuntu, Minikube | 2026-08-03 | `results/20260803T153300Z` | [summary.tsv](results/20260803T153300Z/summary.tsv) | `caa34fb8ba97176b3c199b56ffaeb21dcabbb654fa5efd2b4da3203d79274e6c` |

Основные результаты обоих эталонных прогонов:

- в каждой среде прошли все 21 переносимый assertion и все 14 сценариев сводки;
- проверенный по checksum Minikube `v1.38.1` предоставил effective kubectl `v1.34.0`; системный kubectl не
  использовался;
- direct Pod discovery завершился за `8 017,520 ms` на macOS и `9 124,833 ms` на Ubuntu;
- ServiceMonitor получил 51/73 агрегированных HTTP scrape за `63 724,188/66 243,703 ms` на macOS/Ubuntu;
- после завершения Job Prometheus replicas 0 и 1 были опрошены отдельно; каждая TSDB вернула точную серию
  `inv012_final_snapshot` для текущего Pod Job `service-ready` со значением `42`;
- неготовый ServiceMonitor получил 1 scrape на macOS и 0 на Ubuntu. Число остаётся зависящим от окружения observation, а
  не
  переносимым инвариантом;
- PodMonitor опросил неготовый Pod не менее двух раз;
- active deadline дал `DeadlineExceeded`, TTL удалил завершённый Job, явное удаление Pod заняло `2 022,144 ms` на macOS
  и `4 250,833 ms` на Ubuntu;
- настоящий CronJob сохранил один активный Pod и один Job на следующем тике расписания при `Forbid`.

## Вывод

Доказательства macOS/Minikube поддерживают архитектурное направление: Pod после завершения workload способен отдавать
финальный полный snapshot нескольким Prometheus scrapers, а Kubernetes-ограничители способны ограничить это окно.
PodMonitor пригоден для прямого discovery неготового Pod. Поведение ServiceMonitor относительно readiness нужно считать
наблюдаемым поведением реализации, а не сводить к неподтверждённому правилу «ноль scrape».

INV-012 завершено. Production-направление зафиксировано в [ADR-012](../../docs-ru/06-architecture/adr/ADR-012.md).

## Выход решения

- Прототип: `prototype/`
- Runner: `run-bench.sh`
- Доказательства macOS: `results/20260803T143134Z/`
- Доказательства Ubuntu: `results/20260803T153300Z/`
- Отчёт: [report_ru.md](report_ru.md)
- Рекомендация для ADR: использовать прямой Pod discovery/PodMonitor для намеренно неготового endpoint после workload;
  сохранять явные lifetime/overlap limits; никогда не суммировать snapshots или значения разных scrape.

## Запуск прототипа

Требуются Docker, Helm, `curl` и сетевой доступ к настроенным registries бинарников, образов и chart. Системные Minikube
и `kubectl` не требуются.

```bash
./research/INV-012/run-bench.sh
```

Команда создаёт изолированный профиль Minikube `metricshell-inv012`, устанавливает закреплённую версию monitoring chart,
проводит все сценарии, записывает один каталог результатов, обновляет `latest-results.txt` и удаляет профиль. Сохранять
кластер следует только для диагностики:

Предварительно запускать Minikube не нужно. Runner скачивает официальный Minikube `v1.38.1`, проверяет platform-specific
SHA-256, удаляет прежний профиль с этим исследовательским именем и создаёт Docker-backed cluster с Kubernetes `v1.34.0`.
Все Kubernetes-команды используют соответствующий kubectl `v1.34.0` из Minikube; системный kubectl, включая Ubuntu
`1.30`, фиксируется для диагностики, но не используется. Успешный прогон занимает несколько минут; быстрое завершение
означает ошибку, а не успешное пустое исследование.

Фаза запуска Minikube имеет жёсткий timeout стенда 15 минут. Затем диагностические логи собираются не более двух минут,
а cleanup профиля ограничен 60 секундами. Поэтому зависший `kubeadm init` завершается с явным evidence фазы
`minikube_start`, а однокомандный запуск не остаётся заблокированным на несколько часов.

```bash
INV012_KEEP_MINIKUBE=1 ./research/INV-012/run-bench.sh
```

Просмотр последнего результата:

```bash
cat research/INV-012/latest-results.txt
cat "$(cat research/INV-012/latest-results.txt)/summary.tsv"
cat "$(cat research/INV-012/latest-results.txt)/assertions.tsv"
cat "$(cat research/INV-012/latest-results.txt)/observations.tsv"
cat "$(cat research/INV-012/latest-results.txt)/environment.tsv"
```

Диагностика ранней ошибки:

```bash
latest="$(cat research/INV-012/latest-results.txt)"
cat "$latest/run-summary.tsv"
cat "$latest/failure.tsv"
cat "$latest/docker-info.log"
cat "$latest/minikube-download.log"
cat "$latest/minikube-start.log"
cat "$latest/minikube-cluster.log"
cat "$latest/helm-install.log"
```

`run-summary.tsv` записывает `status=error`, фазу, строку, exit code, команду и нормализованную причину. Та же причина
сразу выводится в терминал без путей хоста. Перед запуском `docker info` должен успешно работать от текущего
пользователя
без `sudo`. Типичные причины на Ubuntu: отсутствующая зависимость, нет прав на Docker daemon, Docker не располагает
требуемыми CPU/RAM либо нет сетевого доступа к images или Helm repository.

На macOS и Ubuntu запускается одна и та же команда. Сравнивать нужно
`benchmark_code_fingerprint_sha256`; результаты с разными отпечатками не образуют одну кросс-платформенную пару.

## Ограничения прототипа

- Целью является проверенный по checksum Minikube `v1.38.1` с Kubernetes и effective kubectl `v1.34.0`, а не production
  managed cluster.
- Обе container-среды используют LinuxKit: macOS/LinuxKit aarch64 и Ubuntu/LinuxKit x86_64. Kubernetes проверен в
  Minikube, а не в managed cluster или native Linux без LinuxKit.
- Prometheus Operator установлен через `kube-prometheus-stack` `88.1.2`; отдельный Prometheus `v3.5.0` проверяет
  прямой Pod discovery.
- Время scheduler и scrape является наблюдением, но не production SLO.
- Прототип использует синтетический полный Prometheus snapshot и считает HTTP scrape; он не реализует все production
  ingestion paths MetricShell.
- Ни один assertion не требует конкретного числа scrape неготовой ServiceMonitor-цели; это зависящее от окружения
  observation.

## Дополнительные benchmarks

Текущий runner покрывает:

- bootstrap Minikube с проверкой checksum и assertion соответствующего кластеру kubectl;
- прямой discovery аннотированного Pod;
- настоящий ServiceMonitor с двумя экземплярами Prometheus;
- готовый и неготовый варианты ServiceMonitor;
- настоящий PodMonitor для неготового Pod;
- причину отказа `activeDeadlineSeconds`;
- удаление по `ttlSecondsAfterFinished`;
- latency явного termination Pod;
- настоящее расписание CronJob и `Forbid` на следующем тике;
- сохранение Helm manifest, CRD monitors, cluster events, сырых logs и отпечатка окружения.

Дополнительно полезно:

- повторить тот же отпечаток на Docker/Ubuntu;
- провести повтор на managed Kubernetes с его CNI и endpoint controller;
- изменить scrape intervals, replica count и post-exit windows;
- проверить NetworkPolicy, Pod disruption, node drain и сбой control plane;
- измерить большие полные exposition payloads без суммирования или слияния snapshots.
