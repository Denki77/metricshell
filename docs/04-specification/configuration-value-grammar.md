# Configuration Value Grammar Specification

[Russian version](../../docs-ru/04-specification/configuration-value-grammar.md)

> Status: Accepted normative specification
> Requirements: FR-024, FR-080–FR-082
> Acceptance criteria: AC-CONF-001–AC-CONF-004
> Decisions: ADR-003, ADR-014

This document is the single normative lexical grammar for configuration values. Other specifications define defaults,
ranges, and cross-field constraints, but must not redefine these token formats.

## Scalar values

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

Units and booleans are case-sensitive. A bare byte-size is bytes. IEC suffixes use powers of 1024. A duration never has
a bare unitless value. A property may reject `0` or narrow the available units through its normative range, but may not assign a
different lexical meaning. Signs, fractions, whitespace, leading zeroes, compound durations, and unspecified suffixes are invalid.
Every parsed value must fit the implementation's unsigned 64-bit intermediate representation before range validation.

Examples: `500us`, `30s`, `2GiB`, and `1024` bytes are valid tokens. `1.5s`, `01s`, `1MB`, `-1`, and bare duration `500`
are invalid.

## List values

Environment-backed lists are split on literal commas and surrounding ASCII whitespace is trimmed from each item. There
is no escaping mechanism in version 1. A comma or backslash inside an item is invalid. An empty environment value
denotes
an explicitly empty list; an empty item in a non-empty list is invalid.

## References

- [Configuration Specification](configuration.md)
- [Runtime Defaults and Resource Limits](runtime-defaults-and-resource-limits.md)
- [Metric Filtering Specification](metrics-filtering.md)
