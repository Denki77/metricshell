# Managed Aggregation — договорённость до ADR

> **Статус:** ненормативный вход для архитектурных исследований
>
> **Назначение:** зафиксировать продуктовую цель и границы исследований, не принимая заранее семантические решения, которые должны быть проверены INV-016–INV-020.
>
> **Зависит от:** расширения scope/requirements Managed Aggregation и завершённых контрактов MetricShell Core.
>
> **Не заменяет:** ADR-001–ADR-015 и принятые спецификации Core.

## 1. Зачем нужен этот документ

Для Managed Aggregation уже определены требования и запланирован цикл архитектурных исследований, но ещё нет принятых ADR по Managed Aggregation. Поэтому неисследованные варианты семантики нельзя представлять как уже принятый нормативный protocol.

Документ фиксирует **чего мы хотим добиться** и **что должны определить исследования**. Это вход для INV-016–INV-020, а не их вывод. После исследований и принятия ADR внешне наблюдаемые решения переносятся в нормативную спецификацию.

## 2. Продуктовая цель: широкая поддержка instrumentation operations

Managed Aggregation нужен, чтобы простые и legacy workloads не были обязаны самостоятельно владеть полноценным metric registry.

Целью не является намеренно ограниченный API `increment-only`. Мы стремимся поддержать полезные instrumentation operations, реально необходимые workloads для поддерживаемых типов метрик, если для них можно определить детерминированную, безопасную и совместимую с Prometheus семантику.

Пространство кандидатов включает как минимум:

```text
counter:   increment / add / absolute update или initialization, где это семантически допустимо
gauge:     set / increment / decrement / add / subtract
histogram: observe и необходимые declaration/configuration operations
registry:  declaration и lifecycle operations, если исследования докажут их необходимость
batch:     несколько связанных operations, если оправдана atomic batching
```

Это **кандидаты для исследования**, а не уже принятые команды или гарантии protocol.

INV-016 должен определить допустимые операции, необходимые ограничения и операции, которые невозможно поддержать без нарушения metric model или существующих Core contracts.

Предпочтение при проектировании:

> Поддерживать максимально широкий практически полезный набор операций. Исключать операцию только при наличии подтверждённой исследованием причины: семантика, корректность, совместимость, безопасность, ресурсы или неоправданная сложность.

## 3. Почему Counter нельзя заранее ограничивать только increment

Prometheus counter представляет накопительное состояние и в нормальном lifecycle не уменьшается, кроме reset. Это ограничивает **семантику результирующей метрики**, но само по себе не доказывает, что клиентский API Managed Aggregation должен предоставлять только `increment`.

Legacy workload может естественным образом знать абсолютное накопительное значение, которое уже ведёт приложение, delta для добавления, начальное значение из application state либо факт reset/new lifecycle.

Нужны ли Managed Aggregation `increment`, `add`, absolute counter update, explicit initialization, reset semantics либо только часть этих возможностей — вопрос INV-016.

Исследование должно разделить:

1. **Валидность семантики метрики** — какое состояние допустимо для counter.
2. **Семантику клиентской операции** — как клиент выражает переход состояния.
3. **Epoch/reset semantics** — когда уменьшение является допустимым reset/new epoch, а когда ошибочной mutation.
4. **Concurrency semantics** — можно ли однозначно определить absolute update при нескольких publishers.
5. **Совместимость с Core** — остаётся ли полученный полный snapshot валидным в принятом Core contract.

До появления evidence спецификация не должна утверждать, что counters навсегда ограничены моделью `increment-only`.

## 4. Что можно считать зафиксированным до исследований

Уже являются продуктовыми/Core-ограничениями:

- Managed Aggregation опционален.
- Snapshot mode остаётся режимом по умолчанию.
- Managed Aggregation остаётся внутри того же binary/process model MetricShell.
- Один managed registry относится к одному логическому запуску workload.
- Независимые приложения не используют один managed registry совместно.
- Managed state передаётся в Core как полный application snapshot.
- Существующая snapshot-семантика Core не изменяется неявно.
- Invalid/rejected managed input не должен повреждать ранее валидное состояние.
- Новый запуск MetricShell по умолчанию начинает новую managed-registry epoch.
- Persistence/replay между рестартами MetricShell не входят в первое расширение.
- Hybrid ownership внешних complete snapshots и managed operations не входит в первое расширение без отдельного исследования и решения.

Они ограничивают исследование, но не определяют managed-registry protocol.

## 5. Что остаётся открытым до исследований

До соответствующего INV и ADR остаются нерешёнными:

- точный набор counter operations и initialization/reset behavior;
- точный набор gauge operations;
- histogram declaration/update semantics;
- declaration и mutation descriptors;
- creation/deletion/staleness series;
- publisher ownership и cleanup;
- batches и atomicity;
- ordering и linearization;
- duplicate delivery и idempotency;
- acknowledgement semantics;
- transport и framing;
- snapshot materialization timing;
- связь acceptance операции с установкой snapshot в Core;
- freeze boundary и in-flight operations;
- конкретные resource limits и overload behavior.

Draft может содержать hypotheses/candidates, но не должен представлять их как принятые нормативные решения через `MUST`/`SHALL` / «ДОЛЖЕН».

## 6. Flow документации

```text
03 Requirements
    ↓
04 Ненормативный behavioral/specification draft
    ↓
05 Architecture Investigation (INV-016–INV-020)
    ↓ evidence
06 ADR(s)
    ↓ принятые решения
04 Accepted normative specification
    ↓
07 Delivery / implementation
```

Pre-ADR документ в `04-specification` является **входом для исследования**, аналогично behavioral model draft. До принятия необходимых ADR он не должен находиться среди accepted normative specifications.

## 7. Правило обновления текущего draft

Draft Managed Aggregation следует переработать так, чтобы:

1. уже принятые product/Core constraints оставались нормативными ссылками;
2. неисследованная семантика была goals, candidates, hypotheses или open questions;
3. `MUST`/`SHALL` / «ДОЛЖЕН» не выбирали operation model до исследований;
4. counter не ограничивался искусственно только `increment`;
5. исследования проверяли максимально широкий практически полезный набор operations;
6. исключение операции требовало явной причины, подтверждённой исследованием;
7. принятые результаты INV/ADR постепенно переносились в нормативную specification.

## 8. Условие завершения

Это временный архитектурный вход. Он выполнил задачу, когда INV-016–INV-020 дали достаточный evidence, необходимые ADR приняты, а Managed Aggregation specification содержит итоговый внешне наблюдаемый нормативный контракт.

Финальная спецификация может оказаться уже пространства кандидатов выше, но только если исследования покажут, почему конкретный вариант небезопасен, неоднозначен, несовместим, неоправданно сложен или иначе непригоден — а не потому, что возможность исключили до исследования.
