# Спецификация runtime state machine

[English version](../../docs/04-specification/runtime-state-machine.md)

> Статус: принятая нормативная спецификация
> Требования: FR-001–FR-006, FR-040–FR-050
> Критерии приёмки: AC-RUN-001–AC-RUN-009, AC-FIN-001–AC-FIN-015
> Решения: ADR-001, ADR-002, ADR-003, ADR-011

## Scope

Это единая нормативная lifecycle model MetricShell Core. Имена состояний являются точными значениями self-metric
runtime state и structured logs. Реализация может иметь приватные substates, но не может экспонировать другой публичный
набор состояний.

## Состояния

Закрытый публичный набор:

~~~text
initializing
starting_workload
running
stopping
finalizing
final_wait
failed
terminated
~~~

| Состояние         | Значение                                                                                                 | Readiness   |
|-------------------|----------------------------------------------------------------------------------------------------------|-------------|
| initializing      | Валидация всей configuration и bind обязательных ресурсов до запуска workload.                           | not ready   |
| starting_workload | Только одна попытка запуска workload.                                                                    | not ready   |
| running           | Workload активен; ingestion и exposition доступны.                                                       | ready       |
| stopping          | Идёт external termination; сигнал передан и выполняется bounded cleanup workload.                        | not ready   |
| finalizing        | Фиксация результата workload, закрытие ingestion, разрешение in-flight ordering и freeze final snapshot. | not ready   |
| final_wait        | Frozen snapshot обслуживается по policy естественного завершения.                                        | not ready   |
| failed            | Зафиксирован невосстановимый сбой MetricShell; остаётся bounded cleanup.                                 | not ready   |
| terminated        | Не осталось endpoints и children под владением MetricShell; процесс завершается только один раз.         | unavailable |

Workload exit является событием, а не состоянием. Forced termination — действие и outcome внутри stopping, а не
публичное состояние. Duration и scrape-count policies используют final_wait; immediate mode обходит его.

## События

Закрытый lifecycle event set:

| Событие                 | Допустимый источник                                              | Результат                                |
|-------------------------|------------------------------------------------------------------|------------------------------------------|
| configuration_validated | initializing                                                     | starting_workload                        |
| initialization_failed   | initializing                                                     | failed                                   |
| workload_started        | starting_workload                                                | running                                  |
| workload_start_failed   | starting_workload                                                | failed                                   |
| workload_exited         | running, stopping                                                | finalizing                               |
| termination_requested   | initializing, starting_workload, running, finalizing, final_wait | stopping или terminated по правилам ниже |
| runtime_failed          | любое нетерминальное состояние                                   | failed                                   |
| finalization_completed  | finalizing                                                       | final_wait или terminated                |
| final_wait_completed    | final_wait                                                       | terminated                               |
| cleanup_completed       | failed                                                           | terminated                               |

Повторные termination signals не создают новое состояние. Они могут сократить remaining grace или запустить немедленный
forced cleanup согласно shutdown policy и обязательно логируются.

## Нормативные переходы

~~~mermaid
stateDiagram-v2
    [*] --> initializing
    initializing --> starting_workload: configuration_validated
    initializing --> failed: initialization_failed
    initializing --> terminated: termination_requested
    starting_workload --> running: workload_started
    starting_workload --> failed: workload_start_failed
    starting_workload --> stopping: termination_requested after spawn
    starting_workload --> terminated: termination_requested before spawn
    running --> finalizing: workload_exited
    running --> stopping: termination_requested
    running --> failed: runtime_failed
    stopping --> finalizing: workload_exited after bounded cleanup
    stopping --> failed: runtime_failed
    finalizing --> final_wait: natural completion and mode duration or scrapes
    finalizing --> terminated: natural completion and mode immediate
    finalizing --> terminated: external termination already active
    finalizing --> terminated: termination_requested
    finalizing --> failed: runtime_failed
    final_wait --> terminated: duration elapsed, required scrapes, or timeout
    final_wait --> terminated: termination_requested
    final_wait --> failed: runtime_failed
    failed --> terminated: cleanup_completed
    terminated --> [*]
~~~

После workload_exited переход в running запрещён. Недопустимый переход является internal failure.

## Правила final wait

Принятые modes: immediate, duration и scrapes. Mode auto отсутствует.

- immediate переводит finalizing непосредственно в terminated;
- duration входит в final_wait до истечения configured duration;
- scrapes входит в final_wait до достижения saturating counter положительного N или finite timeout;
- eligible является только успешный полный response frozen generation, завершённый после входа в final_wait;
- health, readiness, debug, cancelled, failed, pre-final и ineligible responses не считаются;
- после достижения N completion grace drain-ит только уже принятые handlers и не увеличивает N;
- external termination немедленно завершает final_wait и имеет приоритет над обычными условиями завершения.

## Ingestion ordering при workload exit

Переход в finalizing сначала закрывает admission. Candidate, допущенный ранее, может завершиться в remaining
finalization budget. При acceptance он становится final generation; иначе фиксируется предыдущий last-valid snapshot.
Candidates, не допущенные до закрытия, получают frozen.

## Probe и endpoint semantics

| Состояние         |                             health |   readiness | metrics                      |
|-------------------|-----------------------------------:|------------:|------------------------------|
| initializing      | 200 пока возможен bounded progress |         503 | unavailable до bind          |
| starting_workload |                                200 |         503 | available после bind         |
| running           |                                200 |         200 | available                    |
| stopping          |  200 пока возможен bounded cleanup |         503 | available пока server открыт |
| finalizing        |                                200 |         503 | frozen view после freeze     |
| final_wait        |                                200 |         503 | frozen view available        |
| failed            |                                500 |         503 | best effort до начала drain  |
| terminated        |                        unavailable | unavailable | unavailable                  |

Probe requests никогда не считаются final scrapes. Readiness намеренно false вне running.

## Приоритет termination и process result

При конкурирующих условиях действует порядок:

1. невосстановимый internal failure MetricShell;
2. external termination и forced-cleanup policy;
3. сохранённый workload result;
4. нормальное завершение final wait.

Final-wait timeout является нормальной bounded completion reason и не заменяет workload result. Forced cleanup
фиксируется отдельно; он меняет результат только если workload ещё не дал результата. Постоянный registry собственных
exit codes MetricShell определён в configuration specification.

## Observability mapping

Каждый переход выпускает только один state-change log после активации нового состояния. Metric
metricshell_runtime_state использует только значения из этого документа. Mode и completion reason final_wait используют
закрытые enums спецификации self-metrics. Structured logs используют те же registries state, mode, outcome и reason.

## Conformance

Table-driven tests покрывают каждый допустимый и недопустимый переход, concurrent races workload
exit/signal/publication,
probe responses во всех состояниях, все terminal conditions final_wait, повторные signals, forced cleanup, только один
переход в terminated и запуск race detector.

## Ссылки

- [Спецификация configuration](configuration.md)
- [Спецификация self-metrics](self-metrics.md)
- [Спецификация structured logging](structured-logging.md)
- [ADR-002](../06-architecture/adr/ADR-002.md)
- [ADR-003](../06-architecture/adr/ADR-003.md)
- [ADR-011](../06-architecture/adr/ADR-011.md)
