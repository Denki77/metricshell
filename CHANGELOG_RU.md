# Журнал изменений

Здесь фиксируются заметные изменения MetricShell.

После первого stable release проект следует semantic versioning. Пока версия находится в `0.x`, несовместимые изменения
runtime contract могут появляться в minor versions.

## [0.1.2] - 2026-09-11

- Завершены Core implementation waves в `implementation`.
- Complete-snapshot runtime profile отмечен как готовый к эксплуатации.
- Зафиксировано, что publishers заменяют весь набор метрик целиком; partial metric updates не входят в этот release.
- Добавлены Docker-only CI, integration, fault, benchmark и supply-chain gates.
- Добавлены signed release evidence, SBOM и vulnerability-scanning pipeline.
- Добавлены GitHub-facing README, community profile files и MIT license.
