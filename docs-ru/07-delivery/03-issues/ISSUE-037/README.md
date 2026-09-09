# ISSUE-037. Release supply-chain pipeline

**Статус:** Готово
**Готовность:** Готово к разработке

**Эпик:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 6](../../02-epics/EPIC-001-core.md#wave-6)

Реализовать checksums, полный SBOM, signatures, provenance, vulnerability scanning и независимую validation на clean builder.

## Контракт готовности к разработке

- **Нормативные входы:** ADR-013, [Docker and Compose Examples](../../../04-specification/docker-compose-examples.md) и
  release outputs ISSUE-032/ISSUE-036.
- **Зависимости:** ISSUE-032, ISSUE-035 и ISSUE-036.
- **Объём / вне объёма:** Генерировать checksums, полный SBOM, signatures, provenance, vulnerability results и
  independent clean-builder verification. Вне scope: unsigned manual release artifacts.
- **Конфигурация и наблюдаемые отказы:** Любые missing/invalid signature, checksum, provenance subject, SBOM component
  или policy-blocking vulnerability блокируют publication.
- **Критерии приёмки и обязательные тесты:** Tampered binary/checksum/signature; incomplete SBOM; wrong provenance
  subject; clean rebuild; amd64/arm64 OCI verification; offline verification instructions.
- **Условие завершения:** Готово, когда каждый published artifact traceable, signed, reproducible и independently
  verifiable.

## Журнал поставки

- 2026-09-09: переведено в `In Progress`; добавлена local release evidence generation для checksums, detached Ed25519
  signature из внешнего BuildKit signing-key secret, provenance subjects, SBOM components из `go list -m -json all`,
  vulnerability policy decision из `govulncheck -json` и offline verification instructions.
- 2026-09-09: переведено в `Testing`; добавлены tamper tests для binary/checksum mismatch, signature corruption,
  incomplete SBOM, wrong provenance subject и policy-blocking vulnerability reports.
- 2026-09-09: переведено в `Done`; Docker stage `supply-chain-artifacts` и Makefile targets `supply-chain` /
  `supply-chain-ci` экспортируют independently verifiable release evidence из static release output без хранения
  private signing key в repository.
- 2026-09-10: усилена release evidence verification; подписанный manifest `SHA256SUMS` теперь покрывает raw
  scanner/module inputs и generated evidence files, а SBOM verification сверяет каждый Go module с подписанным
  `MODULES.jsonl`, включая versions и sums.

## Подтверждение проверки

- `go test ./internal/supplychain ./internal/releaseverify`
- `make supply-chain-ci IMAGE=metricshell-issue037-supply-chain`
- `make test IMAGE=metricshell-issue037-supply-chain`
- `make wave6 IMAGE=metricshell-wave6`
