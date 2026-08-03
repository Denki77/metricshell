# Отчёт INV-013 — Модели распространения

**Статус:** завершено
**Дата прогонов:** 2026-08-03
**Эталонные прогоны:** `results/20260803T103140Z`, `results/20260803T135959Z`
**Fingerprint:** `01b04f50305cce6474d74f9fa196d35bcd60dd65281134056d568cb2d4e96ea4`
**Решение:** [ADR-013](../../docs-ru/06-architecture/adr/ADR-013.md)

## Цель и область

Сравнить standalone artifact, multi-stage copy, base image и language-specific convenience image без изменения runtime
MetricShell и семантики полного snapshot из ADR-004. Packaging должен сохранять один versioned executable и не создавать
вторую реализацию обработки метрик.

## Прототип

- `cmd/inv013` выводит version, OS, architecture, UID и GID;
- `Dockerfile.binary` создаёт scratch artifact image;
- `Dockerfile.base` создаёт non-root Alpine base candidate;
- `Dockerfile.multistage` демонстрирует non-root PHP Alpine application image;
- `run-bench.sh` регистрирует pinned binfmt/QEMU, собирает и выполняет обе архитектуры, экспортирует standalone artifact
  через BuildKit local output, проверяет hash и сравнивает с no-cache rebuild.

Все внешние build/helper images заданы immutable multi-platform manifest digests. Извлечение artifact не создаёт
остановленный container; static-link check выполняется внутри уже собранного Alpine candidate.

## Среды прогонов

| Среда                             | Дата       | Docker | Архитектура | Результат                  | Статус                |
|-----------------------------------|------------|-------:|-------------|----------------------------|-----------------------|
| Docker Desktop на macOS/LinuxKit  | 2026-08-03 | 29.6.2 | aarch64     | `results/20260803T103140Z` | 24/24 assertions pass |
| Docker Desktop на Ubuntu/LinuxKit | 2026-08-03 | 27.4.0 | x86_64      | `results/20260803T135959Z` | 24/24 assertions pass |

Оба прогона имеют одинаковый fingerprint, указанный выше. Все assertions прошли в обеих средах. Обе container-среды
используют LinuxKit; сравнение покрывает LinuxKit aarch64 и x86_64, но не native Linux без LinuxKit.

## Результаты

### Матрица runtime и архитектур

Один CGO-free artifact выполнен в scratch, Alpine/musl и PHP Alpine. Images `linux/amd64` и `linux/arm64` собраны и
выполнены в каждом прогоне; при необходимости использовался digest-pinned binfmt/QEMU. Каждый candidate сообщил version
`0.13.0-research`, а OCI architecture metadata совпала с запрошенной platform.

### Identity artifact и rebuild

Экспортированный artifact и no-cache rebuild были byte-identical внутри каждой pinned Docker/toolchain environment.
SHA-256 artifact:

```text
f7df9d9d1b96ea9c36e387fe6178df3125490539bb09cbb17e5b8cb3393c5486
```

Это подтверждает deterministic rebuild для проверенной pinned environment, но не является доказательством
reproducibility независимых builders или всей release supply chain.

### Наблюдения размеров images

| Candidate          | macOS/LinuxKit aarch64 | Ubuntu/LinuxKit x86_64 | Runtime        | User          |
|--------------------|-----------------------:|-----------------------:|----------------|---------------|
| standalone/scratch |            1 507 480 B |              678 173 B | no libc        | research root |
| Alpine base        |           10 333 271 B |            4 310 805 B | musl userspace | non-root      |
| PHP multi-stage    |           97 875 364 B |           38 301 971 B | PHP 8.3 Alpine | non-root      |

Размеры зависят от architecture-specific variants закреплённых upstream layers и являются observations, а не
универсальными release limits. Standalone candidate остался минимальным в обеих средах.

### Immutable inputs

Go, PHP, Alpine и binfmt/QEMU references были одинаковыми immutable multi-platform digests. `base-images.tsv` отдельно
фиксирует выбранный native platform image ID, поэтому ожидаемое различие arm64/x86_64 platform IDs не принимается за
mutable-tag drift.

