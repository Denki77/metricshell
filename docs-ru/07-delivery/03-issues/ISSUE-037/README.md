# ISSUE-037. Release supply-chain pipeline

**Статус:** Открыто
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
