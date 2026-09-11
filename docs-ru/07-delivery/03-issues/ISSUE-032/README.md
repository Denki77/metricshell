# ISSUE-032. Статические multi-arch release artifacts

**Статус:** Готово
**Готовность:** Готово к разработке

**Эпик:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 6](../../02-epics/EPIC-001-core.md#wave-6)

**ADR/INV:** ADR-013 / INV-013.  
amd64/arm64, checksum, version, OCI metadata, pinned multi-stage copy.

Обязательные исполняемые примеры image/copy и их release verification должны соответствовать принятой
[спецификации Docker и Docker Compose](../../../04-specification/docker-compose-examples.md).

## Контракт готовности к разработке

- **Нормативные входы:** ADR-013, [Docker and Compose Examples](../../../04-specification/docker-compose-examples.md)
  и [Configuration](../../../04-specification/configuration.md).
- **Зависимости:** ISSUE-001 и ISSUE-036.
- **Объём / вне объёма:** Выпускать static linux amd64/arm64 binaries, checksums, version metadata, pinned OCI copy
  examples и multi-arch image metadata. Вне scope: копирование prototype binaries.
- **Конфигурация и наблюдаемые отказы:** Checksum/architecture/version mismatch вызывает failure до installation;
  mutable tags без digest запрещены в normative examples.
- **Критерии приёмки и обязательные тесты:** Clean cross-build; static linkage inspection; checksum corruption; wrong
  architecture; digest-pinned copy; container `--version`; multi-arch manifest.
- **Условие завершения:** Готово, когда independent clean builders воспроизводят verifiable artifacts обеих
  architectures.

## Журнал поставки

- 2026-09-09: переведено в `In Progress`; добавлен Docker `release` target, выпускающий static linux/amd64 и
  linux/arm64 binaries с release metadata и checksums.
- 2026-09-09: переведено в `Testing`; Docker test stage проверяет release checksums и static multi-arch build coverage,
  unit tests фиксируют Dockerfile/Makefile release contract.
- 2026-09-09: переведено в `Done`; pinned multi-stage copy documentation запрещает mutable artifact tags как release
  evidence.

## Подтверждение проверки

- `go test ./internal/releaseverify`
- `make test IMAGE=metricshell-issue032-release`