### Supply-chain evidence

Runner сохранил checksum artifact, OCI version metadata и SPDX record основного artifact. Registry publication,
signatures, transparency log, SLSA provenance, полный package inventory и vulnerability scanning относятся к production
release pipeline и не имитируются стендом.

## Проверка гипотез

### Static artifact и multi-stage copy достаточны

Подтверждено. Один static executable работал без libc и в обеих musl-based application environments на обеих
архитектурах. Pinned multi-stage copy сохраняет выбор base image и dependencies приложения.

### Base image MetricShell полезен

Подтверждено как optional convenience. Он предоставляет non-root default, но ограничивает downstream `FROM` и добавляет
runtime layers, поэтому не является обязательным элементом core distribution contract.

### Language-specific images должны быть primary

Отвергнуто. PHP example существенно больше scratch artifact на обеих архитектурах и связывает release MetricShell с
lifecycle language runtime, не добавляя core behavior.

## Принятая policy

- Публиковать checksummed static artifacts для `linux/amd64` и `linux/arm64`.
- Предоставлять явный version output и согласованные OCI labels.
- Документировать verified artifact copy и pinned multi-stage copy как default container integrations.
- Оставить minimal non-root base image optional.
- Рассматривать language-specific images как examples, а не обязательные release units.
- Закреплять каждый build/helper image immutable multi-platform digest.
- Для production releases публиковать checksum, OCI metadata, SBOM, signatures и provenance.
- Разделять утверждения о byte-identical local rebuild и независимой supply-chain reproducibility.

## Ограничения

- Обе container-среды используют LinuxKit; native Linux без LinuxKit не проверен.
- Стенд не является registry, signing service или полной release pipeline.
- SPDX идентифицирует основной artifact, но не содержит полный transitive package inventory.
- Проверены scratch, Alpine и PHP Alpine; distroless, Debian/glibc и Windows вне выбранного scope.
- Cross-architecture execution требует разрешения на privileged binfmt helper.
- Размеры images зависят от architecture-specific upstream layers.

## Дополнительные benchmarks

| Benchmark                              | Статус    | Evidence/граница                               |
|----------------------------------------|-----------|------------------------------------------------|
| standalone export и checksum           | покрыто   | artifact directory и `SHA256SUMS`              |
| scratch, Alpine и PHP Alpine execution | покрыто   | assertions и per-image logs                    |
| non-root base/application images       | покрыто   | UID/GID assertions                             |
| amd64/arm64 build и execution          | покрыто   | `architectures.tsv`                            |
| immutable base/helper digests          | покрыто   | `base-images.tsv`                              |
| byte-identical local no-cache rebuild  | покрыто   | assertion hash artifact                        |
| сравнение размеров images              | покрыто   | `observations.tsv` обеих сред                  |
| Ubuntu с matching fingerprint          | покрыто   | 24/24 assertions, fingerprint совпал           |
| независимый clean-builder rebuild      | follow-up | нужен для более сильного reproducibility claim |
| cosign/SLSA/full SBOM/CVE scan         | follow-up | production release engineering                 |

## Вывод

INV-013 подтверждено matching-fingerprint прогонами macOS/LinuxKit и Ubuntu/LinuxKit. Primary distribution model —
checksummed static artifact с pinned multi-stage copy. Minimal base image остаётся optional, language-specific images —
examples, а не primary release surfaces. Решение зафиксировано в
[ADR-013](../../docs-ru/06-architecture/adr/ADR-013.md).

## Выход исследования

- macOS evidence: `results/20260803T103140Z/`
- Ubuntu evidence: `results/20260803T135959Z/`
- SHA-256 artifact: `f7df9d9d1b96ea9c36e387fe6178df3125490539bb09cbb17e5b8cb3393c5486`
- ADR: [ADR-013](../../docs-ru/06-architecture/adr/ADR-013.md)
