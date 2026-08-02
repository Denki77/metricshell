# Отчёт INV-013 — Модели распространения

**Статус:** в процессе
**Дата:** 2026-08-03
**Прогон:** `results/20260803T103140Z`
**Платформа:** Docker 29.6.2, LinuxKit aarch64
**Fingerprint:** `01b04f50305cce6474d74f9fa196d35bcd60dd65281134056d568cb2d4e96ea4`

## Цель и scope

Сравнить standalone, multi-stage, base и language convenience distribution, не меняя MetricShell behavior и complete
snapshot semantics ADR-004.

## Прототип

`cmd/inv013` печатает version/OS/arch/UID/GID. Три Dockerfile создают scratch artifact, non-root Alpine base и non-root
PHP Alpine application. Runner регистрирует pinned binfmt/QEMU, строит и выполняет кандидатов, делает cross-build,
напрямую экспортирует standalone filesystem через BuildKit, хэширует его и сравнивает с no-cache export.

## Результаты

| Кандидат        |       Размер | Runtime    | User          | Результат |
|-----------------|-------------:|------------|---------------|-----------|
| scratch         |  1 507 480 B | no libc    | research root | pass      |
| Alpine base     | 10 333 271 B | musl       | non-root      | pass      |
| PHP multi-stage | 97 875 364 B | PHP Alpine | non-root      | pass      |

Пройдено 24/24. arm64 выполнен native, amd64 — через Docker emulation. Version везде `0.13.0-research`. Rebuild совпал
по SHA-256 `f7df9d9d1b96ea9c36e387fe6178df3125490539bb09cbb17e5b8cb3393c5486`.

Go, PHP, Alpine и binfmt/QEMU images закреплены immutable multi-platform digests. `base-images.tsv` сохраняет эти
references и фактические native arm64 image IDs; Ubuntu должен использовать те же manifest digests и запишет свои
x86_64 IDs. Runner сам регистрирует обе benchmark architectures, поэтому обычный Ubuntu Docker Engine не зависит от
ручной подготовки эмуляции.

Static-link check запускает `ldd` внутри собранного Alpine-кандидата и не монтирует экспортированный host artifact.
Точный экспортированный artifact независимо выполняется в scratch-кандидате, где отсутствует dynamic loader.

## Проверка гипотез

Static artifact и multi-stage copy достаточны — предварительно подтверждено. Base image полезен только optional. Primary
language images не оправданы: PHP example примерно в 65 раз больше scratch и добавляет lifecycle coupling.

## Критерии и policy

Static artifact совместим со scratch/musl, обе architecture выполнены, non-root examples и pinning прошли. Подтверждён
byte-identical rebuild внутри одного pinned toolchain/daemon environment, но не полная supply-chain reproducibility.
Следует публиковать checksummed static binaries и multi-arch artifact, документировать
pinned multi-stage default, оставлять base optional и не делать language images обязательными. Release pipeline должен
добавить signatures, full SBOM и provenance.

## Ограничения и Additional Benchmarks

Ubuntu ожидается. Immutable images устраняют mutable-tag drift, но local rebuild всё ещё разделяет daemon/toolchain
environment. Совпадение Ubuntu подтвердит portable benchmark с теми же inputs, а не полную reproducible-build supply
chain. Registry/cosign/SLSA/independent clean-builder/CVE остаются release work. Все локально возможные варианты —
artifact, pinned images, runtimes, architectures, non-root, versioning, sizes, checksum, rebuild, SPDX — проведены.

## Вывод

Предварительный выбор: standalone static artifact плюс pinned multi-stage copy; base optional, language-specific images
не primary. Финализация после Ubuntu и supply-chain validation.

## Выход

- Evidence: `results/20260803T103140Z/`
- Artifact SHA-256: `f7df9d9d1b96ea9c36e387fe6178df3125490539bb09cbb17e5b8cb3393c5486`
- Ubuntu/ADR: ожидаются
