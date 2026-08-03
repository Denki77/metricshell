# INV-013 — Модели распространения

**Статус:** завершено
**Эталонный прогон macOS:** `results/20260803T103140Z`
**Эталонный прогон Ubuntu/LinuxKit:** `results/20260803T135959Z`
**Отчёт:** [report_ru.md](report_ru.md)
**Решение:** [ADR-013](../../docs-ru/06-architecture/adr/ADR-013.md)

## Вопрос

Как MetricShell должен добавляться в application images?

## Контекст и гипотезы

Distribution не должна менять runtime или contract ADR-004. Один versioned static executable должен работать как
standalone artifact, multi-stage input и в небольшом base image. Language convenience image упрощает onboarding, но
увеличивает size, maintenance и coupling.

Гипотеза: checksummed standalone static binary и документированный multi-stage copy — достаточный default. Base image
может быть optional, convenience images не должны становиться primary model.

## Необходимые доказательства

- export standalone artifact через BuildKit local output и SHA-256 без создания stopped container;
- execution в scratch, Alpine/musl и PHP Alpine;
- non-root base/multi-stage;
- version pinning в binary и OCI labels;
- amd64/arm64 build и execution;
- byte-identical rebuild;
- image sizes и SPDX;
- immutable multi-platform digests всех base/helper images;
- автоматическая регистрация pinned binfmt/QEMU для cross-architecture execution в обычном Docker Engine;
- Ubuntu repeat с тем же fingerprint.

## Подтверждённый результат

В обеих средах пройдено 24/24 assertions. Один CGO-free artifact работал в scratch, Alpine и PHP Alpine. amd64 и arm64
images реально
выполнены, включая amd64 emulation на arm64 host. No-cache rebuild дал тот же SHA-256
`f7df9d9d1b96ea9c36e387fe6178df3125490539bb09cbb17e5b8cb3393c5486`.

Все внешние images закреплены immutable multi-platform digests: Go
`sha256:383395b794dffa5b53012a212365d40c8e37109a626ca30d6151c8348d380b5f`, PHP
`sha256:afdf8b1fee58486ccc0dab5f30f634b86873d56dac985f71ba217945647c05ad` и Alpine
`sha256:d9e853e87e55526f6b2917df91a2115c36dd7c696a35be12163d44e6e2a4b6bc`, а также binfmt/QEMU
`sha256:1b804311fe87047a4c96d38b4b3ef6f62fca8cd125265917a9e3dc3c996c39e6`. Выбранные platform image IDs записаны в
`base-images.tsv`.

Размеры на macOS/aarch64: 1 507 480, 10 333 271 и 97 875 364 bytes; на Ubuntu/x86_64: 678 173, 4 310 805 и
38 301 971 bytes для scratch, Alpine base и PHP multi-stage соответственно. Fingerprint обоих прогонов:
`01b04f50305cce6474d74f9fa196d35bcd60dd65281134056d568cb2d4e96ea4`.

## Принятые значения

- CGO-free static linux/amd64 и linux/arm64 artifact;
- version в binary/OCI, опубликованный SHA-256;
- default integration — verified artifact или pinned multi-stage copy;
- non-root supplied examples;
- base image optional;
- language image только пример;
- checksum, OCI metadata и SPDX для releases;
- все build/helper images задаются immutable multi-platform digest;
- подтверждён только byte-identical rebuild в одном pinned toolchain/daemon environment, но не полная supply-chain
  reproducibility независимых builders.

## Запуск

```bash
./research/INV-013/run-bench.sh
latest="$(cat research/INV-013/latest-results.txt)"
cat "$latest/assertions.tsv"
cat "$latest/architectures.tsv"
cat "$latest/observations.tsv"
cat "$latest/base-images.tsv"
cat "$latest/artifacts/SHA256SUMS"
cat "$latest/sbom.spdx"
cat "$latest/environment.tsv"
cat "$latest/run-summary.tsv"
```

На Ubuntu используется та же команда и сравнивается fingerprint. Готовить эмуляцию вручную не нужно: runner через
закреплённый digest образа binfmt регистрирует amd64 и arm64 до cross-architecture cases. Учётная запись Docker должна
иметь право запуска этого privileged helper container.

Успешный прогон не завершается мгновенно: требуется скачать образы и собрать обе архитектуры. Быстрый выход означает
ошибку, а не пустой результат. Runner сразу обновляет `latest-results.txt` и записывает проблемную фазу в
`summary.tsv` и `run-summary.tsv`, а также печатает нормализованную причину без путей хоста. Перед запуском
`docker info`
должен успешно работать от текущего пользователя без `sudo`. Диагностика сохранённого прогона:

```bash
latest="$(cat research/INV-013/latest-results.txt)"
cat "$latest/run-summary.tsv"
cat "$latest/failure.tsv"
cat "$latest/docker-info.log"
cat "$latest/binfmt-install.log"
cat "$latest/amd64.build.log"
cat "$latest/arm64.build.log"
```

## Ограничения

Это не release signing pipeline. SPDX фиксирует основной artifact, но полный package inventory требует production SBOM
tool. Нет registry signatures, transparency log и vulnerability scan. Проверены scratch и musl images; Windows вне
Linux scope. Sizes зависят от upstream bases.
Для извлечения artifact нужен Docker Buildx с local exporter; его наличие проверяется на preflight.
Проверка static linking выполняется внутри уже собранного Alpine-кандидата и не требует host bind mount.
Cross-architecture execution регистрирует binfmt через privileged helper container. Это меняет регистрацию эмуляции
в Docker host/VM и требует соответствующего разрешения Docker.
Byte-identical rebuild подтверждён отдельно в каждой pinned toolchain/daemon environment. Совпадение между средами
подтверждает переносимость benchmark, но не independent-builder или полную supply-chain reproducibility.
Обе container-среды используют LinuxKit: macOS/LinuxKit aarch64 и Ubuntu/LinuxKit x86_64. Native Linux без LinuxKit
не проверен.

## Additional Benchmarks

| Benchmark                      | Статус            |
|--------------------------------|-------------------|
| standalone/checksum            | покрыто           |
| scratch/Alpine/PHP             | покрыто           |
| non-root                       | покрыто           |
| amd64/arm64 execution          | покрыто           |
| version/OCI                    | покрыто           |
| reproducible rebuild           | покрыто           |
| immutable image digests        | покрыто           |
| pinned binfmt/QEMU setup       | покрыто           |
| sizes/SPDX                     | покрыто           |
| Ubuntu fingerprint             | покрыто: 24/24    |
| cosign/SLSA/full SBOM/CVE scan | release follow-up |

## Лучший follow-up

Проверить published multi-arch manifest, signatures, provenance и full SBOM. Только независимый clean CI worker может
расширить вывод до independent-builder reproducibility.

## Выход

- Prototype: `prototype/`
- macOS evidence: `results/20260803T103140Z/`
- Ubuntu: `results/20260803T135959Z/`
- Отчёт: [report_ru.md](report_ru.md)
- ADR: [ADR-013](../../docs-ru/06-architecture/adr/ADR-013.md)
