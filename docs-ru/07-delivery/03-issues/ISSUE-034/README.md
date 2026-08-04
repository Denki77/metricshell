# ISSUE-034. Настраиваемые capacity и timeout limits

**Статус:** Открыто
**Готовность:** Готово к разработке

**Эпик:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 6](../../02-epics/EPIC-001-core.md#wave-6)

## Нормативные входы

- ADR-003, ADR-005–ADR-008, ADR-010, ADR-011, ADR-014.
- [Спецификация configuration](../../../04-specification/configuration.md).
- [Грамматика значений configuration](../../../04-specification/configuration-value-grammar.md).
- [Runtime defaults и resource limits](../../../04-specification/runtime-defaults-and-resource-limits.md).
- [Application snapshot protocol](../../../04-specification/application-snapshot-protocol.md).

## Зависимости

ISSUE-001 — configuration bootstrap, ISSUE-008 — shutdown budget, ISSUE-016 — ingestion interface, ISSUE-023 —
exposition, а также все transport adapters.

## Scope

Реализовать полный CLI/environment surface version 1, общую scalar/list grammar и precedence; выбор transport;
фиксированные endpoints/paths; все decoded-input, snapshot, file, socket, HTTP, exposition, final-wait, shutdown,
filtering и concurrency limits; absolute deadline;
logging level/selector-value policy; cross-field socket capacity validation; формулу required_nofile; effective
non-secret configuration; постоянные internal exit codes и их structured failure mapping.

## Вне scope

Configuration files, dynamic reload, auto mode, managed-registry mode, listeners неактивных transports, произвольные
endpoint paths и автоматическое изменение RLIMIT.

## Конфигурация и наблюдаемые ошибки

Все properties, options, environment variables, defaults, ranges и units определены configuration specifications.
Invalid/unknown/contradictory input, options неактивного transport, unsafe paths, unavailable binds, недостаточный nofile
и истёкший deadline приводят к failure до запуска workload с документированным exit code и structured error. Runtime
limit exhaustion использует protocol NACK, HTTP status, self-metrics и logs без partial activation.

## Критерии приёмки

- CLI имеет приоритет над environment, environment — над defaults; workload argv после -- сохраняется byte-for-byte.
- Вся static validation и обязательные binds завершаются до запуска workload.
- Effective configuration скрывает secrets и не содержит workload argv/environment values.
- required_nofile вычисляется точно; недостаточный soft limit отклоняется без изменения RLIMIT.
- Каждый internal startup failure возвращает постоянный code 64, 70, 71, 72 или 73 согласно specification.
- Каждый capacity/timeout применяется на границе своего owner и сохраняет last-valid state.

## Обязательная test matrix

Каждый option/environment/default; precedence и repeatable filters; invalid units/ranges/unknown options; cross-field pairs, включая точную socket decoded capacity на limit/limit-1; оба logging levels и selector-value boolean; все
активные/неактивные transports; safe/unsafe paths и symlinks; bind conflicts; nofile на required-1/required;
absolute deadlines до/после now; каждый limit на limit/limit+1; overload/timeouts; debug redaction; exit-code registry и
structured error-code mapping; platform/container E2E.

## Условие завершения

Задача завершена, когда exhaustive configuration-table tests покрывают каждое public property, startup не создаёт
workload side effects при failure, все boundaries имеют наблюдаемые детерминированные ошибки, а container E2E проверяет
limits и exit codes.
