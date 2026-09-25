# ISSUE-MA-005. Operation protocol v1 и bounded framing

**Статус:** Planned  
**Готовность:** Code-ready

**Эпик:** [EPIC-002 Managed Aggregation](../02-epics/EPIC-002-managed-aggregation.md)  
**Wave:** [Wave 3](../02-epics/EPIC-002-managed-aggregation.md#wave-3)

**ADR/INV:** ADR-018 / INV-018; ADR-017 / INV-017

## Нормативные входы

ADR-018 / INV-018, ADR-017, configuration/resource-limit contracts.

## Зависимости

ISSUE-MA-004.

## Scope

Реализовать protocol v1 request/response structures, bounded newline-delimited JSON framing, one operation per connection, deterministic version/framing/protocol/semantic outcomes и bounded parsing.

## Вне scope

Unix listener lifecycle, CLI/PHP wrappers, unsupported alternate managed transports.

## Конфигурация и наблюдаемые ошибки

Настроенный maximum frame size и parsing bounds должны быть положительными, конечными и представимыми; недопустимые значения являются startup configuration errors. Пустой, truncated, malformed, multi-frame, oversized input или неподдерживаемая версия отклоняется на protocol level до owner admission и не меняет registry. Успешные parsing и semantic admission ещё не означают commit. Protocol выдаёт success только из successful committed owner result, определённого в ISSUE-MA-004; semantic rejection, overload и pre-commit cancellation остаются non-success outcomes, а disconnect/timeout после admission, но до response, дают явный unknown client outcome. Класс результата и frame-limit rejection наблюдаемы без записи payload в log.

## Критерии приёмки

- Valid v1 operation парсится в одну domain command.
- Partial frame не доходит до mutation.
- Oversized frame reject в bounded memory.
- Missing/invalid/unsupported version reject deterministic.
- Transport/protocol/semantic/overload/unknown outcomes различимы.
- Success выводится только из successful committed owner result ISSUE-MA-004; parsing или admission сами по себе никогда не дают success.

## Обязательная матрица тестов

Golden corpus; exact frame limit/+1; malformed JSON; partial EOF; versions; semantic rejection mapping; fuzz/property tests; no mutation on parse failure.

## Завершение

Завершено, когда все критерии и обязательные тесты проходят CI, а задача сохраняет ADR-016...ADR-020 и backward compatibility snapshot mode.
