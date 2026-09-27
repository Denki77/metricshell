# Журнал изменений

Здесь фиксируются заметные изменения MetricShell.

После первого stable release проект следует semantic versioning. Пока версия находится в `0.x`, несовместимые изменения
runtime contract могут появляться в minor versions.

## [Unreleased] - 2026-09-27

- Добавлен opt-in ownership mode `managed-registry` с bounded descriptor-driven operations для counter, gauge и classic
  histogram через private Unix protocol.
- Добавлены serialized commit acknowledgement, resource controls, immutable materialization и общий Core install path
  с admission close, bounded drain, freeze и существующим final-scrape lifecycle.
- Добавлены stateless shell/PHP 5.4 clients, fixed-cardinality observability, production E2E и воспроизводимое validation
  evidence. Snapshot mode остаётся default и сохраняет существующий contract.

## [0.1.2] - 2026-09-11

- Завершены Core implementation waves в `implementation`.
- Complete-snapshot runtime profile отмечен как готовый к эксплуатации.
- Зафиксировано, что publishers заменяют весь набор метрик целиком; partial metric updates не входят в этот release.
- Добавлены Docker-only CI, integration, fault, benchmark и supply-chain gates.
- Добавлены signed release evidence, SBOM и vulnerability-scanning pipeline.
- Добавлены GitHub-facing README, community profile files и MIT license.
