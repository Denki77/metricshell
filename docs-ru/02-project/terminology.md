# Терминология

[English version](../../docs/02-project/terminology.md)

| Термин          | Определение                                                                |
|-----------------|----------------------------------------------------------------------------|
| Runtime         | Исполняемый файл MetricShell, выступающий wrapper контейнера.              |
| Workload        | Приложение, запущенное MetricShell.                                        |
| Producer        | Компонент, передающий метрики runtime.                                     |
| Snapshot        | Текущее экспортируемое состояние метрик.                                   |
| Final Snapshot  | Последнее состояние после завершения workload.                             |
| Scrape          | HTTP-запрос Prometheus или совместимого сборщика.                          |
| Exit Strategy   | Политика поведения после завершения workload.                              |
| Local Ingestion | Локальная связь workload с MetricShell.                                    |
| Operator        | Субъект, отвечающий за сборку, конфигурацию, развёртывание и эксплуатацию. |
