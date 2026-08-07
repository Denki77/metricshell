# Спецификация фильтрации метрик

[English version](../../docs/04-specification/metrics-filtering.md)

> Статус: принятая нормативная спецификация
> Требования: FR-032, FR-033
> Критерии приёмки: AC-EXP-003, AC-EXP-004
> Решения: ADR-004, ADR-010, ADR-014

## Назначение

Документ определяет синтаксис, defaults, приоритеты и наблюдаемое поведение фильтрации application metrics.
Фильтрация является policy уровня exposition. Она не изменяет принятый application snapshot, не меняет ownership,
не объединяет состояния producers и не ослабляет полную validation по ADR-004.

## Область действия

Фильтрация применяется только к application metric families. Self-metrics MetricShell данным механизмом не фильтруются.
Первая стабильная версия поддерживает только точные имена families и префиксы families.

Вне scope:

- label matchers;
- фильтрация по values;
- regex и glob patterns;
- параметры фильтра в scrape request;
- producer-specific filtering;
- hot reload фильтров внутри одного workload execution.

## Configuration

| Каноническое свойство       | CLI                                       | Environment                   | Default |
|-----------------------------|-------------------------------------------|-------------------------------|---------|
| `exposition.filter.include` | repeatable `--metrics-include=<selector>` | `METRICSHELL_METRICS_INCLUDE` | пусто   |
| `exposition.filter.exclude` | repeatable `--metrics-exclude=<selector>` | `METRICSHELL_METRICS_EXCLUDE` | пусто   |

Environment использует правила comma-list из
[грамматики значений конфигурации](configuration-value-grammar.md): в version 1 escaping отсутствует, comma или
backslash
внутри selector недопустимы. CLI option повторяется. Если одно свойство задано одновременно через CLI и environment,
CLI полностью заменяет environment-список; списки не объединяются.

Configuration валидируется до запуска workload и остаётся неизменной до конца execution.

## Синтаксис selector

```ebnf
selector      = selector-type, ":", metric-token ;
selector-type = "name" | "prefix" ;
metric-token  = prometheus-name-start, { prometheus-name-char } ;
```

Правила:

- первый символ: `A-Z`, `a-z`, `_`, `:`;
- последующие символы дополнительно допускают `0-9`;
- matching case-sensitive;
- внешние пробелы environment item удаляются;
- пробел внутри selector не допускается;
- пустые `name:` и `prefix:` не допускаются;
- regex, `*`, `?` и label selectors не допускаются.

Примеры:

```text
name:http_requests
prefix:application_
prefix:worker_queue_
```

`name:` сравнивается с объявленным base name metric family. Derived counter `_total` и histogram `_bucket`, `_sum`,
`_count` sample names не являются selector inputs и отдельно не фильтруются.

## Defaults

- Пустой include означает, что допустимы все валидные application families.
- Пустой exclude ничего не исключает.
- При двух пустых списках публикуется полный active application snapshot.
- Self-metrics MetricShell публикуются всегда.

## Порядок и приоритеты

Для каждого выбранного immutable snapshot MetricShell:

1. полностью парсит и валидирует candidate до фильтрации;
2. после успеха атомарно устанавливает полный unfiltered application snapshot;
3. при пустом include рассматривает все application families;
4. иначе оставляет families, совпавшие хотя бы с одним include selector;
5. удаляет families, совпавшие хотя бы с одним exclude selector;
6. exclude всегда имеет приоритет над include;
7. отдаёт retained family целиком с metadata и всеми component samples;
8. добавляет полный обязательный набор self-metrics.

Фильтр не может сделать невалидный candidate валидным. Ошибка в family, которая затем была бы исключена, всё равно
атомарно отклоняет весь candidate.

## Атомарность family

- HELP и TYPE следуют за family;
- все samples counter, gauge или histogram получают одно решение;
- нельзя оставить только часть buckets histogram;
- excluded family не оставляет stale samples в response;
- исключение всех application families допустимо и даёт response только с self-metrics.

## Зарезервированный namespace

Префикс `metricshell_` зарезервирован для self-metrics. Application candidate с family на `metricshell_` атомарно
отклоняется с reason `reserved_name`. Скрыть collision фильтром нельзя.

## Повторы

Повторяющиеся одинаковые selectors допускаются и deduplicate в effective configuration. Невалидные selectors приводят
к startup failure. Selector, пока не совпавший ни с одной family, остаётся валидным.

## Наблюдаемость

Effective non-secret configuration показывает нормализованное число include/exclude rules. По default selector values
отсутствуют в logs. Они могут появиться только в `configuration.validated`, когда оператор явно задаёт
`log.selector_values=true`; продолжают действовать size/redaction rules structured logging. Family names никогда не
копируются в metric labels.

```text
metricshell_filter_rules{kind="include"}
metricshell_filter_rules{kind="exclude"}
metricshell_filter_families{outcome="included"}
metricshell_filter_families{outcome="excluded"}
```

## Примеры

### Включение одного namespace с исключением debug families

```text
METRICSHELL_METRICS_INCLUDE=prefix:application_
METRICSHELL_METRICS_EXCLUDE=prefix:application_debug_
```

`application_requests_total` публикуется, `application_debug_cache_entries` — нет.

### Исключение имеет приоритет над точным включением

```text
--metrics-include=name:http_requests
--metrics-exclude=name:http_requests
```

Family исключается, потому что exclude имеет итоговый приоритет.

### Self-metrics нельзя отфильтровать

```text
--metrics-exclude=prefix:metricshell_
```

Selector принимается как no-op для application domain и не влияет на self-metrics MetricShell.

## Conformance

Тесты обязаны доказать defaults, exact/prefix matching, case sensitivity, приоритет exclude, family atomicity,
validation-before-filtering, self-metrics-only response, rejection reserved namespace, отсутствие mixed generations и
замену environment-списка CLI-значением.

## Ссылки

- [Функциональные требования](../03-requirements/functional-requirements.md#fr-033--фильтрация)
- [Критерии приёмки](../03-requirements/acceptance-criteria.md)
- [ADR-004](../06-architecture/adr/ADR-004.md)
- [ADR-010](../06-architecture/adr/ADR-010.md)
- [Спецификация self-metrics](self-metrics.md)
- [Грамматика значений конфигурации](configuration-value-grammar.md)
