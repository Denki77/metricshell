# ISSUE-010. Контракт health и readiness

**Статус:** Готово
**Готовность:** Готово к разработке

**Эпик:** [EPIC-001 Core](../../02-epics/EPIC-001-core.md)  
**Wave:** [Wave 2](../../02-epics/EPIC-001-core.md#wave-2)

**ADR/INV:** ADR-002, ADR-011.  
**Acceptance criteria:**

- health отражает способность supervisor продолжать bounded operation;
- readiness различает startup, active exposition и intentionally-unready final phase;
- probes никогда не считаются final scrapes;
- endpoint states документированы.

## Контракт готовности к разработке

- **Нормативные входы:** ADR-002 и
  ADR-011, [Configuration](../../../04-specification/configuration.md), [Runtime State Machine](../../../04-specification/runtime-state-machine.md)
  и [Structured Logging](../../../04-specification/structured-logging.md).
- **Зависимости:** ISSUE-007; блокирует probe handling в ISSUE-023 и final-scrape logic в ISSUE-026.
- **Объём / вне объёма:** Реализовать fixed `/healthz` и `/readyz` semantics для каждого public state. Вне scope:
  configurable probe paths и учёт probes как scrapes.
- **Конфигурация и наблюдаемые отказы:** Probe responses bounded и выводятся из state; unavailable/failed states
  возвращают deterministic statuses без изменения lifecycle.
- **Критерии приёмки и обязательные тесты:** Таблица state-by-endpoint status; transition races; requests во время
  shutdown; method/path errors; доказательство, что probes не увеличивают final-scrape count.
- **Условие завершения:** Готово, когда specification table и HTTP integration fixtures совпадают для каждого state.

## Журнал выполнения

- 2026-08-24: задача переведена в `В работе`; реализован bounded HTTP probe adapter поверх lifecycle state source.
- 2026-08-24: задача переведена в `Тестирование`; полная матрица state/endpoint, shutdown states, concurrent transitions,
  method/path errors и исключение final scrapes прошли race detector и проверку real HTTP server в Docker.
- 2026-08-24: задача переведена в `Готово`; нормативная таблица EN/RU и Docker fixture совпадают для всех восьми public
  states.

## Свидетельства проверки

- Точные routes `GET /healthz` и `GET /readyz` возвращают state-derived bounded text responses; другие methods дают
  `405` с `Allow: GET`, другие paths — `404`.
- Initializing, starting, stopping, finalizing и final-wait healthy, но unready; running healthy и ready; failed даёт
  `500`/`503`; принятый request, увидевший terminated, получает deterministic `503 unavailable`.
- Probe handling только читает synchronized lifecycle state. Concurrent transition/request tests проходят race
  detector, а routing probes не может вызвать final-scrape completion `/metrics`.
- HTTP integration fixture выполняет 18 requests через real loopback server внутри scratch integration image.
- `implementation/README.md` и `implementation/README_RU.md` проверены на полноту и обновлены синхронно.
