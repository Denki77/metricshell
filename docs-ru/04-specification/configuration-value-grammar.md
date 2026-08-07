# Спецификация грамматических значений конфигурации

[English version](../../docs/04-specification/configuration-value-grammar.md)

> Статус: Принятая нормативная спецификация
> Требования: FR-024, FR-080–FR-082
> Критерии приёмки: AC-CONF-001–AC-CONF-004
> Решения: ADR-003, ADR-014

Этот документ задаёт единственную нормативную лексическую грамматику значений конфигурации. Остальные спецификации
определяют defaults, диапазоны и cross-field constraints, но не переопределяют форматы токенов.

## Скалярные значения

```ebnf
duration  = "0" | positive-decimal, duration-unit ;
duration-unit = "ns" | "us" | "ms" | "s" | "m" | "h" ;
byte-size = "0" | positive-decimal, [ byte-unit ] ;
byte-unit = "B" | "KiB" | "MiB" | "GiB" ;
count     = "0" | positive-decimal ;
boolean   = "true" | "false" ;
positive-decimal = nonzero-digit, { digit } ;
digit     = "0" | nonzero-digit ;
nonzero-digit = "1" | "2" | "3" | "4" | "5" | "6" | "7" | "8" | "9" ;
```

Регистры unit и boolean учитываются. Byte-size без suffix означает bytes. IEC suffix используют степени 1024. Duration
без unit недопустим. Свойство может запрещать `0` или сужать доступные units своим нормативным диапазоном, но не может задавать
другое лексическое значение. Знаки, дроби, whitespace, ведущие нули, составные durations и неуказанные suffix
недопустимы. Перед проверкой диапазона каждое значение должно помещаться в unsigned 64-bit промежуточное представление реализации.

Примеры: `500us`, `30s`, `2GiB` и `1024` bytes — корректные токены. `1.5s`, `01s`, `1MB`, `-1` и duration `500` без
unit — некорректные.

## Списковые значения

Списки из environment разделяются literal comma, а окружающий ASCII whitespace каждого элемента удаляется. В version 1
escaping отсутствует. Comma или backslash внутри элемента недопустимы. Пустое environment value задаёт явно пустой
список; пустой элемент в непустом списке недопустим.

## Ссылки

- [Спецификация конфигурации](configuration.md)
- [Runtime defaults и ограничения ресурсов](runtime-defaults-and-resource-limits.md)
- [Спецификация фильтрации метрик](metrics-filtering.md)
